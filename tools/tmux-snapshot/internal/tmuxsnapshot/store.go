package tmuxsnapshot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (a *App) stateDir() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		base = filepath.Join(a.Home, ".local", "state")
	}
	return filepath.Join(base, "tmux-snapshot")
}

func ensureStateDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create snapshot directory: %w", err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("set snapshot directory permissions: %w", err)
	}
	return nil
}

func (a *App) nextSnapshotPath() (string, error) {
	dir := a.stateDir()
	if err := ensureStateDir(dir); err != nil {
		return "", err
	}

	timestamp := a.Clock.Now().UTC().Format("20060102T150405Z")
	for suffix := 0; ; suffix++ {
		name := timestamp + ".json"
		if suffix > 0 {
			name = fmt.Sprintf("%s-%d.json", timestamp, suffix)
		}
		path := filepath.Join(dir, name)
		_, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			return path, nil
		}
		if err != nil {
			return "", fmt.Errorf("check snapshot path %s: %w", path, err)
		}
	}
}

func writeAtomic(path string, data []byte) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create snapshot directory: %w", err)
	}

	temp, err := os.CreateTemp(parent, ".tmux-snapshot-*")
	if err != nil {
		return fmt.Errorf("create temporary snapshot: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("set temporary snapshot permissions: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write snapshot: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync snapshot: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary snapshot: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replace snapshot: %w", err)
	}
	return nil
}

func updateLatest(stateDir, snapshotPath string) error {
	latest := filepath.Join(stateDir, "latest")
	if info, err := os.Lstat(latest); err == nil {
		if info.IsDir() {
			return fmt.Errorf("snapshot latest path is a directory: %s", latest)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect latest snapshot link: %w", err)
	}

	temp, err := os.CreateTemp(stateDir, ".latest-*")
	if err != nil {
		return fmt.Errorf("create temporary latest link: %w", err)
	}
	tempName := temp.Name()
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("close temporary latest link: %w", err)
	}
	if err := os.Remove(tempName); err != nil {
		return fmt.Errorf("prepare temporary latest link: %w", err)
	}
	defer os.Remove(tempName)

	if err := os.Symlink(filepath.Base(snapshotPath), tempName); err != nil {
		return fmt.Errorf("create latest snapshot link: %w", err)
	}
	if err := os.Rename(tempName, latest); err != nil {
		return fmt.Errorf("update latest snapshot link: %w", err)
	}
	return nil
}

func (a *App) writeSnapshot(requested string, data []byte) (string, error) {
	path := requested
	auto := path == ""
	if auto {
		var err error
		path, err = a.nextSnapshotPath()
		if err != nil {
			return "", err
		}
	}
	if err := writeAtomic(path, data); err != nil {
		return "", err
	}
	if auto {
		if err := updateLatest(a.stateDir(), path); err != nil {
			return "", err
		}
	}
	return path, nil
}

func (a *App) resolveSnapshot(requested string) (string, error) {
	if requested != "" {
		return requested, nil
	}

	dir := a.stateDir()
	latest := filepath.Join(dir, "latest")
	if info, err := os.Stat(latest); err == nil {
		if !info.IsDir() {
			return latest, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect latest snapshot: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("no snapshots were found in %s", dir)
	}
	if err != nil {
		return "", fmt.Errorf("read snapshot directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return "", fmt.Errorf("inspect snapshot %s: %w", entry.Name(), err)
		}
		if info.Mode().IsRegular() {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no snapshots were found in %s", dir)
	}
	sort.Strings(names)
	return filepath.Join(dir, names[len(names)-1]), nil
}
