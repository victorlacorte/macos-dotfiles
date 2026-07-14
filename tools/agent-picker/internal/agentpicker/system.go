package agentpicker

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Command struct {
	Name   string
	Args   []string
	Input  io.Reader
	Env    map[string]string
	Stderr io.Writer
}

type Runner interface {
	LookPath(string) (string, error)
	Run(context.Context, Command) (string, error)
}

type FileSystem interface {
	Glob(string) ([]string, error)
	Stat(string) (os.FileInfo, error)
}

type Clock interface {
	Now() time.Time
}

type OSRunner struct{}

func (OSRunner) LookPath(name string) (string, error) { return exec.LookPath(name) }

func (OSRunner) Run(ctx context.Context, command Command) (string, error) {
	cmd := exec.CommandContext(ctx, command.Name, command.Args...)
	var stdout bytes.Buffer
	cmd.Stdin = command.Input
	cmd.Stdout = &stdout
	cmd.Stderr = command.Stderr
	if len(command.Env) > 0 {
		env := make([]string, 0, len(os.Environ())+len(command.Env))
		for _, entry := range os.Environ() {
			key, _, _ := strings.Cut(entry, "=")
			if _, overridden := command.Env[key]; !overridden {
				env = append(env, entry)
			}
		}
		for key, value := range command.Env {
			env = append(env, key+"="+value)
		}
		cmd.Env = env
	}
	err := cmd.Run()
	return stdout.String(), err
}

type OSFileSystem struct{}

func (OSFileSystem) Glob(pattern string) ([]string, error) { return filepath.Glob(pattern) }
func (OSFileSystem) Stat(name string) (os.FileInfo, error) { return os.Stat(name) }

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

type App struct {
	Runner     Runner
	FS         FileSystem
	Clock      Clock
	Home       string
	Executable string
	Stdout     io.Writer
	Stderr     io.Writer
}

func NewApp() *App {
	home, _ := os.UserHomeDir()
	executable, _ := os.Executable()
	return &App{
		Runner:     OSRunner{},
		FS:         OSFileSystem{},
		Clock:      RealClock{},
		Home:       home,
		Executable: executable,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}
}

func (a *App) run(ctx context.Context, name string, args ...string) (string, error) {
	return a.Runner.Run(ctx, Command{Name: name, Args: args})
}

func (a *App) tmux(ctx context.Context, args ...string) string {
	out, err := a.run(ctx, "tmux", args...)
	if err != nil {
		return ""
	}
	return trimLine(out)
}
