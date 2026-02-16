package config

import (
	"strings"
	"testing"
)

var (
	_ Validatable = LLMProviderConfig{}
	_ Validatable = ChannelConfig{}
	_ Validatable = SecurityConfig{}
	_ Validatable = CostsConfig{}
	_ Validatable = WebConfig{}
)

func TestValidateStartup_HardFailNoLLM(t *testing.T) {
	cfg := &Config{
		LLM:      map[string]LLMProviderConfig{},
		Channels: map[string]ChannelConfig{"telegram": {Enabled: true, Token: "t", AllowedUsers: []int64{1}}},
		Security: SecurityConfig{Mode: SecurityModeStandard},
	}

	_, err := ValidateStartup(cfg)
	if err == nil {
		t.Fatalf("expected error for missing llm profiles")
	}
}

func TestValidateStartup_HardFailNoChannels(t *testing.T) {
	cfg := &Config{
		LLM:      map[string]LLMProviderConfig{"default": {Provider: "anthropic", APIKey: "k", Model: "m"}},
		Channels: map[string]ChannelConfig{},
		Security: SecurityConfig{Mode: SecurityModeStandard},
	}

	_, err := ValidateStartup(cfg)
	if err == nil {
		t.Fatalf("expected error for missing channels")
	}
}

func TestValidateStartup_AnthropicRequiresAPIKey(t *testing.T) {
	cfg := &Config{
		LLM:      map[string]LLMProviderConfig{"default": {Provider: "anthropic", APIKey: "", Model: "m"}},
		Channels: map[string]ChannelConfig{"telegram": {Enabled: true, Token: "t", AllowedUsers: []int64{1}}},
		Security: SecurityConfig{Mode: SecurityModeStandard},
	}

	_, err := ValidateStartup(cfg)
	if err == nil || !strings.Contains(err.Error(), "api_key is required") {
		t.Fatalf("expected anthropic api_key validation error, got %v", err)
	}
}

func TestValidateStartup_OllamaDoesNotRequireAPIKey(t *testing.T) {
	cfg := &Config{
		LLM:      map[string]LLMProviderConfig{"default": {Provider: "ollama", APIKey: "", Model: "llama3"}},
		Channels: map[string]ChannelConfig{"telegram": {Enabled: true, Token: "t", AllowedUsers: []int64{1}}},
		Security: SecurityConfig{Mode: SecurityModeStandard},
	}

	_, err := ValidateStartup(cfg)
	if err != nil {
		t.Fatalf("expected ollama config to pass without api key, got %v", err)
	}
}

func TestValidateStartup_TelegramAllowedUsersEmptyWarnsOnly(t *testing.T) {
	cfg := &Config{
		LLM:      map[string]LLMProviderConfig{"default": {Provider: "anthropic", APIKey: "k", Model: "m"}},
		Channels: map[string]ChannelConfig{"telegram": {Enabled: true, Token: "t", AllowedUsers: []int64{}}},
		Security: SecurityConfig{Mode: SecurityModeStandard},
	}

	report, err := ValidateStartup(cfg)
	if err != nil {
		t.Fatalf("expected no hard error, got %v", err)
	}
	if report == nil || len(report.Warnings) == 0 {
		t.Fatalf("expected warning for empty telegram allowed_users")
	}
}
