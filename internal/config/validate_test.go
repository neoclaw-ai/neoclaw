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
	_ Validatable = ContextConfig{}
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

func TestValidateStartup_RequestTimeoutZeroIsAllowed(t *testing.T) {
	cfg := &Config{
		LLM:      map[string]LLMProviderConfig{"default": {Provider: "anthropic", APIKey: "k", Model: "m", RequestTimeout: 0}},
		Channels: map[string]ChannelConfig{"telegram": {Enabled: true, Token: "t"}},
		Security: SecurityConfig{Mode: SecurityModeStandard},
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected request_timeout=0 to be allowed, got %v", err)
	}
}

func TestValidateStartup_RequestTimeoutNegativeFails(t *testing.T) {
	cfg := &Config{
		LLM:      map[string]LLMProviderConfig{"default": {Provider: "anthropic", APIKey: "k", Model: "m", RequestTimeout: -1 * time.Second}},
		Channels: map[string]ChannelConfig{"telegram": {Enabled: true, Token: "t"}},
		Security: SecurityConfig{Mode: SecurityModeStandard},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "request_timeout must be >= 0") {
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

func TestValidateStartup_CostsDailyLimitCannotExceedMonthly(t *testing.T) {
	cfg := &Config{
		LLM: map[string]LLMProviderConfig{
			"default": {Provider: "anthropic", APIKey: "k", Model: "m", RequestTimeout: time.Second},
		},
		Channels: map[string]ChannelConfig{
			"telegram": {Enabled: true, Token: "t"},
		},
		Security: SecurityConfig{Mode: SecurityModeStandard},
		Costs: CostsConfig{
			DailyLimit:   11,
			MonthlyLimit: 10,
		},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "daily_limit cannot be greater than monthly_limit") {
		t.Fatalf("expected daily vs monthly validation error, got %v", err)
	}
}

func TestValidateStartup_CostsAllowZeroValues(t *testing.T) {
	cfg := &Config{
		LLM: map[string]LLMProviderConfig{
			"default": {Provider: "anthropic", APIKey: "k", Model: "m", RequestTimeout: time.Second},
		},
		Channels: map[string]ChannelConfig{
			"telegram": {Enabled: true, Token: "t"},
		},
		Security: SecurityConfig{Mode: SecurityModeStandard},
		Costs: CostsConfig{
			DailyLimit:   0,
			MonthlyLimit: 0,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected zero costs to be valid, got %v", err)
	}
}

func TestValidateStartup_NegativeNumericFieldsFail(t *testing.T) {
	base := &Config{
		LLM: map[string]LLMProviderConfig{
			"default": {Provider: "anthropic", APIKey: "k", Model: "m", RequestTimeout: time.Second, MaxTokens: 1000},
		},
		Channels: map[string]ChannelConfig{
			"telegram": {Enabled: true, Token: "t"},
		},
		Security: SecurityConfig{Mode: SecurityModeStandard, CommandTimeout: time.Second},
		Costs:    CostsConfig{DailyLimit: 0, MonthlyLimit: 0},
		Context: ContextConfig{
			MaxTokens:        1,
			RecentMessages:   1,
			MaxToolCalls:     1,
			ToolOutputLength: 1,
			DailyLogLookback: time.Second,
		},
	}

	cases := []struct {
		name    string
		mutate  func(cfg *Config)
		wantErr string
	}{
		{
			name: "llm.max_tokens",
			mutate: func(cfg *Config) {
				llm := cfg.LLM["default"]
				llm.MaxTokens = -1
				cfg.LLM["default"] = llm
			},
			wantErr: "max_tokens must be >= 0",
		},
		{
			name: "security.command_timeout",
			mutate: func(cfg *Config) {
				cfg.Security.CommandTimeout = -1 * time.Second
			},
			wantErr: "command_timeout must be >= 0",
		},
		{
			name: "costs.daily_limit",
			mutate: func(cfg *Config) {
				cfg.Costs.DailyLimit = -1
			},
			wantErr: "daily_limit must be >= 0",
		},
		{
			name: "context.recent_messages",
			mutate: func(cfg *Config) {
				cfg.Context.RecentMessages = -1
			},
			wantErr: "recent_messages must be >= 0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := *base
			cfg.LLM = map[string]LLMProviderConfig{
				"default": base.LLM["default"],
			}
			tc.mutate(&cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected %q validation error, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateStartup_WebSearchProviderAllowlist(t *testing.T) {
	cfg := &Config{
		LLM: map[string]LLMProviderConfig{
			"default": {Provider: "anthropic", APIKey: "k", Model: "m", RequestTimeout: time.Second},
		},
		Channels: map[string]ChannelConfig{
			"telegram": {Enabled: true, Token: "t"},
		},
		Security: SecurityConfig{Mode: SecurityModeStandard},
		Web: WebConfig{
			Search: WebSearchConfig{Provider: "duckduckgo"},
		},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "unsupported web.search.provider") {
		t.Fatalf("expected unsupported provider error, got %v", err)
	}
}
