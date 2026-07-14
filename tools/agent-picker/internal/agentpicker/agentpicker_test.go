package agentpicker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeRunner struct {
	mu        sync.Mutex
	available map[string]bool
	handle    func(Command) (string, error)
	handleCtx func(context.Context, Command) (string, error)
	commands  []Command
	lookups   []string
}

func (r *fakeRunner) LookPath(name string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lookups = append(r.lookups, name)
	if r.available[name] {
		return "/fake/" + filepath.Base(name), nil
	}
	return "", errors.New("not found")
}

func (r *fakeRunner) Run(ctx context.Context, command Command) (string, error) {
	r.mu.Lock()
	r.commands = append(r.commands, command)
	handle, handleCtx := r.handle, r.handleCtx
	r.mu.Unlock()
	if handleCtx != nil {
		return handleCtx(ctx, command)
	}
	if handle != nil {
		return handle(command)
	}
	return "", nil
}

func (r *fakeRunner) Commands() []Command {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Command(nil), r.commands...)
}

func (r *fakeRunner) Lookups() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.lookups...)
}

type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time { return c.now }

func newTestApp(runner *fakeRunner, clock *fakeClock, home string) *App {
	return &App{Runner: runner, FS: OSFileSystem{}, Clock: clock, Home: home, Executable: "/bin/agent picker"}
}

func fakeAgentResponse(command Command) (string, bool) {
	joined := strings.Join(command.Args, " ")
	switch {
	case command.Name == "claude" && joined == "agents --json":
		return `[{"pid":100,"status":"waiting","sessionId":"claude-id","cwd":"/tmp/claude","kind":"interactive"}]`, true
	case command.Name == "ps" && joined == "-Ao pid=,ppid=,tty=,comm=":
		return "100 1 ttys001 claude\n200 1 ttys002 codex\n", true
	case command.Name == "tmux" && strings.HasPrefix(joined, "list-panes -a"):
		return "/dev/ttys001\t%8\twork\twork:1.1\t/tmp/claude\n" +
			"/dev/ttys002\t%9\tcodex-project\tcodex-project:1.1\t/tmp/codex\n", true
	default:
		return "", false
	}
}

func TestProvidersAndAggregation(t *testing.T) {
	home := t.TempDir()
	claudeHome := filepath.Join(home, "Claude Home")
	codexHome := filepath.Join(home, "Codex Home")
	t.Setenv("CLAUDE_CONFIG_DIR", claudeHome)
	t.Setenv("CODEX_HOME", codexHome)
	waitFile := filepath.Join(claudeHome, "projects", "one", "wait-id.jsonl")
	oldRollout := filepath.Join(codexHome, "sessions", "2026", "rollout-old.jsonl")
	newRollout := filepath.Join(codexHome, "sessions", "2026", "rollout-new.jsonl")
	for _, file := range []string{waitFile, oldRollout, newRollout} {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	waitTime, oldTime, newTime := now.Add(-time.Minute), now.Add(-3*time.Minute), now.Add(-2*time.Minute)
	for file, stamp := range map[string]time.Time{waitFile: waitTime, oldRollout: oldTime, newRollout: newTime} {
		if err := os.Chtimes(file, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	runner := &fakeRunner{available: map[string]bool{"claude": true, "codex": true, "lsof": true}}
	runner.handle = func(command Command) (string, error) {
		joined := command.Name + " " + strings.Join(command.Args, " ")
		switch {
		case joined == "claude agents --json":
			return `[
 {"pid":100,"status":"waiting","sessionId":"wait-id","cwd":"/tmp/Path With Spaces","kind":"interactive"},
 {"pid":101,"status":"busy","sessionId":"busy-id","cwd":"` + home + `","kind":"interactive"},
 {"pid":999,"status":"idle","sessionId":"hidden","cwd":"/tmp","kind":"interactive"},
 {"pid":998,"status":"busy","sessionId":"worker","cwd":"/tmp","kind":"background"}
]`, nil
		case joined == "ps -Ao pid=,ppid=,tty=,comm=":
			return "100 1 ttys001 claude\n101 1 ttys002 claude\n" +
				"200 1 ttys003 /mock/codex\n201 200 ttys003 codex\n206 1 ttys003 codex\n202 1 ttys004 codex\n203 1 ttys004 codex-helper\n", nil
		case strings.HasPrefix(joined, "tmux list-panes"):
			return "/dev/ttys001\t%1\twork\twork:1.1\t/tmp/ignored\n" +
				"/dev/ttys002\t%2\tclaude-two\tclaude-two:1.1\t/tmp/ignored\n" +
				"/dev/ttys003\t%3\tcodex-three\tcodex-three:1.1\t" + home + "/Project With Spaces\n" +
				"/dev/ttys004\t%4\twork\twork:2.1\t/tmp/loose path\n", nil
		case joined == "lsof -a -p 200,202 -Fn":
			return "p200\nn" + oldRollout + "\nn" + newRollout + "\np202\n", nil
		case strings.HasPrefix(joined, "lsof "):
			return "", nil
		case strings.HasPrefix(joined, "tmux show-option"):
			return "", nil
		default:
			return "", fmt.Errorf("unexpected command: %s", joined)
		}
	}
	app := newTestApp(runner, &fakeClock{now: now}, home)
	agents := app.Agents(context.Background(), "all")
	commands := runner.Commands()
	if got := countCommand(commands, "ps", "-Ao pid=,ppid=,tty=,comm="); got != 1 {
		t.Fatalf("process inventory ran %d times, want 1:\n%s", got, commandLines(commands))
	}
	if got := countCommandPrefix(commands, "tmux", "list-panes -a"); got != 1 {
		t.Fatalf("pane inventory ran %d times, want 1:\n%s", got, commandLines(commands))
	}
	if len(agents) != 4 {
		t.Fatalf("got %d agents: %#v", len(agents), agents)
	}
	if agents[0].Provider != "claude" || agents[0].State != "waiting" || agents[0].Activity.Unix() != waitTime.Unix() {
		t.Fatalf("waiting Claude agent not ranked first: %#v", agents[0])
	}
	assertAgent(t, agents, Agent{Provider: "claude", Pane: "%2", PID: 101, State: "working", Location: "claude-two:1.1", Path: "~"})
	assertAgent(t, agents, Agent{Provider: "codex", Pane: "%3", PID: 200, State: "running", Location: "codex-three:1.1", Path: "~/Project With Spaces", Activity: newTime})
	assertAgent(t, agents, Agent{Provider: "codex", Pane: "%4", PID: 202, State: "running", Location: "work:2.1", Path: "/tmp/loose path"})
	rows := app.Rows(context.Background(), "all")
	if !strings.Contains(rows, "\tclaude\twaiting\t   1m\t") || !strings.Contains(rows, "\tcodex\trunning\t   2m\t") {
		t.Fatalf("unexpected formatted rows:\n%s", rows)
	}
}

func TestDiscoveryTasksOverlap(t *testing.T) {
	entered := make(chan string, 4)
	release := make(chan struct{})
	runner := &fakeRunner{available: map[string]bool{"claude": true, "codex": true}}
	runner.handleCtx = func(ctx context.Context, command Command) (string, error) {
		joined := strings.Join(command.Args, " ")
		name := ""
		switch {
		case command.Name == "ps":
			name = "process"
		case command.Name == "claude":
			name = "claude"
		case command.Name == "tmux" && strings.HasPrefix(joined, "list-panes"):
			name = "panes"
		case command.Name == "tmux" && strings.Contains(joined, "@codex_agent_process_name"):
			name = "codex"
		}
		if name != "" {
			entered <- name
			select {
			case <-release:
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		if command.Name == "claude" {
			return "[]", nil
		}
		return "", nil
	}
	app := newTestApp(runner, &fakeClock{now: time.Now()}, t.TempDir())
	done := make(chan struct{})
	go func() {
		_ = app.Agents(context.Background(), "all")
		close(done)
	}()

	seen := make(map[string]bool)
	for len(seen) < 4 {
		select {
		case name := <-entered:
			seen[name] = true
		case <-time.After(time.Second):
			t.Fatalf("tasks did not overlap, entered: %#v\n%s", seen, commandLines(runner.Commands()))
		}
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("discovery did not finish after releasing tasks")
	}
}

func TestCodexBatchesLsofAndKeepsPartialOutput(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", filepath.Join(home, "codex"))
	one := filepath.Join(home, "codex", "sessions", "one", "rollout-one.jsonl")
	two := filepath.Join(home, "codex", "sessions", "two", "rollout-two.jsonl")
	for _, path := range []string{one, two} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	oneTime, twoTime := now.Add(-time.Minute), now.Add(-2*time.Minute)
	if err := os.Chtimes(one, oneTime, oneTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(two, twoTime, twoTime); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{available: map[string]bool{"codex": true, "lsof": true}}
	runner.handle = func(command Command) (string, error) {
		joined := command.Name + " " + strings.Join(command.Args, " ")
		switch {
		case joined == "ps -Ao pid=,ppid=,tty=,comm=":
			return "20 1 ttys002 codex\n10 1 ttys001 codex\n", nil
		case strings.HasPrefix(joined, "tmux list-panes"):
			return "/dev/ttys001\t%1\tone\tone:1.1\t/tmp/one\n" +
				"/dev/ttys002\t%2\ttwo\ttwo:1.1\t/tmp/two\n", nil
		case joined == "lsof -a -p 10,20 -Fn":
			return "p10\nn" + one + "\np20\nn" + two + "\n", errors.New("process exited")
		}
		return "", nil
	}
	app := newTestApp(runner, &fakeClock{now: now}, home)
	agents := app.Agents(context.Background(), "codex")
	assertAgent(t, agents, Agent{Provider: "codex", Pane: "%1", PID: 10, State: "running", Activity: oneTime, Location: "one:1.1", Path: "/tmp/one"})
	assertAgent(t, agents, Agent{Provider: "codex", Pane: "%2", PID: 20, State: "running", Activity: twoTime, Location: "two:1.1", Path: "/tmp/two"})
	if got := countCommand(runner.Commands(), "lsof", "-a -p 10,20 -Fn"); got != 1 {
		t.Fatalf("lsof ran %d times, want one batch:\n%s", got, commandLines(runner.Commands()))
	}
}

func assertAgent(t *testing.T, agents []Agent, want Agent) {
	t.Helper()
	for _, got := range agents {
		if got.Provider == want.Provider && got.Pane == want.Pane {
			if !got.Activity.Equal(want.Activity) {
				t.Fatalf("activity mismatch: got %v, want %v", got.Activity, want.Activity)
			}
			got.Activity = want.Activity
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("agent mismatch\n got: %#v\nwant: %#v", got, want)
			}
			return
		}
	}
	t.Fatalf("agent not found: %#v", want)
}

func TestProviderDegradation(t *testing.T) {
	runner := &fakeRunner{available: map[string]bool{"claude": true, "codex": true}}
	runner.handle = func(command Command) (string, error) {
		if command.Name == "claude" {
			return "{malformed", nil
		}
		if command.Name == "ps" {
			return "200 1 ttys001 codex\n", nil
		}
		if command.Name == "tmux" && len(command.Args) > 0 && command.Args[0] == "list-panes" {
			return "/dev/ttys001\t%1\twork\twork:1.1\t/tmp/work\n", nil
		}
		return "", nil
	}
	app := newTestApp(runner, &fakeClock{now: time.Now()}, t.TempDir())
	agents := app.Agents(context.Background(), "all")
	if len(agents) != 1 || agents[0].Provider != "codex" || !agents[0].Activity.IsZero() {
		t.Fatalf("malformed Claude or missing lsof affected Codex: %#v", agents)
	}
	runner.available["codex"] = false
	if got := app.Agents(context.Background(), "codex"); len(got) != 0 {
		t.Fatalf("missing provider command should disable provider: %#v", got)
	}
}

func TestClaudeDiscoveryUsesPathResolvedClaude(t *testing.T) {
	runner := &fakeRunner{available: map[string]bool{"claude": true}}
	runner.handle = func(command Command) (string, error) {
		if command.Name == "claude" && strings.Join(command.Args, " ") == "agents --json" {
			return "[]", nil
		}
		return "", nil
	}
	app := newTestApp(runner, &fakeClock{now: time.Now()}, t.TempDir())

	if agents := app.Agents(context.Background(), "claude"); len(agents) != 0 {
		t.Fatalf("unexpected Claude agents: %#v", agents)
	}
	if !reflect.DeepEqual(runner.Lookups(), []string{"claude"}) {
		t.Fatalf("Claude executable lookups: got %#v, want exactly claude", runner.Lookups())
	}
	commands := runner.Commands()
	if !hasCommand(commands, "claude", "agents --json") {
		t.Fatalf("Claude discovery command missing from %#v", commands)
	}
	for _, command := range commands {
		if command.Name == "tmux" && len(command.Args) > 0 && command.Args[0] == "show-option" {
			t.Fatalf("Claude discovery queried a tmux option:\n%s", commandLines(commands))
		}
	}
}

func TestSplitShellWords(t *testing.T) {
	got, err := SplitShellWords(`--border "rounded border" --prompt=Agent\ Picker\>\ `)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--border", "rounded border", "--prompt=Agent Picker> "}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for _, value := range []string{"--bind 'unterminated", "--preview=$(uname)", "--preview=`uname`"} {
		if _, err := SplitShellWords(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestExactRowFormattingAndSorting(t *testing.T) {
	now := time.Unix(1_000, 0)
	agents := []Agent{
		{Provider: "claude", Pane: "%3", PID: 3, State: "working", Location: "work:2.1", Path: "/tmp/three", Activity: time.Unix(820, 0)},
		{Provider: "codex", Pane: "%2", PID: 2, State: "running", Location: "codex-two:1.1", Path: "/tmp/two"},
		{Provider: "claude", Pane: "%1", PID: 1, State: "waiting", Location: "work:1.1", Path: "/tmp/one", Activity: time.Unix(940, 0)},
	}
	SortAgents(agents, now)
	rows := FormatRows(agents, now)
	want := []string{
		"0\t%1\t1\tclaude\twaiting\t   1m\twork:1.1     \t/tmp/one  \t1",
		"2\t%2\t2\tcodex\trunning\t    -\tcodex-two:1.1\t/tmp/two  \t0",
		"3\t%3\t3\tclaude\tworking\t   3m\twork:2.1     \t/tmp/three\t3",
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows mismatch\n got: %#v\nwant: %#v", rows, want)
	}
	for _, row := range rows {
		fields := strings.Split(row, "\t")
		if len(fields) != 9 {
			t.Fatalf("row has %d TSV fields, want 9: %q", len(fields), row)
		}
		starts := terminalFieldStarts(fields[6:])
		if starts[1] != 16 || starts[2] != 32 {
			t.Fatalf("unaligned presentation columns in %q: path=%d age=%d", row, starts[1], starts[2])
		}
	}
}

func terminalFieldStarts(fields []string) []int {
	starts := make([]int, len(fields))
	column := 0
	for i, field := range fields {
		starts[i] = column
		column += len([]rune(field))
		if i < len(fields)-1 {
			column = (column/8 + 1) * 8
		}
	}
	return starts
}

func TestCLICommandsAndProviders(t *testing.T) {
	for _, command := range []string{"popup", "select", "list"} {
		for _, dash := range []string{"-provider", "--provider"} {
			for _, provider := range []string{"all", "claude", "codex"} {
				name := command + "/" + dash + "/" + provider
				t.Run(name, func(t *testing.T) {
					runner := &fakeRunner{available: map[string]bool{"tmux": true, "fzf": true, "claude": true, "codex": true}}
					runner.handle = func(command Command) (string, error) {
						out, _ := fakeAgentResponse(command)
						return out, nil
					}
					app := newTestApp(runner, &fakeClock{now: time.Now()}, t.TempDir())
					stdout, stderr := &strings.Builder{}, &strings.Builder{}
					app.Stdout, app.Stderr = stdout, stderr
					args := []string{command, dash, provider}
					if command == "popup" {
						args = append(args, "client-one")
					}
					if code := app.Main(context.Background(), args); code != 0 {
						t.Fatalf("exit code %d; stderr:\n%s", code, stderr)
					}
					commands := runner.Commands()
					calls := commandLines(commands)
					switch command {
					case "popup":
						want := "display-popup -c client-one"
						if !strings.Contains(calls, want) || !strings.Contains(calls, "select -provider '"+provider+"'") {
							t.Fatalf("popup was not dispatched with provider %q:\n%s", provider, calls)
						}
					case "select":
						if !strings.Contains(calls, "fzf ") || !hasCommandEnv(commands, "fzf", "AGENT_PICKER_PROVIDER", provider) {
							t.Fatalf("select was not dispatched with provider %q:\n%s", provider, calls)
						}
					case "list":
						wantClaude := provider == "all" || provider == "claude"
						wantCodex := provider == "all" || provider == "codex"
						if strings.Contains(calls, "claude agents --json") != wantClaude ||
							!strings.Contains(calls, "ps -Ao pid=,ppid=,tty=,comm=") ||
							strings.Contains(calls, "@codex_agent_process_name") != wantCodex {
							t.Fatalf("list was not dispatched with provider %q:\n%s", provider, calls)
						}
					}
				})
			}
		}
	}
}

func TestCLIPopupOptionalClient(t *testing.T) {
	runner := &fakeRunner{available: map[string]bool{"tmux": true, "codex": true}}
	runner.handle = func(command Command) (string, error) {
		out, _ := fakeAgentResponse(command)
		return out, nil
	}
	app := newTestApp(runner, &fakeClock{}, t.TempDir())
	app.Stdout, app.Stderr = &strings.Builder{}, &strings.Builder{}
	if code := app.Main(context.Background(), []string{"popup"}); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if calls := commandLines(runner.Commands()); strings.Contains(calls, "display-popup -c") {
		t.Fatalf("popup without client unexpectedly used -c:\n%s", calls)
	}
}

func TestCLIHelp(t *testing.T) {
	for _, command := range []string{"popup", "select", "list"} {
		for _, help := range []string{"-h", "--help"} {
			t.Run(command+"/"+help, func(t *testing.T) {
				runner := &fakeRunner{available: map[string]bool{"tmux": true, "fzf": true}}
				app := newTestApp(runner, &fakeClock{}, t.TempDir())
				stderr := &strings.Builder{}
				app.Stdout, app.Stderr = &strings.Builder{}, stderr
				if code := app.Main(context.Background(), []string{command, help}); code != 0 {
					t.Fatalf("exit code %d; stderr:\n%s", code, stderr)
				}
				if !strings.Contains(stderr.String(), "Usage: agent-picker "+command) {
					t.Fatalf("missing command usage:\n%s", stderr)
				}
				if commands := runner.Commands(); len(commands) != 0 {
					t.Fatalf("help executed commands:\n%s", commandLines(commands))
				}
				if lookups := runner.Lookups(); len(lookups) != 0 {
					t.Fatalf("help looked up external tools: %#v", lookups)
				}
			})
		}
	}
}

func TestCLIMalformedInput(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing command", want: "missing command"},
		{name: "unknown command", args: []string{"bogus"}, want: "unknown command"},
		{name: "old implicit popup", args: []string{"client-one"}, want: "unknown command"},
		{name: "unknown flag", args: []string{"popup", "-bogus"}, want: "flag provided but not defined"},
		{name: "missing flag value", args: []string{"select", "-provider"}, want: "flag needs an argument"},
		{name: "invalid provider", args: []string{"list", "--provider", "other"}, want: "invalid provider"},
		{name: "select positional", args: []string{"select", "client"}, want: "unexpected argument"},
		{name: "list positional", args: []string{"list", "client"}, want: "unexpected argument"},
		{name: "multiple clients", args: []string{"popup", "one", "two"}, want: "at most one"},
		{name: "flag after client", args: []string{"popup", "client", "-provider", "codex"}, want: "at most one"},
		{name: "removed list flag", args: []string{"popup", "--list"}, want: "flag provided but not defined"},
		{name: "removed legacy flag", args: []string{"select", "--legacy-claude"}, want: "flag provided but not defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{available: map[string]bool{"tmux": true, "fzf": true}}
			app := newTestApp(runner, &fakeClock{}, t.TempDir())
			stderr := &strings.Builder{}
			app.Stdout, app.Stderr = &strings.Builder{}, stderr
			if code := app.Main(context.Background(), tt.args); code != 2 {
				t.Fatalf("exit code %d, want 2; stderr:\n%s", code, stderr)
			}
			if !strings.Contains(stderr.String(), tt.want) || !strings.Contains(stderr.String(), "Usage") {
				t.Fatalf("stderr missing %q or usage:\n%s", tt.want, stderr)
			}
			if commands := runner.Commands(); len(commands) != 0 {
				t.Fatalf("malformed input executed commands:\n%s", commandLines(commands))
			}
			if lookups := runner.Lookups(); len(lookups) != 0 {
				t.Fatalf("malformed input looked up external tools: %#v", lookups)
			}
		})
	}
}

func TestSelectionNavigationAndCancellation(t *testing.T) {
	tests := []struct {
		name        string
		selected    string
		parent      string
		want        []string
		notWant     string
		sessionName string
	}{
		{name: "cancel", notWant: "switch-client"},
		{name: "without parent", selected: "2\t%5\t202\tcodex\trunning\t    -\twork:3.1\t/tmp\t0\n", sessionName: "work", want: []string{"switch-client -t work", "select-window -t %5", "select-pane -t %5"}},
		{name: "prefixed session", selected: "2\t%4\t200\tcodex\trunning\t    -\tcodex-four:1.1\t/tmp\t0\n", parent: "outer-client", sessionName: "codex-four", want: []string{"switch-client -c outer-client -t codex-four", "select-window -t %4", "select-pane -t %4"}, notWant: "attach-session"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AGENT_PICKER_CLIENT", tt.parent)
			runner := &fakeRunner{available: map[string]bool{"tmux": true, "fzf": true, "codex": true}}
			runner.handle = func(command Command) (string, error) {
				joined := strings.Join(command.Args, " ")
				switch {
				case command.Name == "fzf":
					if !strings.Contains(joined, "kill {3}") || !strings.Contains(joined, "list -provider") {
						t.Fatalf("termination/reload binding missing: %s", joined)
					}
					return tt.selected, nil
				case strings.HasPrefix(joined, "display-message -p -t %5"):
					return tt.sessionName, nil
				case strings.HasPrefix(joined, "display-message -p -t %4"):
					return tt.sessionName, nil
				}
				if out, ok := fakeAgentResponse(command); ok {
					return out, nil
				}
				return "", nil
			}
			app := newTestApp(runner, &fakeClock{now: time.Now()}, t.TempDir())
			app.Stdout, app.Stderr = &strings.Builder{}, &strings.Builder{}
			app.Select(context.Background(), "codex")
			calls := commandLines(runner.Commands())
			for _, want := range tt.want {
				if !strings.Contains(calls, "tmux "+want) {
					t.Fatalf("missing %q in calls:\n%s", want, calls)
				}
			}
			if tt.notWant != "" && strings.Contains(calls, tt.notWant) {
				t.Fatalf("unexpected %q in calls:\n%s", tt.notWant, calls)
			}
		})
	}
}

func TestSelectionStartsFZFBeforeDiscoveryCompletes(t *testing.T) {
	discoveryStarted := make(chan struct{})
	fzfStarted := make(chan struct{})
	releaseDiscovery := make(chan struct{})
	runner := &fakeRunner{available: map[string]bool{"tmux": true, "fzf": true, "claude": true}}
	var discoveryOnce, fzfOnce sync.Once
	runner.handleCtx = func(ctx context.Context, command Command) (string, error) {
		switch command.Name {
		case "claude":
			discoveryOnce.Do(func() { close(discoveryStarted) })
			select {
			case <-releaseDiscovery:
				return "[]", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		case "fzf":
			fzfOnce.Do(func() { close(fzfStarted) })
			_, err := io.ReadAll(command.Input)
			return "", err
		}
		return "", nil
	}
	app := newTestApp(runner, &fakeClock{now: time.Now()}, t.TempDir())
	app.Stderr = &strings.Builder{}
	done := make(chan struct{})
	go func() {
		app.Select(context.Background(), "claude")
		close(done)
	}()

	for name, started := range map[string]<-chan struct{}{"fzf": fzfStarted, "discovery": discoveryStarted} {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatalf("%s did not start", name)
		}
	}
	select {
	case <-done:
		t.Fatal("selection finished before blocked discovery was released")
	default:
	}
	close(releaseDiscovery)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("selection did not finish")
	}
}

func TestSelectionEscapeCancelsBlockedDiscovery(t *testing.T) {
	discoveryStarted := make(chan struct{})
	discoveryStopped := make(chan struct{})
	runner := &fakeRunner{available: map[string]bool{"tmux": true, "fzf": true, "claude": true}}
	var startedOnce, stoppedOnce sync.Once
	runner.handleCtx = func(ctx context.Context, command Command) (string, error) {
		switch command.Name {
		case "claude":
			startedOnce.Do(func() { close(discoveryStarted) })
			<-ctx.Done()
			stoppedOnce.Do(func() { close(discoveryStopped) })
			return "", ctx.Err()
		case "fzf":
			<-discoveryStarted
			return "", nil
		}
		return "", nil
	}
	app := newTestApp(runner, &fakeClock{now: time.Now()}, t.TempDir())
	app.Stderr = &strings.Builder{}
	done := make(chan struct{})
	go func() {
		app.Select(context.Background(), "claude")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("selection leaked after escape")
	}
	select {
	case <-discoveryStopped:
	case <-time.After(time.Second):
		t.Fatal("blocked discovery subprocess was not canceled")
	}
	if calls := commandLines(runner.Commands()); strings.Contains(calls, "no running Claude agents") {
		t.Fatalf("escape was mistaken for an empty result:\n%s", calls)
	}
}

func TestSelectionReceivesAlignedTSVRows(t *testing.T) {
	runner := &fakeRunner{available: map[string]bool{"tmux": true, "fzf": true, "codex": true}}
	collections := 0
	runner.handle = func(command Command) (string, error) {
		joined := strings.Join(command.Args, " ")
		switch {
		case command.Name == "ps":
			collections++
			return "200 1 ttys001 codex\n201 1 ttys002 codex\n", nil
		case command.Name == "tmux" && strings.HasPrefix(joined, "list-panes"):
			return "/dev/ttys001\t%5\twork\twork:1.1\t/tmp/x\n" +
				"/dev/ttys002\t%6\tcodex-long\tcodex-long:1.1\t/tmp/a much longer path\n", nil
		case command.Name == "fzf":
			if !strings.Contains(joined, "kill {3}") {
				t.Fatalf("kill binding no longer targets the PID field: %s", joined)
			}
			input, err := io.ReadAll(command.Input)
			if err != nil {
				t.Fatal(err)
			}
			rows := string(input)
			lines := strings.Split(strings.TrimSuffix(rows, "\n"), "\n")
			if len(lines) != 2 {
				t.Fatalf("fzf received %d rows, want 2: %q", len(lines), rows)
			}
			for i, line := range lines {
				fields := strings.Split(line, "\t")
				if len(fields) != 9 {
					t.Fatalf("fzf row %d has %d TSV fields: %q", i, len(fields), line)
				}
				wantPane, wantPID := []string{"%5", "%6"}[i], []string{"200", "201"}[i]
				if fields[1] != wantPane || fields[2] != wantPID || fields[8] != "0" {
					t.Fatalf("navigation/kill/age fields changed in row %d: %#v", i, fields)
				}
				if len(fields[6]) != len("codex-long:1.1") || len(fields[7]) != len("/tmp/a much longer path") {
					t.Fatalf("presentation fields are not padded to common widths: %#v", fields)
				}
			}
			return lines[0] + "\n", nil
		case command.Name == "tmux" && strings.HasPrefix(joined, "display-message -p -t %5"):
			return "work", nil
		}
		return "", nil
	}
	app := newTestApp(runner, &fakeClock{now: time.Now()}, t.TempDir())
	app.Stdout, app.Stderr = &strings.Builder{}, &strings.Builder{}
	app.Select(context.Background(), "codex")
	if collections != 1 {
		t.Fatalf("select collected agents %d times, want 1", collections)
	}
	calls := commandLines(runner.Commands())
	for _, want := range []string{"tmux switch-client -t work", "tmux select-window -t %5", "tmux select-pane -t %5"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("padded fields affected navigation; missing %q in calls:\n%s", want, calls)
		}
	}
}

func TestSelectionFZFOptions(t *testing.T) {
	runner := &fakeRunner{available: map[string]bool{"tmux": true, "fzf": true, "codex": true}}
	runner.handle = func(command Command) (string, error) {
		joined := strings.Join(command.Args, " ")
		if command.Name == "tmux" && joined == "show-option -gqv @agent_fzf_options" {
			return `--prompt "Agent > " --border=rounded`, nil
		}
		if command.Name == "fzf" {
			if !containsArgument(command.Args, "--prompt", "Agent > ") ||
				!containsArgument(command.Args, "--border=rounded", "") {
				t.Fatalf("options were not parsed as arguments: %#v", command.Args)
			}
			if _, ok := command.Env["CLAUDE_AGENT_PICKER"]; ok {
				t.Fatal("removed CLAUDE_AGENT_PICKER environment variable was set")
			}
		}
		if out, ok := fakeAgentResponse(command); ok {
			return out, nil
		}
		return "", nil
	}
	app := newTestApp(runner, &fakeClock{now: time.Now()}, t.TempDir())
	app.Stdout, app.Stderr = &strings.Builder{}, &strings.Builder{}
	app.Select(context.Background(), "all")
}

func containsArgument(arguments []string, name, following string) bool {
	for i, argument := range arguments {
		if argument == name && (following == "" || i+1 < len(arguments) && arguments[i+1] == following) {
			return true
		}
	}
	return false
}

func TestPopupOptionsAndPrefixedSessionName(t *testing.T) {
	runner := &fakeRunner{available: map[string]bool{"tmux": true, "codex": true}}
	runner.handle = func(command Command) (string, error) {
		joined := strings.Join(command.Args, " ")
		switch {
		case joined == "show-option -gqv @agent_popup_width":
			return "81%", nil
		case joined == "show-option -gqv @agent_popup_height":
			return "72%", nil
		}
		if out, ok := fakeAgentResponse(command); ok {
			return out, nil
		}
		return "", nil
	}
	app := newTestApp(runner, &fakeClock{now: time.Now()}, t.TempDir())
	app.Stderr = &strings.Builder{}
	app.Popup(context.Background(), "all", "client-one")
	calls := commandLines(runner.Commands())
	for _, want := range []string{
		"tmux display-popup -c client-one -w 81% -h 72% -E AGENT_PICKER_CLIENT='client-one' '/bin/agent picker' select -provider 'all'",
	} {
		if !strings.Contains(calls, want) {
			t.Fatalf("missing %q in calls:\n%s", want, calls)
		}
	}
	for _, unwanted := range []string{"list-clients", "detach-client"} {
		if strings.Contains(calls, unwanted) {
			t.Fatalf("session name unexpectedly caused %q:\n%s", unwanted, calls)
		}
	}
}

func TestPopupNoHost(t *testing.T) {
	runner := &fakeRunner{available: map[string]bool{"tmux": true, "codex": true}}
	runner.handle = func(command Command) (string, error) {
		out, _ := fakeAgentResponse(command)
		return out, nil
	}
	app := newTestApp(runner, &fakeClock{}, t.TempDir())
	app.Stderr = &strings.Builder{}
	app.Popup(context.Background(), "all", "")
	calls := commandLines(runner.Commands())
	if strings.Contains(calls, "display-popup -c") || !strings.Contains(calls, "tmux display-popup -w 90% -h 90%") {
		t.Fatalf("no-host popup arguments are wrong:\n%s", calls)
	}
}

func TestPopupStartsWithoutDiscoveryOrGlobalState(t *testing.T) {
	runner := &fakeRunner{available: map[string]bool{"tmux": true}}
	app := newTestApp(runner, &fakeClock{}, t.TempDir())
	app.Stderr = &strings.Builder{}
	app.Popup(context.Background(), "claude", "client-one")
	calls := commandLines(runner.Commands())
	if !strings.Contains(calls, "tmux display-popup -c client-one") {
		t.Fatalf("popup was not displayed:\n%s", calls)
	}
	for _, unwanted := range []string{"claude ", "ps ", "list-panes", "lsof ", "@agent_parent", "set-option"} {
		if strings.Contains(calls, unwanted) {
			t.Fatalf("popup unexpectedly called %q:\n%s", unwanted, calls)
		}
	}
}

func TestEmptySelectClosesFZFAndMessagesOriginatingClient(t *testing.T) {
	tests := []struct {
		provider string
		message  string
	}{
		{provider: "all", message: "agent-picker: no running agents found"},
		{provider: "claude", message: "agent-picker: no running Claude agents found"},
		{provider: "codex", message: "agent-picker: no running Codex agents found"},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			t.Setenv("AGENT_PICKER_CLIENT", "origin-client")
			runner := &fakeRunner{available: map[string]bool{
				"tmux": true, "fzf": true, "claude": true, "codex": true,
			}}
			runner.handle = func(command Command) (string, error) {
				if command.Name == "claude" {
					return "[]", nil
				}
				if command.Name == "fzf" {
					_, err := io.ReadAll(command.Input)
					return "", err
				}
				return "", nil
			}
			app := newTestApp(runner, &fakeClock{}, t.TempDir())
			app.Stderr = &strings.Builder{}
			app.Select(context.Background(), tt.provider)
			calls := commandLines(runner.Commands())
			if !strings.Contains(calls, "tmux display-message -c origin-client "+tt.message) {
				t.Fatalf("missing empty-state message:\n%s", calls)
			}
			if !strings.Contains(calls, "fzf ") {
				t.Fatalf("empty select did not start fzf:\n%s", calls)
			}
			if strings.Contains(calls, "@agent_parent") || strings.Contains(calls, "set-option") {
				t.Fatalf("empty select mutated or read global state:\n%s", calls)
			}
		})
	}
}

func TestEmptyListIsSilentAndSuccessful(t *testing.T) {
	runner := &fakeRunner{available: map[string]bool{}}
	app := newTestApp(runner, &fakeClock{}, t.TempDir())
	stdout, stderr := &strings.Builder{}, &strings.Builder{}
	app.Stdout, app.Stderr = stdout, stderr
	if code := app.Main(context.Background(), []string{"list", "-provider", "all"}); code != 0 {
		t.Fatalf("exit code %d; stderr:\n%s", code, stderr)
	}
	if stdout.Len() != 0 {
		t.Fatalf("empty list wrote output: %q", stdout.String())
	}
}

func commandLines(commands []Command) string {
	var lines []string
	for _, command := range commands {
		lines = append(lines, command.Name+" "+strings.Join(command.Args, " "))
	}
	return strings.Join(lines, "\n")
}

func hasCommand(commands []Command, name, args string) bool {
	for _, command := range commands {
		if command.Name == name && strings.Join(command.Args, " ") == args {
			return true
		}
	}
	return false
}

func hasCommandEnv(commands []Command, name, key, value string) bool {
	for _, command := range commands {
		if command.Name == name && command.Env[key] == value {
			return true
		}
	}
	return false
}

func countCommand(commands []Command, name, args string) int {
	count := 0
	for _, command := range commands {
		if command.Name == name && strings.Join(command.Args, " ") == args {
			count++
		}
	}
	return count
}

func countCommandPrefix(commands []Command, name, argsPrefix string) int {
	count := 0
	for _, command := range commands {
		if command.Name == name && strings.HasPrefix(strings.Join(command.Args, " "), argsPrefix) {
			count++
		}
	}
	return count
}
