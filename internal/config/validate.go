package config

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/machinae/betterclaw/internal/logging"
)

// Validatable is implemented by config sections that can self-validate.
type Validatable interface {
	Validate() error
}

func (c LLMProviderConfig) Validate() error {
	if c.Provider == "" {
		return errors.New("provider is required")
	}
	if c.Model == "" {
		return errors.New("model is required")
	}

	switch c.Provider {
	case "anthropic", "openrouter":
		if c.APIKey == "" {
			return errors.New("api_key is required")
		}
	case "ollama":
		// Local provider, no API key required.
	default:
		return fmt.Errorf("unsupported provider %q", c.Provider)
	}
	return nil
}

func (c ChannelConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Token == "" {
		return errors.New("token is required when enabled=true")
	}
	return nil
}

func (c SecurityConfig) Validate() error {
	return validateSecurityMode(c.Mode)
}

func (c CostsConfig) Validate() error {
	return nil
}

func (c WebConfig) Validate() error {
	return nil
}

// ValidateStartup validates startup configuration and logs warning-level issues.
func ValidateStartup(cfg *Config) error {
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
		if name == defaultTelegramChannel && chCfg.Enabled && len(chCfg.AllowedUsers) == 0 {
			logging.Logger().Warn("channels.telegram.allowed_users is empty. You will not be able to use Telegram until you set this value")
		}
	}

	if runtime.GOOS == "linux" && !isLandlockAvailable() {
		logging.Logger().Warn("landlock is unavailable on this host. Upgrade your linux version for higher security")
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
