package tmuxsnapshot

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Command struct {
	Name        string
	Args        []string
	Input       io.Reader
	Stderr      io.Writer
	Interactive bool
}

type Runner interface {
	Run(context.Context, Command) (string, error)
}

type OSRunner struct{}

func (OSRunner) Run(ctx context.Context, command Command) (string, error) {
	cmd := exec.CommandContext(ctx, command.Name, command.Args...)
	cmd.Stdin = command.Input
	cmd.Stderr = command.Stderr
	if command.Interactive {
		cmd.Stdout = os.Stdout
		err := cmd.Run()
		return "", err
	}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	return stdout.String(), err
}

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

type App struct {
	Runner Runner
	Clock  Clock
	Home   string

	Stdout io.Writer
	Stderr io.Writer
}

func NewApp() *App {
	home, _ := os.UserHomeDir()
	return &App{
		Runner: OSRunner{},
		Clock:  RealClock{},
		Home:   home,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

func (a *App) run(ctx context.Context, name string, args ...string) (string, error) {
	return a.runWithStderr(ctx, a.Stderr, name, args...)
}

func (a *App) runQuiet(ctx context.Context, name string, args ...string) (string, error) {
	return a.runWithStderr(ctx, nil, name, args...)
}

func (a *App) runWithStderr(ctx context.Context, stderr io.Writer, name string, args ...string) (string, error) {
	return a.Runner.Run(ctx, Command{Name: name, Args: args, Stderr: stderr})
}

func trimOutput(output string) string {
	return strings.TrimSuffix(strings.TrimSuffix(output, "\n"), "\r")
}
