// Package config loads NeoClaw runtime configuration from a TOML file and environment variables, exposing typed structs and accessors for all sections.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

const defaultAgent = "default"

const (
	// SecurityModeStandard is the default sandbox/security behavior.
	SecurityModeStandard = "standard"
	// SecurityModeDanger disables sandbox protections and approval checks.
	SecurityModeDanger = "danger"
	// SecurityModeStrict enables stricter sandbox policy where supported.
	SecurityModeStrict = "strict"
)

// Config is the runtime configuration loaded from defaults, config.toml, and env vars.
type Config struct {
	// HomeDir is runtime-resolved from NEOCLAW_HOME and not read from config.
	HomeDir string `mapstructure:"-"`
	// Agent is runtime-selected (MVP default: "default"), not read from config.
	Agent    string                       `mapstructure:"-"`
	Channels map[string]ChannelConfig     `mapstructure:"channels"`
	LLM      map[string]LLMProviderConfig `mapstructure:"llm"`
	Security SecurityConfig               `mapstructure:"security"`
	Costs    CostsConfig                  `mapstructure:"costs"`
	Context  ContextConfig                `mapstructure:"context"`
	Web      WebConfig                    `mapstructure:"web"`
}

// ChannelConfig configures one inbound/outbound channel.
type ChannelConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Token   string `mapstructure:"token"`
}

// LLMProviderConfig configures one LLM provider profile.
type LLMProviderConfig struct {
	APIKey         string        `mapstructure:"api_key"`
	Provider       string        `mapstructure:"provider"`
	Model          string        `mapstructure:"model"`
	MaxTokens      int           `mapstructure:"max_tokens"`
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
}

// SecurityConfig controls command execution and sandbox behavior.
type SecurityConfig struct {
	// Workspace is derived from DataDir and Agent and is not configurable.
	Workspace      string        `mapstructure:"-"`
	CommandTimeout time.Duration `mapstructure:"command_timeout"`
	Mode           string        `mapstructure:"mode"`
}

// CostsConfig defines soft USD spending limits.
type CostsConfig struct {
	DailyLimit   float64 `mapstructure:"daily_limit"`
	MonthlyLimit float64 `mapstructure:"monthly_limit"`
}

// ContextConfig controls agent context window, prompt composition, and circuit-breaker behavior.
type ContextConfig struct {
	MaxTokens            int `mapstructure:"max_tokens"`
	RecentMessages       int `mapstructure:"recent_messages"`
	MaxToolCalls         int `mapstructure:"max_tool_calls"`
	ToolOutputLength     int `mapstructure:"tool_output_length"`
	DailyLogLookbackDays int `mapstructure:"daily_log_lookback_days"`
}

// WebConfig configures built-in web tool behavior.
type WebConfig struct {
	Search WebSearchConfig `mapstructure:"search"`
}

// WebSearchConfig configures the web search provider.
type WebSearchConfig struct {
	Provider string `mapstructure:"provider"`
	APIKey   string `mapstructure:"api_key"`
}

var defaultConfig = Config{
	Channels: map[string]ChannelConfig{
		"telegram": {
			Enabled: true,
			Token:   "",
		},
	},
	LLM: map[string]LLMProviderConfig{
		"default": {
			APIKey:         "",
			Provider:       "anthropic",
			Model:          "claude-sonnet-4-6",
			MaxTokens:      8192,
			RequestTimeout: 5 * time.Minute,
		},
	},
	Security: SecurityConfig{
		CommandTimeout: 5 * time.Minute,
		Mode:           SecurityModeStandard,
	},
	Costs: CostsConfig{
		DailyLimit:   0,
		MonthlyLimit: 0,
	},
	Context: ContextConfig{
		MaxTokens:            10000,
		RecentMessages:       12,
		MaxToolCalls:         15,
		ToolOutputLength:     12000,
		DailyLogLookbackDays: 2,
	},
	Web: WebConfig{
		Search: WebSearchConfig{
			Provider: "",
			APIKey:   "",
		},
	},
}

// defaultUserConfig is the minimal bootstrap config written for first-time
// users. It intentionally contains only user-editable essentials and not the
// full runtime default surface.
var defaultUserConfig = Config{
	Channels: map[string]ChannelConfig{
		"telegram": {
			Enabled: true,
			Token:   "",
		},
	},
	LLM: map[string]LLMProviderConfig{
		"default": {
			APIKey:         "$ANTHROPIC_API_KEY",
			Provider:       "anthropic",
			Model:          "claude-sonnet-4-6",
			RequestTimeout: 5 * time.Minute,
		},
	},
	Costs: CostsConfig{
		DailyLimit:   0,
		MonthlyLimit: 0,
	},
	Security: SecurityConfig{
		CommandTimeout: 5 * time.Minute,
		Mode:           SecurityModeStandard,
	},
}

// homeDir returns the NeoClaw home directory.
// Uses NEOCLAW_HOME env var if set, otherwise defaults to ~/.neoclaw.
func homeDir() (string, error) {
	if dir := os.Getenv("NEOCLAW_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return defaultHomePath(home), nil
}

// Load merges hardcoded defaults and config file values in that order.
// The runtime data directory is NEOCLAW_HOME/data (default: ~/.neoclaw/data).
// Config is always at $NEOCLAW_HOME/config.toml.
func Load() (*Config, error) {
	homeDir, err := homeDir()
	if err != nil {
		return nil, err
	}

	v := viper.New()
	setDefaults(v)
	v.SetConfigFile(homeConfigPath(homeDir))
	v.SetConfigType("toml")

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read config file: %w", err)
		}
	}

	var cfg Config
	decodeHook := mapstructure.ComposeDecodeHookFunc(
		expandEnvStringHook(),
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	)

	if err := v.Unmarshal(&cfg, func(c *mapstructure.DecoderConfig) {
		c.DecodeHook = decodeHook
	}); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	applyZeroValueDefaults(&cfg)
	cfg.HomeDir = homeDir
	cfg.Agent = defaultAgent
	cfg.Security.Workspace = cfg.WorkspaceDir()

	return &cfg, nil
}

// Write writes the merged configuration (defaults overlaid by user
// config) to w in TOML format.
func Write(w io.Writer) error {
	if w == nil {
		return errors.New("writer is required")
	}

	homeDir, err := homeDir()
	if err != nil {
		return err
	}

	v := viper.New()
	setDefaults(v)
	v.SetConfigFile(homeConfigPath(homeDir))
	v.SetConfigType("toml")

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read config file: %w", err)
		}
	}

	// Keep duration fields human-readable in generated TOML.
	v.Set("llm.default.request_timeout", v.GetDuration("llm.default.request_timeout").String())
	v.Set("security.command_timeout", v.GetDuration("security.command_timeout").String())

	if err := v.WriteConfigTo(w); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// DefaultUserConfigTOML renders the minimal bootstrap user config as TOML.
func DefaultUserConfigTOML() (string, error) {
	v := viper.New()
	v.SetConfigType("toml")

	for profile, llm := range defaultUserConfig.LLM {
		v.Set("llm."+profile+".api_key", llm.APIKey)
		v.Set("llm."+profile+".provider", llm.Provider)
		v.Set("llm."+profile+".model", llm.Model)
		v.Set("llm."+profile+".request_timeout", llm.RequestTimeout.String())
	}
	for channel, ch := range defaultUserConfig.Channels {
		v.Set("channels."+channel+".enabled", ch.Enabled)
		v.Set("channels."+channel+".token", ch.Token)
	}
	v.Set("costs.daily_limit", defaultUserConfig.Costs.DailyLimit)
	v.Set("costs.monthly_limit", defaultUserConfig.Costs.MonthlyLimit)
	v.Set("security.mode", defaultUserConfig.Security.Mode)
	v.Set("security.command_timeout", defaultUserConfig.Security.CommandTimeout.String())

	var out bytes.Buffer
	if err := v.WriteConfigTo(&out); err != nil {
		return "", fmt.Errorf("write default user config: %w", err)
	}
	return out.String(), nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("channels.telegram.enabled", defaultConfig.Channels["telegram"].Enabled)
	v.SetDefault("channels.telegram.token", defaultConfig.Channels["telegram"].Token)

	v.SetDefault("llm.default.api_key", defaultConfig.LLM["default"].APIKey)
	v.SetDefault("llm.default.provider", defaultConfig.LLM["default"].Provider)
	v.SetDefault("llm.default.model", defaultConfig.LLM["default"].Model)
	v.SetDefault("llm.default.max_tokens", defaultConfig.LLM["default"].MaxTokens)
	v.SetDefault("llm.default.request_timeout", defaultConfig.LLM["default"].RequestTimeout)

	v.SetDefault("security.command_timeout", defaultConfig.Security.CommandTimeout)
	v.SetDefault("security.mode", defaultConfig.Security.Mode)

	v.SetDefault("costs.daily_limit", defaultConfig.Costs.DailyLimit)
	v.SetDefault("costs.monthly_limit", defaultConfig.Costs.MonthlyLimit)

	v.SetDefault("context.max_tokens", defaultConfig.Context.MaxTokens)
	v.SetDefault("context.recent_messages", defaultConfig.Context.RecentMessages)
	v.SetDefault("context.max_tool_calls", defaultConfig.Context.MaxToolCalls)
	v.SetDefault("context.tool_output_length", defaultConfig.Context.ToolOutputLength)
	v.SetDefault("context.daily_log_lookback_days", defaultConfig.Context.DailyLogLookbackDays)

	v.SetDefault("web.search.provider", defaultConfig.Web.Search.Provider)
	v.SetDefault("web.search.api_key", defaultConfig.Web.Search.APIKey)
}

// applyZeroValueDefaults replaces explicit zero numeric config values with runtime defaults.
func applyZeroValueDefaults(cfg *Config) {
	if cfg == nil {
		return
	}

	if cfg.Security.CommandTimeout == 0 {
		cfg.Security.CommandTimeout = defaultConfig.Security.CommandTimeout
	}

	if cfg.Context.MaxTokens == 0 {
		cfg.Context.MaxTokens = defaultConfig.Context.MaxTokens
	}
	if cfg.Context.RecentMessages == 0 {
		cfg.Context.RecentMessages = defaultConfig.Context.RecentMessages
	}
	if cfg.Context.MaxToolCalls == 0 {
		cfg.Context.MaxToolCalls = defaultConfig.Context.MaxToolCalls
	}
	if cfg.Context.ToolOutputLength == 0 {
		cfg.Context.ToolOutputLength = defaultConfig.Context.ToolOutputLength
	}
	if cfg.Context.DailyLogLookbackDays == 0 {
		cfg.Context.DailyLogLookbackDays = defaultConfig.Context.DailyLogLookbackDays
	}

	for name, llm := range cfg.LLM {
		if llm.MaxTokens == 0 {
			llm.MaxTokens = defaultConfig.LLM["default"].MaxTokens
		}
		if llm.RequestTimeout == 0 {
			llm.RequestTimeout = defaultConfig.LLM["default"].RequestTimeout
		}
		cfg.LLM[name] = llm
	}
}

// DefaultLLM returns the default LLM profile with fallback defaults.
func (c *Config) DefaultLLM() LLMProviderConfig {
	if llm, ok := c.LLM["default"]; ok {
		return llm
	}
	return defaultConfig.LLM["default"]
}

// TelegramChannel returns Telegram channel config with fallback defaults.
func (c *Config) TelegramChannel() ChannelConfig {
	if ch, ok := c.Channels["telegram"]; ok {
		return ch
	}
	return defaultConfig.Channels["telegram"]
}

func validateSecurityMode(mode string) error {
	switch mode {
	case SecurityModeStandard, SecurityModeDanger, SecurityModeStrict:
		return nil
	default:
		return fmt.Errorf("invalid security.mode %s (allowed: %s, %s, %s)", mode, SecurityModeStandard, SecurityModeDanger, SecurityModeStrict)
	}
}

// Validatable is implemented by config sections that can self-validate.
type Validatable interface {
	Validate() error
}

// Validate checks required LLM provider fields and provider-specific rules.
func (c LLMProviderConfig) Validate() error {
	if c.Provider == "" {
		return errors.New("provider is required")
	}
	if c.Model == "" {
		return errors.New("model is required")
	}
	if c.MaxTokens < 0 {
		return errors.New("max_tokens must be >= 0")
	}
	if c.RequestTimeout < 0 {
		return errors.New("request_timeout must be >= 0")
	}

	switch c.Provider {
	case "anthropic", "openrouter":
		if c.APIKey == "" {
			return errors.New("api_key is required")
		}
	case "ollama":
		// Local provider, no API key required.
	default:
		return fmt.Errorf("unsupported provider %s", c.Provider)
	}
	return nil
}

// Validate checks required channel fields when the channel is enabled.
func (c ChannelConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Token == "" {
		return errors.New("token is required when enabled=true")
	}
	return nil
}

// Validate checks security mode values.
func (c SecurityConfig) Validate() error {
	if err := validateSecurityMode(c.Mode); err != nil {
		return err
	}
	if c.CommandTimeout < 0 {
		return errors.New("command_timeout must be >= 0")
	}
	return nil
}

// Validate validates cost limits.
func (c CostsConfig) Validate() error {
	if c.DailyLimit < 0 {
		return errors.New("daily_limit must be >= 0")
	}
	if c.MonthlyLimit < 0 {
		return errors.New("monthly_limit must be >= 0")
	}
	if c.DailyLimit > 0 && c.MonthlyLimit > 0 && c.DailyLimit > c.MonthlyLimit {
		return errors.New("daily_limit cannot be greater than monthly_limit")
	}
	return nil
}

// Validate validates context settings.
func (c ContextConfig) Validate() error {
	if c.MaxTokens < 0 {
		return errors.New("max_tokens must be >= 0")
	}
	if c.RecentMessages < 0 {
		return errors.New("recent_messages must be >= 0")
	}
	if c.MaxToolCalls < 0 {
		return errors.New("max_tool_calls must be >= 0")
	}
	if c.ToolOutputLength < 0 {
		return errors.New("tool_output_length must be >= 0")
	}
	if c.DailyLogLookbackDays < 0 {
		return errors.New("daily_log_lookback_days must be >= 0")
	}
	return nil
}

// Validate validates web settings.
func (c WebConfig) Validate() error {
	switch strings.ToLower(strings.TrimSpace(c.Search.Provider)) {
	case "", "brave":
		return nil
	default:
		return fmt.Errorf("unsupported web.search.provider %s", c.Search.Provider)
	}
}

func (cfg *Config) firstValidationError() error {
	var errs []error

	if len(cfg.LLM) == 0 {
		errs = append(errs, errors.New("at least one llm.* profile is required"))
	}
	if len(cfg.Channels) == 0 {
		errs = append(errs, errors.New("at least one channels.* entry is required"))
	}

	if err := cfg.Security.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("security: %w", err))
	}
	if err := cfg.Costs.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("costs: %w", err))
	}
	if err := cfg.Context.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("context: %w", err))
	}
	if err := cfg.Web.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("web: %w", err))
	}

	for name, llmCfg := range cfg.LLM {
		if err := llmCfg.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("llm.%s: %w", name, err))
		}
	}
	for name, chCfg := range cfg.Channels {
		if err := chCfg.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("channels.%s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Validate validates startup configuration and returns the first fatal error.
func (cfg *Config) Validate() error {
	cfgCopy := *cfg
	applyZeroValueDefaults(&cfgCopy)
	if err := cfgCopy.firstValidationError(); err != nil {
		return err
	}
	return nil
}

func expandEnvStringHook() mapstructure.DecodeHookFuncType {
	return func(from reflect.Type, to reflect.Type, data any) (any, error) {
		if from.Kind() != reflect.String || to.Kind() != reflect.String {
			return data, nil
		}
		value, ok := data.(string)
		if !ok {
			return data, nil
		}
		return os.ExpandEnv(value), nil
	}
}
