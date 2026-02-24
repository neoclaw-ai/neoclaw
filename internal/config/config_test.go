package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoad_FileOverridesDefaults(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), ".neoclaw")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	t.Setenv("NEOCLAW_HOME", dataDir)

	configBody := `
[llm.default]
api_key = "test-key"
provider = "openrouter"
model = "deepseek/deepseek-chat"
request_timeout = "45s"

[channels.telegram]
enabled = false
token = "bot-token"
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
	if llm.RequestTimeout != 45*time.Second {
		t.Fatalf("expected request timeout %v, got %v", 45*time.Second, llm.RequestTimeout)
	}

	telegram := cfg.TelegramChannel()
	if telegram.Enabled {
		t.Fatalf("expected telegram channel to be disabled from file")
	}
	if telegram.Token != "bot-token" {
		t.Fatalf("expected telegram token from file, got %q", telegram.Token)
	}
}

func TestLoad_ExpandsEnvVarsInStringValues(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), ".neoclaw")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	t.Setenv("NEOCLAW_HOME", dataDir)
	t.Setenv("ANTHROPIC_API_KEY", "expanded-key")
	t.Setenv("BRAVE_API_KEY", "expanded-brave-key")

	configBody := `
	[llm.default]
	api_key = "$ANTHROPIC_API_KEY"
	provider = "anthropic"
	model = "claude-sonnet-4-6"

	[web.search]
	api_key = "$BRAVE_API_KEY"
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
	if cfg.Web.Search.APIKey != "expanded-brave-key" {
		t.Fatalf("expected expanded web search api key %q, got %q", "expanded-brave-key", cfg.Web.Search.APIKey)
	}
}

func TestLoad_DefaultsApplyWithoutConfigFile(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), ".neoclaw")
	t.Setenv("NEOCLAW_HOME", dataDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Agent != defaultAgent {
		t.Fatalf("expected default agent %q, got %q", defaultAgent, cfg.Agent)
	}
	if cfg.HomeDir != dataDir {
		t.Fatalf("expected home dir %q, got %q", dataDir, cfg.HomeDir)
	}
	expectedDataDir := filepath.Join(dataDir, "data")
	if cfg.DataDir() != expectedDataDir {
		t.Fatalf("expected data dir %q, got %q", expectedDataDir, cfg.DataDir())
	}
	llm := cfg.DefaultLLM()
	if llm.Provider != defaultConfig.LLM["default"].Provider {
		t.Fatalf("expected default provider %q, got %q", defaultConfig.LLM["default"].Provider, llm.Provider)
	}
	if llm.Model != defaultConfig.LLM["default"].Model {
		t.Fatalf("expected default model %q, got %q", defaultConfig.LLM["default"].Model, llm.Model)
	}
	if llm.MaxTokens != defaultConfig.LLM["default"].MaxTokens {
		t.Fatalf("expected default max tokens %d, got %d", defaultConfig.LLM["default"].MaxTokens, llm.MaxTokens)
	}
	if llm.RequestTimeout != defaultConfig.LLM["default"].RequestTimeout {
		t.Fatalf("expected default request timeout %v, got %v", defaultConfig.LLM["default"].RequestTimeout, llm.RequestTimeout)
	}
	if cfg.Security.CommandTimeout != 5*time.Minute {
		t.Fatalf("expected default timeout 5m, got %v", cfg.Security.CommandTimeout)
	}
	if cfg.Security.Mode != SecurityModeStandard {
		t.Fatalf("expected default security mode %q, got %q", SecurityModeStandard, cfg.Security.Mode)
	}
	if cfg.Context.MaxTokens != defaultConfig.Context.MaxTokens {
		t.Fatalf("expected default context max tokens %d, got %d", defaultConfig.Context.MaxTokens, cfg.Context.MaxTokens)
	}
	if cfg.Context.RecentMessages != defaultConfig.Context.RecentMessages {
		t.Fatalf("expected default recent messages %d, got %d", defaultConfig.Context.RecentMessages, cfg.Context.RecentMessages)
	}
	expectedWorkspace := filepath.Join(expectedDataDir, "agents", defaultAgent, "workspace")
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
	dataDir := filepath.Join(t.TempDir(), ".neoclaw")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	t.Setenv("NEOCLAW_HOME", dataDir)

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
	dataDir := filepath.Join(t.TempDir(), ".neoclaw")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	t.Setenv("NEOCLAW_HOME", dataDir)

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

func TestLoad_ZeroNumericValuesFallbackToDefaults(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), ".neoclaw")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	t.Setenv("NEOCLAW_HOME", dataDir)

	configBody := `
[llm.default]
api_key = "test-key"
provider = "anthropic"
model = "claude-sonnet-4-6"
max_tokens = 0
request_timeout = "0s"

[security]
command_timeout = "0s"

[context]
max_tokens = 0
recent_messages = 0
max_tool_calls = 0
tool_output_length = 0
daily_log_lookback = "0s"
`
	if err := os.WriteFile(filepath.Join(dataDir, "config.toml"), []byte(configBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	llm := cfg.DefaultLLM()
	if llm.MaxTokens != defaultConfig.LLM["default"].MaxTokens {
		t.Fatalf("expected default llm max tokens %d, got %d", defaultConfig.LLM["default"].MaxTokens, llm.MaxTokens)
	}
	if llm.RequestTimeout != defaultConfig.LLM["default"].RequestTimeout {
		t.Fatalf("expected default request timeout %v, got %v", defaultConfig.LLM["default"].RequestTimeout, llm.RequestTimeout)
	}
	if cfg.Security.CommandTimeout != defaultConfig.Security.CommandTimeout {
		t.Fatalf("expected default command timeout %v, got %v", defaultConfig.Security.CommandTimeout, cfg.Security.CommandTimeout)
	}
	if cfg.Context.MaxTokens != defaultConfig.Context.MaxTokens {
		t.Fatalf("expected default context max tokens %d, got %d", defaultConfig.Context.MaxTokens, cfg.Context.MaxTokens)
	}
	if cfg.Context.RecentMessages != defaultConfig.Context.RecentMessages {
		t.Fatalf("expected default context recent_messages %d, got %d", defaultConfig.Context.RecentMessages, cfg.Context.RecentMessages)
	}
	if cfg.Context.MaxToolCalls != defaultConfig.Context.MaxToolCalls {
		t.Fatalf("expected default context max_tool_calls %d, got %d", defaultConfig.Context.MaxToolCalls, cfg.Context.MaxToolCalls)
	}
	if cfg.Context.ToolOutputLength != defaultConfig.Context.ToolOutputLength {
		t.Fatalf("expected default context tool_output_length %d, got %d", defaultConfig.Context.ToolOutputLength, cfg.Context.ToolOutputLength)
	}
	if cfg.Context.DailyLogLookback != defaultConfig.Context.DailyLogLookback {
		t.Fatalf("expected default context daily_log_lookback %v, got %v", defaultConfig.Context.DailyLogLookback, cfg.Context.DailyLogLookback)
	}
}

func TestLoad_NeoClawHomeOverridesDefault(t *testing.T) {
	customDir := filepath.Join(t.TempDir(), "custom-home")
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		t.Fatalf("mkdir custom dir: %v", err)
	}
	t.Setenv("NEOCLAW_HOME", customDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	expectedDataDir := filepath.Join(customDir, "data")
	if cfg.DataDir() != expectedDataDir {
		t.Fatalf("expected data dir %q, got %q", expectedDataDir, cfg.DataDir())
	}
}

func TestHomeDir_DefaultsToUserHome(t *testing.T) {
	t.Setenv("NEOCLAW_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("get user home: %v", err)
	}

	dir, err := homeDir()
	if err != nil {
		t.Fatalf("home dir: %v", err)
	}
	expected := filepath.Join(home, ".neoclaw")
	if dir != expected {
		t.Fatalf("expected %q, got %q", expected, dir)
	}
}

func TestHomeDir_RespectsEnvVar(t *testing.T) {
	customDir := "/tmp/my-neoclaw"
	t.Setenv("NEOCLAW_HOME", customDir)

	dir, err := homeDir()
	if err != nil {
		t.Fatalf("home dir: %v", err)
	}
	if dir != customDir {
		t.Fatalf("expected %q, got %q", customDir, dir)
	}
}

func TestPathResolutionMethods(t *testing.T) {
	cfg := &Config{HomeDir: "/tmp/neoclaw", Agent: "default"}

	if cfg.ConfigPath() != "/tmp/neoclaw/config.toml" {
		t.Fatalf("unexpected config path: %q", cfg.ConfigPath())
	}
	if cfg.DataDir() != "/tmp/neoclaw/data" {
		t.Fatalf("unexpected data dir: %q", cfg.DataDir())
	}
	if cfg.PolicyDir() != "/tmp/neoclaw/data/policy" {
		t.Fatalf("unexpected policy dir: %q", cfg.PolicyDir())
	}
	if cfg.AllowedCommandsPath() != "/tmp/neoclaw/data/policy/allowed_commands.json" {
		t.Fatalf("unexpected allowed commands path: %q", cfg.AllowedCommandsPath())
	}
	if cfg.AllowedDomainsPath() != "/tmp/neoclaw/data/policy/allowed_domains.json" {
		t.Fatalf("unexpected allowed domains path: %q", cfg.AllowedDomainsPath())
	}
	if cfg.AllowedUsersPath() != "/tmp/neoclaw/data/policy/allowed_users.json" {
		t.Fatalf("unexpected allowed users path: %q", cfg.AllowedUsersPath())
	}
	if cfg.CostsPath() != "/tmp/neoclaw/data/logs/costs.jsonl" {
		t.Fatalf("unexpected costs path: %q", cfg.CostsPath())
	}
	if cfg.PIDPath() != "/tmp/neoclaw/data/claw.pid" {
		t.Fatalf("unexpected pid path: %q", cfg.PIDPath())
	}
	if cfg.SoulPath() != "/tmp/neoclaw/data/agents/default/SOUL.md" {
		t.Fatalf("unexpected soul path: %q", cfg.SoulPath())
	}
	if cfg.UserPath() != "/tmp/neoclaw/data/agents/default/USER.md" {
		t.Fatalf("unexpected user path: %q", cfg.UserPath())
	}
	if cfg.MemoryPath() != "/tmp/neoclaw/data/agents/default/memory/memory.md" {
		t.Fatalf("unexpected memory path: %q", cfg.MemoryPath())
	}
	if cfg.CLIContextPath() != "/tmp/neoclaw/data/agents/default/sessions/cli/default.jsonl" {
		t.Fatalf("unexpected cli context path: %q", cfg.CLIContextPath())
	}
}

func TestWrite_PrintsDefaultsAndOverrides(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), ".neoclaw")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	t.Setenv("NEOCLAW_HOME", dataDir)

	configBody := `
[llm.default]
api_key = "test-key"
provider = "openrouter"
model = "deepseek/deepseek-chat"
`
	if err := os.WriteFile(filepath.Join(dataDir, "config.toml"), []byte(configBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var out bytes.Buffer
	if err := Write(&out); err != nil {
		t.Fatalf("write merged toml: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "[llm.default]") {
		t.Fatalf("expected llm.default section, got %q", got)
	}
	if !strings.Contains(got, "provider = 'openrouter'") {
		t.Fatalf("expected override provider in output, got %q", got)
	}
	if !strings.Contains(got, "[costs]") {
		t.Fatalf("expected defaults section costs in output, got %q", got)
	}
}
