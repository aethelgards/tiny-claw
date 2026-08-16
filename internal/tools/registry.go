package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aethelgards/tiny-claw/internal/domainerr"
	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/aethelgards/tiny-claw/internal/trace"
)

type MiddlewareFunc func(ctx context.Context, call schema.ToolCall) (allow bool, rejectReason error)

type Registry interface {
	Registry(tool BaseTool) error
	Use(mw MiddlewareFunc)
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
	mws     []MiddlewareFunc
}

func (t *toolRegistry) Use(mw MiddlewareFunc) {
	t.mws = append(t.mws, mw)
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
	ctx, toolSpan := trace.StartSpan(ctx, "Tool.Execute")
	toolSpan.AddAttribute("tool_name", call.Name)
	toolSpan.AddAttribute("tool_args", string(call.Arguments))
	defer toolSpan.EndSpan()

	tool, ok := t.toolMap[call.Name]
	if !ok {
		toolSpan.AddAttribute("output_preview_err", fmt.Sprintf("tool %s not found", call.Name))
		slog.WarnContext(ctx, "tool not found", "name", call.Name)
		return schema.ToolResult{
			ToolCallID: call.ID,
			Output:     fmt.Sprintf("tool %s not found", call.Name),
			IsError:    true,
			Err:        domainerr.NewToolNotFoundError(call.Name),
		}
	}
	for _, mw := range t.mws {
		var mwSpan *trace.Span
		ctx, mwSpan = trace.StartSpan(ctx, "Tool.Middleware")
		allow, rejectReason := mw(ctx, call)
		if !allow {
			return schema.ToolResult{
				ToolCallID: call.ID,
				Output:     fmt.Sprintf("执行被系统拦截。原因: %s", rejectReason),
				IsError:    true,
				Err:        domainerr.NewPermissionDenyError(call.Name),
			}
		}
		mwSpan.EndSpan()
	}
	execute, err := tool.Execute(ctx, call.Arguments)
	if err != nil {
		slog.WarnContext(ctx, "tool execution failed", "name", call.Name, "args", string(call.Arguments))
		toolSpan.AddAttribute("output_preview_err", err.Error())
		return schema.ToolResult{
			ToolCallID: call.ID,
			Output:     err.Error(),
			IsError:    true,
			Err:        err,
		}
	}
	toolSpan.AddAttribute("output_preview", execute)
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
