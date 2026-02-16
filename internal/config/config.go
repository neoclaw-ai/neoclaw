package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

const defaultAgent = "default"

// Config is the runtime configuration loaded from defaults, config.toml, and env vars.
type Config struct {
	// DataDir is runtime-resolved from BETTERCLAW_HOME and not read from config.
	DataDir  string         `mapstructure:"-"`
	Agent    string         `mapstructure:"agent"`
	Telegram TelegramConfig `mapstructure:"telegram"`
	LLM      LLMConfig      `mapstructure:"llm"`
	Security SecurityConfig `mapstructure:"security"`
	Network  NetworkConfig  `mapstructure:"network"`
	Costs    CostsConfig    `mapstructure:"costs"`
	Web      WebConfig      `mapstructure:"web"`
}

type TelegramConfig struct {
	Token        string  `mapstructure:"token"`
	AllowedUsers []int64 `mapstructure:"allowed_users"`
}

type LLMConfig struct {
	Provider string `mapstructure:"provider"`
	Model    string `mapstructure:"model"`
}

type SecurityConfig struct {
	Workspace       string        `mapstructure:"workspace"`
	AllowedBinaries []string      `mapstructure:"allowed_binaries"`
	CommandTimeout  time.Duration `mapstructure:"command_timeout"`
	RestrictReads   bool          `mapstructure:"restrict_reads"`
}

type NetworkConfig struct {
	AllowedDomains []string `mapstructure:"allowed_domains"`
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

// Load merges hardcoded defaults, config file values, and env vars in that order.
// The data directory is determined by BETTERCLAW_HOME (default: ~/.betterclaw).
// Config is always at $BETTERCLAW_HOME/config.toml, .env at $BETTERCLAW_HOME/.env.
func Load() (*Config, error) {
	dataDir, err := HomeDir()
	if err != nil {
		return nil, err
	}

	if err := godotenv.Load(filepath.Join(dataDir, ".env")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load .env: %w", err)
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
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	)

	if err := v.Unmarshal(&cfg, func(c *mapstructure.DecoderConfig) {
		c.DecodeHook = decodeHook
	}); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	cfg.DataDir = dataDir

	return &cfg, nil
}

func setDefaults(v *viper.Viper, dataDir string) {
	v.SetDefault("agent", defaultAgent)

	v.SetDefault("telegram.token", "")
	v.SetDefault("telegram.allowed_users", []int64{})

	v.SetDefault("llm.provider", "anthropic")
	v.SetDefault("llm.model", "claude-sonnet-4-5-20250514")

	v.SetDefault("security.workspace", filepath.Join(dataDir, "agents", defaultAgent, "workspace"))
	v.SetDefault("security.allowed_binaries", []string{"git", "go", "python3", "node", "cat", "ls", "grep", "find", "curl"})
	v.SetDefault("security.command_timeout", "5m")
	v.SetDefault("security.restrict_reads", false)

	v.SetDefault("network.allowed_domains", []string{"api.anthropic.com", "api.openrouter.ai"})

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
