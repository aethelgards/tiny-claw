// Package observability provides data models for the visualization
// and observability system of tiny-claw agent sessions.
package observability

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Layout of the on-disk trace store, rooted at the storage data directory:
//
//	{dataDir}/sessions/{session_id}.json        one file per session
//	{dataDir}/spans/{session_id}/spans.json     all trace spans for a session
//	{dataDir}/tools/{session_id}/tool_calls.json all tool call records for a session
const (
	sessionsDir = "sessions"
	spansDir    = "spans"
	toolsDir    = "tools"

	spansFile     = "spans.json"
	toolCallsFile = "tool_calls.json"
)

// Storage handles persistence of observability data as JSON files
// under a single data directory.
type Storage struct {
	dataDir string

	// mu serializes read-modify-write cycles on the append-only files
	// (spans.json, tool_calls.json) within this process.
	mu sync.Mutex
}

// NewStorage creates a new storage instance rooted at dataDir.
func NewStorage(dataDir string) *Storage {
	return &Storage{dataDir: dataDir}
}

// SaveSession persists a session to disk as an individual JSON file,
// overwriting any previous version of the same session ID.
func (s *Storage) SaveSession(session SessionData) error {
	if session.ID == "" {
		return errors.New("observability: session ID must not be empty")
	}
	path := s.sessionPath(session.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("observability: create sessions dir: %w", err)
	}
	if err := writeJSONAtomic(path, session); err != nil {
		return fmt.Errorf("observability: save session %s: %w", session.ID, err)
	}
	return nil
}

// LoadSession loads a session from disk by ID. It returns an error if the
// session does not exist or cannot be decoded.
func (s *Storage) LoadSession(id string) (*SessionData, error) {
	if id == "" {
		return nil, errors.New("observability: session ID must not be empty")
	}
	raw, err := os.ReadFile(s.sessionPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("observability: session %q not found", id)
		}
		return nil, fmt.Errorf("observability: read session %s: %w", id, err)
	}
	var session SessionData
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, fmt.Errorf("observability: decode session %s: %w", id, err)
	}
	return &session, nil
}

// ListSessions returns a paginated list of sessions ordered by creation
// time, most recent first. An offset beyond the end of the list yields an
// empty result; limit <= 0 means no limit.
func (s *Storage) ListSessions(offset, limit int) ([]SessionData, error) {
	dir := filepath.Join(s.dataDir, sessionsDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SessionData{}, nil
		}
		return nil, fmt.Errorf("observability: list sessions dir: %w", err)
	}

	sessions := make([]SessionData, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("observability: read session file %s: %w", entry.Name(), err)
		}
		var session SessionData
		if err := json.Unmarshal(raw, &session); err != nil {
			return nil, fmt.Errorf("observability: decode session file %s: %w", entry.Name(), err)
		}
		sessions = append(sessions, session)
	}

	sort.SliceStable(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	if offset < 0 {
		offset = 0
	}
	if offset >= len(sessions) {
		return []SessionData{}, nil
	}
	sessions = sessions[offset:]
	if limit > 0 && limit < len(sessions) {
		sessions = sessions[:limit]
	}
	return sessions, nil
}

// SaveTrace appends a trace entry to the span log of its session.
func (s *Storage) SaveTrace(trace TraceEntry) error {
	if trace.SessionID == "" {
		return errors.New("observability: trace session ID must not be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := loadJSONFile[TraceEntry](s.spansPath(trace.SessionID))
	if err != nil {
		return err
	}
	return saveJSONFile(s.spansPath(trace.SessionID), append(existing, trace), "save trace")
}

// LoadTraces loads all trace entries recorded for a session, in insertion
// order. A session without traces yields an empty slice.
func (s *Storage) LoadTraces(sessionID string) ([]TraceEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return loadJSONFile[TraceEntry](s.spansPath(sessionID))
}

// SaveToolCall appends a tool call record to the tool call log of its session.
func (s *Storage) SaveToolCall(record ToolCallRecord) error {
	if record.SessionID == "" {
		return errors.New("observability: tool call session ID must not be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := loadJSONFile[ToolCallRecord](s.toolCallsPath(record.SessionID))
	if err != nil {
		return err
	}
	return saveJSONFile(s.toolCallsPath(record.SessionID), append(existing, record), "save tool call")
}

// LoadToolCalls loads all tool call records for a session, in insertion
// order. A session without tool calls yields an empty slice.
func (s *Storage) LoadToolCalls(sessionID string) ([]ToolCallRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return loadJSONFile[ToolCallRecord](s.toolCallsPath(sessionID))
}

func (s *Storage) sessionPath(id string) string {
	return filepath.Join(s.dataDir, sessionsDir, id+".json")
}

func (s *Storage) spansPath(sessionID string) string {
	return filepath.Join(s.dataDir, spansDir, sessionID, spansFile)
}

func (s *Storage) toolCallsPath(sessionID string) string {
	return filepath.Join(s.dataDir, toolsDir, sessionID, toolCallsFile)
}

// loadJSONFile reads a JSON array file into a slice. A missing file is not
// an error: it yields an empty slice so that first writes and queries on
// unknown sessions behave uniformly.
func loadJSONFile[T any](path string) ([]T, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []T{}, nil
		}
		return nil, fmt.Errorf("observability: read %s: %w", path, err)
	}
	var out []T
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("observability: decode %s: %w", path, err)
	}
	return out, nil
}

// saveJSONFile marshals v and atomically replaces path with it.
func saveJSONFile[T any](path string, v T, action string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("observability: create dir for %s: %w", action, err)
	}
	if err := writeJSONAtomic(path, v); err != nil {
		return fmt.Errorf("observability: %s: %w", action, err)
	}
	return nil
}

// writeJSONAtomic writes v as indented JSON via a temp file in the target
// directory followed by a rename, so readers never observe partial content.
func writeJSONAtomic(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	raw = append(raw, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
