package tmuxsnapshot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
)

func (a *App) tmuxValue(ctx context.Context, args ...string) string {
	output, err := a.runQuiet(ctx, "tmux", args...)
	if err != nil {
		return ""
	}
	return trimOutput(output)
}

func (a *App) tmuxOK(ctx context.Context, args ...string) error {
	_, err := a.run(ctx, "tmux", args...)
	return err
}

func (a *App) tmuxQuietOK(ctx context.Context, args ...string) error {
	_, err := a.runQuiet(ctx, "tmux", args...)
	return err
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func parseBaseIndex(value string) int {
	index, err := strconv.Atoi(value)
	if err != nil || index < 0 {
		return 0
	}
	return index
}

func normalizeRenumber(value string) string {
	if value == "off" {
		return "off"
	}
	return "on"
}

func (a *App) restoreSession(ctx context.Context, session Session) (bool, error) {
	if len(session.Windows) == 0 {
		return false, fmt.Errorf("session %q has no windows, skipping", session.Name)
	}
	if a.tmuxQuietOK(ctx, "has-session", "-t", "="+session.Name) == nil {
		return false, nil
	}
	if !directoryExists(session.Path) {
		fmt.Fprintf(a.Stderr, "tmux-snapshot: session %q path does not exist, skipping: %s\n", session.Name, session.Path)
		return false, nil
	}

	if err := a.tmuxOK(ctx, "new-session", "-d", "-s", session.Name, "-c", session.Path); err != nil {
		return false, fmt.Errorf("could not create session %q", session.Name)
	}
	rollback := func(message string) (bool, error) {
		_ = a.tmuxOK(ctx, "kill-session", "-t", "="+session.Name)
		return false, fmt.Errorf("%s", message)
	}

	baseIndex := parseBaseIndex(a.tmuxValue(ctx, "show-option", "-gqv", "base-index"))
	renumber := a.tmuxValue(ctx, "show-option", "-t", session.Name, "-qv", "renumber-windows")
	if renumber == "" {
		renumber = a.tmuxValue(ctx, "show-option", "-gqv", "renumber-windows")
	}
	renumber = normalizeRenumber(renumber)
	if err := a.tmuxOK(ctx, "set-option", "-t", session.Name, "renumber-windows", "off"); err != nil {
		return rollback(fmt.Sprintf("could not prepare session %q", session.Name))
	}

	activeIndex := session.Windows[0].Index
	baseOccupied := false
	for _, window := range session.Windows {
		if window.Active {
			activeIndex = window.Index
		}
		if window.Index == baseIndex {
			baseOccupied = true
		}
	}

	for _, window := range session.Windows {
		path := window.Path
		if !directoryExists(path) {
			path = session.Path
		}
		target := fmt.Sprintf("=%s:%d", session.Name, window.Index)
		var err error
		if window.Index == baseIndex {
			err = a.tmuxOK(ctx, "respawn-window", "-k", "-t", target, "-c", path)
			if err == nil && window.ManualName {
				err = a.tmuxOK(ctx, "rename-window", "-t", target, window.Name)
			}
		} else {
			args := []string{"new-window", "-d", "-t", target, "-c", path}
			if window.ManualName {
				args = append(args, "-n", window.Name)
			}
			err = a.tmuxOK(ctx, args...)
		}
		if err != nil {
			return rollback(fmt.Sprintf("could not restore window %d in %q", window.Index, session.Name))
		}
	}

	if !baseOccupied {
		target := fmt.Sprintf("=%s:%d", session.Name, baseIndex)
		if err := a.tmuxOK(ctx, "kill-window", "-t", target); err != nil {
			return rollback(fmt.Sprintf("could not remove the initial window in %q", session.Name))
		}
	}
	if err := a.tmuxOK(ctx, "set-option", "-t", session.Name, "renumber-windows", renumber); err != nil {
		return rollback(fmt.Sprintf("could not restore session options in %q", session.Name))
	}
	if err := a.tmuxOK(ctx, "select-window", "-t", fmt.Sprintf("=%s:%d", session.Name, activeIndex)); err != nil {
		return rollback(fmt.Sprintf("could not select the active window in %q", session.Name))
	}
	return true, nil
}

func (a *App) restoreSnapshot(ctx context.Context, path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("snapshot does not exist: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}
	snapshot, err := decodeSnapshot(data)
	if err != nil {
		return err
	}

	var restoreErr error
	attachedSession := ""
	for _, session := range snapshot.Sessions {
		restored, err := a.restoreSession(ctx, session)
		if err != nil {
			fmt.Fprintf(a.Stderr, "tmux-snapshot: %s\n", err)
			restoreErr = fmt.Errorf("one or more sessions could not be restored")
		}
		if restored && session.Attached {
			attachedSession = session.Name
		}
	}

	if attachedSession != "" {
		var err error
		if os.Getenv("TMUX") != "" {
			err = a.tmuxOK(ctx, "switch-client", "-t", "="+attachedSession)
		} else {
			_, err = a.Runner.Run(ctx, Command{
				Name:        "tmux",
				Args:        []string{"attach-session", "-t", "=" + attachedSession},
				Input:       os.Stdin,
				Stderr:      a.Stderr,
				Interactive: true,
			})
		}
		if err != nil {
			fmt.Fprintf(a.Stderr, "tmux-snapshot: could not attach to session %q\n", attachedSession)
			restoreErr = fmt.Errorf("could not attach to restored session")
		}
	}
	return restoreErr
}
