package lark

import (
	"context"
	"time"

	"github.com/aethelgards/tiny-claw/internal/approval"
	"github.com/aethelgards/tiny-claw/internal/config"
	ctxpkg "github.com/aethelgards/tiny-claw/internal/context"
	"github.com/aethelgards/tiny-claw/internal/engine"
	"github.com/aethelgards/tiny-claw/internal/memory"
	"github.com/aethelgards/tiny-claw/internal/observability"
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
	extractorOpt   ctxpkg.Option
	embedder       provider.Embedder
	storage        *observability.Storage
}

func NewEngineProcessor(p provider.LLMProvider, r tools.Registry, s config.Settings, b *Bot, composer *ctxpkg.PromptComposer, opts ...EngineProcessorOption) *EngineProcessor {
	extractTimeout, _ := time.ParseDuration(s.Memory.ExtractTimeout)
	extractor := memory.NewLLMExtractor(p).WithTimeout(extractTimeout)
	extractorOpt := ctxpkg.WithMemoryExtractor(memory.NewSessionHook(nil, extractor))
	ep := &EngineProcessor{
		provider:       p,
		registry:       r,
		settings:       s,
		bot:            b,
		promptComposer: composer,
		summarizer:     engine.NewLLMSummarizer(p),
		extractorOpt:   extractorOpt,
	}
	for _, opt := range opts {
		opt(ep)
	}
	return ep
}

type EngineProcessorOption func(*EngineProcessor)

func WithEmbedder(e provider.Embedder) EngineProcessorOption {
	return func(ep *EngineProcessor) {
		ep.embedder = e
	}
}

func (p *EngineProcessor) SetMemoryStore(store *memory.MemoryStore) {
	if p.extractorOpt != nil {
		extractTimeout, _ := time.ParseDuration(p.settings.Memory.ExtractTimeout)
		extractor := memory.NewLLMExtractor(p.provider).WithTimeout(extractTimeout)
		var hookOpts []memory.SessionHookOption
		if p.embedder != nil {
			hookOpts = append(hookOpts, memory.WithSessionEmbedder(p.embedder))
		}
		p.extractorOpt = ctxpkg.WithMemoryExtractor(memory.NewSessionHook(store, extractor, hookOpts...))
	}
}

func (p *EngineProcessor) WithStorage(s *observability.Storage) *EngineProcessor {
	p.storage = s
	return p
}

// Process 为该消息创建独立 engine + reporter，绑定按 ChatID 复用/新建的会话后执行。
func (p *EngineProcessor) Process(ctx context.Context, msg IncomingMessage) error {
	reporter := NewLarkReporter(p.bot, msg.ChatID, msg.TenantKey)
	ctx = approval.WithApprovalContext(ctx, reporter, msg.OpenID)
	agent := engine.NewAgentEngine(p.provider, p.registry, p.settings, reporter, p.promptComposer, ctxpkg.NewRecoveryManager(), engine.NewReminderInjector(3))
	agent.WithSession(engine.GlobalSessionMessage.GetOrCreate(msg.ChatID, p.settings.WorkDir, ctxpkg.WithSummarizer(p.summarizer), p.extractorOpt))
	if p.storage != nil {
		agent.WithStorage(p.storage)
	}
	return agent.Run(ctx, msg.Text)
}