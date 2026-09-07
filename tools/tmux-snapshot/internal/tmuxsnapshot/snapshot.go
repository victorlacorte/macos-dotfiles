package tmuxsnapshot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

const snapshotVersion = 2

type Snapshot struct {
	Version  int       `json:"version"`
	Sessions []Session `json:"sessions"`
}

type Session struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Attached bool   `json:"attached"`
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
	versionDecoder := json.NewDecoder(bytes.NewReader(data))
	var version struct {
		Version int `json:"version"`
	}
	if err := versionDecoder.Decode(&version); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot: %w", err)
	}
	if version.Version != snapshotVersion {
		return Snapshot{}, fmt.Errorf("unsupported snapshot version %d", version.Version)
	}

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
