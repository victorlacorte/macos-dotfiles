package agentpicker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	available map[string]bool
	handle    func(Command) (string, error)
	commands  []Command
	lookups   []string
}

func (r *fakeRunner) LookPath(name string) (string, error) {
	r.lookups = append(r.lookups, name)
	if r.available[name] {
		return "/fake/" + filepath.Base(name), nil
	}
	return "", errors.New("not found")
}

func (r *fakeRunner) Run(_ context.Context, command Command) (string, error) {
	r.commands = append(r.commands, command)
	if r.handle != nil {
		return r.handle(command)
	}
	return "", nil
}

type fakeClock struct {
	now    time.Time
	sleeps int
}

func (c *fakeClock) Now() time.Time      { return c.now }
func (c *fakeClock) Sleep(time.Duration) { c.sleeps++ }

func newTestApp(runner *fakeRunner, clock *fakeClock, home string) *App {
	return &App{Runner: runner, FS: OSFileSystem{}, Clock: clock, Home: home, Executable: "/bin/agent picker"}
}

func fakeAgentResponse(command Command) (string, bool) {
	joined := strings.Join(command.Args, " ")
	switch {
	case command.Name == "claude" && joined == "agents --json":
		return `[{"pid":100,"status":"waiting","sessionId":"claude-id","cwd":"/tmp/claude","kind":"interactive"}]`, true
	case command.Name == "ps" && joined == "-Ao pid=,tty=":
		return "100 ttys001\n", true
	case command.Name == "ps" && joined == "-Ao pid=,ppid=,tty=,comm=":
		return "200 1 ttys002 codex\n", true
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
		case joined == "ps -Ao pid=,tty=":
			return "100 ttys001\n101 ttys002\n", nil
		case joined == "ps -Ao pid=,ppid=,tty=,comm=":
			return "200 1 ttys003 /mock/codex\n201 200 ttys003 codex\n206 1 ttys003 codex\n202 1 ttys004 codex\n203 1 ttys004 codex-helper\n", nil
		case strings.HasPrefix(joined, "tmux list-panes"):
			return "/dev/ttys001\t%1\twork\twork:1.1\t/tmp/ignored\n" +
				"/dev/ttys002\t%2\tclaude-two\tclaude-two:1.1\t/tmp/ignored\n" +
				"/dev/ttys003\t%3\tcodex-three\tcodex-three:1.1\t" + home + "/Project With Spaces\n" +
				"/dev/ttys004\t%4\twork\twork:2.1\t/tmp/loose path\n", nil
		case joined == "lsof -a -p 200 -Fn":
			return "p200\nn" + oldRollout + "\nn" + newRollout + "\n", nil
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
	if len(agents) != 4 {
		t.Fatalf("got %d agents: %#v", len(agents), agents)
	}
	if agents[0].Provider != "claude" || agents[0].State != "waiting" || agents[0].Activity.Unix() != waitTime.Unix() {
		t.Fatalf("waiting Claude agent not ranked first: %#v", agents[0])
	}
	assertAgent(t, agents, Agent{Provider: "claude", Pane: "%2", PID: 101, Kind: "dedicated", State: "working", Location: "claude-two:1.1", Path: "~"})
	assertAgent(t, agents, Agent{Provider: "codex", Pane: "%3", PID: 200, Kind: "dedicated", State: "running", Location: "codex-three:1.1", Path: "~/Project With Spaces", Activity: newTime})
	assertAgent(t, agents, Agent{Provider: "codex", Pane: "%4", PID: 202, Kind: "loose", State: "running", Location: "work:2.1", Path: "/tmp/loose path"})
	rows := app.Rows(context.Background(), "all")
	if !strings.Contains(rows, "\tclaude\twaiting\t   1m\t") || !strings.Contains(rows, "\tcodex\trunning\t   2m\t") {
		t.Fatalf("unexpected formatted rows:\n%s", rows)
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
		{Provider: "claude", Pane: "%3", PID: 3, Kind: "loose", State: "working", Location: "work:2.1", Path: "/tmp/three", Activity: time.Unix(820, 0)},
		{Provider: "codex", Pane: "%2", PID: 2, Kind: "dedicated", State: "running", Location: "codex-two:1.1", Path: "/tmp/two"},
		{Provider: "claude", Pane: "%1", PID: 1, Kind: "loose", State: "waiting", Location: "work:1.1", Path: "/tmp/one", Activity: time.Unix(940, 0)},
	}
	SortAgents(agents, now)
	rows := FormatRows(agents, now)
	want := []string{
		"0\t%1\t1\tloose\tclaude\twaiting\t   1m\twork:1.1     \t/tmp/one  \t1",
		"2\t%2\t2\tdedicated\tcodex\trunning\t    -\tcodex-two:1.1\t/tmp/two  \t0",
		"3\t%3\t3\tloose\tclaude\tworking\t   3m\twork:2.1     \t/tmp/three\t3",
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows mismatch\n got: %#v\nwant: %#v", rows, want)
	}
	for _, row := range rows {
		fields := strings.Split(row, "\t")
		if len(fields) != 10 {
			t.Fatalf("row has %d TSV fields, want 10: %q", len(fields), row)
		}
		starts := terminalFieldStarts(fields[7:])
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
					calls := commandLines(runner.commands)
					switch command {
					case "popup":
						want := "display-popup -c client-one"
						if !strings.Contains(calls, want) || !strings.Contains(calls, "select -provider '"+provider+"'") {
							t.Fatalf("popup was not dispatched with provider %q:\n%s", provider, calls)
						}
					case "select":
						if !strings.Contains(calls, "fzf ") || runner.commands[len(runner.commands)-1].Env["AGENT_PICKER_PROVIDER"] != provider {
							t.Fatalf("select was not dispatched with provider %q:\n%s", provider, calls)
						}
					case "list":
						wantClaude := provider == "all" || provider == "claude"
						wantCodex := provider == "all" || provider == "codex"
						if strings.Contains(calls, "claude agents --json") != wantClaude ||
							strings.Contains(calls, "ps -Ao pid=,ppid=,tty=,comm=") != wantCodex {
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
	if calls := commandLines(runner.commands); strings.Contains(calls, "display-popup -c") {
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
				if len(runner.commands) != 0 {
					t.Fatalf("help executed commands:\n%s", commandLines(runner.commands))
				}
				if len(runner.lookups) != 0 {
					t.Fatalf("help looked up external tools: %#v", runner.lookups)
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
			if len(runner.commands) != 0 {
				t.Fatalf("malformed input executed commands:\n%s", commandLines(runner.commands))
			}
			if len(runner.lookups) != 0 {
				t.Fatalf("malformed input looked up external tools: %#v", runner.lookups)
			}
		})
	}
}

func TestSelectionNavigationAndCancellation(t *testing.T) {
	tests := []struct {
		name     string
		selected string
		parent   string
		origin   string
		want     []string
		notWant  string
	}{
		{name: "cancel", notWant: "switch-client"},
		{name: "loose", selected: "2\t%5\t202\tloose\tcodex\trunning\t    -\twork:3.1\t/tmp\t0\n", want: []string{"switch-client -t work", "select-window -t %5", "select-pane -t %5"}},
		{name: "dedicated", selected: "2\t%4\t200\tdedicated\tcodex\trunning\t    -\tcodex-four:1.1\t/tmp\t0\n", parent: "outer-client", origin: "origin-window", want: []string{"switch-client -c outer-client -t origin-window", "attach-session -t codex-four"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{available: map[string]bool{"tmux": true, "fzf": true, "codex": true}}
			runner.handle = func(command Command) (string, error) {
				joined := strings.Join(command.Args, " ")
				switch {
				case command.Name == "fzf":
					if !strings.Contains(joined, "kill {3}") || !strings.Contains(joined, "list -provider") {
						t.Fatalf("termination/reload binding missing: %s", joined)
					}
					return tt.selected, nil
				case strings.Contains(joined, "@agent_parent"):
					return tt.parent, nil
				case strings.Contains(joined, "@codex_agent_origin"):
					return tt.origin, nil
				case strings.HasPrefix(joined, "display-message -p -t %5"):
					return "work", nil
				case strings.HasPrefix(joined, "display-message -p -t %4"):
					return "codex-four", nil
				}
				if out, ok := fakeAgentResponse(command); ok {
					return out, nil
				}
				return "", nil
			}
			app := newTestApp(runner, &fakeClock{now: time.Now()}, t.TempDir())
			app.Stdout, app.Stderr = &strings.Builder{}, &strings.Builder{}
			app.Select(context.Background(), "codex")
			calls := commandLines(runner.commands)
			for _, want := range tt.want {
				if !strings.Contains(calls, "tmux "+want) {
					t.Fatalf("missing %q in calls:\n%s", want, calls)
				}
			}
			if tt.notWant != "" && strings.Contains(calls, tt.notWant) {
				t.Fatalf("unexpected %q in calls:\n%s", tt.notWant, calls)
			}
			if tt.name == "dedicated" && !runner.commands[len(runner.commands)-1].Interactive {
				t.Fatal("dedicated session attach must inherit the popup terminal")
			}
		})
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
			lines := strings.Split(strings.TrimSuffix(command.Input, "\n"), "\n")
			if len(lines) != 2 {
				t.Fatalf("fzf received %d rows, want 2: %q", len(lines), command.Input)
			}
			for i, line := range lines {
				fields := strings.Split(line, "\t")
				if len(fields) != 10 {
					t.Fatalf("fzf row %d has %d TSV fields: %q", i, len(fields), line)
				}
				wantPane, wantPID := []string{"%5", "%6"}[i], []string{"200", "201"}[i]
				if fields[1] != wantPane || fields[2] != wantPID || fields[9] != "0" {
					t.Fatalf("navigation/kill/age fields changed in row %d: %#v", i, fields)
				}
				if len(fields[7]) != len("codex-long:1.1") || len(fields[8]) != len("/tmp/a much longer path") {
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
	calls := commandLines(runner.commands)
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

func TestPopupOptionsAndDedicatedDetach(t *testing.T) {
	clock := &fakeClock{now: time.Now()}
	listCount := 0
	runner := &fakeRunner{available: map[string]bool{"tmux": true, "codex": true}}
	runner.handle = func(command Command) (string, error) {
		joined := strings.Join(command.Args, " ")
		switch {
		case joined == "show-option -gqv @agent_popup_width":
			return "81%", nil
		case joined == "show-option -gqv @agent_popup_height":
			return "72%", nil
		case joined == "list-clients -F #{client_name} #{session_name}":
			return "client-one codex-project\nother work\n", nil
		case joined == "list-clients -F #{session_name}":
			listCount++
			if listCount == 1 {
				return "codex-project\n", nil
			}
			return "work\n", nil
		case joined == "show-options -gqv @agent_parent":
			return "host-client", nil
		}
		if out, ok := fakeAgentResponse(command); ok {
			return out, nil
		}
		return "", nil
	}
	app := newTestApp(runner, clock, t.TempDir())
	app.Stderr = &strings.Builder{}
	app.Popup(context.Background(), "all", "client-one")
	calls := commandLines(runner.commands)
	for _, want := range []string{
		"tmux detach-client -s codex-project",
		"tmux display-popup -c host-client -w 81% -h 72% -E '/bin/agent picker' select -provider 'all'",
	} {
		if !strings.Contains(calls, want) {
			t.Fatalf("missing %q in calls:\n%s", want, calls)
		}
	}
	if clock.sleeps != 1 {
		t.Fatalf("detach polling slept %d times, want 1", clock.sleeps)
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
	calls := commandLines(runner.commands)
	if strings.Contains(calls, "display-popup -c") || !strings.Contains(calls, "tmux display-popup -w 90% -h 90%") {
		t.Fatalf("no-host popup arguments are wrong:\n%s", calls)
	}
}

func TestEmptyPopupKeepsDedicatedSessionAttached(t *testing.T) {
	runner := &fakeRunner{available: map[string]bool{"tmux": true}}
	runner.handle = func(command Command) (string, error) {
		if strings.Join(command.Args, " ") == "list-clients -F #{client_name} #{session_name}" {
			return "client-one claude-project\n", nil
		}
		return "", nil
	}
	app := newTestApp(runner, &fakeClock{}, t.TempDir())
	app.Stderr = &strings.Builder{}
	app.Popup(context.Background(), "claude", "client-one")
	calls := commandLines(runner.commands)
	if !strings.Contains(calls, "tmux display-message agent-picker: no running Claude agents found") {
		t.Fatalf("missing empty-state message:\n%s", calls)
	}
	for _, unwanted := range []string{"list-clients", "detach-client", "set-option -g @agent_parent", "display-popup"} {
		if strings.Contains(calls, unwanted) {
			t.Fatalf("empty popup unexpectedly called %q:\n%s", unwanted, calls)
		}
	}
}

func TestEmptySelectMessagesAndSkipsFZF(t *testing.T) {
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
			runner := &fakeRunner{available: map[string]bool{
				"tmux": true, "fzf": true, "claude": true, "codex": true,
			}}
			runner.handle = func(command Command) (string, error) {
				if command.Name == "claude" {
					return "[]", nil
				}
				return "", nil
			}
			app := newTestApp(runner, &fakeClock{}, t.TempDir())
			app.Stderr = &strings.Builder{}
			app.Select(context.Background(), tt.provider)
			calls := commandLines(runner.commands)
			if !strings.Contains(calls, "tmux display-message "+tt.message) {
				t.Fatalf("missing empty-state message:\n%s", calls)
			}
			if strings.Contains(calls, "fzf ") {
				t.Fatalf("empty select invoked fzf:\n%s", calls)
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
