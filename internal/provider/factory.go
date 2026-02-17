package provider

import (
	"fmt"
	"strings"

	"github.com/machinae/betterclaw/internal/config"
)

func resolveMaxTokens(requestMaxTokens, configuredMaxTokens int) int {
	if requestMaxTokens > 0 {
		return requestMaxTokens
	}
	return configuredMaxTokens
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
