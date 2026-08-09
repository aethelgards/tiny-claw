package provider

import (
	"testing"

	"github.com/aethelgards/tiny-claw/internal/config"
)

func TestNewProviderRouting(t *testing.T) {
	base := config.Settings{Model: "glm-4.6", APIKey: "sk-x"}

	p, err := NewProvider(base)
	if err != nil {
		t.Fatalf("NewProvider(openai default): %v", err)
	}
	if _, ok := p.(*OpenAIProvider); !ok {
		t.Errorf("default provider type = %T, want *OpenAIProvider", p)
	}

	base.Provider = "claude"
	p, err = NewProvider(base)
	if err != nil {
		t.Fatalf("NewProvider(claude): %v", err)
	}
	if _, ok := p.(*ClaudeProvider); !ok {
		t.Errorf("claude provider type = %T, want *ClaudeProvider", p)
	}
}

func TestNewProviderUnknown(t *testing.T) {
	_, err := NewProvider(config.Settings{Provider: "gemini", Model: "x", APIKey: "y"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestNewOpenAIProviderMissingAPIKey(t *testing.T) {
	_, err := NewOpenAIProvider(config.Settings{Model: "glm-4.6"})
	if err == nil {
		t.Fatal("expected error when apiKey is empty")
	}
}
