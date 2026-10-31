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

type claudeSession struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	CWD       string `json:"cwd"`
	Kind      string `json:"kind"`
	modified  time.Time
}

// The Claude adapter retains behavior adapted from
// craftzdog/tmux-claude-session-manager. See THIRD_PARTY_NOTICES.md.
func (a *App) collectClaudeAgents(ctx context.Context, inventory *discoveryInventory) []Agent {
	config := getenv("CLAUDE_CONFIG_DIR")
	if config == "" {
		config = filepath.Join(a.Home, ".claude")
	}
	matches, _ := a.FS.Glob(filepath.Join(config, "sessions", "*.json"))
	sessions := make(map[int]claudeSession)
	for _, match := range matches {
		data, err := a.FS.ReadFile(match)
		if err != nil {
			continue
		}
		var record claudeSession
		if json.Unmarshal(data, &record) != nil || record.Kind != "interactive" || record.SessionID == "" {
			continue
		}
		info, err := a.FS.Stat(match)
		if err != nil || info.IsDir() {
			continue
		}
		record.modified = info.ModTime()
		sessions[record.PID] = record
	}
	inventory.wg.Wait()
	leaders, _ := passiveSessions(inventory, func(p process) bool {
		record, ok := sessions[p.pid]
		// A record older than this process belongs to an earlier use of the PID.
		return ok && record.modified.Unix() >= p.started.Unix() && a.claudeProcess(p)
	})
	var agents []Agent
	for tty, process := range leaders {
		record, pane := sessions[process.pid], inventory.panes[tty]
		agents = append(agents, Agent{
			Provider: "claude", Pane: pane.ID, PID: process.pid, Identity: process.identity(),
			Activity: a.transcriptMTime(config, record.SessionID),
			Location: pane.Location, Path: shortenHome(record.CWD, a.Home),
		})
	}
	return agents
}

func (a *App) transcriptMTime(config, sessionID string) time.Time {
	if sessionID == "" || strings.ContainsAny(sessionID, `/\*?[]`) {
		return time.Time{}
	}
	var newest time.Time
	matches, _ := a.FS.Glob(filepath.Join(config, "projects", "*", sessionID+".jsonl"))
	for _, match := range matches {
		info, err := a.FS.Stat(match)
		if err == nil && !info.IsDir() && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
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
		if !a.interactiveProcess(process, "codex") {
			continue
		}
		agents = append(agents, Agent{
			Provider: "codex", Pane: pane.ID, PID: process.pid, Identity: process.identity(),
			Activity: activity[process.pid],
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
	processName := a.option(ctx, "@cursor_agent_process_name", "")
	automatic := processName == ""
	names := []string{processName}
	if processName == "" {
		names = []string{"cursor-agent", "agent"}
	}
	var installs []*cursorInstall
	for _, name := range names {
		installed, err := a.Runner.LookPath(name)
		if err != nil {
			continue
		}
		if resolved, err := a.FS.EvalSymlinks(installed); err == nil {
			installed = resolved
		}
		root := cursorInstallRoot(installed)
		if automatic && name == "agent" && root == "" {
			continue // Do not mistake an unrelated agent executable for Cursor.
		}
		installs = append(installs, &cursorInstall{
			fs:     a.FS,
			binary: installed,
			root:   root,
			launchers: map[string]bool{
				"agent": true, "cursor-agent": true, filepath.Base(name): true,
			},
			resolved: make(map[string]bool),
		})
	}
	inventory.wg.Wait()
	if len(installs) == 0 {
		return nil
	}
	leaders, owners := passiveSessions(inventory, func(process process) bool {
		for _, cursor := range installs {
			if cursor.matches(process.command) {
				return true
			}
		}
		return false
	})
	ttys := make([]string, 0, len(leaders))
	for tty := range leaders {
		ttys = append(ttys, tty)
	}
	sort.Strings(ttys)
	chatDir := filepath.Clean(cursorChatsDir(a.Home)) + string(filepath.Separator)
	activity := a.openFileMTimes(ctx, owners, func(path string) bool {
		return strings.HasPrefix(path, chatDir) &&
			(filepath.Base(path) == "store.db" || filepath.Base(path) == "store.db-wal")
	})

	agents := make([]Agent, 0, len(ttys))
	for _, tty := range ttys {
		process, pane := leaders[tty], inventory.panes[tty]
		if !a.interactiveProcess(process, "cursor") {
			continue
		}
		agents = append(agents, Agent{
			Provider: "cursor", Pane: pane.ID, PID: process.pid, Identity: process.identity(),
			Activity: activity[process.pid],
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
		return c.launchers[filepath.Base(command)] || filepath.Base(command) == "node"
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
