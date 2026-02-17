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
	dataDir := filepath.Join(t.TempDir(), ".betterclaw")
	cfg := &config.Config{
		DataDir: dataDir,
		Agent:   "default",
	}

	if err := Initialize(cfg); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	requiredPaths := []string{
		filepath.Join(dataDir, "config.toml"),
		filepath.Join(dataDir, "allowed_domains.json"),
		filepath.Join(dataDir, "allowed_bins.json"),
		filepath.Join(dataDir, "costs.jsonl"),
		filepath.Join(dataDir, "agents", "default", "SOUL.md"),
		filepath.Join(dataDir, "agents", "default", "jobs.json"),
		filepath.Join(dataDir, "agents", "default", "memory", "memory.md"),
		filepath.Join(dataDir, "agents", "default", "sessions", "cli", "default.jsonl"),
		filepath.Join(dataDir, "agents", "default", "workspace"),
	}

	for _, path := range requiredPaths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %q to exist: %v", path, err)
		}
	}

	soulPath := filepath.Join(dataDir, "agents", "default", "SOUL.md")
	soulRaw, err := os.ReadFile(soulPath)
	if err != nil {
		t.Fatalf("read SOUL.md: %v", err)
	}
	soul := string(soulRaw)
	if !strings.Contains(soul, "## Persona") || !strings.Contains(soul, "## User") || !strings.Contains(soul, "## Preferences") {
		t.Fatalf("expected SOUL.md template sections, got %q", soul)
	}

	domainsPath := filepath.Join(dataDir, "allowed_domains.json")
	domainsRaw, err := os.ReadFile(domainsPath)
	if err != nil {
		t.Fatalf("read allowed domains file: %v", err)
	}
	domains := string(domainsRaw)
	if !strings.Contains(domains, "api.anthropic.com") || !strings.Contains(domains, "api.openrouter.ai") {
		t.Fatalf("expected bootstrap allowed domains file to contain default domains, got %q", domains)
	}

	configPath := filepath.Join(dataDir, "config.toml")
	configRaw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	configText := string(configRaw)
	if !strings.Contains(configText, "[llm.default]") || !strings.Contains(configText, "[channels.telegram]") {
		t.Fatalf("expected bootstrap config to contain minimal sections, got %q", configText)
	}

	binsPath := filepath.Join(dataDir, "allowed_bins.json")
	binsRaw, err := os.ReadFile(binsPath)
	if err != nil {
		t.Fatalf("read allowed bins file: %v", err)
	}
	var bins []string
	if err := json.Unmarshal(binsRaw, &bins); err != nil {
		t.Fatalf("parse allowed bins file as json array: %v", err)
	}
	if len(bins) == 0 {
		t.Fatalf("expected bootstrap allowed bins file to have at least one entry")
	}
}

func TestInitializeIsIdempotent(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), ".betterclaw")
	cfg := &config.Config{
		DataDir: dataDir,
		Agent:   "default",
	}

	if err := Initialize(cfg); err != nil {
		t.Fatalf("first initialize: %v", err)
	}

	jobsPath := filepath.Join(dataDir, "agents", "default", "jobs.json")
	customContent := []byte("[{\"name\":\"keep-me\"}]\n")
	if err := os.WriteFile(jobsPath, customContent, 0o644); err != nil {
		t.Fatalf("seed custom jobs content: %v", err)
	}
	configPath := filepath.Join(dataDir, "config.toml")
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
