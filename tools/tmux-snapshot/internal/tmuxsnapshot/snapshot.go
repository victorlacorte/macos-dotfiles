package tmuxsnapshot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

const snapshotVersion = 1

type Snapshot struct {
	Version  int       `json:"version"`
	Sessions []Session `json:"sessions"`
}

type Session struct {
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	Attached bool     `json:"attached"`
	Windows  []Window `json:"windows"`
}

type Window struct {
	Index      int    `json:"index"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Active     bool   `json:"active"`
	ManualName bool   `json:"manualName"`
}

func (s Snapshot) Validate() error {
	if s.Version != snapshotVersion {
		return fmt.Errorf("unsupported snapshot version %d", s.Version)
	}
	if len(s.Sessions) == 0 {
		return fmt.Errorf("snapshot contains no sessions")
	}

	sessionNames := make(map[string]struct{}, len(s.Sessions))
	for i, session := range s.Sessions {
		if session.Name == "" {
			return fmt.Errorf("session %d has an empty name", i)
		}
		if session.Path == "" {
			return fmt.Errorf("session %q has an empty path", session.Name)
		}
		if _, exists := sessionNames[session.Name]; exists {
			return fmt.Errorf("duplicate session %q", session.Name)
		}
		sessionNames[session.Name] = struct{}{}

		windowIndexes := make(map[int]struct{}, len(session.Windows))
		for j, window := range session.Windows {
			if window.Index < 0 {
				return fmt.Errorf("session %q window %d has a negative index", session.Name, j)
			}
			if window.Path == "" {
				return fmt.Errorf("session %q window %d has an empty path", session.Name, window.Index)
			}
			if _, exists := windowIndexes[window.Index]; exists {
				return fmt.Errorf("session %q has duplicate window index %d", session.Name, window.Index)
			}
			windowIndexes[window.Index] = struct{}{}
		}
	}
	return nil
}

func encodeSnapshot(snapshot Snapshot) ([]byte, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode snapshot: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

func decodeSnapshot(data []byte) (Snapshot, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var snapshot Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Snapshot{}, fmt.Errorf("decode snapshot: multiple JSON values")
		}
		return Snapshot{}, fmt.Errorf("decode snapshot: trailing data: %w", err)
	}
	if err := snapshot.Validate(); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}
