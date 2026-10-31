package agentpicker

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func foregroundProcess(pid, ppid int, command string) process {
	return process{pid: pid, ppid: ppid, pgid: 100, tpgid: 100, tty: "ttys001", state: "S+",
		command: command, started: time.Date(2006, 1, 2, 12, 0, 0, 0, time.UTC)}
}

func processLine(p process) string {
	return fmt.Sprintf("%d %d %d %d %s %s %s %s\n", p.pid, p.ppid, p.pgid, p.tpgid, p.tty, p.state,
		p.started.UTC().Format("Mon Jan 2 15:04:05 2006"), p.command)
}

func TestForegroundSessionsAndWrapperOwnership(t *testing.T) {
	for _, tc := range []struct {
		name      string
		processes []process
		leader    int
		owners    []int
	}{
		{"wrapper", []process{foregroundProcess(100, 1, "codex"), foregroundProcess(110, 100, "sh"), foregroundProcess(120, 110, "codex")}, 100, []int{100, 110, 120}},
		{"ambiguous siblings", []process{foregroundProcess(100, 1, "codex"), foregroundProcess(200, 1, "codex")}, 0, nil},
		{"foreground beats old background", []process{func() process { p := foregroundProcess(50, 1, "codex"); p.pgid = 50; return p }(), foregroundProcess(100, 1, "codex")}, 100, []int{100}},
		{"stopped", []process{func() process { p := foregroundProcess(100, 1, "codex"); p.state = "T+"; return p }()}, 0, nil},
		{"missing start", []process{func() process { p := foregroundProcess(100, 1, "codex"); p.started = time.Time{}; return p }()}, 0, nil},
		{"no pane tty", []process{func() process { p := foregroundProcess(100, 1, "codex"); p.tty = "??"; return p }()}, 0, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inv := &discoveryInventory{processes: make(map[int]process), panes: map[string]pane{"ttys001": {ID: "%1"}}}
			for _, p := range tc.processes {
				inv.processes[p.pid] = p
			}
			leaders, owners := passiveSessions(inv, func(p process) bool { return p.command == "codex" })
			if leaders["ttys001"].pid != tc.leader || len(owners) != len(tc.owners) {
				t.Fatalf("leaders=%#v owners=%#v", leaders, owners)
			}
			for _, pid := range tc.owners {
				if owners[pid] != tc.leader {
					t.Fatalf("PID %d lost session ownership: %#v", pid, owners)
				}
			}
		})
	}
}

func TestProcessSnapshotParsing(t *testing.T) {
	p := foregroundProcess(100, 1, "/Applications/Path With  Spaces/codex")
	got := parseProcesses(processLine(p) + "bad pid data\n" + strings.Replace(processLine(p), "Jan", "invalid", 1))
	if len(got) != 1 || !reflect.DeepEqual(got[100], p) {
		t.Fatalf("parsed %#v", got)
	}
}

func TestInteractiveInvocationModes(t *testing.T) {
	for _, tc := range []struct {
		provider string
		argv     []string
		want     bool
	}{
		{"codex", []string{"codex"}, true},
		{"codex", []string{"codex", "-p", "review", "resume", "--last"}, true},
		{"codex", []string{"codex", "-c", "x=exec", "--cd", "/a path", "say exec --print literally"}, true},
		{"codex", []string{"codex", "--", "exec"}, true},
		{"codex", []string{"codex", "exec", "--ephemeral"}, false},
		{"codex", []string{"node", "/npm/codex.js", "exec", "hello"}, false},
		{"codex", []string{"codex", "prompt.js"}, true},
		{"codex", []string{"codex", "--model", "anything", "app-server"}, false},
		{"codex", []string{"codex", "review"}, false},
		{"codex", []string{"codex", "cloud", "list"}, false},
		{"codex", []string{"codex", "resume", "--help"}, false},
		{"codex", []string{"codex", "--profile"}, false},
		{"claude", []string{"claude", "say -p literally"}, true},
		{"claude", []string{"claude", "--resume"}, true},
		{"claude", []string{"claude", "-r", "--continue"}, true},
		{"claude", []string{"claude", "--resume", "session-id", "--print"}, false},
		{"claude", []string{"claude", "--system-prompt", "--print", "hello"}, true},
		{"claude", []string{"node", "/a path/@anthropic-ai/claude-code/cli.js", "-p", "hello"}, false},
		{"claude", []string{"claude", "-phello"}, false},
		{"claude", []string{"claude", "--bg", "hello"}, false},
		{"claude", []string{"claude", "agents", "--json"}, false},
		{"cursor", []string{"agent", "--model=print", "hello"}, true},
		{"cursor", []string{"agent", "resume"}, true},
		{"cursor", []string{"node", "/install/index.js", "--print", "hello"}, false},
		{"cursor", []string{"agent", "-fp", "hello"}, false},
		{"cursor", []string{"agent", "--version"}, false},
		{"cursor", []string{"agent", "ls"}, false},
		{"cursor", []string{"node", "/install/index.js", "--type=worker"}, false},
		{"cursor", nil, false},
	} {
		t.Run(tc.provider+"/"+strings.Join(tc.argv, " "), func(t *testing.T) {
			if got := interactiveInvocation(tc.provider, tc.argv); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestClaudeRejectsStaleAndUnrelatedRecords(t *testing.T) {
	for _, tc := range []struct {
		name, command string
		argv          []string
		stale         bool
		want          int
	}{
		{"native", "claude", []string{"claude"}, false, 1},
		{"npm", "node", []string{"node", "/npm/@anthropic-ai/claude-code/cli.js"}, false, 1},
		{"shell reused PID", "/bin/zsh", []string{"zsh"}, false, 0},
		{"unrelated node", "node", []string{"node", "/project/cli.js"}, false, 0},
		{"old record reused by Claude", "claude", []string{"claude"}, true, 0},
		{"print despite record kind", "claude", []string{"claude", "-p", "hello"}, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("CLAUDE_CONFIG_DIR", home)
			if err := os.MkdirAll(filepath.Join(home, "sessions"), 0o755); err != nil {
				t.Fatal(err)
			}
			record := filepath.Join(home, "sessions", "100.json")
			if err := os.WriteFile(record, []byte(`{"pid":100,"kind":"interactive","sessionId":"test","cwd":"/project"}`), 0o644); err != nil {
				t.Fatal(err)
			}
			p := foregroundProcess(100, 1, tc.command)
			if tc.stale {
				old := p.started.Add(-time.Minute)
				if err := os.Chtimes(record, old, old); err != nil {
					t.Fatal(err)
				}
			}
			app := newTestApp(&fakeRunner{argv: map[int][]string{100: tc.argv}}, &fakeClock{}, home)
			inv := &discoveryInventory{processes: map[int]process{100: p}, panes: map[string]pane{"ttys001": {ID: "%1"}}}
			if got := app.collectClaudeAgents(context.Background(), inv); len(got) != tc.want {
				t.Fatalf("agents=%#v", got)
			}
		})
	}
}

func TestCursorLauncherDiscovery(t *testing.T) {
	for _, tc := range []struct {
		name, available, configured string
		want                        int
	}{
		{"agent only", "agent", "", 1}, {"legacy", "cursor-agent", "", 1},
		{"explicit agent", "agent", "agent", 1}, {"explicit missing override", "agent", "missing", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			launcher, _ := installCursor(t, home, tc.available)
			r := &fakeRunner{paths: map[string]string{tc.available: launcher}, argv: map[int][]string{100: {launcher}}}
			r.handle = func(c Command) (string, error) {
				if c.Name == "tmux" {
					return tc.configured, nil
				}
				return "", nil
			}
			app := newTestApp(r, &fakeClock{}, home)
			inv := &discoveryInventory{processes: map[int]process{100: foregroundProcess(100, 1, launcher)}, panes: map[string]pane{"ttys001": {ID: "%1"}}}
			if got := app.collectCursorAgents(context.Background(), inv); len(got) != tc.want {
				t.Fatalf("agents=%#v", got)
			}
		})
	}
	t.Run("unrelated agent on PATH", func(t *testing.T) {
		r := &fakeRunner{paths: map[string]string{"agent": "/unrelated/bin/agent"}}
		app := newTestApp(r, &fakeClock{}, t.TempDir())
		inv := &discoveryInventory{processes: map[int]process{100: foregroundProcess(100, 1, "/unrelated/bin/agent")}, panes: map[string]pane{"ttys001": {ID: "%1"}}}
		if got := app.collectCursorAgents(context.Background(), inv); len(got) != 0 {
			t.Fatalf("agents=%#v", got)
		}
	})
}

func TestCursorDiscoversBothInstallations(t *testing.T) {
	for _, configured := range []string{"", "agent", "cursor-agent"} {
		t.Run("configured="+configured, func(t *testing.T) {
			home := t.TempDir()
			legacy, _ := installCursor(t, filepath.Join(home, "legacy"), "cursor-agent")
			modern, _ := installCursor(t, filepath.Join(home, "modern"), "agent")
			r := &fakeRunner{
				paths: map[string]string{"cursor-agent": legacy, "agent": modern},
				argv:  map[int][]string{100: {legacy}, 200: {modern}},
				handle: func(c Command) (string, error) {
					if c.Name == "tmux" {
						return configured, nil
					}
					return "", nil
				},
			}
			second := foregroundProcess(200, 1, modern)
			second.tty = "ttys002"
			inv := &discoveryInventory{
				processes: map[int]process{100: foregroundProcess(100, 1, legacy), 200: second},
				panes:     map[string]pane{"ttys001": {ID: "%1"}, "ttys002": {ID: "%2"}},
			}
			app := newTestApp(r, &fakeClock{}, home)
			got := app.collectCursorAgents(context.Background(), inv)
			want := []int{100, 200}
			if configured == "agent" {
				want = []int{200}
			} else if configured == "cursor-agent" {
				want = []int{100}
			}
			var pids []int
			for _, agent := range got {
				pids = append(pids, agent.PID)
			}
			if !reflect.DeepEqual(pids, want) {
				t.Fatalf("PIDs=%v want=%v", pids, want)
			}
		})
	}
}

func TestCollectorsOmitHeadlessAndUnreadableProcesses(t *testing.T) {
	for _, provider := range []string{"codex", "cursor"} {
		for _, unreadable := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/unreadable=%v", provider, unreadable), func(t *testing.T) {
				home := t.TempDir()
				launcher, _ := installCursor(t, home, "agent")
				command := "codex"
				args := []string{"codex", "exec", "hello"}
				if provider == "cursor" {
					command = launcher
					args = []string{launcher, "-p", "hello"}
				}
				r := &fakeRunner{available: map[string]bool{"codex": true}, paths: map[string]string{"agent": launcher}, argv: map[int][]string{100: args}}
				if unreadable {
					r.argErrors = map[int]error{100: errors.New("access denied")}
				}
				app := newTestApp(r, &fakeClock{}, home)
				inv := &discoveryInventory{processes: map[int]process{100: foregroundProcess(100, 1, command)}, panes: map[string]pane{"ttys001": {ID: "%1"}}}
				collect := app.collectCodexAgents
				if provider == "cursor" {
					collect = app.collectCursorAgents
				}
				if got := collect(context.Background(), inv); len(got) != 0 {
					t.Fatalf("agents=%#v", got)
				}
			})
		}
	}
}

func TestTranscriptActivityUsesNewestMatch(t *testing.T) {
	home := t.TempDir()
	now := time.Now().Truncate(time.Second)
	for i, project := range []string{"a", "b"} {
		path := filepath.Join(home, "projects", project, "session.jsonl")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		stamp := now.Add(time.Duration(i-1) * time.Hour)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	app := newTestApp(&fakeRunner{}, &fakeClock{}, home)
	if got := app.transcriptMTime(home, "session"); !got.Equal(now) {
		t.Fatalf("activity=%v want=%v", got, now)
	}
	for _, session := range []string{"missing", "*", "../session", ""} {
		if got := app.transcriptMTime(home, session); !got.IsZero() {
			t.Fatalf("invalid or absent session %q got activity %v", session, got)
		}
	}
}

func TestCursorActivityAcceptsOnlyDatabaseWrites(t *testing.T) {
	for _, name := range []string{"store.db", "store.db-wal", "store.db-shm", "store.db-backup", "logs.sqlite"} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("CURSOR_CONFIG_DIR", home)
			launcher, _ := installCursor(t, home, "agent")
			path := filepath.Join(home, "chats", "chat", name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			r := &fakeRunner{available: map[string]bool{"lsof": true}, paths: map[string]string{"agent": launcher}}
			r.handle = func(c Command) (string, error) {
				if c.Name == "lsof" {
					return "p100\nn" + path + "\n", nil
				}
				return "", nil
			}
			app := newTestApp(r, &fakeClock{}, home)
			inv := &discoveryInventory{processes: map[int]process{100: foregroundProcess(100, 1, launcher)}, panes: map[string]pane{"ttys001": {ID: "%1"}}}
			got := app.collectCursorAgents(context.Background(), inv)
			if len(got) != 1 {
				t.Fatalf("agents=%#v", got)
			}
			wantKnown := name == "store.db" || name == "store.db-wal"
			if got[0].Activity.IsZero() == wantKnown {
				t.Fatalf("activity=%v wantKnown=%v", got[0].Activity, wantKnown)
			}
		})
	}
}

func TestListIdentitiesPreservesDefaultColumns(t *testing.T) {
	r := &fakeRunner{available: map[string]bool{"codex": true}}
	r.handle = func(c Command) (string, error) {
		if c.Name == "ps" {
			return processLine(foregroundProcess(100, 1, "codex")), nil
		}
		if c.Name == "tmux" && c.Args[0] == "list-panes" {
			return "/dev/ttys001\t%1\twork\twork:1.0\t/project\n", nil
		}
		return "", nil
	}
	app := newTestApp(r, &fakeClock{}, t.TempDir())
	app.Stderr = &strings.Builder{}
	var previous []string
	for i, args := range [][]string{{"list", "-provider", "codex"}, {"list", "-provider", "codex", "-identities"}} {
		output := &strings.Builder{}
		app.Stdout = output
		if code := app.Main(context.Background(), args); code != 0 {
			t.Fatalf("exit=%d", code)
		}
		fields := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\t")
		if len(fields) != 7+i {
			t.Fatalf("fields=%#v", fields)
		}
		if i == 0 {
			previous = fields
		} else if !reflect.DeepEqual(previous, fields[:7]) || len(fields[7]) != 64 {
			t.Fatalf("identity output changed existing columns: %#v", fields)
		}
	}
}

func TestTerminateRevalidatesBeforeSignalling(t *testing.T) {
	original := foregroundProcess(100, 1, "codex")
	for _, tc := range []struct {
		name                                             string
		change                                           func(*process)
		identity                                         string
		argv                                             []string
		missing, paneMoved, argsUnavailable, signalFails bool
	}{
		{name: "unchanged signals once"},
		{name: "reused PID", change: func(p *process) { p.started = p.started.Add(time.Minute) }},
		{name: "changed command", change: func(p *process) { p.command = "sh" }},
		{name: "changed tty", change: func(p *process) { p.tty = "ttys002" }},
		{name: "backgrounded", change: func(p *process) { p.tpgid = 200 }},
		{name: "stopped", change: func(p *process) { p.state = "T+" }},
		{name: "changed identity", identity: strings.Repeat("a", 64)},
		{name: "exited", missing: true},
		{name: "moved pane", paneMoved: true},
		{name: "headless after discovery", argv: []string{"codex", "exec", "hello"}},
		{name: "unreadable argv", argsUnavailable: true},
		{name: "signal failure", signalFails: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRunner{available: map[string]bool{"codex": true}, argv: map[int][]string{100: {"codex"}}, argErrors: make(map[int]error)}
			fresh := original
			if tc.change != nil {
				tc.change(&fresh)
			}
			signals := 0
			r.handle = func(c Command) (string, error) {
				if c.Name == "ps" {
					if c.Env["LC_ALL"] != "C" || c.Env["TZ"] != "UTC" {
						t.Error("ps dates are not normalized")
					}
					if c.Args[0] == "-p" {
						if tc.missing {
							return "", nil
						}
						if tc.argv != nil {
							r.argv[100] = tc.argv
						}
						if tc.argsUnavailable {
							r.argErrors[100] = errors.New("unavailable")
						}
						return processLine(fresh), nil
					}
					return processLine(original), nil
				}
				if c.Name == "tmux" && c.Args[0] == "list-panes" {
					pane := "%1"
					if tc.paneMoved {
						pane = "%2"
					}
					return "/dev/ttys001\t" + pane + "\twork\twork:1.0\t/project\n", nil
				}
				if c.Name == "kill" {
					signals++
					if !reflect.DeepEqual(c.Args, []string{"-TERM", "100"}) {
						t.Errorf("unexpected signal: %#v", c)
					}
					if tc.signalFails {
						return "", errors.New("signal denied")
					}
				}
				return "", nil
			}
			app := newTestApp(r, &fakeClock{}, t.TempDir())
			identity := original.identity()
			if tc.identity != "" {
				identity = tc.identity
			}
			err := app.terminate(context.Background(), "codex", "%1", 100, identity)
			wantSignals := 0
			if tc.name == "unchanged signals once" || tc.signalFails {
				wantSignals = 1
			}
			if signals != wantSignals {
				t.Fatalf("signals=%d want=%d error=%v", signals, wantSignals, err)
			}
			if (err == nil) != (tc.name == "unchanged signals once") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestTerminateCLIRejectsUnsafeTargets(t *testing.T) {
	for _, args := range [][]string{
		{}, {"-provider", "all"}, {"-provider", "codex", "-pane", "%1", "-pid", "-1", "-identity", strings.Repeat("a", 64)},
		{"-provider", "codex", "-pane", "%1", "-pid", "1", "-identity", strings.Repeat("a", 64)},
		{"-provider", "codex", "-pane", "%1", "-pid", "100", "-identity", "invalid"},
	} {
		r := &fakeRunner{}
		app := newTestApp(r, &fakeClock{}, t.TempDir())
		app.Stderr = &strings.Builder{}
		if code := app.Main(context.Background(), append([]string{"terminate"}, args...)); code != 2 {
			t.Fatalf("code=%d args=%#v", code, args)
		}
		if len(r.Commands()) != 0 {
			t.Fatalf("invalid target executed commands: %#v", r.Commands())
		}
	}
}

func TestDecodeProcessArguments(t *testing.T) {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, 3)
	data = append(data, []byte("/bin/agent\x00\x00\x00agent\x00say --print literally\x00\x00SECRET=not-an-argument\x00")...)
	got, err := decodeProcessArgs(data)
	if err != nil || !reflect.DeepEqual(got, []string{"agent", "say --print literally", ""}) {
		t.Fatalf("args=%q err=%v", got, err)
	}
	for _, bad := range [][]byte{nil, {1, 0, 0, 0}, {255, 255, 255, 255}, data[:len(data)-len("SECRET=not-an-argument\x00")-1]} {
		if _, err := decodeProcessArgs(bad); err == nil {
			t.Fatal("accepted truncated or invalid arguments")
		}
	}
}

func TestOSProcessArgumentsPreserveBoundaries(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestProcessArgumentsChild$", "--", "say --print literally", "quotes ' and spaces", "")
	cmd.Env = append(os.Environ(), "AGENT_PICKER_ARGUMENT_CHILD=1", "ARGUMENT_TEST_SECRET=not-an-argument")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	got, err := (OSRunner{}).ProcessArgs(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, cmd.Args) {
		t.Fatalf("argument boundaries changed: got %q want %q", got, cmd.Args)
	}
}

func TestProcessArgumentsChild(t *testing.T) {
	if os.Getenv("AGENT_PICKER_ARGUMENT_CHILD") == "1" {
		time.Sleep(time.Hour)
	}
}
