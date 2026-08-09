package engine

import (
	"context"
	"log/slog"
	"sync"

	"github.com/aethelgards/tiny-claw/internal/config"
	ctxpkg "github.com/aethelgards/tiny-claw/internal/context"
	"github.com/aethelgards/tiny-claw/internal/provider"
	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/aethelgards/tiny-claw/internal/tools"
	"github.com/pkg/errors"
)

type AgentEngine struct {
	provider       provider.LLMProvider
	registry       tools.Registry
	reporter       Reporter
	WorkDir        string
	EnableThink    bool
	promptComposer *ctxpkg.PromptComposer
	session        *Session // 多轮会话；nil 时每次 Run 使用独立上下文
}

func NewAgentEngine(p provider.LLMProvider, r tools.Registry, settings config.Settings, reporter Reporter, promptComposer *ctxpkg.PromptComposer) *AgentEngine {
	return &AgentEngine{
		provider:       p,
		registry:       r,
		WorkDir:        settings.WorkDir,
		EnableThink:    settings.EnableThinking,
		reporter:       reporter,
		promptComposer: promptComposer,
	}
}

// WithSession 绑定持久化会话；为 nil 时每次 Run 使用独立上下文（无多轮记忆）。
func (e *AgentEngine) WithSession(s *Session) *AgentEngine {
	e.session = s
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

	turnCount := 0
	for {
		turnCount++
		slog.InfoContext(ctx, "turn start", slog.Int("turnCount", turnCount))

		availableTools := e.registry.GetAvailableTools()

		if e.EnableThink {
			e.reporter.OnThinking(ctx)
		}

		// Single-stage tool loop: reasoning and action happen in one response.
		// A separate tool-less "thinking" pass was removed — it made the model
		// draft real tool calls as fake text, then treat that draft as already
		// executed (skipping the actual call). EnableThink is retained as a
		// config field for compatibility but no longer forks the loop.
		resp, err := e.provider.Generate(ctx, ctxHistory, availableTools)
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

		execResult := e.parallelExecTools(ctx, resp.ToolCalls)
		ctxHistory = append(ctxHistory, execResult...)
		if e.session != nil {
			turnMsgs = append(turnMsgs, execResult...)
		}
	}

	if e.session != nil && len(turnMsgs) > 0 {
		e.session.Append(ctx, turnMsgs...)
	}
	return nil
}

func (e *AgentEngine) parallelExecTools(ctx context.Context, calls []schema.ToolCall) []schema.Message {
	var wg sync.WaitGroup
	var execResult = make([]schema.Message, len(calls))
	for idx, call := range calls {
		parallelId := idx
		wg.Go(func() {
			e.reporter.OnToolCall(ctx, call.Name, string(call.Arguments))
			slog.InfoContext(ctx, "🛠->️ tool call", slog.Int("parallelIdx", parallelId), slog.String("toolName", call.Name), slog.String("args", string(call.Arguments)))
			result := e.registry.Execute(ctx, call)
			if result.IsError {
				slog.ErrorContext(ctx, "❌->tool call failed", slog.String("failedInfo", result.Output))
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
	return execResult
}
