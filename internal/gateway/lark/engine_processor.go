package lark

import (
	"context"

	"github.com/aethelgards/tiny-claw/internal/approval"
	"github.com/aethelgards/tiny-claw/internal/config"
	ctxpkg "github.com/aethelgards/tiny-claw/internal/context"
	"github.com/aethelgards/tiny-claw/internal/engine"
	"github.com/aethelgards/tiny-claw/internal/provider"
	"github.com/aethelgards/tiny-claw/internal/tools"
)

// EngineProcessor 为每条消息装配 AgentEngine 与 LarkReporter 并运行。
// 会话状态存于进程级 GlobalSessionMessage（按 ChatID 复用），跨消息续聊；
// 会话超窗压缩时注入基于 LLM 的摘要器（engine.NewLLMSummarizer）。
type EngineProcessor struct {
	provider       provider.LLMProvider
	registry       tools.Registry
	settings       config.Settings
	bot            *Bot
	promptComposer *ctxpkg.PromptComposer
	summarizer     ctxpkg.Summarizer
}

func NewEngineProcessor(p provider.LLMProvider, r tools.Registry, s config.Settings, b *Bot, composer *ctxpkg.PromptComposer) *EngineProcessor {
	return &EngineProcessor{
		provider:       p,
		registry:       r,
		settings:       s,
		bot:            b,
		promptComposer: composer,
		summarizer:     engine.NewLLMSummarizer(p),
	}
}

// Process 为该消息创建独立 engine + reporter，绑定按 ChatID 复用/新建的会话后执行。
func (p *EngineProcessor) Process(ctx context.Context, msg IncomingMessage) error {
	reporter := NewLarkReporter(p.bot, msg.ChatID, msg.TenantKey)
	// 注入审批上下文：审批中间件通过 ctx 取 reporter（发卡片）与发起人 open_id（身份校验）
	ctx = approval.WithApprovalContext(ctx, reporter, msg.OpenID)
	agent := engine.NewAgentEngine(p.provider, p.registry, p.settings, reporter, p.promptComposer, ctxpkg.NewRecoveryManager(), engine.NewReminderInjector(3))
	agent.WithSession(engine.GlobalSessionMessage.GetOrCreate(msg.ChatID, p.settings.WorkDir, ctxpkg.WithSummarizer(p.summarizer)))
	return agent.Run(ctx, msg.Text)
}
