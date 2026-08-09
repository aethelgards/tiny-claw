package provider

import (
	"context"
	"fmt"

	"github.com/aethelgards/tiny-claw/internal/config"
	"github.com/aethelgards/tiny-claw/internal/schema"
)

type LLMProvider interface {
	Generate(ctx context.Context, messages []schema.Message, availableTools []schema.ToolDefinition) (*schema.Message, error)
}

// NewProvider 按 settings.Provider 路由到对应实现。
// 空 provider 视为 "openai"（与 spec 的缺省策略一致）。
func NewProvider(settings config.Settings) (LLMProvider, error) {
	switch settings.Provider {
	case "", "openai":
		return NewOpenAIProvider(settings)
	case "claude":
		return NewClaudeProvider(settings)
	default:
		return nil, fmt.Errorf("未知 provider %q（支持: openai, claude）", settings.Provider)
	}
}
