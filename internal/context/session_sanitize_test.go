package context

import (
	"testing"

	"github.com/aethelgards/tiny-claw/internal/schema"
)

func TestSanitizeToolMessages_Empty(t *testing.T) {
	msgs := []schema.Message{}
	result := sanitizeToolMessages(msgs)
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d messages", len(result))
	}
}

func TestSanitizeToolMessages_NoToolCalls(t *testing.T) {
	msgs := []schema.Message{
		{Role: schema.RoleUser, Content: "hello"},
		{Role: schema.RoleAssistant, Content: "hi"},
		{Role: schema.RoleUser, Content: "how are you"},
	}
	result := sanitizeToolMessages(msgs)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
}

func TestSanitizeToolMessages_ValidToolSequence(t *testing.T) {
	msgs := []schema.Message{
		{Role: schema.RoleUser, Content: "do something"},
		{Role: schema.RoleAssistant, Content: "ok", ToolCalls: []schema.ToolCall{{ID: "tc-1", Name: "bash"}}},
		{Role: schema.RoleUser, Content: "result", ToolCallID: "tc-1"},
		{Role: schema.RoleAssistant, Content: "done"},
	}
	result := sanitizeToolMessages(msgs)
	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}
}

func TestSanitizeToolMessages_OrphanedToolResult(t *testing.T) {
	msgs := []schema.Message{
		{Role: schema.RoleUser, Content: "hello"},
		{Role: schema.RoleUser, Content: "orphaned result", ToolCallID: "tc-orphan"},
		{Role: schema.RoleAssistant, Content: "response"},
	}
	result := sanitizeToolMessages(msgs)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages (orphan dropped), got %d", len(result))
	}
	if result[0].Content != "hello" {
		t.Fatalf("first message should be 'hello', got %q", result[0].Content)
	}
	if result[1].Content != "response" {
		t.Fatalf("second message should be 'response', got %q", result[1].Content)
	}
}

func TestSanitizeToolMessages_MultipleOrphans(t *testing.T) {
	msgs := []schema.Message{
		{Role: schema.RoleUser, Content: "orphan1", ToolCallID: "tc-1"},
		{Role: schema.RoleUser, Content: "orphan2", ToolCallID: "tc-2"},
		{Role: schema.RoleAssistant, Content: "response"},
	}
	result := sanitizeToolMessages(msgs)
	if len(result) != 1 {
		t.Fatalf("expected 1 message (orphans dropped), got %d", len(result))
	}
}

func TestSanitizeToolMessages_ValidParallelToolCalls(t *testing.T) {
	msgs := []schema.Message{
		{Role: schema.RoleUser, Content: "do parallel work"},
		{Role: schema.RoleAssistant, Content: "ok", ToolCalls: []schema.ToolCall{
			{ID: "tc-1", Name: "bash"},
			{ID: "tc-2", Name: "read_file"},
		}},
		{Role: schema.RoleUser, Content: "result1", ToolCallID: "tc-1"},
		{Role: schema.RoleUser, Content: "result2", ToolCallID: "tc-2"},
		{Role: schema.RoleAssistant, Content: "done"},
	}
	result := sanitizeToolMessages(msgs)
	if len(result) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(result))
	}
}

func TestSanitizeToolMessages_ToolResultAfterAssistantWithoutToolCalls(t *testing.T) {
	msgs := []schema.Message{
		{Role: schema.RoleUser, Content: "hello"},
		{Role: schema.RoleAssistant, Content: "response without tool calls"},
		{Role: schema.RoleUser, Content: "orphaned result", ToolCallID: "tc-1"},
	}
	result := sanitizeToolMessages(msgs)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages (orphan dropped), got %d", len(result))
	}
}

func TestSanitizeToolMessages_ToolResultPrecededByUserThenAssistant(t *testing.T) {
	msgs := []schema.Message{
		{Role: schema.RoleUser, Content: "first"},
		{Role: schema.RoleAssistant, Content: "ok", ToolCalls: []schema.ToolCall{{ID: "tc-1", Name: "bash"}}},
		{Role: schema.RoleUser, Content: "result", ToolCallID: "tc-1"},
	}
	result := sanitizeToolMessages(msgs)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
}
