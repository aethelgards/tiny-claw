package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aethelgards/tiny-claw/internal/config"
	ctxpkg "github.com/aethelgards/tiny-claw/internal/context"
	"github.com/aethelgards/tiny-claw/internal/helper"
	"github.com/aethelgards/tiny-claw/internal/observability"
	"github.com/aethelgards/tiny-claw/internal/provider"
	reporter2 "github.com/aethelgards/tiny-claw/internal/reporter"
	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/aethelgards/tiny-claw/internal/tools"
	"github.com/aethelgards/tiny-claw/internal/trace"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

type AgentEngine struct {
	provider         provider.LLMProvider
	registry         tools.Registry
	reporter         reporter2.Reporter
	WorkDir          string
	EnableThink      bool
	promptComposer   *ctxpkg.PromptComposer
	session          *ctxpkg.Session // 多轮会话；nil 时每次 Run 使用独立上下文
	recovery         *ctxpkg.RecoveryManager
	reminderInjector ReminderInjector
	settings         config.Settings
}

func NewAgentEngine(p provider.LLMProvider,
	r tools.Registry,
	settings config.Settings,
	reporter reporter2.Reporter,
	promptComposer *ctxpkg.PromptComposer,
	recovery *ctxpkg.RecoveryManager,
	reminderInjector ReminderInjector,
) *AgentEngine {
	return &AgentEngine{
		provider:         p,
		registry:         r,
		WorkDir:          settings.WorkDir,
		EnableThink:      settings.EnableThinking,
		reporter:         reporter,
		promptComposer:   promptComposer,
		recovery:         recovery,
		reminderInjector: reminderInjector,
		settings:         settings,
	}
}

// WithSession 绑定持久化会话；为 nil 时每次 Run 使用独立上下文（无多轮记忆）。
func (e *AgentEngine) WithSession(s *ctxpkg.Session) *AgentEngine {
	e.session = s
	observability.NewCostTracker(e.provider, e.settings.Model, s)
	return e
}

func (e *AgentEngine) Run(ctx context.Context, userPrompt string) error {
	slog.InfoContext(ctx, "engine start", slog.String("workDir", e.WorkDir), slog.Bool("enableThinking", e.EnableThink))
	ctxHistory := []schema.Message{e.promptComposer.Build()}
	userMsg := schema.Message{
		Role:    schema.RoleUser,
		Content: userPrompt,
	}

	// turnMsgs 收集本轮全部消息，整轮成功后才写入会话：
	// 中途失败（provider 报错）不持久化，避免孤儿 User 消息破坏 user/assistant 交替。
	var turnMsgs []schema.Message
	if e.session != nil {
		ctxHistory = append(ctxHistory, e.session.GetWorkingMemory(ctx)...)
		turnMsgs = append(turnMsgs, userMsg)
	}
	ctxHistory = append(ctxHistory, userMsg)

	ctx, rootSpan := trace.StartSpan(ctx, "agent.Run")
	defer func() {
		rootSpan.EndSpan()
		if err := trace.ExportTraceToFile(rootSpan, e.WorkDir, e.session.ID); err != nil {
			slog.ErrorContext(ctx, "failed to export trace to file", slog.String("error", err.Error()))
		}
	}()

	turnCount := 0
	for {
		turnCount++
		slog.InfoContext(ctx, "turn start", slog.Int("turnCount", turnCount))
		ctx, loopSpan := trace.StartSpan(ctx, fmt.Sprintf("turn->%d", turnCount))
		defer loopSpan.EndSpan()
		availableTools := e.registry.GetAvailableTools()

		if e.EnableThink {
			e.reporter.OnThinking(ctx)
		}

		loopSpan.AddAttribute("ctx_message_count", len(ctxHistory))

		ctx, actionSpan := trace.StartSpan(ctx, "LLM.Action")

		resp, err := e.provider.Generate(ctx, ctxHistory, availableTools)
		actionSpan.EndSpan()
		if err != nil {
			return errors.Wrap(err, "provider.Generate failed")
		}
		ctxHistory = append(ctxHistory, *resp)
		if e.session != nil {
			turnMsgs = append(turnMsgs, *resp)
		}
		if resp.Content != "" {
			slog.InfoContext(ctx, "llm", slog.String("content", resp.Content))
			e.reporter.OnMessage(ctx, resp.Content)
		}
		if len(resp.ToolCalls) == 0 {
			slog.InfoContext(ctx, "loop finish")
			break
		}

		slog.InfoContext(ctx, "llm tool call", slog.Int("toolCallCount", len(resp.ToolCalls)))

		execResult, failedToolCallMap := e.parallelExecTools(ctx, false, resp.ToolCalls)

		loopSpan.EndSpan()

		ctxHistory = append(ctxHistory, execResult...)

		if e.session != nil {
			turnMsgs = append(turnMsgs, execResult...)
		}

		if remind := e.reminderInjector.CheckAndRemind(ctx, failedToolCallMap); remind != nil {
			slog.InfoContext(ctx, "loop Run checkAndRemind", slog.String("injectRemind", remind.Content))
			ctxHistory = append(ctxHistory, *remind)
			if e.session != nil {
				turnMsgs = append(turnMsgs, *remind)
			}
		}
	}

	if e.session != nil && len(turnMsgs) > 0 {
		e.session.Append(ctx, turnMsgs...)
	}
	return nil
}

func (e *AgentEngine) parallelExecTools(ctx context.Context, isSubAgent bool, calls []schema.ToolCall) ([]schema.Message, map[string]lo.Tuple2[schema.ToolCall, schema.ToolResult]) {
	var wg sync.WaitGroup
	var execResult = make([]schema.Message, len(calls))
	failedToolCallMap := helper.NewSyncMap[string, lo.Tuple2[schema.ToolCall, schema.ToolResult]]()
	for idx, call := range calls {
		parallelId := idx
		wg.Go(func() {
			if isSubAgent {
				e.reporter.OnToolCall(ctx, fmt.Sprintf("subAgent: %s", call.Name), string(call.Arguments))
			} else {
				e.reporter.OnToolCall(ctx, call.Name, string(call.Arguments))
			}
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
			e.reporter.OnToolResult(ctx, call.Name, result.Output, result.IsError)
			execResult[parallelId] = observationMsg
		})
	}
	wg.Wait()
	return execResult, failedToolCallMap.ToMap()
}
