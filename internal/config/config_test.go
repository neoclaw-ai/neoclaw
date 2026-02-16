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
agent = "custom-agent"

[llm]
provider = "openrouter"
model = "deepseek/deepseek-chat"
`
	if err := os.WriteFile(filepath.Join(dataDir, "config.toml"), []byte(configBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Agent != "custom-agent" {
		t.Fatalf("expected file agent %q, got %q", "custom-agent", cfg.Agent)
	}
	if cfg.LLM.Provider != "openrouter" {
		t.Fatalf("expected file provider %q, got %q", "openrouter", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "deepseek/deepseek-chat" {
		t.Fatalf("expected file model %q, got %q", "deepseek/deepseek-chat", cfg.LLM.Model)
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
	if cfg.LLM.Provider != "anthropic" {
		t.Fatalf("expected default provider %q, got %q", "anthropic", cfg.LLM.Provider)
	}
	if cfg.Security.CommandTimeout != 5*time.Minute {
		t.Fatalf("expected default timeout 5m, got %v", cfg.Security.CommandTimeout)
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
