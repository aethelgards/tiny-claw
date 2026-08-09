package tools

import (
	"context"
	"encoding/json"
	errors "errors"
	"fmt"
	"log/slog"

	"github.com/aethelgards/tiny-claw/internal/schema"
)

type Registry interface {
	Registry(tool BaseTool) error
	GetAvailableTools() []schema.ToolDefinition
	Execute(ctx context.Context, call schema.ToolCall) schema.ToolResult
}

type BaseTool interface {
	Name() string
	Definition() schema.ToolDefinition
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}

type toolRegistry struct {
	toolMap map[string]BaseTool
	tools   []schema.ToolDefinition
}

func (t *toolRegistry) Registry(tool BaseTool) error {
	if tool == nil {
		return errors.New("registry tool cannot be nil")
	}
	_, ok := t.toolMap[tool.Name()]
	if ok {
		return fmt.Errorf("tool %s already registered", tool.Name())
	}
	t.toolMap[tool.Name()] = tool
	t.tools = append(t.tools, tool.Definition())
	slog.Info("registered tool", slog.String("name", tool.Name()))
	return nil
}

func (t *toolRegistry) GetAvailableTools() []schema.ToolDefinition {
	return t.tools
}

func (t *toolRegistry) Execute(ctx context.Context, call schema.ToolCall) schema.ToolResult {
	tool, ok := t.toolMap[call.Name]
	if !ok {
		slog.WarnContext(ctx, "tool not found", "name", call.Name)
		return schema.ToolResult{
			ToolCallID: call.ID,
			Output:     fmt.Sprintf("tool %s not found", call.Name),
			IsError:    true,
		}
	}
	execute, err := tool.Execute(ctx, call.Arguments)
	if err != nil {
		slog.WarnContext(ctx, "tool execution failed", "name", call.Name, "args", string(call.Arguments))
		return schema.ToolResult{
			ToolCallID: call.ID,
			Output:     err.Error(),
			IsError:    true,
		}
	}
	return schema.ToolResult{
		ToolCallID: call.ID,
		Output:     execute,
		IsError:    false,
	}
}

func NewToolRegistry() Registry {
	return &toolRegistry{
		toolMap: make(map[string]BaseTool),
	}
}
