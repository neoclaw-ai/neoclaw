package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

const defaultAgent = "default"

const (
	// SecurityModeStandard is the default sandbox/security behavior.
	SecurityModeStandard = "standard"
	// SecurityModeDangerFullAccess disables sandbox protections.
	SecurityModeDangerFullAccess = "danger-full-access"
	// SecurityModeStrict enables stricter sandbox policy where supported.
	SecurityModeStrict     = "strict"
	defaultLLMProfile      = "default"
	defaultTelegramChannel = "telegram"
)

// Config is the runtime configuration loaded from defaults, config.toml, and env vars.
type Config struct {
	// DataDir is runtime-resolved from BETTERCLAW_HOME and not read from config.
	DataDir string `mapstructure:"-"`
	// Agent is runtime-selected (MVP default: "default"), not read from config.
	Agent    string                       `mapstructure:"-"`
	Channels map[string]ChannelConfig     `mapstructure:"channels"`
	LLM      map[string]LLMProviderConfig `mapstructure:"llm"`
	Security SecurityConfig               `mapstructure:"security"`
	Costs    CostsConfig                  `mapstructure:"costs"`
	Web      WebConfig                    `mapstructure:"web"`
}

// ChannelConfig configures one inbound/outbound channel.
type ChannelConfig struct {
	Enabled      bool    `mapstructure:"enabled"`
	Token        string  `mapstructure:"token"`
	AllowedUsers []int64 `mapstructure:"allowed_users"`
}

// LLMProviderConfig configures one LLM provider profile.
type LLMProviderConfig struct {
	APIKey    string `mapstructure:"api_key"`
	Provider  string `mapstructure:"provider"`
	Model     string `mapstructure:"model"`
	MaxTokens int    `mapstructure:"max_tokens"`
}

// SecurityConfig controls command execution and sandbox behavior.
type SecurityConfig struct {
	// Workspace is derived from DataDir and Agent and is not configurable.
	Workspace      string        `mapstructure:"-"`
	CommandTimeout time.Duration `mapstructure:"command_timeout"`
	Mode           string        `mapstructure:"mode"`
}

// CostsConfig defines soft spending and circuit-breaker limits.
type CostsConfig struct {
	HourlyLimit            float64       `mapstructure:"hourly_limit"`
	DailyLimit             float64       `mapstructure:"daily_limit"`
	MonthlyLimit           float64       `mapstructure:"monthly_limit"`
	CircuitBreakerMaxCalls int           `mapstructure:"circuit_breaker_max_calls"`
	CircuitBreakerWindow   time.Duration `mapstructure:"circuit_breaker_window"`
	MaxContextTokens       int           `mapstructure:"max_context_tokens"`
	RecentMessages         int           `mapstructure:"recent_messages"`
}

// WebConfig configures built-in web tool behavior.
type WebConfig struct {
	Search WebSearchConfig `mapstructure:"search"`
}

// WebSearchConfig configures the web search provider.
type WebSearchConfig struct {
	Provider string `mapstructure:"provider"`
}

var defaultConfig = Config{
	Channels: map[string]ChannelConfig{
		defaultTelegramChannel: {
			Enabled:      true,
			Token:        "",
			AllowedUsers: []int64{},
		},
	},
	LLM: map[string]LLMProviderConfig{
		defaultLLMProfile: {
			APIKey:    "",
			Provider:  "anthropic",
			Model:     "claude-sonnet-4-6",
			MaxTokens: 8192,
		},
	},
	Security: SecurityConfig{
		CommandTimeout: 5 * time.Minute,
		Mode:           SecurityModeStandard,
	},
	Costs: CostsConfig{
		HourlyLimit:            2.0,
		DailyLimit:             20.0,
		MonthlyLimit:           100.0,
		CircuitBreakerMaxCalls: 10,
		CircuitBreakerWindow:   60 * time.Second,
		MaxContextTokens:       4000,
		RecentMessages:         10,
	},
	Web: WebConfig{
		Search: WebSearchConfig{
			Provider: "brave",
		},
	},
}

// HomeDir returns the BetterClaw home directory.
// Uses BETTERCLAW_HOME env var if set, otherwise defaults to ~/.betterclaw.
func HomeDir() (string, error) {
	if dir := os.Getenv("BETTERCLAW_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".betterclaw"), nil
}

// Load merges hardcoded defaults and config file values in that order.
// The data directory is determined by BETTERCLAW_HOME (default: ~/.betterclaw).
// Config is always at $BETTERCLAW_HOME/config.toml.
func Load() (*Config, error) {
	dataDir, err := HomeDir()
	if err != nil {
		return nil, err
	}

	v := viper.New()
	setDefaults(v, dataDir)
	v.SetConfigFile(filepath.Join(dataDir, "config.toml"))
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
	cfg.DataDir = dataDir
	cfg.Agent = defaultAgent
	cfg.Security.Workspace = cfg.WorkspaceDir()

	return &cfg, nil
}

func setDefaults(v *viper.Viper, dataDir string) {
	v.SetDefault("channels."+defaultTelegramChannel+".enabled", defaultConfig.Channels[defaultTelegramChannel].Enabled)
	v.SetDefault("channels."+defaultTelegramChannel+".token", defaultConfig.Channels[defaultTelegramChannel].Token)
	v.SetDefault("channels."+defaultTelegramChannel+".allowed_users", defaultConfig.Channels[defaultTelegramChannel].AllowedUsers)

	v.SetDefault("llm."+defaultLLMProfile+".api_key", defaultConfig.LLM[defaultLLMProfile].APIKey)
	v.SetDefault("llm."+defaultLLMProfile+".provider", defaultConfig.LLM[defaultLLMProfile].Provider)
	v.SetDefault("llm."+defaultLLMProfile+".model", defaultConfig.LLM[defaultLLMProfile].Model)
	v.SetDefault("llm."+defaultLLMProfile+".max_tokens", defaultConfig.LLM[defaultLLMProfile].MaxTokens)

	v.SetDefault("security.command_timeout", defaultConfig.Security.CommandTimeout)
	v.SetDefault("security.mode", defaultConfig.Security.Mode)

	v.SetDefault("costs.hourly_limit", defaultConfig.Costs.HourlyLimit)
	v.SetDefault("costs.daily_limit", defaultConfig.Costs.DailyLimit)
	v.SetDefault("costs.monthly_limit", defaultConfig.Costs.MonthlyLimit)
	v.SetDefault("costs.circuit_breaker_max_calls", defaultConfig.Costs.CircuitBreakerMaxCalls)
	v.SetDefault("costs.circuit_breaker_window", defaultConfig.Costs.CircuitBreakerWindow)
	v.SetDefault("costs.max_context_tokens", defaultConfig.Costs.MaxContextTokens)
	v.SetDefault("costs.recent_messages", defaultConfig.Costs.RecentMessages)

	v.SetDefault("web.search.provider", defaultConfig.Web.Search.Provider)
}

// AgentDir returns the active agent directory under DataDir.
func (c *Config) AgentDir() string {
	return filepath.Join(c.DataDir, "agents", c.Agent)
}

// WorkspaceDir returns the active agent workspace directory.
func (c *Config) WorkspaceDir() string {
	return filepath.Join(c.AgentDir(), "workspace")
}

// DefaultLLM returns the default LLM profile with fallback defaults.
func (c *Config) DefaultLLM() LLMProviderConfig {
	if llm, ok := c.LLM[defaultLLMProfile]; ok {
		return llm
	}
	return defaultConfig.LLM[defaultLLMProfile]
}

// TelegramChannel returns Telegram channel config with fallback defaults.
func (c *Config) TelegramChannel() ChannelConfig {
	if ch, ok := c.Channels[defaultTelegramChannel]; ok {
		return ch
	}
	return defaultConfig.Channels[defaultTelegramChannel]
}

func validateSecurityMode(mode string) error {
	switch mode {
	case SecurityModeStandard, SecurityModeDangerFullAccess, SecurityModeStrict:
		return nil
	default:
		return fmt.Errorf("invalid security.mode %q (allowed: %q, %q, %q)", mode, SecurityModeStandard, SecurityModeDangerFullAccess, SecurityModeStrict)
	}
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
