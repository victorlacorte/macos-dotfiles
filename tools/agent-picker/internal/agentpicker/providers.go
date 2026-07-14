package agentpicker

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The Claude adapter retains behavior adapted from
// craftzdog/tmux-claude-session-manager. See THIRD_PARTY_NOTICES.md.
func (a *App) claudeAgents(ctx context.Context) []Agent {
	if _, err := a.Runner.LookPath("claude"); err != nil {
		return nil
	}
	out, err := a.run(ctx, "claude", "agents", "--json")
	if err != nil {
		return nil
	}
	var records []struct {
		PID       int    `json:"pid"`
		Status    string `json:"status"`
		SessionID string `json:"sessionId"`
		CWD       string `json:"cwd"`
		Kind      string `json:"kind"`
	}
	if err := json.Unmarshal([]byte(out), &records); err != nil {
		return nil
	}

	ttys := a.processTTYs(ctx)
	panes := a.panes(ctx)
	config := getenv("CLAUDE_CONFIG_DIR")
	if config == "" {
		config = filepath.Join(a.Home, ".claude")
	}

	var agents []Agent
	for _, record := range records {
		if record.Kind != "interactive" {
			continue
		}
		pane, ok := panes[ttys[record.PID]]
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

func (a *App) codexAgents(ctx context.Context) []Agent {
	processName := a.option(ctx, "@codex_agent_process_name", "codex")
	if _, err := a.Runner.LookPath(processName); err != nil {
		return nil
	}
	out, err := a.run(ctx, "ps", "-Ao", "pid=,ppid=,tty=,comm=")
	if err != nil {
		return nil
	}
	type process struct {
		pid, ppid int
		tty       string
	}
	processes := make(map[int]process)
	base := filepath.Base(processName)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || filepath.Base(strings.Join(fields[3:], " ")) != base {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 == nil && err2 == nil {
			processes[pid] = process{pid: pid, ppid: ppid, tty: fields[2]}
		}
	}
	panes := a.panes(ctx)
	chosen := make(map[string]process)
	for _, process := range processes {
		if _, ok := panes[process.tty]; !ok {
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
	var agents []Agent
	for _, tty := range ttys {
		process, pane := chosen[tty], panes[tty]
		agents = append(agents, Agent{
			Provider: "codex", Pane: pane.ID, PID: process.pid,
			State: "running", Activity: a.codexRolloutMTime(ctx, process.pid),
			Location: pane.Location, Path: shortenHome(pane.Path, a.Home),
		})
	}
	return agents
}

func (a *App) codexRolloutMTime(ctx context.Context, pid int) time.Time {
	if _, err := a.Runner.LookPath("lsof"); err != nil {
		return time.Time{}
	}
	out, err := a.run(ctx, "lsof", "-a", "-p", strconv.Itoa(pid), "-Fn")
	if err != nil {
		return time.Time{}
	}
	base := getenv("CODEX_HOME")
	if base == "" {
		base = filepath.Join(a.Home, ".codex")
	}
	sessions := filepath.Clean(filepath.Join(base, "sessions")) + string(filepath.Separator)
	var newest time.Time
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "n"+sessions) {
			continue
		}
		path := strings.TrimPrefix(line, "n")
		name := filepath.Base(path)
		if !strings.HasPrefix(name, "rollout-") || filepath.Ext(name) != ".jsonl" {
			continue
		}
		info, err := a.FS.Stat(path)
		if err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
}

type pane struct {
	ID, Location, Path string
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

func (a *App) processTTYs(ctx context.Context) map[int]string {
	out, err := a.run(ctx, "ps", "-Ao", "pid=,tty=")
	if err != nil {
		return nil
	}
	ttys := make(map[int]string)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if pid, err := strconv.Atoi(fields[0]); err == nil {
			ttys[pid] = fields[1]
		}
	}
	return ttys
}
