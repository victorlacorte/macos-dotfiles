package tmuxsnapshot

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	unitSeparator   = "\x1f"
	recordSeparator = "\x1e"
)

type savedSession struct {
	session Session
	created int64
}

func splitTmuxRecords(output string) ([][]string, error) {
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
	return records, nil
}

func parseTmuxBool(value, field string) (bool, error) {
	switch value {
	case "1", "on", "yes":
		return true, nil
	case "0", "off", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid %s value %q", field, value)
	}
}

func parseSavedSessions(output string) ([]savedSession, error) {
	records, err := splitTmuxRecords(output)
	if err != nil {
		return nil, err
	}
	sessions := make([]savedSession, 0, len(records))
	names := make(map[string]struct{}, len(records))
	for _, fields := range records {
		if len(fields) != 4 {
			return nil, fmt.Errorf("session record has %d fields, want 4", len(fields))
		}
		created, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid session_created value %q", fields[0])
		}
		attached, err := parseTmuxBool(fields[3], "session_attached")
		if err != nil {
			return nil, err
		}
		if fields[1] == "" {
			return nil, fmt.Errorf("session has an empty name")
		}
		if fields[2] == "" {
			return nil, fmt.Errorf("session %q has an empty path", fields[1])
		}
		if _, exists := names[fields[1]]; exists {
			return nil, fmt.Errorf("duplicate session %q", fields[1])
		}
		names[fields[1]] = struct{}{}
		sessions = append(sessions, savedSession{
			session: Session{Name: fields[1], Path: fields[2], Attached: attached},
			created: created,
		})
	}
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no tmux sessions are available to save")
	}
	sort.SliceStable(sessions, func(i, j int) bool {
		if sessions[i].created != sessions[j].created {
			return sessions[i].created < sessions[j].created
		}
		return sessions[i].session.Name < sessions[j].session.Name
	})
	return sessions, nil
}

func parseSavedWindows(output string) (map[string][]Window, error) {
	records, err := splitTmuxRecords(output)
	if err != nil {
		return nil, err
	}
	windows := make(map[string][]Window)
	for _, fields := range records {
		if len(fields) != 6 {
			return nil, fmt.Errorf("window record has %d fields, want 6", len(fields))
		}
		if fields[0] == "" {
			return nil, fmt.Errorf("window has an empty session name")
		}
		index, err := strconv.Atoi(fields[1])
		if err != nil || index < 0 {
			return nil, fmt.Errorf("invalid window index %q", fields[1])
		}
		active, err := parseTmuxBool(fields[4], "window_active")
		if err != nil {
			return nil, err
		}
		automaticRename, err := parseTmuxBool(fields[5], "automatic-rename")
		if err != nil {
			return nil, err
		}
		if fields[3] == "" {
			return nil, fmt.Errorf("window %d in session %q has an empty path", index, fields[0])
		}
		windows[fields[0]] = append(windows[fields[0]], Window{
			Index:      index,
			Name:       fields[2],
			Path:       fields[3],
			Active:     active,
			ManualName: !automaticRename,
		})
	}
	for sessionName := range windows {
		sort.Slice(windows[sessionName], func(i, j int) bool {
			return windows[sessionName][i].Index < windows[sessionName][j].Index
		})
		seenIndexes := make(map[int]struct{}, len(windows[sessionName]))
		for _, window := range windows[sessionName] {
			if _, exists := seenIndexes[window.Index]; exists {
				return nil, fmt.Errorf("session %q has duplicate window index %d", sessionName, window.Index)
			}
			seenIndexes[window.Index] = struct{}{}
		}
	}
	return windows, nil
}

func (a *App) captureSnapshot(ctx context.Context) (Snapshot, error) {
	sessionFormat := "#{session_created}" + unitSeparator +
		"#{session_name}" + unitSeparator +
		"#{session_path}" + unitSeparator +
		"#{session_attached}" + recordSeparator
	sessionOutput, err := a.runQuiet(ctx, "tmux", "list-sessions", "-F", sessionFormat)
	if err != nil {
		return Snapshot{}, fmt.Errorf("no tmux server or sessions are available to save")
	}

	sessions, err := parseSavedSessions(sessionOutput)
	if err != nil {
		return Snapshot{}, err
	}

	windowFormat := "#{session_name}" + unitSeparator +
		"#{window_index}" + unitSeparator +
		"#{window_name}" + unitSeparator +
		"#{pane_current_path}" + unitSeparator +
		"#{window_active}" + unitSeparator +
		"#{automatic-rename}" + recordSeparator
	windowOutput, err := a.runQuiet(ctx, "tmux", "list-windows", "-a", "-F", windowFormat)
	if err != nil {
		return Snapshot{}, fmt.Errorf("could not capture tmux windows")
	}
	windows, err := parseSavedWindows(windowOutput)
	if err != nil {
		return Snapshot{}, err
	}

	snapshot := Snapshot{Version: snapshotVersion, Sessions: make([]Session, 0, len(sessions))}
	for _, saved := range sessions {
		saved.session.Windows = windows[saved.session.Name]
		if len(saved.session.Windows) == 0 {
			return Snapshot{}, fmt.Errorf("session %q has no windows", saved.session.Name)
		}
		snapshot.Sessions = append(snapshot.Sessions, saved.session)
	}
	if err := snapshot.Validate(); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func (a *App) Save(ctx context.Context, requested string) (string, error) {
	snapshot, err := a.captureSnapshot(ctx)
	if err != nil {
		return "", err
	}
	data, err := encodeSnapshot(snapshot)
	if err != nil {
		return "", err
	}
	return a.writeSnapshot(requested, data)
}
