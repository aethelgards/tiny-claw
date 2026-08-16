package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	ctxpkg "github.com/aethelgards/tiny-claw/internal/context"
	"github.com/aethelgards/tiny-claw/internal/helper"
	"github.com/aethelgards/tiny-claw/internal/provider"
	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/aethelgards/tiny-claw/internal/subagent"
	"github.com/aethelgards/tiny-claw/internal/tools"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

type SubAgentEngine struct {
	provider         provider.LLMProvider
	reminderInjector ReminderInjector
	registry         tools.Registry
	recovery         *ctxpkg.RecoveryManager
}

var subAgentPrompt = `你是一个专门负责深度探索的探路者（Explorer SubAgent）你的任务是根据主架构师的指令，在当前工作区内仔细阅读代码，查阅日志，搜集足够多的信息
【核心纪律】
1. 你必须，且只能依靠工具，（如：bash的find/grep 或者read_file）去寻找答案，绝不允许凭空捏造或猜测
2. 如果你没有找到准确的答案，你必须继续使用工具深入探索
3. 当且仅当你找到了确切的线索后，停止调用工具，直接输出一段纯文本只作为你的终极汇报，主架构师会根据你的回报做进一步的决策。
`

func NewSubAgentEngine(
	provider provider.LLMProvider,
	reminderInjector ReminderInjector,
	registry tools.Registry,
	recovery *ctxpkg.RecoveryManager,
) subagent.AgentRunner {
	return &SubAgentEngine{
		provider:         provider,
		reminderInjector: reminderInjector,
		registry:         registry,
		recovery:         recovery,
	}
}

func (e *SubAgentEngine) RunSub(ctx context.Context, taskPrompt string, readOnlyRegistry tools.Registry) (string, error) {
	ctxHistory := []schema.Message{
		{
			Role:    schema.RoleSystem,
			Content: subAgentPrompt,
		},
		{
			Role:    schema.RoleUser,
			Content: taskPrompt,
		},
	}
	const maxSubTurns = 10
	turnCount := 0
	for {
		turnCount++
		if turnCount > maxSubTurns {
			return "", fmt.Errorf("子智能体探索过于深入，超过%d轮被强制召回，请主Agent给他更明确的指令", maxSubTurns)
		}

		availableTools := readOnlyRegistry.GetAvailableTools()

		resp, err := e.provider.Generate(ctx, ctxHistory, availableTools)
		if err != nil {
			return "", errors.Wrap(err, "provider.Generate failed")
		}
		ctxHistory = append(ctxHistory, *resp)
		if resp.Content != "" {
			slog.InfoContext(ctx, "llm", slog.String("content", resp.Content))
		}
		if len(resp.ToolCalls) == 0 {
			slog.InfoContext(ctx, "sub agent loop finish")
			return resp.Content, nil
		}

		slog.InfoContext(ctx, "sub agent llm tool call", slog.Int("toolCallCount", len(resp.ToolCalls)))

		execResult, failedToolCallMap := e.parallelExecTools(ctx, true, resp.ToolCalls)

		ctxHistory = append(ctxHistory, execResult...)

		if remind := e.reminderInjector.CheckAndRemind(ctx, failedToolCallMap); remind != nil {
			slog.InfoContext(ctx, "loop Run checkAndRemind", slog.String("injectRemind", remind.Content))
			ctxHistory = append(ctxHistory, *remind)
		}
	}
}

func (e *SubAgentEngine) parallelExecTools(ctx context.Context, isSubAgent bool, calls []schema.ToolCall) ([]schema.Message, map[string]lo.Tuple2[schema.ToolCall, schema.ToolResult]) {
	var wg sync.WaitGroup
	var execResult = make([]schema.Message, len(calls))
	failedToolCallMap := helper.NewSyncMap[string, lo.Tuple2[schema.ToolCall, schema.ToolResult]]()
	for idx, call := range calls {
		parallelId := idx
		wg.Go(func() {
			slog.InfoContext(ctx, "🛠->️ tool call", slog.Int("parallelIdx", parallelId), slog.String("toolName", call.Name), slog.String("args", string(call.Arguments)))
			result := e.registry.Execute(ctx, call)
			if result.IsError {
				slog.WarnContext(ctx, "❌->tool call failed", slog.String("failedInfo", result.Output))
				if errAnalyseResult := e.recovery.AnalyseAndInject(call.Name, result.Err); errAnalyseResult != "" {
					slog.InfoContext(ctx, "🔧-> err analyse result",
						slog.String("result", errAnalyseResult),
						slog.String("toolName", call.Name),
						slog.String("originErr", result.Output))
					result.Output = errAnalyseResult

					failedToolCallMap.Put(
						helper.GenerateFingerprint(call.Name, call.Arguments),
						lo.Tuple2[schema.ToolCall, schema.ToolResult]{
							A: call,
							B: result,
						},
					)

				}
			} else {
				slog.InfoContext(ctx, "✅->tool call succeeded", slog.String("successInfo", result.Output))
			}
			observationMsg := schema.Message{
				Role:       schema.RoleUser,
				Content:    result.Output,
				ToolCallID: call.ID,
			}
			execResult[parallelId] = observationMsg
		})
	}
	wg.Wait()
	return execResult, failedToolCallMap.ToMap()
}
