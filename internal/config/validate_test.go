package config

import (
	"strings"
	"testing"
	"time"
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
		Channels: map[string]ChannelConfig{"telegram": {Enabled: true, Token: "t"}},
		Security: SecurityConfig{Mode: SecurityModeStandard},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected error for missing llm profiles")
	}
}

func TestValidateStartup_HardFailNoChannels(t *testing.T) {
	cfg := &Config{
		LLM:      map[string]LLMProviderConfig{"default": {Provider: "anthropic", APIKey: "k", Model: "m", RequestTimeout: time.Second}},
		Channels: map[string]ChannelConfig{},
		Security: SecurityConfig{Mode: SecurityModeStandard},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected error for missing channels")
	}
}

func TestValidateStartup_AnthropicRequiresAPIKey(t *testing.T) {
	cfg := &Config{
		LLM:      map[string]LLMProviderConfig{"default": {Provider: "anthropic", APIKey: "", Model: "m", RequestTimeout: time.Second}},
		Channels: map[string]ChannelConfig{"telegram": {Enabled: true, Token: "t"}},
		Security: SecurityConfig{Mode: SecurityModeStandard},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "api_key is required") {
		t.Fatalf("expected anthropic api_key validation error, got %v", err)
	}
}

func TestValidateStartup_OllamaDoesNotRequireAPIKey(t *testing.T) {
	cfg := &Config{
		LLM:      map[string]LLMProviderConfig{"default": {Provider: "ollama", APIKey: "", Model: "llama3", RequestTimeout: time.Second}},
		Channels: map[string]ChannelConfig{"telegram": {Enabled: true, Token: "t"}},
		Security: SecurityConfig{Mode: SecurityModeStandard},
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected ollama config to pass without api key, got %v", err)
	}
}

func TestValidateStartup_RequestTimeoutMustBePositive(t *testing.T) {
	cfg := &Config{
		LLM:      map[string]LLMProviderConfig{"default": {Provider: "anthropic", APIKey: "k", Model: "m", RequestTimeout: 0}},
		Channels: map[string]ChannelConfig{"telegram": {Enabled: true, Token: "t"}},
		Security: SecurityConfig{Mode: SecurityModeStandard},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "request_timeout must be > 0") {
		t.Fatalf("expected request_timeout validation error, got %v", err)
	}
}

func TestValidateStartup_WebBraveMissingAPIKeyDoesNotHardFail(t *testing.T) {
	cfg := &Config{
		LLM: map[string]LLMProviderConfig{
			"default": {Provider: "anthropic", APIKey: "k", Model: "m", RequestTimeout: time.Second},
		},
		Channels: map[string]ChannelConfig{
			"telegram": {Enabled: true, Token: "t"},
		},
		Security: SecurityConfig{Mode: SecurityModeStandard},
		Web: WebConfig{
			Search: WebSearchConfig{Provider: "brave", APIKey: ""},
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected no hard error for missing brave web.search api key, got %v", err)
	}
}

func TestValidateStartup_InvalidModeFails(t *testing.T) {
	cfg := &Config{
		LLM: map[string]LLMProviderConfig{
			"default": {Provider: "anthropic", APIKey: "k", Model: "m", RequestTimeout: time.Second},
		},
		Channels: map[string]ChannelConfig{
			"telegram": {Enabled: true, Token: "t"},
		},
		Security: SecurityConfig{Mode: "banana"},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "invalid security.mode") {
		t.Fatalf("expected invalid mode error, got %v", err)
	}
}

func TestValidateStartup_DangerFullAccessIsInvalidMode(t *testing.T) {
	cfg := &Config{
		LLM: map[string]LLMProviderConfig{
			"default": {Provider: "anthropic", APIKey: "k", Model: "m", RequestTimeout: time.Second},
		},
		Channels: map[string]ChannelConfig{
			"telegram": {Enabled: true, Token: "t"},
		},
		Security: SecurityConfig{Mode: "danger-full-access"},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "invalid security.mode") {
		t.Fatalf("expected invalid mode error, got %v", err)
	}
}

func TestValidateStartup_StrictModeIsValid(t *testing.T) {
	cfg := &Config{
		LLM: map[string]LLMProviderConfig{
			"default": {Provider: "anthropic", APIKey: "k", Model: "m", RequestTimeout: time.Second},
		},
		Channels: map[string]ChannelConfig{
			"telegram": {Enabled: true, Token: "t"},
		},
		Security: SecurityConfig{Mode: SecurityModeStrict},
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected strict mode to pass config validation, got %v", err)
	}
}

func TestValidateStartup_DangerModeIsValid(t *testing.T) {
	cfg := &Config{
		LLM: map[string]LLMProviderConfig{
			"default": {Provider: "anthropic", APIKey: "k", Model: "m", RequestTimeout: time.Second},
		},
		Channels: map[string]ChannelConfig{
			"telegram": {Enabled: true, Token: "t"},
		},
		Security: SecurityConfig{Mode: SecurityModeDanger},
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected danger mode to pass validation, got %v", err)
	}
}
