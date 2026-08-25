package observability

import (
	"context"
	"log/slog"
	"time"

	ctxpkg "github.com/aethelgards/tiny-claw/internal/context"
	"github.com/aethelgards/tiny-claw/internal/provider"
	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/pkg/errors"
)

var PricingModel = map[string]struct {
	InputPrice  float64
	OutputPrice float64
}{
	"deepseek-v4-flash": {
		InputPrice:  0.15,
		OutputPrice: 0.15,
	},
}

type CostTracker struct {
	nextProvider provider.LLMProvider
	modelName    string
	session      *ctxpkg.Session
}

func NewCostTracker(next provider.LLMProvider, modelName string, session *ctxpkg.Session) *CostTracker {
	return &CostTracker{
		nextProvider: next,
		modelName:    modelName,
		session:      session,
	}
}

func (t *CostTracker) Generate(ctx context.Context, msgs []schema.Message, avaliableTools []schema.ToolDefinition) (*schema.Message, error) {
	startTime := time.Now()
	respMsg, err := t.nextProvider.Generate(ctx, msgs, avaliableTools)
	latency := time.Since(startTime)
	if err != nil {
		return nil, errors.Wrapf(err, "tracker error generating message for %s", t.modelName)
	}
	if respMsg.Usage != nil {
		var cost float64
		if price, ok := PricingModel[t.modelName]; ok {
			cost = float64(respMsg.Usage.PromptTokens)*price.InputPrice + float64(respMsg.Usage.CompletionTokens)*price.OutputPrice
		}
		slog.InfoContext(ctx, "tracker model cost", slog.Float64("cost", cost), slog.Int64("timeCost", latency.Microseconds()))
		if t.session != nil {
			t.session.RecordUsage(respMsg.Usage.PromptTokens, respMsg.Usage.CompletionTokens, cost)
		}
	}

	return respMsg, nil
}
