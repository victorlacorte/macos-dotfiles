package tmuxsnapshot

import (
	"fmt"
	"os"
	"path/filepath"
)

func (a *App) snapshotPath() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		base = filepath.Join(a.Home, ".local", "state")
	}
	return filepath.Join(base, "tmux-snapshot", "latest")
}

func (a *App) writeSnapshot(data []byte) (string, error) {
	path := a.snapshotPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create snapshot directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", fmt.Errorf("set snapshot directory permissions: %w", err)
	}

	temp, err := os.CreateTemp(dir, ".tmux-snapshot-*")
	if err != nil {
		return "", fmt.Errorf("create temporary snapshot: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return "", fmt.Errorf("write snapshot: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return "", fmt.Errorf("sync snapshot: %w", err)
	}
	if err := temp.Close(); err != nil {
		return "", fmt.Errorf("close temporary snapshot: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return "", fmt.Errorf("replace snapshot: %w", err)
	}
	return path, nil
}
