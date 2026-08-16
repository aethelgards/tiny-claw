package subagent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/aethelgards/tiny-claw/internal/tools"
	"github.com/pkg/errors"
)

type AgentRunner interface {
	RunSub(ctx context.Context, taskPrompt string, readOnlyRegistry tools.Registry) (string, error)
}

type SubAgentTool struct {
	runner           AgentRunner
	readOnlyRegistry tools.Registry
}

func (s *SubAgentTool) Name() string {
	return "spawn_subagent"
}

func (s *SubAgentTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        s.Name(),
		Description: "派出一个专门用于深度探索（Exploration）的子智能体。当你需要阅读大量代码、跨文件查找逻辑时请调用此工具。它在探索完毕后，会给你返回一份极度精炼的摘要报告。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_prompt": map[string]any{
					"type":        "string",
					"description": "给智能体下达的明确指令",
				},
			},
			"required": []string{"task_prompt"},
		},
	}
}

type subagentArgs struct {
	TaskPrompt string `json:"task_prompt"`
}

func (s *SubAgentTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var inputArgs subagentArgs
	if err := json.Unmarshal(args, &inputArgs); err != nil {
		return "", errors.Wrapf(err, "请求参数解析失败")
	}
	slog.InfoContext(ctx, "subAgent start", slog.String("task_prompt", inputArgs.TaskPrompt))
	summary, err := s.runner.RunSub(ctx, inputArgs.TaskPrompt, s.readOnlyRegistry)
	if err != nil {
		return "", errors.Wrapf(err, "subAgent run sun agent failed, err:%s", err.Error())
	}

	slog.InfoContext(ctx, "subAgent end", slog.String("summary", summary))

	return fmt.Sprintf("【子智能体探索报告】%s", summary), nil
}

func NewSubAgentTool(runner AgentRunner, readOnlyRegistry tools.Registry) tools.BaseTool {
	return &SubAgentTool{
		runner:           runner,
		readOnlyRegistry: readOnlyRegistry,
	}
}
