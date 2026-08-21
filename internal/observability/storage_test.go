package observability

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	return NewStorage(t.TempDir())
}

func testSession(id string, createdAt time.Time) SessionData {
	return SessionData{
		ID:        id,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		Prompt:    "prompt for " + id,
		Model:     "glm-4.6",
		Status:    StatusCompleted,
		Turns:     3,
		TotalTokens: TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
		TotalCost:  0.0123,
		DurationMS: 4567,
		Tags:       []string{"test", id},
		Summary:    "summary of " + id,
	}
}

func TestStorageSaveAndLoadSession(t *testing.T) {
	s := newTestStorage(t)

	session := testSession("sess-001", time.Now().UTC())
	if err := s.SaveSession(session); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}

	got, err := s.LoadSession("sess-001")
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if got.ID != session.ID {
		t.Errorf("ID = %q, want %q", got.ID, session.ID)
	}
	if got.Prompt != session.Prompt {
		t.Errorf("Prompt = %q, want %q", got.Prompt, session.Prompt)
	}
	if got.Model != session.Model {
		t.Errorf("Model = %q, want %q", got.Model, session.Model)
	}
	if got.Status != session.Status {
		t.Errorf("Status = %q, want %q", got.Status, session.Status)
	}
	if got.Turns != session.Turns {
		t.Errorf("Turns = %d, want %d", got.Turns, session.Turns)
	}
	if got.TotalTokens.TotalTokens != session.TotalTokens.TotalTokens {
		t.Errorf("TotalTokens = %d, want %d", got.TotalTokens.TotalTokens, session.TotalTokens.TotalTokens)
	}
	if got.TotalCost != session.TotalCost {
		t.Errorf("TotalCost = %f, want %f", got.TotalCost, session.TotalCost)
	}
	if !got.CreatedAt.Equal(session.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, session.CreatedAt)
	}

	// Overwrite should work (upsert semantics).
	session.Status = StatusFailed
	if err := s.SaveSession(session); err != nil {
		t.Fatalf("SaveSession(overwrite) error = %v", err)
	}
	got, err = s.LoadSession("sess-001")
	if err != nil {
		t.Fatalf("LoadSession(after overwrite) error = %v", err)
	}
	if got.Status != StatusFailed {
		t.Errorf("Status after overwrite = %q, want %q", got.Status, StatusFailed)
	}

	// Loading a nonexistent session must fail.
	if _, err := s.LoadSession("does-not-exist"); err == nil {
		t.Error("LoadSession(nonexistent) error = nil, want error")
	}
}

func TestStorageListSessions(t *testing.T) {
	s := newTestStorage(t)

	base := time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		session := testSession(
			"sess-"+string(rune('a'+i)),
			base.Add(time.Duration(i)*time.Minute),
		)
		if err := s.SaveSession(session); err != nil {
			t.Fatalf("SaveSession(%d) error = %v", i, err)
		}
	}

	all, err := s.ListSessions(0, 100)
	if err != nil {
		t.Fatalf("ListSessions(0, 100) error = %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("len(ListSessions) = %d, want 5", len(all))
	}

	// Most recent first.
	for i := 1; i < len(all); i++ {
		if all[i-1].CreatedAt.Before(all[i].CreatedAt) {
			t.Errorf("sessions not sorted by CreatedAt desc: index %d (%v) before index %d (%v)",
				i-1, all[i-1].CreatedAt, i, all[i].CreatedAt)
		}
	}

	page, err := s.ListSessions(1, 2)
	if err != nil {
		t.Fatalf("ListSessions(1, 2) error = %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("len(page) = %d, want 2", len(page))
	}
	if page[0].ID != all[1].ID || page[1].ID != all[2].ID {
		t.Errorf("page = [%s, %s], want [%s, %s]", page[0].ID, page[1].ID, all[1].ID, all[2].ID)
	}

	// Offset beyond the end yields an empty list, not an error.
	tail, err := s.ListSessions(10, 5)
	if err != nil {
		t.Fatalf("ListSessions(10, 5) error = %v", err)
	}
	if len(tail) != 0 {
		t.Errorf("len(tail) = %d, want 0", len(tail))
	}

	// Empty storage yields an empty list, not an error.
	empty := NewStorage(filepath.Join(t.TempDir(), "missing"))
	gotEmpty, err := empty.ListSessions(0, 10)
	if err != nil {
		t.Fatalf("ListSessions on empty storage error = %v", err)
	}
	if len(gotEmpty) != 0 {
		t.Errorf("len(empty) = %d, want 0", len(gotEmpty))
	}
}

func TestStorageSaveAndLoadTrace(t *testing.T) {
	s := newTestStorage(t)

	start := time.Now().UTC()
	traces := []TraceEntry{
		{
			SessionID:  "sess-trace",
			SpanID:     "span-root",
			Name:       "agent.turn",
			StartTime:  start,
			EndTime:    start.Add(2 * time.Second),
			DurationMS: 2000,
			Status:     SpanOK,
		},
		{
			SessionID: "sess-trace",
			SpanID:    "span-child",
			ParentID:  "span-root",
			Name:      "tool.bash",
			StartTime: start.Add(100 * time.Millisecond),
			EndTime:   start.Add(900 * time.Millisecond),
			DurationMS: 800,
			Attributes: map[string]any{"command": "go test ./..."},
			Status:    SpanError,
			Error:     "exit status 1",
		},
	}
	for i, tr := range traces {
		if err := s.SaveTrace(tr); err != nil {
			t.Fatalf("SaveTrace(%d) error = %v", i, err)
		}
	}

	got, err := s.LoadTraces("sess-trace")
	if err != nil {
		t.Fatalf("LoadTraces() error = %v", err)
	}
	if len(got) != len(traces) {
		t.Fatalf("len(LoadTraces) = %d, want %d", len(got), len(traces))
	}
	for i, want := range traces {
		have := got[i]
		if have.SpanID != want.SpanID {
			t.Errorf("trace[%d].SpanID = %q, want %q", i, have.SpanID, want.SpanID)
		}
		if have.Name != want.Name {
			t.Errorf("trace[%d].Name = %q, want %q", i, have.Name, want.Name)
		}
		if have.Status != want.Status {
			t.Errorf("trace[%d].Status = %q, want %q", i, have.Status, want.Status)
		}
		if have.Error != want.Error {
			t.Errorf("trace[%d].Error = %q, want %q", i, have.Error, want.Error)
		}
		if !have.StartTime.Equal(want.StartTime) {
			t.Errorf("trace[%d].StartTime = %v, want %v", i, have.StartTime, want.StartTime)
		}
		if want.Attributes != nil && have.Attributes["command"] != want.Attributes["command"] {
			t.Errorf("trace[%d].Attributes[command] = %v, want %v",
				i, have.Attributes["command"], want.Attributes["command"])
		}
	}

	// Appending a trace accumulates in load order.
	extra := TraceEntry{
		SessionID: "sess-trace",
		SpanID:    "span-third",
		Name:      "agent.finalize",
		StartTime: start.Add(3 * time.Second),
		EndTime:   start.Add(4 * time.Second),
		Status:    SpanOK,
	}
	if err := s.SaveTrace(extra); err != nil {
		t.Fatalf("SaveTrace(extra) error = %v", err)
	}
	got, err = s.LoadTraces("sess-trace")
	if err != nil {
		t.Fatalf("LoadTraces(after append) error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(LoadTraces after append) = %d, want 3", len(got))
	}
	if got[2].SpanID != extra.SpanID {
		t.Errorf("last span = %q, want %q", got[2].SpanID, extra.SpanID)
	}

	// Unknown session has no traces.
	noTraces, err := s.LoadTraces("unknown-session")
	if err != nil {
		t.Fatalf("LoadTraces(unknown) error = %v", err)
	}
	if len(noTraces) != 0 {
		t.Errorf("len(LoadTraces(unknown)) = %d, want 0", len(noTraces))
	}
}

func TestStorageSaveAndLoadToolCall(t *testing.T) {
	s := newTestStorage(t)

	start := time.Now().UTC()
	calls := []ToolCallRecord{
		{
			SessionID:  "sess-tools",
			SpanID:     "call-1",
			ToolName:   "read_file",
			Arguments:  `{"path":"main.go"}`,
			Result:     "package main",
			IsError:    false,
			DurationMS: 12,
			StartTime:  start,
		},
		{
			SessionID:  "sess-tools",
			SpanID:     "call-2",
			ToolName:   "bash",
			Arguments:  `{"command":"exit 1"}`,
			Result:     "command failed",
			IsError:    true,
			DurationMS: 340,
			StartTime:  start.Add(time.Second),
		},
	}
	for i, c := range calls {
		if err := s.SaveToolCall(c); err != nil {
			t.Fatalf("SaveToolCall(%d) error = %v", i, err)
		}
	}

	got, err := s.LoadToolCalls("sess-tools")
	if err != nil {
		t.Fatalf("LoadToolCalls() error = %v", err)
	}
	if len(got) != len(calls) {
		t.Fatalf("len(LoadToolCalls) = %d, want %d", len(got), len(calls))
	}
	for i, want := range calls {
		have := got[i]
		if have.SpanID != want.SpanID {
			t.Errorf("call[%d].SpanID = %q, want %q", i, have.SpanID, want.SpanID)
		}
		if have.ToolName != want.ToolName {
			t.Errorf("call[%d].ToolName = %q, want %q", i, have.ToolName, want.ToolName)
		}
		if have.Arguments != want.Arguments {
			t.Errorf("call[%d].Arguments = %q, want %q", i, have.Arguments, want.Arguments)
		}
		if have.Result != want.Result {
			t.Errorf("call[%d].Result = %q, want %q", i, have.Result, want.Result)
		}
		if have.IsError != want.IsError {
			t.Errorf("call[%d].IsError = %v, want %v", i, have.IsError, want.IsError)
		}
		if have.DurationMS != want.DurationMS {
			t.Errorf("call[%d].DurationMS = %d, want %d", i, have.DurationMS, want.DurationMS)
		}
		if !have.StartTime.Equal(want.StartTime) {
			t.Errorf("call[%d].StartTime = %v, want %v", i, have.StartTime, want.StartTime)
		}
	}

	// Unknown session has no tool calls.
	none, err := s.LoadToolCalls("unknown-session")
	if err != nil {
		t.Fatalf("LoadToolCalls(unknown) error = %v", err)
	}
	if len(none) != 0 {
		t.Errorf("len(LoadToolCalls(unknown)) = %d, want 0", len(none))
	}
}
