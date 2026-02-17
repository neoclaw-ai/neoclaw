package provider

import (
	"testing"

	"github.com/machinae/betterclaw/internal/config"
)

func TestNewProviderFromConfig_SelectsAnthropic(t *testing.T) {
	p, err := NewProviderFromConfig(config.LLMProviderConfig{
		Provider: "anthropic",
		APIKey:   "k",
		Model:    "claude-sonnet-4-6",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*anthropicProvider); !ok {
		t.Fatalf("expected anthropic provider, got %T", p)
	}
}

func TestNewProviderFromConfig_SelectsOpenRouter(t *testing.T) {
	p, err := NewProviderFromConfig(config.LLMProviderConfig{
		Provider: "openrouter",
		APIKey:   "k",
		Model:    "deepseek/deepseek-chat",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(*openRouterProvider); !ok {
		t.Fatalf("expected openrouter provider, got %T", p)
	}
}

func TestNewProviderFromConfig_UnsupportedProvider(t *testing.T) {
	_, err := NewProviderFromConfig(config.LLMProviderConfig{
		Provider: "nope",
		APIKey:   "k",
		Model:    "m",
	})
	if err == nil {
		t.Fatalf("expected error for unsupported provider")
	}
}
