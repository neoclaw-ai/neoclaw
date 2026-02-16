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
	SecurityModeStandard         = "standard"
	SecurityModeDangerFullAccess = "danger-full-access"
	SecurityModeStrict           = "strict"
	defaultLLMProfile            = "default"
	defaultLLMProvider           = "anthropic"
	defaultLLMModel              = "claude-sonnet-4-5"
	defaultTelegramChannel       = "telegram"
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

type ChannelConfig struct {
	Enabled      bool    `mapstructure:"enabled"`
	Token        string  `mapstructure:"token"`
	AllowedUsers []int64 `mapstructure:"allowed_users"`
}

type LLMProviderConfig struct {
	APIKey   string `mapstructure:"api_key"`
	Provider string `mapstructure:"provider"`
	Model    string `mapstructure:"model"`
}

type SecurityConfig struct {
	// Workspace is derived from DataDir and Agent and is not configurable.
	Workspace      string        `mapstructure:"-"`
	CommandTimeout time.Duration `mapstructure:"command_timeout"`
	Mode           string        `mapstructure:"mode"`
}

type CostsConfig struct {
	HourlyLimit            float64       `mapstructure:"hourly_limit"`
	DailyLimit             float64       `mapstructure:"daily_limit"`
	MonthlyLimit           float64       `mapstructure:"monthly_limit"`
	CircuitBreakerMaxCalls int           `mapstructure:"circuit_breaker_max_calls"`
	CircuitBreakerWindow   time.Duration `mapstructure:"circuit_breaker_window"`
}

type WebConfig struct {
	Search WebSearchConfig `mapstructure:"search"`
}

type WebSearchConfig struct {
	Provider string `mapstructure:"provider"`
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
	v.SetDefault("channels."+defaultTelegramChannel+".enabled", true)
	v.SetDefault("channels."+defaultTelegramChannel+".token", "")
	v.SetDefault("channels."+defaultTelegramChannel+".allowed_users", []int64{})

	v.SetDefault("llm."+defaultLLMProfile+".api_key", "")
	v.SetDefault("llm."+defaultLLMProfile+".provider", defaultLLMProvider)
	v.SetDefault("llm."+defaultLLMProfile+".model", defaultLLMModel)

	v.SetDefault("security.command_timeout", "5m")
	v.SetDefault("security.mode", SecurityModeStandard)

	v.SetDefault("costs.hourly_limit", 2.0)
	v.SetDefault("costs.daily_limit", 20.0)
	v.SetDefault("costs.monthly_limit", 100.0)
	v.SetDefault("costs.circuit_breaker_max_calls", 10)
	v.SetDefault("costs.circuit_breaker_window", "60s")

	v.SetDefault("web.search.provider", "brave")
}

func (c *Config) AgentDir() string {
	return filepath.Join(c.DataDir, "agents", c.Agent)
}

func (c *Config) WorkspaceDir() string {
	return filepath.Join(c.AgentDir(), "workspace")
}

func (c *Config) DefaultLLM() LLMProviderConfig {
	if llm, ok := c.LLM[defaultLLMProfile]; ok {
		return llm
	}
	return LLMProviderConfig{
		APIKey:   "",
		Provider: defaultLLMProvider,
		Model:    defaultLLMModel,
	}
}

func (c *Config) TelegramChannel() ChannelConfig {
	if ch, ok := c.Channels[defaultTelegramChannel]; ok {
		return ch
	}
	return ChannelConfig{
		Enabled:      true,
		Token:        "",
		AllowedUsers: []int64{},
	}
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
