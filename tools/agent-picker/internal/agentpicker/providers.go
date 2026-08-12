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

// passiveSessions elects the topmost matching process on each pane's tty and
// maps every matching process in that session to its elected leader.
func passiveSessions(inventory *discoveryInventory, matches func(process) bool) (leaders map[string]process, owners map[int]int) {
	leaders = make(map[string]process)
	owners = make(map[int]int)
	matched := make(map[int]process)
	for pid, process := range inventory.processes {
		if matches(process) {
			matched[pid] = process
		}
	}

	roots := make(map[int]process)
	for pid, process := range matched {
		if _, ok := inventory.panes[process.tty]; !ok {
			continue
		}
		root := passiveSessionLeader(process, matched)
		roots[pid] = root
		current, ok := leaders[root.tty]
		if !ok || root.pid < current.pid {
			leaders[root.tty] = root
		}
	}
	for pid, root := range roots {
		if leader, ok := leaders[root.tty]; ok && leader.pid == root.pid {
			owners[pid] = leader.pid
		}
	}
	return leaders, owners
}

func passiveSessionLeader(start process, matched map[int]process) process {
	leader := start
	for depth := 0; depth < len(matched); depth++ {
		parent, ok := matched[leader.ppid]
		if !ok || parent.tty != leader.tty {
			break
		}
		leader = parent
	}
	return leader
}

func (a *App) collectCodexAgents(ctx context.Context, inventory *discoveryInventory) []Agent {
	processName := a.option(ctx, "@codex_agent_process_name", "codex")
	_, lookupErr := a.Runner.LookPath(processName)
	inventory.wg.Wait()
	if lookupErr != nil {
		return nil
	}

	base := filepath.Base(processName)
	leaders, owners := passiveSessions(inventory, func(process process) bool {
		return filepath.Base(process.command) == base
	})
	ttys := make([]string, 0, len(leaders))
	for tty := range leaders {
		ttys = append(ttys, tty)
	}
	sort.Strings(ttys)
	activity := a.openFileMTimes(ctx, owners, func(path string) bool {
		base := getenv("CODEX_HOME")
		if base == "" {
			base = filepath.Join(a.Home, ".codex")
		}
		sessions := filepath.Clean(filepath.Join(base, "sessions")) + string(filepath.Separator)
		name := filepath.Base(path)
		return strings.HasPrefix(path, sessions) &&
			strings.HasPrefix(name, "rollout-") && filepath.Ext(name) == ".jsonl"
	})

	agents := make([]Agent, 0, len(ttys))
	for _, tty := range ttys {
		process, pane := leaders[tty], inventory.panes[tty]
		agents = append(agents, Agent{
			Provider: "codex", Pane: pane.ID, PID: process.pid,
			State: "running", Activity: activity[process.pid],
			Location: pane.Location, Path: shortenHome(pane.Path, a.Home),
		})
	}
	return agents
}

func (a *App) openFileMTimes(ctx context.Context, owners map[int]int, accept func(string) bool) map[int]time.Time {
	activity := make(map[int]time.Time)
	if len(owners) == 0 {
		return activity
	}
	if _, err := a.Runner.LookPath("lsof"); err != nil {
		return activity
	}
	pids := make([]int, 0, len(owners))
	for pid := range owners {
		pids = append(pids, pid)
	}
	sortedPIDs := append([]int(nil), pids...)
	sort.Ints(sortedPIDs)
	values := make([]string, len(sortedPIDs))
	for i, pid := range sortedPIDs {
		values[i] = strconv.Itoa(pid)
	}
	out, _ := a.run(ctx, "lsof", "-a", "-p", strings.Join(values, ","), "-Fn")

	currentPID := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "p") {
			currentPID, _ = strconv.Atoi(strings.TrimPrefix(line, "p"))
			continue
		}
		if currentPID == 0 || !strings.HasPrefix(line, "n") {
			continue
		}
		path := strings.TrimPrefix(line, "n")
		if !accept(path) {
			continue
		}
		info, err := a.FS.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		leaderPID, ok := owners[currentPID]
		if ok && info.ModTime().After(activity[leaderPID]) {
			activity[leaderPID] = info.ModTime()
		}
	}
	return activity
}

// Cursor CLI processes are identified by their installation directory. Their
// launcher replaces argv[0] with the invoked path, so the reported command is
// usually a link such as ~/.local/bin/agent, and the bundled executable is a
// plain "node".
func (a *App) collectCursorAgents(ctx context.Context, inventory *discoveryInventory) []Agent {
	processName := a.option(ctx, "@cursor_agent_process_name", "cursor-agent")
	installed, lookupErr := a.Runner.LookPath(processName)
	inventory.wg.Wait()
	if lookupErr != nil {
		return nil
	}
	if resolved, err := a.FS.EvalSymlinks(installed); err == nil {
		installed = resolved
	}

	cursor := &cursorInstall{
		fs:     a.FS,
		binary: installed,
		root:   cursorInstallRoot(installed),
		launchers: map[string]bool{
			"agent": true, "cursor-agent": true, filepath.Base(processName): true,
		},
		resolved: make(map[string]bool),
	}
	leaders, owners := passiveSessions(inventory, func(process process) bool {
		return cursor.matches(process.command)
	})
	ttys := make([]string, 0, len(leaders))
	for tty := range leaders {
		ttys = append(ttys, tty)
	}
	sort.Strings(ttys)
	chatDir := filepath.Clean(cursorChatsDir(a.Home)) + string(filepath.Separator)
	activity := a.openFileMTimes(ctx, owners, func(path string) bool {
		return strings.HasPrefix(path, chatDir) &&
			strings.HasPrefix(filepath.Base(path), "store.db")
	})

	agents := make([]Agent, 0, len(ttys))
	for _, tty := range ttys {
		process, pane := leaders[tty], inventory.panes[tty]
		agents = append(agents, Agent{
			Provider: "cursor", Pane: pane.ID, PID: process.pid,
			State: "running", Activity: activity[process.pid],
			Location: pane.Location, Path: shortenHome(pane.Path, a.Home),
		})
	}
	return agents
}

// cursorInstallRoot returns the directory holding every installed version, so
// that sessions started before an upgrade still match.
func cursorInstallRoot(binary string) string {
	for dir := filepath.Dir(binary); dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		if filepath.Base(dir) == "cursor-agent" {
			return dir + string(filepath.Separator)
		}
	}
	return ""
}

type cursorInstall struct {
	fs        FileSystem
	binary    string
	root      string
	launchers map[string]bool
	resolved  map[string]bool
}

func (c *cursorInstall) matches(command string) bool {
	if c.root != "" && strings.HasPrefix(command, c.root) {
		return true
	}
	if !c.launchers[filepath.Base(command)] {
		return false
	}
	if inside, ok := c.resolved[command]; ok {
		return inside
	}
	real, err := c.fs.EvalSymlinks(command)
	inside := err == nil && real == c.binary
	c.resolved[command] = inside
	return inside
}

// The Cursor CLI keeps chat databases under its configuration directory.
func cursorChatsDir(home string) string {
	if dir := strings.TrimSpace(getenv("CURSOR_CONFIG_DIR")); dir != "" {
		return filepath.Join(dir, "chats")
	}
	if xdg := strings.TrimSpace(getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "cursor", "chats")
	}
	return filepath.Join(home, ".cursor", "chats")
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
