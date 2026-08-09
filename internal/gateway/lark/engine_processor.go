package lark

import (
	"context"

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
	summarizer     engine.Summarizer
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
	agent := engine.NewAgentEngine(p.provider, p.registry, p.settings, reporter, p.promptComposer)
	agent.WithSession(engine.GlobalSessionMessage.GetOrCreate(msg.ChatID, p.settings.WorkDir, engine.WithSummarizer(p.summarizer)))
	return agent.Run(ctx, msg.Text)
}
