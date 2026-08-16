package engine

import (
	"log/slog"
	"os"

	"github.com/aethelgards/tiny-claw/internal/config"
	ctxpkg "github.com/aethelgards/tiny-claw/internal/context"
	"github.com/aethelgards/tiny-claw/internal/provider"
	"github.com/aethelgards/tiny-claw/internal/subagent"
	"github.com/aethelgards/tiny-claw/internal/tools"
)

// NewSubAgent 装配 spawn_subagent 工具：只读工具集 + 子智能体引擎 + 恢复管理器。
func NewSubAgent(settings *config.Settings, provider provider.LLMProvider) tools.BaseTool {
	reg := tools.NewToolRegistry()
	for name, tool := range map[string]tools.BaseTool{
		"read_file":   tools.NewReadFileTool(settings.WorkDir),
		"delete_file": tools.NewDeleteFileTool(settings.WorkDir),
		"bash":        tools.NewBashTool(settings.WorkDir),
	} {
		if err := reg.Registry(tool); err != nil {
			slog.Error("工具注册失败", "tool", name, "err", err)
			os.Exit(1)
		}
	}
	agentEngine := NewSubAgentEngine(provider, NewReminderInjector(3), reg, ctxpkg.NewRecoveryManager())

	return subagent.NewSubAgentTool(agentEngine, reg)
}
