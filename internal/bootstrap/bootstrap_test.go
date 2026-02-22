package bootstrap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/machinae/betterclaw/internal/config"
)

func TestInitializeCreatesRequiredFilesAndDirs(t *testing.T) {
	homeDir := filepath.Join(t.TempDir(), ".betterclaw")
	cfg := &config.Config{
		HomeDir: homeDir,
		Agent:   "default",
	}

	if err := Initialize(cfg); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	requiredPaths := []string{
		cfg.ConfigPath(),
		cfg.AllowedDomainsPath(),
		cfg.AllowedCommandsPath(),
		cfg.AllowedUsersPath(),
		cfg.LogsDir(),
		cfg.CostsPath(),
		cfg.SoulPath(),
		cfg.JobsPath(),
		cfg.MemoryPath(),
		cfg.CLIContextPath(),
		cfg.WorkspaceDir(),
	}

	for _, path := range requiredPaths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %q to exist: %v", path, err)
		}
	}

	soulPath := cfg.SoulPath()
	soulRaw, err := os.ReadFile(soulPath)
	if err != nil {
		t.Fatalf("read SOUL.md: %v", err)
	}
	soul := string(soulRaw)
	if !strings.Contains(soul, "## Persona") || !strings.Contains(soul, "## User") || !strings.Contains(soul, "## Preferences") {
		t.Fatalf("expected SOUL.md template sections, got %q", soul)
	}

	domainsPath := cfg.AllowedDomainsPath()
	domainsRaw, err := os.ReadFile(domainsPath)
	if err != nil {
		t.Fatalf("read allowed domains file: %v", err)
	}
	var domainsDoc map[string][]string
	if err := json.Unmarshal(domainsRaw, &domainsDoc); err != nil {
		t.Fatalf("parse allowed domains file as json object: %v", err)
	}
	domainsAllow := domainsDoc["allow"]
	if !containsString(domainsAllow, "api.anthropic.com") || !containsString(domainsAllow, "api.openrouter.ai") || !containsString(domainsAllow, "api.search.brave.com") {
		t.Fatalf("expected bootstrap allowed domains allow list to contain default domains, got %#v", domainsAllow)
	}
	if len(domainsDoc["deny"]) != 0 {
		t.Fatalf("expected bootstrap allowed domains deny list to be empty, got %#v", domainsDoc["deny"])
	}

	configPath := cfg.ConfigPath()
	configRaw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	configText := string(configRaw)
	if !strings.Contains(configText, "[llm.default]") || !strings.Contains(configText, "[channels.telegram]") {
		t.Fatalf("expected bootstrap config to contain minimal sections, got %q", configText)
	}
	if !strings.Contains(configText, "[costs]") || !strings.Contains(configText, "daily_limit = 0") || !strings.Contains(configText, "monthly_limit = 0") {
		t.Fatalf("expected bootstrap config to expose disabled spend limits, got %q", configText)
	}
	if !strings.Contains(configText, "[security]") || !strings.Contains(configText, "mode = 'standard'") || !strings.Contains(configText, "command_timeout = '5m0s'") {
		t.Fatalf("expected bootstrap config to contain explicit security defaults, got %q", configText)
	}

	commandsPath := cfg.AllowedCommandsPath()
	commandsRaw, err := os.ReadFile(commandsPath)
	if err != nil {
		t.Fatalf("read allowed commands file: %v", err)
	}
	var commandsDoc map[string][]string
	if err := json.Unmarshal(commandsRaw, &commandsDoc); err != nil {
		t.Fatalf("parse allowed commands file as json object: %v", err)
	}
	commandsAllow := commandsDoc["allow"]
	if len(commandsAllow) != 23 {
		t.Fatalf("expected 23 default allowed commands, got %d", len(commandsAllow))
	}
	if !containsString(commandsAllow, "curl *") {
		t.Fatalf("expected default allowed commands to include curl *, got %#v", commandsAllow)
	}
	if len(commandsDoc["deny"]) != 0 {
		t.Fatalf("expected bootstrap allowed commands deny list to be empty, got %#v", commandsDoc["deny"])
	}

	usersPath := cfg.AllowedUsersPath()
	usersRaw, err := os.ReadFile(usersPath)
	if err != nil {
		t.Fatalf("read allowed users file: %v", err)
	}
	var usersDoc map[string]any
	if err := json.Unmarshal(usersRaw, &usersDoc); err != nil {
		t.Fatalf("parse allowed users file as json object: %v", err)
	}
	usersValue, ok := usersDoc["users"]
	if !ok {
		t.Fatalf("expected allowed users file to include users key")
	}
	usersSlice, ok := usersValue.([]any)
	if !ok {
		t.Fatalf("expected users key to be array, got %T", usersValue)
	}
	if len(usersSlice) != 0 {
		t.Fatalf("expected bootstrap allowed users file to start empty, got %d entries", len(usersSlice))
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestInitializeIsIdempotent(t *testing.T) {
	homeDir := filepath.Join(t.TempDir(), ".betterclaw")
	cfg := &config.Config{
		HomeDir: homeDir,
		Agent:   "default",
	}

	if err := Initialize(cfg); err != nil {
		t.Fatalf("first initialize: %v", err)
	}

	jobsPath := cfg.JobsPath()
	customContent := []byte("[{\"name\":\"keep-me\"}]\n")
	if err := os.WriteFile(jobsPath, customContent, 0o644); err != nil {
		t.Fatalf("seed custom jobs content: %v", err)
	}
	configPath := cfg.ConfigPath()
	customConfig := []byte("[llm.default]\napi_key = \"keep-me\"\nprovider = \"anthropic\"\nmodel = \"claude-sonnet-4-6\"\n")
	if err := os.WriteFile(configPath, customConfig, 0o644); err != nil {
		t.Fatalf("seed custom config content: %v", err)
	}

	if err := Initialize(cfg); err != nil {
		t.Fatalf("second initialize: %v", err)
	}

	got, err := os.ReadFile(jobsPath)
	if err != nil {
		t.Fatalf("read jobs file: %v", err)
	}
	if string(got) != string(customContent) {
		t.Fatalf("expected existing file content to remain unchanged")
	}

	configGot, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	if string(configGot) != string(customConfig) {
		t.Fatalf("expected existing config content to remain unchanged")
	}
}
