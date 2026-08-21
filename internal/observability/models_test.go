package observability

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSessionDataCreation(t *testing.T) {
	now := time.Now()
	session := SessionData{
		ID:        "session-123",
		CreatedAt: now,
		UpdatedAt: now,
		Prompt:    "Write a hello world program",
		Model:     "claude-sonnet-4-5",
		Status:    StatusRunning,
		Turns:     3,
		TotalTokens: TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
		TotalCost:  0.015,
		DurationMS: 5000,
		Tags:       []string{"test", "demo"},
		Summary:    "A test session",
	}

	if session.ID != "session-123" {
		t.Errorf("ID = %q, want %q", session.ID, "session-123")
	}
	if session.Prompt != "Write a hello world program" {
		t.Errorf("Prompt = %q, want %q", session.Prompt, "Write a hello world program")
	}
	if session.Model != "claude-sonnet-4-5" {
		t.Errorf("Model = %q, want %q", session.Model, "claude-sonnet-4-5")
	}
	if session.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", session.Status, StatusRunning)
	}
	if session.Turns != 3 {
		t.Errorf("Turns = %d, want %d", session.Turns, 3)
	}
	if session.TotalCost != 0.015 {
		t.Errorf("TotalCost = %f, want %f", session.TotalCost, 0.015)
	}
	if session.DurationMS != 5000 {
		t.Errorf("DurationMS = %d, want %d", session.DurationMS, 5000)
	}
	if len(session.Tags) != 2 || session.Tags[0] != "test" || session.Tags[1] != "demo" {
		t.Errorf("Tags = %v, want [test demo]", session.Tags)
	}
	if session.Summary != "A test session" {
		t.Errorf("Summary = %q, want %q", session.Summary, "A test session")
	}

	// Verify status constants
	if StatusRunning != "running" {
		t.Errorf("StatusRunning = %q, want %q", StatusRunning, "running")
	}
	if StatusCompleted != "completed" {
		t.Errorf("StatusCompleted = %q, want %q", StatusCompleted, "completed")
	}
	if StatusFailed != "failed" {
		t.Errorf("StatusFailed = %q, want %q", StatusFailed, "failed")
	}

	// Verify JSON serialization round-trip
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var decoded SessionData
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if decoded.ID != session.ID {
		t.Errorf("round-trip ID = %q, want %q", decoded.ID, session.ID)
	}
	if decoded.Status != session.Status {
		t.Errorf("round-trip Status = %q, want %q", decoded.Status, session.Status)
	}
	if decoded.TotalTokens.TotalTokens != 150 {
		t.Errorf("round-trip TotalTokens.TotalTokens = %d, want %d", decoded.TotalTokens.TotalTokens, 150)
	}
}

func TestTokenUsageCalculation(t *testing.T) {
	usage := TokenUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	if usage.PromptTokens != 100 {
		t.Errorf("PromptTokens = %d, want %d", usage.PromptTokens, 100)
	}
	if usage.CompletionTokens != 50 {
		t.Errorf("CompletionTokens = %d, want %d", usage.CompletionTokens, 50)
	}
	if usage.TotalTokens != usage.PromptTokens+usage.CompletionTokens {
		t.Errorf("TotalTokens = %d, want PromptTokens + CompletionTokens = %d",
			usage.TotalTokens, usage.PromptTokens+usage.CompletionTokens)
	}

	// Verify JSON field names
	data, err := json.Marshal(usage)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	for _, key := range []string{"prompt_tokens", "completion_tokens", "total_tokens"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("JSON missing key %q in %s", key, string(data))
		}
	}
}

func TestTraceEntryCreation(t *testing.T) {
	start := time.Now()
	end := start.Add(250 * time.Millisecond)
	entry := TraceEntry{
		SessionID:  "session-123",
		SpanID:     "span-456",
		ParentID:   "span-parent",
		Name:       "llm.call",
		StartTime:  start,
		EndTime:    end,
		DurationMS: end.Sub(start).Milliseconds(),
		Attributes: map[string]any{
			"model":      "claude-sonnet-4-5",
			"turn":       1,
			"input_text": "hello",
		},
		Status:  SpanOK,
		Error:   "",
	}

	if entry.SessionID != "session-123" {
		t.Errorf("SessionID = %q, want %q", entry.SessionID, "session-123")
	}
	if entry.SpanID != "span-456" {
		t.Errorf("SpanID = %q, want %q", entry.SpanID, "span-456")
	}
	if entry.ParentID != "span-parent" {
		t.Errorf("ParentID = %q, want %q", entry.ParentID, "span-parent")
	}
	if entry.Name != "llm.call" {
		t.Errorf("Name = %q, want %q", entry.Name, "llm.call")
	}
	if entry.DurationMS != 250 {
		t.Errorf("DurationMS = %d, want %d", entry.DurationMS, 250)
	}
	if entry.Status != SpanOK {
		t.Errorf("Status = %q, want %q", entry.Status, SpanOK)
	}
	if entry.Attributes["model"] != "claude-sonnet-4-5" {
		t.Errorf("Attributes[model] = %v, want %q", entry.Attributes["model"], "claude-sonnet-4-5")
	}

	// Verify span status constants
	if SpanOK != "ok" {
		t.Errorf("SpanOK = %q, want %q", SpanOK, "ok")
	}
	if SpanError != "error" {
		t.Errorf("SpanError = %q, want %q", SpanError, "error")
	}

	// Verify error span
	errEntry := TraceEntry{
		SessionID: "session-123",
		SpanID:    "span-789",
		Name:      "tool.call",
		Status:    SpanError,
		Error:     "tool execution failed",
	}
	if errEntry.Status != SpanError {
		t.Errorf("errEntry.Status = %q, want %q", errEntry.Status, SpanError)
	}
	if errEntry.Error != "tool execution failed" {
		t.Errorf("errEntry.Error = %q, want %q", errEntry.Error, "tool execution failed")
	}

	// Verify JSON round-trip
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var decoded TraceEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if decoded.SpanID != entry.SpanID {
		t.Errorf("round-trip SpanID = %q, want %q", decoded.SpanID, entry.SpanID)
	}
	if decoded.Attributes["turn"] != float64(1) {
		t.Errorf("round-trip Attributes[turn] = %v, want %v", decoded.Attributes["turn"], float64(1))
	}
}

func TestToolCallRecordCreation(t *testing.T) {
	start := time.Now()
	record := ToolCallRecord{
		SessionID:  "session-123",
		SpanID:     "span-456",
		ToolName:   "read_file",
		Arguments:  `{"path":"/tmp/test.txt"}`,
		Result:     "file contents here",
		IsError:    false,
		DurationMS: 42,
		StartTime:  start,
	}

	if record.SessionID != "session-123" {
		t.Errorf("SessionID = %q, want %q", record.SessionID, "session-123")
	}
	if record.ToolName != "read_file" {
		t.Errorf("ToolName = %q, want %q", record.ToolName, "read_file")
	}
	if record.Arguments != `{"path":"/tmp/test.txt"}` {
		t.Errorf("Arguments = %q, want %q", record.Arguments, `{"path":"/tmp/test.txt"}`)
	}
	if record.Result != "file contents here" {
		t.Errorf("Result = %q, want %q", record.Result, "file contents here")
	}
	if record.IsError {
		t.Errorf("IsError = true, want false")
	}
	if record.DurationMS != 42 {
		t.Errorf("DurationMS = %d, want %d", record.DurationMS, 42)
	}

	// Verify error record
	errRecord := ToolCallRecord{
		SessionID: "session-123",
		SpanID:    "span-999",
		ToolName:  "write_file",
		IsError:   true,
	}
	if !errRecord.IsError {
		t.Errorf("errRecord.IsError = false, want true")
	}

	// Verify JSON round-trip
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var decoded ToolCallRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if decoded.ToolName != record.ToolName {
		t.Errorf("round-trip ToolName = %q, want %q", decoded.ToolName, record.ToolName)
	}
	if decoded.IsError != record.IsError {
		t.Errorf("round-trip IsError = %v, want %v", decoded.IsError, record.IsError)
	}
}
