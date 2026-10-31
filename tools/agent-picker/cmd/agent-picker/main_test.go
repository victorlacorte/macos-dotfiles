package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("AGENT_PICKER_TEST_HELPER") == "1" {
		runHelper(filepath.Base(os.Args[0]), os.Args[1:])
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runHelper(name string, args []string) {
	if log := os.Getenv("AGENT_PICKER_TEST_LOG"); log != "" {
		file, _ := os.OpenFile(log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if file != nil {
			_, _ = file.WriteString(strings.Join(args, " ") + "\n")
			_ = file.Close()
		}
	}
	joined := strings.Join(args, " ")
	switch name {
	case "tmux":
		switch joined {
		case "show-option -gqv @agent_popup_width":
			_, _ = os.Stdout.WriteString("77%")
		case "show-option -gqv @agent_popup_height":
			_, _ = os.Stdout.WriteString("66%")
		default:
			if len(args) > 0 && args[0] == "list-panes" {
				_, _ = os.Stdout.WriteString(os.Getenv("AGENT_PICKER_TEST_PANES"))
			}
		}
	case "ps":
		if strings.Contains(joined, "ppid=") {
			started := os.Getenv("AGENT_PICKER_TEST_STARTED")
			if started == "" {
				started = "Mon Jan 2 12:00:00 2006"
			}
			for _, item := range []struct{ key, tty, command string }{
				{"CLAUDE", "ttys001", "claude"}, {"CODEX", "ttys002", "codex"},
				{"CURSOR", "ttys003", os.Getenv("AGENT_PICKER_TEST_CURSOR_PROCESS")},
			} {
				pid := os.Getenv("AGENT_PICKER_TEST_" + item.key + "_PID")
				if pid != "" {
					fmt.Printf("%s 1 %s %s %s S %s %s\n", pid, pid, pid, item.tty, started, item.command)
				}
			}
		}
	case "lsof":
		_, _ = os.Stdout.WriteString(os.Getenv("AGENT_PICKER_TEST_LSOF"))
	case "jq":
		_, _ = os.Stderr.WriteString("jq must not be called\n")
		os.Exit(9)
	case "claude", "codex", "cursor-agent":
		if os.Getenv("AGENT_PICKER_TEST_AGENT") == "1" {
			time.Sleep(time.Hour)
		}
		return
	case "fzf":
		return
	}
}

func TestPopupBlackBoxWithExplicitCommand(t *testing.T) {
	tmp := t.TempDir()
	binary := filepath.Join(tmp, "agent-picker")
	buildPicker(t, binary, filepath.Join(tmp, "go-cache"))
	aliasTools(t, tmp, "tmux", "ps", "codex")
	log := filepath.Join(tmp, "tmux.log")
	command := exec.Command(binary, "popup", "blackbox")
	command.Env = append(os.Environ(),
		"PATH="+tmp+string(os.PathListSeparator)+os.Getenv("PATH"),
		"AGENT_PICKER_TEST_HELPER=1", "AGENT_PICKER_TEST_LOG="+log,
		"AGENT_PICKER_TEST_PANES=/dev/ttys002\t%2\tcodex-two\tcodex-two:1.1\t/tmp/Codex Path\n",
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run picker: %v\n%s", err, output)
	}
	contents, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	calls := string(contents)
	for _, want := range []string{
		"display-popup -c blackbox -w 77% -h 66% -E",
		"AGENT_PICKER_CLIENT='blackbox'",
		"select -provider 'all'",
	} {
		if !strings.Contains(calls, want) {
			t.Fatalf("missing %q in black-box calls:\n%s", want, calls)
		}
	}
}

func TestListBlackBoxWithProviderAliases(t *testing.T) {
	tmp := t.TempDir()
	binary := filepath.Join(tmp, "agent-picker")
	buildPicker(t, binary, filepath.Join(tmp, "go-cache"))
	aliasTools(t, tmp, "tmux", "fzf", "ps", "lsof", "jq", "codex", "claude")
	cursorVersion := filepath.Join(tmp, "share", "cursor-agent", "versions", "x")
	cursorBinary := filepath.Join(cursorVersion, "cursor-agent")
	if err := os.MkdirAll(cursorVersion, 0o755); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cursorBinary, contents, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(cursorBinary, filepath.Join(tmp, "cursor-agent")); err != nil {
		t.Fatal(err)
	}
	resolvedCursorVersion, err := filepath.EvalSymlinks(cursorVersion)
	if err != nil {
		t.Fatal(err)
	}
	claudePID := startAgentHelper(t, filepath.Join(tmp, "claude"))
	codexPID := startAgentHelper(t, filepath.Join(tmp, "codex"))
	cursorPID := startAgentHelper(t, cursorBinary)

	claudeHome := filepath.Join(tmp, "claude home")
	codexHome := filepath.Join(tmp, "codex home")
	claudeSession := filepath.Join(claudeHome, "sessions", "session.json")
	transcript := filepath.Join(claudeHome, "projects", "project", "session-id.jsonl")
	rollout := filepath.Join(codexHome, "sessions", "2026", "rollout-test.jsonl")
	chat := filepath.Join(tmp, ".cursor", "chats", "hash", "chat-id", "store.db-wal")
	for _, file := range []string{transcript, rollout, chat} {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(claudeSession), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudeSession, []byte(fmt.Sprintf(`{"pid":%d,"sessionId":"session-id","cwd":"/tmp/Claude Path","kind":"interactive"}`, claudePID)), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, "list", "--provider", "all")
	command.Env = append(os.Environ(),
		"PATH="+tmp+string(os.PathListSeparator)+os.Getenv("PATH"),
		"HOME="+tmp, "CLAUDE_CONFIG_DIR="+claudeHome, "CODEX_HOME="+codexHome,
		"CURSOR_CONFIG_DIR="+filepath.Join(tmp, ".cursor"),
		"AGENT_PICKER_TEST_HELPER=1",
		"AGENT_PICKER_TEST_PANES=/dev/ttys001\t%1\twork\twork:1.1\t/tmp/Claude Path\n/dev/ttys002\t%2\tcodex-two\tcodex-two:1.1\t/tmp/Codex Path\n/dev/ttys003\t%3\tcursor-three\tcursor-three:1.1\t/tmp/Cursor Path\n",
		fmt.Sprintf("AGENT_PICKER_TEST_LSOF=p%d\nn%s\np%d\nn%s\n", codexPID, rollout, cursorPID, chat),
		fmt.Sprintf("AGENT_PICKER_TEST_CLAUDE_PID=%d", claudePID),
		fmt.Sprintf("AGENT_PICKER_TEST_CODEX_PID=%d", codexPID),
		fmt.Sprintf("AGENT_PICKER_TEST_CURSOR_PID=%d", cursorPID),
		"AGENT_PICKER_TEST_CURSOR_PROCESS="+filepath.Join(resolvedCursorVersion, "node"),
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run list: %v\n%s", err, output)
	}
	rows := string(output)
	for _, want := range []string{
		"\tclaude\t", "\tcodex \t", "\tcursor\t   0m\t",
		"/tmp/Claude Path", "/tmp/Codex Path", "/tmp/Cursor Path",
	} {
		if !strings.Contains(rows, want) {
			t.Fatalf("missing %q in black-box rows:\n%s", want, rows)
		}
	}
}

func startAgentHelper(t *testing.T, executable string) int {
	return startAgentProcess(t, executable).Process.Pid
}

func startAgentProcess(t *testing.T, executable string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(executable)
	cmd.Env = append(os.Environ(), "AGENT_PICKER_TEST_HELPER=1", "AGENT_PICKER_TEST_AGENT=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	return cmd
}

func TestTerminateBlackBoxRechecksIdentityAndSignalsFixture(t *testing.T) {
	tmp := t.TempDir()
	binary := filepath.Join(tmp, "agent-picker")
	buildPicker(t, binary, filepath.Join(tmp, "go-cache"))
	aliasTools(t, tmp, "tmux", "ps", "lsof", "codex")
	agent := startAgentProcess(t, filepath.Join(tmp, "codex"))
	env := append(os.Environ(), "PATH="+tmp+string(os.PathListSeparator)+os.Getenv("PATH"), "AGENT_PICKER_TEST_HELPER=1",
		fmt.Sprintf("AGENT_PICKER_TEST_CODEX_PID=%d", agent.Process.Pid),
		"AGENT_PICKER_TEST_PANES=/dev/ttys002\t%2\twork\twork:1.0\t/project\n")
	list := exec.Command(binary, "list", "-provider", "codex", "-identities")
	list.Env = env
	output, err := list.CombinedOutput()
	if err != nil {
		t.Fatalf("list: %v\n%s", err, output)
	}
	fields := strings.Split(strings.TrimSuffix(string(output), "\n"), "\t")
	if len(fields) != 8 {
		t.Fatalf("unexpected row: %q", output)
	}
	args := []string{"terminate", "-provider", fields[2], "-pane", fields[0], "-pid", fields[1], "-identity", fields[7]}
	stale := exec.Command(binary, args...)
	stale.Env = append(append([]string{}, env...), "AGENT_PICKER_TEST_STARTED=Mon Jan 2 12:00:01 2006")
	if output, err := stale.CombinedOutput(); err == nil || !strings.Contains(string(output), "session changed") {
		t.Fatalf("stale identity: %v\n%s", err, output)
	}
	if err := agent.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("stale identity signalled fixture: %v", err)
	}
	terminate := exec.Command(binary, args...)
	terminate.Env = env
	if output, err := terminate.CombinedOutput(); err != nil {
		t.Fatalf("terminate: %v\n%s", err, output)
	}
	done := make(chan error, 1)
	go func() { done <- agent.Wait() }()
	select {
	case err := <-done:
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGTERM {
			t.Fatalf("fixture did not receive SIGTERM: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("fixture did not exit")
	}
}

func buildPicker(t *testing.T, binary, cache string) {
	t.Helper()
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Env = append(os.Environ(), "GOCACHE="+cache, "GOENV=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build agent-picker: %v\n%s", err, output)
	}
}

func aliasTools(t *testing.T, directory string, names ...string) {
	t.Helper()
	for _, name := range names {
		if err := os.Symlink(os.Args[0], filepath.Join(directory, name)); err != nil {
			t.Fatal(err)
		}
	}
}
