package schema

import "encoding/json"

type Role string

const (
	RoleSystem    = "System"
	RoleUser      = "User"
	RoleAssistant = "Assistant"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`

	ToolCalls []ToolCall `json:"tool_calls"`

	ToolCallID string `json:"tool_call_id"`
	Usage      *Usage `json:"usage"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Output     string `json:"output"`
	IsError    bool   `json:"is_error"`
	Err        error  `json:"-"`
}

type Usage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
}

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}
