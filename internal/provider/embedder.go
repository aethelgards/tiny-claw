package provider

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/aethelgards/tiny-claw/internal/config"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type OpenAIEmbedder struct {
	client  openai.Client
	model   string
	timeout time.Duration
}

func NewOpenAIEmbedder(cfg *config.EmbeddingConfig, globalAPIKey, globalBaseURL string) (*OpenAIEmbedder, error) {
	if cfg == nil || cfg.Model == "" {
		return nil, errors.New("embedding config is required")
	}

	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = globalAPIKey
	}
	if apiKey == "" {
		return nil, errors.New("embedding apiKey is required")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = globalBaseURL
	}

	timeout := 5 * time.Second
	if cfg.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(cfg.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid embedding timeout: %w", err)
		}
	}

	return &OpenAIEmbedder{
		client: openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithBaseURL(baseURL),
		),
		model:   cfg.Model,
		timeout: timeout,
	}, nil
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	resp, err := e.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: e.model,
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: texts,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("embedding API request failed: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, errors.New("embedding API returned empty data")
	}

	result := make([][]float32, len(texts))
	for _, d := range resp.Data {
		if int(d.Index) >= len(texts) {
			continue
		}
		vec := l2Normalize(d.Embedding)
		result[d.Index] = vec
	}

	return result, nil
}

func l2Normalize(vec []float64) []float32 {
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		result := make([]float32, len(vec))
		for i, v := range vec {
			result[i] = float32(v)
		}
		return result
	}
	result := make([]float32, len(vec))
	for i, v := range vec {
		result[i] = float32(v / norm)
	}
	return result
}
