package tmuxsnapshot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const (
	unitSeparator   = "\x1f"
	recordSeparator = "\x1e"
)

func splitTmuxRecords(output string) [][]string {
	var records [][]string
	for index, record := range strings.Split(output, recordSeparator) {
		if index > 0 {
			record = strings.TrimPrefix(record, "\r\n")
			record = strings.TrimPrefix(record, "\n")
		}
		if record == "" {
			continue
		}
		records = append(records, strings.Split(record, unitSeparator))
	}
	return records
}

func parseSessionAttached(value string) (bool, error) {
	clients, err := strconv.ParseInt(value, 10, 64)
	if err != nil || clients < 0 {
		return false, fmt.Errorf("invalid session_attached value %q", value)
	}
	return clients > 0, nil
}

func parseSavedSessions(output string) ([]Session, error) {
	records := splitTmuxRecords(output)
	sessions := make([]Session, 0, len(records))
	for _, fields := range records {
		if len(fields) != 3 {
			return nil, fmt.Errorf("session record has %d fields, want 3", len(fields))
		}
		attached, err := parseSessionAttached(fields[2])
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, Session{Name: fields[0], Path: fields[1], Attached: attached})
	}
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no tmux sessions are available to save")
	}
	return sessions, nil
}

func (a *App) captureSnapshot(ctx context.Context) (Snapshot, error) {
	sessionFormat := "#{session_name}" + unitSeparator +
		"#{session_path}" + unitSeparator +
		"#{session_attached}" + recordSeparator
	sessionOutput, err := a.runQuiet(ctx, "tmux", "list-sessions", "-O", "index", "-F", sessionFormat)
	if err != nil {
		return Snapshot{}, fmt.Errorf("no tmux server or sessions are available to save")
	}

	sessions, err := parseSavedSessions(sessionOutput)
	if err != nil {
		return Snapshot{}, err
	}

	return Snapshot{Version: snapshotVersion, Sessions: sessions}, nil
}

func (a *App) Save(ctx context.Context) (string, error) {
	snapshot, err := a.captureSnapshot(ctx)
	if err != nil {
		return "", err
	}
	data, err := encodeSnapshot(snapshot)
	if err != nil {
		return "", err
	}
	return a.writeSnapshot(data)
}
