package tmuxsnapshot

import (
	"context"
	"errors"
	"fmt"
	"os"
)

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

func (a *App) restoreSession(ctx context.Context, session Session) (bool, error) {
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
