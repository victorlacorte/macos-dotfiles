package tmuxsnapshot

import (
	"context"
	"errors"
	"fmt"
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

func TestSaveUsesOneTmuxCallAndPreservesSessionData(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	mainPath := filepath.Join(stateHome, "main\tproject\npath")
	alphaPath := filepath.Join(stateHome, "alpha")
	runner := &fakeRunner{
		handler: func(command Command) (string, error) {
			if command.Name != "tmux" || len(command.Args) == 0 || command.Args[0] != "list-sessions" {
				return "", fmt.Errorf("unexpected tmux command: %#v", command)
			}
			return tmuxOutput(
				[]string{"older", "/tmp/older", "0"},
				[]string{"main", mainPath, "2"},
				[]string{"alpha", alphaPath, "1"},
			), nil
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
		Version: 2,
		Sessions: []Session{
			{Name: "older", Path: "/tmp/older"},
			{Name: "main", Path: mainPath, Attached: true},
			{Name: "alpha", Path: alphaPath, Attached: true},
		},
	}
	if !reflect.DeepEqual(snapshot, want) {
		t.Fatalf("snapshot mismatch:\n got: %#v\nwant: %#v", snapshot, want)
	}
	if strings.Contains(string(data), `"windows"`) {
		t.Fatalf("snapshot unexpectedly contains window data: %s", data)
	}

	commands := runner.Commands()
	if commandCount(commands, "tmux") != 1 {
		t.Fatalf("save made %d tmux calls, want 1: %#v", commandCount(commands, "tmux"), commands)
	}
	if len(commands) != 1 || !reflect.DeepEqual(commands[0].Args, []string{
		"list-sessions", "-O", "index", "-F",
		"#{session_name}" + unitSeparator + "#{session_path}" + unitSeparator + "#{session_attached}" + recordSeparator,
	}) {
		t.Fatalf("save used unexpected commands: %#v", commands)
	}
	if mode := fileMode(t, filepath.Dir(path)); mode != 0o700 {
		t.Fatalf("state directory mode: got %o, want 700", mode)
	}
	if mode := fileMode(t, path); mode != 0o600 {
		t.Fatalf("snapshot mode: got %o, want 600", mode)
	}
	target, err := os.Readlink(filepath.Join(filepath.Dir(path), "latest"))
	if err != nil {
		t.Fatal(err)
	}
	if target != filepath.Base(path) {
		t.Fatalf("latest target: got %q, want %q", target, filepath.Base(path))
	}
}

func TestSaveRejectsInvalidSessionAttachedCount(t *testing.T) {
	for _, value := range []string{"-1", "many"} {
		t.Run(value, func(t *testing.T) {
			stateHome := t.TempDir()
			t.Setenv("XDG_STATE_HOME", stateHome)
			runner := &fakeRunner{
				handler: func(command Command) (string, error) {
					return tmuxOutput([]string{"work", "/tmp", value}), nil
				},
			}
			app := testApp(runner, t.TempDir())
			if _, err := app.Save(context.Background(), ""); err == nil {
				t.Fatal("save unexpectedly succeeded")
			}
		})
	}
}

func TestSaveCollisionSuffixAndDefaultResolution(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	runner := &fakeRunner{
		handler: func(command Command) (string, error) {
			if command.Args[0] != "list-sessions" {
				return "", fmt.Errorf("unexpected tmux command: %#v", command)
			}
			return tmuxOutput([]string{"work", "/tmp", "0"}), nil
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
	if commandCount(runner.Commands(), "tmux") != 2 {
		t.Fatalf("save made an unexpected number of tmux calls: %#v", runner.Commands())
	}
}

func TestSnapshotValidation(t *testing.T) {
	valid := `{"version":2,"sessions":[{"name":"s","path":"/tmp","attached":false}]}`
	tests := []struct {
		name    string
		data    string
		wantErr string
	}{
		{name: "malformed json", data: `{`, wantErr: "decode snapshot"},
		{name: "version 1", data: `{"version":1,"sessions":[{"name":"s","path":"/tmp","attached":false}]}`, wantErr: "unsupported snapshot version 1"},
		{name: "version 1 legacy windows", data: `{"version":1,"sessions":[{"name":"s","path":"/tmp","attached":false,"windows":[]}]}`, wantErr: "unsupported snapshot version 1"},
		{name: "zero sessions", data: `{"version":2,"sessions":[]}`, wantErr: "snapshot contains no sessions"},
		{name: "unknown top-level field", data: `{"version":2,"sessions":[],"extra":true}`, wantErr: "decode snapshot"},
		{name: "legacy windows field", data: `{"version":2,"sessions":[{"name":"s","path":"/tmp","attached":false,"windows":[]}]}`, wantErr: "decode snapshot"},
		{name: "trailing value", data: valid + ` {}`, wantErr: "multiple JSON values"},
		{name: "empty name", data: `{"version":2,"sessions":[{"name":"","path":"/tmp","attached":false}]}`, wantErr: "empty name"},
		{name: "empty path", data: `{"version":2,"sessions":[{"name":"s","path":"","attached":false}]}`, wantErr: "empty path"},
		{name: "duplicate session name", data: `{"version":2,"sessions":[{"name":"s","path":"/tmp","attached":false},{"name":"s","path":"/var","attached":true}]}`, wantErr: "duplicate session"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeSnapshot([]byte(tt.data))
			if err == nil {
				t.Fatal("decode unexpectedly succeeded")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error: got %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestRestoreCreatesOneDefaultWindowPerMissingSession(t *testing.T) {
	dir := t.TempDir()
	snapshot := Snapshot{
		Version:  2,
		Sessions: []Session{{Name: "work", Path: dir}},
	}
	file := writeTestSnapshot(t, snapshot)
	runner := &fakeRunner{
		handler: func(command Command) (string, error) {
			if command.Args[0] == "has-session" {
				return "", errors.New("missing")
			}
			if command.Args[0] == "new-session" {
				return "", nil
			}
			return "", fmt.Errorf("unexpected tmux command: %#v", command)
		},
	}
	app := testApp(runner, t.TempDir())
	if err := app.restoreSnapshot(context.Background(), file); err != nil {
		t.Fatal(err)
	}
	commands := runner.Commands()
	if !commandHas(commands, "tmux", "new-session", "-d", "-s", "work", "-c", dir) {
		t.Fatalf("session was not created with the recorded directory: %#v", commands)
	}
	if commandCount(commands, "tmux") != 2 || len(commands) != 2 {
		t.Fatalf("restore made unexpected commands: %#v", commands)
	}
	for _, command := range commands {
		if len(command.Args) > 0 && (strings.Contains(command.Args[0], "window") || strings.Contains(command.Args[0], "option")) {
			t.Fatalf("restore issued a window or option command: %#v", command)
		}
	}
}

func TestRestoreCreatesMissingSessionsInSnapshotOrder(t *testing.T) {
	dir := t.TempDir()
	snapshot := Snapshot{
		Version: 2,
		Sessions: []Session{
			{Name: "zebra", Path: dir},
			{Name: "alpha", Path: dir},
			{Name: "middle", Path: dir},
		},
	}
	file := writeTestSnapshot(t, snapshot)
	for _, test := range []struct {
		name     string
		existing string
		want     []string
	}{
		{name: "empty server", want: []string{"zebra", "alpha", "middle"}},
		{name: "middle session already exists", existing: "=alpha", want: []string{"zebra", "middle"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeRunner{
				handler: func(command Command) (string, error) {
					switch command.Args[0] {
					case "has-session":
						if command.Args[2] == test.existing {
							return "", nil
						}
						return "", errors.New("missing")
					case "new-session":
						return "", nil
					default:
						return "", fmt.Errorf("unexpected tmux command: %#v", command)
					}
				},
			}
			app := testApp(runner, t.TempDir())
			if err := app.restoreSnapshot(context.Background(), file); err != nil {
				t.Fatal(err)
			}

			var got []string
			for _, command := range runner.Commands() {
				if command.Args[0] == "new-session" {
					got = append(got, command.Args[3])
				}
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("new-session order: got %v, want %v", got, test.want)
			}
		})
	}
}

func TestRestoreSkipsExistingAndMissingSessionsAndContinuesAfterFailure(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "gone")
	snapshot := Snapshot{
		Version: 2,
		Sessions: []Session{
			{Name: "existing", Path: dir, Attached: true},
			{Name: "gone", Path: missing},
			{Name: "broken", Path: dir},
			{Name: "later", Path: dir},
		},
	}
	file := writeTestSnapshot(t, snapshot)
	runner := &fakeRunner{
		handler: func(command Command) (string, error) {
			switch command.Args[0] {
			case "has-session":
				if command.Args[2] == "=existing" {
					return "", nil
				}
				return "", errors.New("missing")
			case "new-session":
				if command.Args[3] == "broken" {
					return "", errors.New("creation failure")
				}
				return "", nil
			default:
				return "", fmt.Errorf("unexpected tmux command: %#v", command)
			}
		},
	}
	stderr := &strings.Builder{}
	app := testApp(runner, t.TempDir())
	app.Stderr = stderr
	t.Setenv("TMUX", "")
	if err := app.restoreSnapshot(context.Background(), file); err == nil {
		t.Fatal("restore unexpectedly succeeded")
	}
	commands := runner.Commands()
	if commandHas(commands, "tmux", "new-session", "-d", "-s", "existing", "-c", dir) {
		t.Fatal("existing session was recreated")
	}
	if commandHas(commands, "tmux", "new-session", "-d", "-s", "gone", "-c", missing) {
		t.Fatal("missing-directory session was created")
	}
	if !commandHas(commands, "tmux", "new-session", "-d", "-s", "later", "-c", dir) {
		t.Fatalf("later session was not restored after failure: %#v", commands)
	}
	for _, command := range commands {
		if command.Args[0] == "kill-session" {
			t.Fatalf("failed session was rolled back instead of left alone: %#v", commands)
		}
	}
	if !strings.Contains(stderr.String(), `session "gone" path does not exist, skipping`) {
		t.Fatalf("missing-directory warning not reported: %q", stderr.String())
	}
}

func TestRestoreAttachesLastNewlyRestoredAttachedSession(t *testing.T) {
	dir := t.TempDir()
	snapshot := Snapshot{
		Version: 2,
		Sessions: []Session{
			{Name: "first", Path: dir, Attached: true},
			{Name: "last", Path: dir, Attached: true},
		},
	}
	file := writeTestSnapshot(t, snapshot)
	for _, test := range []struct {
		name        string
		tmux        string
		command     string
		interactive bool
	}{
		{name: "inside tmux", tmux: "/tmp/client,1", command: "switch-client"},
		{name: "outside tmux", command: "attach-session", interactive: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("TMUX", test.tmux)
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
			if !commandHas(commands, "tmux", test.command, "-t", "=last") {
				t.Fatalf("missing final client command: %#v", commands)
			}
			if commandHas(commands, "tmux", "switch-client", "-t", "=first") ||
				commandHas(commands, "tmux", "attach-session", "-t", "=first") {
				t.Fatalf("attached selection did not use the last restored session: %#v", commands)
			}
			for _, command := range commands {
				if command.Args[0] == "attach-session" && command.Interactive != test.interactive {
					t.Fatalf("attach interaction: got %t, want %t", command.Interactive, test.interactive)
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
