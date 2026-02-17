package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_FileOverridesDefaults(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), ".betterclaw")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	t.Setenv("BETTERCLAW_HOME", dataDir)

	configBody := `
[llm.default]
api_key = "test-key"
provider = "openrouter"
model = "deepseek/deepseek-chat"

[channels.telegram]
enabled = false
token = "bot-token"
allowed_users = [123]
`
	if err := os.WriteFile(filepath.Join(dataDir, "config.toml"), []byte(configBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	llm := cfg.DefaultLLM()
	if llm.APIKey != "test-key" {
		t.Fatalf("expected api key %q, got %q", "test-key", llm.APIKey)
	}
	if llm.Provider != "openrouter" {
		t.Fatalf("expected provider %q, got %q", "openrouter", llm.Provider)
	}
	if llm.Model != "deepseek/deepseek-chat" {
		t.Fatalf("expected model %q, got %q", "deepseek/deepseek-chat", llm.Model)
	}

	telegram := cfg.TelegramChannel()
	if telegram.Enabled {
		t.Fatalf("expected telegram channel to be disabled from file")
	}
	if telegram.Token != "bot-token" {
		t.Fatalf("expected telegram token from file, got %q", telegram.Token)
	}
	if len(telegram.AllowedUsers) != 1 || telegram.AllowedUsers[0] != 123 {
		t.Fatalf("expected allowed_users [123], got %v", telegram.AllowedUsers)
	}
}

func TestLoad_ExpandsEnvVarsInStringValues(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), ".betterclaw")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	t.Setenv("BETTERCLAW_HOME", dataDir)
	t.Setenv("ANTHROPIC_API_KEY", "expanded-key")

	configBody := `
[llm.default]
api_key = "$ANTHROPIC_API_KEY"
provider = "anthropic"
model = "claude-sonnet-4-6"
`
	if err := os.WriteFile(filepath.Join(dataDir, "config.toml"), []byte(configBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DefaultLLM().APIKey != "expanded-key" {
		t.Fatalf("expected expanded api key %q, got %q", "expanded-key", cfg.DefaultLLM().APIKey)
	}
}

func TestLoad_DefaultsApplyWithoutConfigFile(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), ".betterclaw")
	t.Setenv("BETTERCLAW_HOME", dataDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Agent != defaultAgent {
		t.Fatalf("expected default agent %q, got %q", defaultAgent, cfg.Agent)
	}
	if cfg.DataDir != dataDir {
		t.Fatalf("expected data dir %q, got %q", dataDir, cfg.DataDir)
	}
	llm := cfg.DefaultLLM()
	if llm.Provider != defaultConfig.LLM[defaultLLMProfile].Provider {
		t.Fatalf("expected default provider %q, got %q", defaultConfig.LLM[defaultLLMProfile].Provider, llm.Provider)
	}
	if llm.Model != defaultConfig.LLM[defaultLLMProfile].Model {
		t.Fatalf("expected default model %q, got %q", defaultConfig.LLM[defaultLLMProfile].Model, llm.Model)
	}
	if llm.MaxTokens != defaultConfig.LLM[defaultLLMProfile].MaxTokens {
		t.Fatalf("expected default max tokens %d, got %d", defaultConfig.LLM[defaultLLMProfile].MaxTokens, llm.MaxTokens)
	}
	if cfg.Security.CommandTimeout != 5*time.Minute {
		t.Fatalf("expected default timeout 5m, got %v", cfg.Security.CommandTimeout)
	}
	if cfg.Security.Mode != SecurityModeStandard {
		t.Fatalf("expected default security mode %q, got %q", SecurityModeStandard, cfg.Security.Mode)
	}
	if cfg.Costs.MaxContextTokens != defaultConfig.Costs.MaxContextTokens {
		t.Fatalf("expected default context max tokens %d, got %d", defaultConfig.Costs.MaxContextTokens, cfg.Costs.MaxContextTokens)
	}
	if cfg.Costs.RecentMessages != defaultConfig.Costs.RecentMessages {
		t.Fatalf("expected default recent messages %d, got %d", defaultConfig.Costs.RecentMessages, cfg.Costs.RecentMessages)
	}
	expectedWorkspace := filepath.Join(dataDir, "agents", defaultAgent, "workspace")
	if cfg.Security.Workspace != expectedWorkspace {
		t.Fatalf("expected derived workspace %q, got %q", expectedWorkspace, cfg.Security.Workspace)
	}

	telegram := cfg.TelegramChannel()
	if !telegram.Enabled {
		t.Fatalf("expected default telegram channel enabled")
	}
	if telegram.Token != "" {
		t.Fatalf("expected default empty token, got %q", telegram.Token)
	}
}

func TestLoad_ValidSecurityModeFromFile(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), ".betterclaw")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	t.Setenv("BETTERCLAW_HOME", dataDir)

	configBody := `
[security]
mode = "strict"
`
	if err := os.WriteFile(filepath.Join(dataDir, "config.toml"), []byte(configBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Security.Mode != SecurityModeStrict {
		t.Fatalf("expected security mode %q, got %q", SecurityModeStrict, cfg.Security.Mode)
	}
}

func TestLoad_InvalidSecurityModeDoesNotFail(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), ".betterclaw")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	t.Setenv("BETTERCLAW_HOME", dataDir)

	configBody := `
[security]
mode = "banana"
`
	if err := os.WriteFile(filepath.Join(dataDir, "config.toml"), []byte(configBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Security.Mode != "banana" {
		t.Fatalf("expected raw security mode to be loaded for startup validation, got %q", cfg.Security.Mode)
	}
}

func TestLoad_BetterclawHomeOverridesDefault(t *testing.T) {
	customDir := filepath.Join(t.TempDir(), "custom-home")
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		t.Fatalf("mkdir custom dir: %v", err)
	}
	t.Setenv("BETTERCLAW_HOME", customDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DataDir != customDir {
		t.Fatalf("expected data dir %q, got %q", customDir, cfg.DataDir)
	}
}

func TestHomeDir_DefaultsToUserHome(t *testing.T) {
	t.Setenv("BETTERCLAW_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("get user home: %v", err)
	}

	dir, err := HomeDir()
	if err != nil {
		t.Fatalf("home dir: %v", err)
	}
	expected := filepath.Join(home, ".betterclaw")
	if dir != expected {
		t.Fatalf("expected %q, got %q", expected, dir)
	}
}

func TestHomeDir_RespectsEnvVar(t *testing.T) {
	customDir := "/tmp/my-betterclaw"
	t.Setenv("BETTERCLAW_HOME", customDir)

	dir, err := HomeDir()
	if err != nil {
		t.Fatalf("home dir: %v", err)
	}
	if dir != customDir {
		t.Fatalf("expected %q, got %q", customDir, dir)
	}
}
