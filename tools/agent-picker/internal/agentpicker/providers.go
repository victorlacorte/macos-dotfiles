package agentpicker

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type process struct {
	pid, ppid int
	tty       string
	command   string
}

type pane struct {
	ID, Location, Path string
}

type discoveryInventory struct {
	wg        sync.WaitGroup
	processes map[int]process
	panes     map[string]pane
}

func (a *App) startInventory(ctx context.Context) *discoveryInventory {
	inventory := &discoveryInventory{}
	inventory.wg.Add(2)
	go func() {
		defer inventory.wg.Done()
		inventory.processes = a.processInventory(ctx)
	}()
	go func() {
		defer inventory.wg.Done()
		inventory.panes = a.panes(ctx)
	}()
	return inventory
}

// The Claude adapter retains behavior adapted from
// craftzdog/tmux-claude-session-manager. See THIRD_PARTY_NOTICES.md.
func (a *App) collectClaudeAgents(ctx context.Context, inventory *discoveryInventory) []Agent {
	var records []struct {
		PID       int    `json:"pid"`
		Status    string `json:"status"`
		SessionID string `json:"sessionId"`
		CWD       string `json:"cwd"`
		Kind      string `json:"kind"`
	}
	available := false
	if _, err := a.Runner.LookPath("claude"); err == nil {
		out, runErr := a.run(ctx, "claude", "agents", "--json")
		if runErr == nil && json.Unmarshal([]byte(out), &records) == nil {
			available = true
		}
	}

	inventory.wg.Wait()
	if !available {
		return nil
	}
	config := getenv("CLAUDE_CONFIG_DIR")
	if config == "" {
		config = filepath.Join(a.Home, ".claude")
	}

	var agents []Agent
	for _, record := range records {
		if record.Kind != "interactive" {
			continue
		}
		process, ok := inventory.processes[record.PID]
		if !ok {
			continue
		}
		pane, ok := inventory.panes[process.tty]
		if !ok {
			continue
		}
		state := "?"
		switch record.Status {
		case "waiting", "idle":
			state = record.Status
		case "busy":
			state = "working"
		}
		agents = append(agents, Agent{
			Provider: "claude", Pane: pane.ID, PID: record.PID,
			State: state, Activity: a.transcriptMTime(config, record.SessionID),
			Location: pane.Location, Path: shortenHome(record.CWD, a.Home),
		})
	}
	return agents
}

func (a *App) transcriptMTime(config, sessionID string) time.Time {
	matches, _ := a.FS.Glob(filepath.Join(config, "projects", "*", sessionID+".jsonl"))
	for _, match := range matches {
		info, err := a.FS.Stat(match)
		if err == nil && !info.IsDir() {
			return info.ModTime()
		}
	}
	return time.Time{}
}

func (a *App) collectCodexAgents(ctx context.Context, inventory *discoveryInventory) []Agent {
	processName := a.option(ctx, "@codex_agent_process_name", "codex")
	_, lookupErr := a.Runner.LookPath(processName)
	inventory.wg.Wait()
	if lookupErr != nil {
		return nil
	}

	base := filepath.Base(processName)
	processes := make(map[int]process)
	for pid, process := range inventory.processes {
		if filepath.Base(process.command) == base {
			processes[pid] = process
		}
	}
	chosen := make(map[string]process)
	for _, process := range processes {
		if _, ok := inventory.panes[process.tty]; !ok {
			continue
		}
		if parent, ok := processes[process.ppid]; ok && parent.tty == process.tty {
			continue
		}
		current, ok := chosen[process.tty]
		if !ok || process.pid < current.pid {
			chosen[process.tty] = process
		}
	}
	ttys := make([]string, 0, len(chosen))
	for tty := range chosen {
		ttys = append(ttys, tty)
	}
	sort.Strings(ttys)
	pids := make([]int, 0, len(ttys))
	for _, tty := range ttys {
		pids = append(pids, chosen[tty].pid)
	}
	activity := a.codexRolloutMTimes(ctx, pids)

	agents := make([]Agent, 0, len(ttys))
	for _, tty := range ttys {
		process, pane := chosen[tty], inventory.panes[tty]
		agents = append(agents, Agent{
			Provider: "codex", Pane: pane.ID, PID: process.pid,
			State: "running", Activity: activity[process.pid],
			Location: pane.Location, Path: shortenHome(pane.Path, a.Home),
		})
	}
	return agents
}

func (a *App) codexRolloutMTimes(ctx context.Context, pids []int) map[int]time.Time {
	activity := make(map[int]time.Time)
	if len(pids) == 0 {
		return activity
	}
	if _, err := a.Runner.LookPath("lsof"); err != nil {
		return activity
	}
	sortedPIDs := append([]int(nil), pids...)
	sort.Ints(sortedPIDs)
	values := make([]string, len(sortedPIDs))
	for i, pid := range sortedPIDs {
		values[i] = strconv.Itoa(pid)
	}
	out, _ := a.run(ctx, "lsof", "-a", "-p", strings.Join(values, ","), "-Fn")

	base := getenv("CODEX_HOME")
	if base == "" {
		base = filepath.Join(a.Home, ".codex")
	}
	sessions := filepath.Clean(filepath.Join(base, "sessions")) + string(filepath.Separator)
	currentPID := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "p") {
			currentPID, _ = strconv.Atoi(strings.TrimPrefix(line, "p"))
			continue
		}
		if currentPID == 0 || !strings.HasPrefix(line, "n"+sessions) {
			continue
		}
		path := strings.TrimPrefix(line, "n")
		name := filepath.Base(path)
		if !strings.HasPrefix(name, "rollout-") || filepath.Ext(name) != ".jsonl" {
			continue
		}
		info, err := a.FS.Stat(path)
		if err == nil && info.ModTime().After(activity[currentPID]) {
			activity[currentPID] = info.ModTime()
		}
	}
	return activity
}

func (a *App) panes(ctx context.Context) map[string]pane {
	out, err := a.run(ctx, "tmux", "list-panes", "-a", "-F", "#{pane_tty}\t#{pane_id}\t#{session_name}\t#{session_name}:#{window_index}.#{pane_index}\t#{pane_current_path}")
	if err != nil {
		return nil
	}
	panes := make(map[string]pane)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.SplitN(line, "\t", 5)
		if len(fields) < 5 {
			continue
		}
		tty := strings.TrimPrefix(fields[0], "/dev/")
		panes[tty] = pane{ID: fields[1], Location: fields[3], Path: fields[4]}
	}
	return panes
}

func (a *App) processInventory(ctx context.Context) map[int]process {
	out, err := a.run(ctx, "ps", "-Ao", "pid=,ppid=,tty=,comm=")
	if err != nil {
		return nil
	}
	processes := make(map[int]process)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 == nil && err2 == nil {
			processes[pid] = process{pid: pid, ppid: ppid, tty: fields[2], command: strings.Join(fields[3:], " ")}
		}
	}
	return processes
}
