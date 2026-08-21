// Package observability provides data models for the visualization
// and observability system of tiny-claw agent sessions.
package observability

import "time"

// SessionStatus represents the status of a session.
type SessionStatus string

const (
	StatusRunning   SessionStatus = "running"
	StatusCompleted SessionStatus = "completed"
	StatusFailed    SessionStatus = "failed"
)

// TokenUsage tracks token consumption for a session or a single turn.
type TokenUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

// SessionData represents a complete agent session.
type SessionData struct {
	ID          string        `json:"id"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Prompt      string        `json:"prompt"`
	Model       string        `json:"model"`
	Status      SessionStatus `json:"status"`
	Turns       int           `json:"turns"`
	TotalTokens TokenUsage    `json:"total_tokens"`
	TotalCost   float64       `json:"total_cost"`
	DurationMS  int64         `json:"duration_ms"`
	Tags        []string      `json:"tags,omitempty"`
	Summary     string        `json:"summary,omitempty"`
}

// SpanStatus represents the status of a trace span.
type SpanStatus string

const (
	SpanOK    SpanStatus = "ok"
	SpanError SpanStatus = "error"
)

// TraceEntry represents a single span in the trace.
type TraceEntry struct {
	SessionID  string         `json:"session_id"`
	SpanID     string         `json:"span_id"`
	ParentID   string         `json:"parent_id,omitempty"`
	Name       string         `json:"name"`
	StartTime  time.Time      `json:"start_time"`
	EndTime    time.Time      `json:"end_time"`
	DurationMS int64          `json:"duration_ms"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Status     SpanStatus     `json:"status"`
	Error      string         `json:"error,omitempty"`
}

// ToolCallRecord represents a tool execution within a session.
type ToolCallRecord struct {
	SessionID  string    `json:"session_id"`
	SpanID     string    `json:"span_id"`
	ToolName   string    `json:"tool_name"`
	Arguments  string    `json:"arguments"`
	Result     string    `json:"result"`
	IsError    bool      `json:"is_error"`
	DurationMS int64     `json:"duration_ms"`
	StartTime  time.Time `json:"start_time"`
}
