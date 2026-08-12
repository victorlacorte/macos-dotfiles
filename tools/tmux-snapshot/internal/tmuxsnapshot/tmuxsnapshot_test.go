package tmuxsnapshot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeRunner struct {
	mu       sync.Mutex
	handler  func(Command) (string, error)
	commands []Command
}

func (r *fakeRunner) Run(_ context.Context, command Command) (string, error) {
	r.mu.Lock()
	r.commands = append(r.commands, command)
	handler := r.handler
	r.mu.Unlock()
	if handler == nil {
		return "", nil
	}
	return handler(command)
}

func (r *fakeRunner) Commands() []Command {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Command(nil), r.commands...)
}

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time { return c.now }

func testApp(runner Runner, home string) *App {
	return &App{
		Runner: runner,
		Clock:  fakeClock{now: time.Date(2026, 8, 12, 13, 6, 0, 0, time.UTC)},
		Home:   home,
		Stdout: &strings.Builder{},
		Stderr: &strings.Builder{},
	}
}

func commandHas(commands []Command, name string, args ...string) bool {
	for _, command := range commands {
		if command.Name == name && reflect.DeepEqual(command.Args, args) {
			return true
		}
	}
	return false
}

func commandCount(commands []Command, name string) int {
	count := 0
	for _, command := range commands {
		if command.Name == name {
			count++
		}
	}
	return count
}

func tmuxOutput(records ...[]string) string {
	var builder strings.Builder
	for _, fields := range records {
		builder.WriteString(strings.Join(fields, unitSeparator))
		builder.WriteString(recordSeparator)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func TestSaveUsesTwoTmuxCallsAndPreservesPaths(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	mainPath := filepath.Join(stateHome, "main\tproject\npath")
	windowPath := filepath.Join(stateHome, "window\tpath\nline")
	runner := &fakeRunner{
		handler: func(command Command) (string, error) {
			switch command.Args[0] {
			case "list-sessions":
				return tmuxOutput(
					[]string{"20", "older", "/tmp/older", "0"},
					[]string{"10", "main", mainPath, "1"},
				), nil
			case "list-windows":
				return tmuxOutput(
					[]string{"main", "3", "automatic\tname\nline", windowPath, "1", "off"},
					[]string{"older", "0", "shell", "/tmp/older", "1", "on"},
					[]string{"main", "1", "editor", mainPath, "0", "on"},
				), nil
			default:
				return "", errors.New("unexpected tmux command")
			}
		},
	}
	app := testApp(runner, t.TempDir())

	path, err := app.Save(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(stateHome, "tmux-snapshot", "20260812T130600Z.json")
	if path != wantPath {
		t.Fatalf("snapshot path: got %q, want %q", path, wantPath)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := decodeSnapshot(data)
	if err != nil {
		t.Fatal(err)
	}
	want := Snapshot{
		Version: 1,
		Sessions: []Session{
			{
				Name:     "main",
				Path:     mainPath,
				Attached: true,
				Windows: []Window{
					{Index: 1, Name: "editor", Path: mainPath, Active: false, ManualName: false},
					{Index: 3, Name: "automatic\tname\nline", Path: windowPath, Active: true, ManualName: true},
				},
			},
			{
				Name:    "older",
				Path:    "/tmp/older",
				Windows: []Window{{Index: 0, Name: "shell", Path: "/tmp/older", Active: true}},
			},
		},
	}
	if !reflect.DeepEqual(snapshot, want) {
		t.Fatalf("snapshot mismatch:\n got: %#v\nwant: %#v", snapshot, want)
	}
	commands := runner.Commands()
	if commandCount(commands, "tmux") != 2 {
		t.Fatalf("save made %d tmux calls, want 2: %#v", commandCount(commands, "tmux"), commands)
	}
	for _, command := range commands {
		if command.Name != "tmux" {
			t.Fatalf("unexpected command: %#v", command)
		}
	}

	stateDir := filepath.Dir(path)
	if mode := fileMode(t, stateDir); mode != 0o700 {
		t.Fatalf("state directory mode: got %o, want 700", mode)
	}
	if mode := fileMode(t, path); mode != 0o600 {
		t.Fatalf("snapshot mode: got %o, want 600", mode)
	}
	target, err := os.Readlink(filepath.Join(stateDir, "latest"))
	if err != nil {
		t.Fatal(err)
	}
	if target != filepath.Base(path) {
		t.Fatalf("latest target: got %q, want %q", target, filepath.Base(path))
	}
}

func TestSaveCollisionSuffixAndDefaultResolution(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	runner := &fakeRunner{
		handler: func(command Command) (string, error) {
			if command.Args[0] == "list-sessions" {
				return tmuxOutput([]string{"1", "work", "/tmp", "0"}), nil
			}
			return tmuxOutput([]string{"work", "0", "shell", "/tmp", "1", "on"}), nil
		},
	}
	app := testApp(runner, t.TempDir())
	first, err := app.Save(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.Save(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(first) != "20260812T130600Z.json" ||
		filepath.Base(second) != "20260812T130600Z-1.json" {
		t.Fatalf("collision paths: %q, %q", first, second)
	}
	resolved, err := app.resolveSnapshot("")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != filepath.Join(stateHome, "tmux-snapshot", "latest") {
		t.Fatalf("resolved default: got %q", resolved)
	}
}

func TestSnapshotValidation(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "malformed json", data: `{`},
		{name: "zero sessions", data: `{"version":1,"sessions":[]}`},
		{name: "unknown field", data: `{"version":1,"sessions":[],"extra":true}`},
		{name: "duplicate window index", data: `{"version":1,"sessions":[{"name":"s","path":"/tmp","windows":[{"index":0,"path":"/tmp"},{"index":0,"path":"/tmp"}]}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := decodeSnapshot([]byte(tt.data)); err == nil {
				t.Fatal("decode unexpectedly succeeded")
			}
		})
	}
}

func TestRestoreRespawnsBaseWindowAndRestoresOptions(t *testing.T) {
	dir := t.TempDir()
	windowDir := filepath.Join(dir, "window")
	if err := os.Mkdir(windowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	snapshot := Snapshot{
		Version: 1,
		Sessions: []Session{{
			Name: "work", Path: dir, Attached: true,
			Windows: []Window{
				{Index: 1, Name: "editor", Path: dir, ManualName: true},
				{Index: 3, Name: "shell", Path: filepath.Join(dir, "missing"), Active: true},
			},
		}},
	}
	file := writeTestSnapshot(t, snapshot)
	runner := &fakeRunner{
		handler: func(command Command) (string, error) {
			if command.Name != "tmux" {
				return "", errors.New("unexpected command")
			}
			switch command.Args[0] {
			case "has-session":
				return "", errors.New("missing")
			case "show-option":
				if command.Args[len(command.Args)-1] == "base-index" {
					return "1\n", nil
				}
				if command.Args[len(command.Args)-1] == "renumber-windows" &&
					command.Args[1] == "-t" {
					return "off\n", nil
				}
				return "on\n", nil
			default:
				return "", nil
			}
		},
	}
	app := testApp(runner, t.TempDir())
	if err := app.restoreSnapshot(context.Background(), file); err != nil {
		t.Fatal(err)
	}
	commands := runner.Commands()
	if !commandHas(commands, "tmux", "respawn-window", "-k", "-t", "=work:1", "-c", dir) {
		t.Fatalf("base window was not respawned: %#v", commands)
	}
	if !commandHas(commands, "tmux", "rename-window", "-t", "=work:1", "editor") {
		t.Fatalf("manual base name was not restored: %#v", commands)
	}
	if !commandHas(commands, "tmux", "new-window", "-d", "-t", "=work:3", "-c", dir) {
		t.Fatalf("fallback window was not created: %#v", commands)
	}
	if !commandHas(commands, "tmux", "set-option", "-t", "work", "renumber-windows", "off") {
		t.Fatalf("renumber-windows was not restored: %#v", commands)
	}
	if !commandHas(commands, "tmux", "select-window", "-t", "=work:3") {
		t.Fatalf("active window was not selected: %#v", commands)
	}
}

func TestRestoreKillsInitialBaseWindowWhenVacant(t *testing.T) {
	dir := t.TempDir()
	snapshot := Snapshot{
		Version: 1,
		Sessions: []Session{{
			Name: "work", Path: dir,
			Windows: []Window{{Index: 2, Name: "shell", Path: dir, ManualName: true}},
		}},
	}
	file := writeTestSnapshot(t, snapshot)
	runner := &fakeRunner{
		handler: func(command Command) (string, error) {
			if command.Args[0] == "has-session" {
				return "", errors.New("missing")
			}
			if command.Args[0] == "show-option" && command.Args[len(command.Args)-1] == "base-index" {
				return "0", nil
			}
			return "", nil
		},
	}
	app := testApp(runner, t.TempDir())
	if err := app.restoreSnapshot(context.Background(), file); err != nil {
		t.Fatal(err)
	}
	commands := runner.Commands()
	if !commandHas(commands, "tmux", "new-window", "-d", "-t", "=work:2", "-c", dir, "-n", "shell") {
		t.Fatalf("snapshot window was not created: %#v", commands)
	}
	if !commandHas(commands, "tmux", "kill-window", "-t", "=work:0") {
		t.Fatalf("initial base window was not killed: %#v", commands)
	}
}

func TestRestoreIsolationAndExistingOrMissingSessions(t *testing.T) {
	dir := t.TempDir()
	snapshot := Snapshot{
		Version: 1,
		Sessions: []Session{
			{Name: "existing", Path: dir, Attached: true, Windows: []Window{{Index: 0, Path: dir}}},
			{Name: "gone", Path: filepath.Join(dir, "gone"), Windows: []Window{{Index: 0, Path: dir}}},
			{Name: "broken", Path: dir, Windows: []Window{{Index: 0, Path: dir}, {Index: 2, Path: dir}}},
			{Name: "later", Path: dir, Windows: []Window{{Index: 0, Path: dir}}},
		},
	}
	file := writeTestSnapshot(t, snapshot)
	runner := &fakeRunner{
		handler: func(command Command) (string, error) {
			if command.Args[0] == "has-session" {
				if strings.Contains(command.Args[2], "existing") {
					return "", nil
				}
				return "", errors.New("missing")
			}
			if command.Args[0] == "respawn-window" && strings.Contains(command.Args[3], "broken") {
				return "", errors.New("window failure")
			}
			return "", nil
		},
	}
	app := testApp(runner, t.TempDir())
	err := app.restoreSnapshot(context.Background(), file)
	if err == nil {
		t.Fatal("restore unexpectedly succeeded")
	}
	commands := runner.Commands()
	if commandHas(commands, "tmux", "new-session", "-d", "-s", "existing", "-c", dir) {
		t.Fatal("existing session was recreated")
	}
	if !commandHas(commands, "tmux", "new-session", "-d", "-s", "later", "-c", dir) {
		t.Fatal("later session was not restored after failure")
	}
	if !commandHas(commands, "tmux", "kill-session", "-t", "=broken") {
		t.Fatal("failed session was not rolled back")
	}
	if commandHas(commands, "tmux", "attach-session", "-t", "=existing") {
		t.Fatal("existing attached session was unexpectedly attached")
	}
}

func TestRestoreSwitchesInsideTmuxAndAttachesOutside(t *testing.T) {
	dir := t.TempDir()
	snapshot := Snapshot{
		Version:  1,
		Sessions: []Session{{Name: "work", Path: dir, Attached: true, Windows: []Window{{Index: 0, Path: dir}}}},
	}
	file := writeTestSnapshot(t, snapshot)
	for _, test := range []struct {
		name     string
		tmux     string
		expected []string
		attached bool
	}{
		{name: "inside", tmux: "/tmp/client,1", expected: []string{"switch-client", "-t", "=work"}},
		{name: "outside", expected: []string{"attach-session", "-t", "=work"}, attached: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("TMUX", test.tmux)
			if !test.attached {
				t.Setenv("TMUX", "/tmp/client,1")
			} else {
				t.Setenv("TMUX", "")
			}
			runner := &fakeRunner{
				handler: func(command Command) (string, error) {
					if command.Args[0] == "has-session" {
						return "", errors.New("missing")
					}
					return "", nil
				},
			}
			app := testApp(runner, t.TempDir())
			if err := app.restoreSnapshot(context.Background(), file); err != nil {
				t.Fatal(err)
			}
			commands := runner.Commands()
			if !commandHas(commands, "tmux", test.expected...) {
				t.Fatalf("missing final client command: %#v", commands)
			}
			if test.attached {
				for _, command := range commands {
					if command.Args[0] == "attach-session" && !command.Interactive {
						t.Fatal("attach-session was not interactive")
					}
				}
			}
		})
	}
}

func TestCLIUsageAndErrors(t *testing.T) {
	app := testApp(&fakeRunner{}, t.TempDir())
	for _, args := range [][]string{
		nil,
		{"save", "one", "two"},
		{"restore", "one", "two"},
		{"bogus"},
	} {
		if code := app.Main(context.Background(), args); code != 2 {
			t.Fatalf("args %#v: code %d, want 2", args, code)
		}
	}
}

func writeTestSnapshot(t *testing.T, snapshot Snapshot) string {
	t.Helper()
	data, err := encodeSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "snapshot.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}
