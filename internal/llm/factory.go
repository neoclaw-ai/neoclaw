package llm

import (
	"fmt"
	"strings"

	"github.com/machinae/betterclaw/internal/config"
)

const defaultMaxTokens = 8192

func normalizeMaxTokens(v int) int {
	if v <= 0 {
		return defaultMaxTokens
	}
	return v
}

// NewProviderFromConfig builds an LLM provider from the selected LLM profile.
func NewProviderFromConfig(cfg config.LLMProviderConfig) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "anthropic":
		return newAnthropicProvider(cfg)
	case "openrouter":
		return newOpenRouterProvider(cfg)
	default:
		return nil, fmt.Errorf("unsupported provider %q", cfg.Provider)
	}
}
