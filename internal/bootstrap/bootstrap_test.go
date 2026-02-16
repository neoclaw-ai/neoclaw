package bootstrap

import (
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
		filepath.Join(dataDir, "allowed_domains.json"),
		filepath.Join(dataDir, "allowed_bins.json"),
		filepath.Join(dataDir, "costs.jsonl"),
		filepath.Join(dataDir, "agents", "default", "AGENT.md"),
		filepath.Join(dataDir, "agents", "default", "SOUL.md"),
		filepath.Join(dataDir, "agents", "default", "USER.md"),
		filepath.Join(dataDir, "agents", "default", "IDENTITY.md"),
		filepath.Join(dataDir, "agents", "default", "TOOLS.md"),
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

	domainsPath := filepath.Join(dataDir, "allowed_domains.json")
	domainsRaw, err := os.ReadFile(domainsPath)
	if err != nil {
		t.Fatalf("read allowed domains file: %v", err)
	}
	domains := string(domainsRaw)
	if !strings.Contains(domains, "api.anthropic.com") || !strings.Contains(domains, "api.openrouter.ai") {
		t.Fatalf("expected bootstrap allowed domains file to contain default domains, got %q", domains)
	}

	binsPath := filepath.Join(dataDir, "allowed_bins.json")
	binsRaw, err := os.ReadFile(binsPath)
	if err != nil {
		t.Fatalf("read allowed bins file: %v", err)
	}
	bins := string(binsRaw)
	if !strings.Contains(bins, "\"git\"") || !strings.Contains(bins, "\"go\"") || !strings.Contains(bins, "\"curl\"") {
		t.Fatalf("expected bootstrap allowed bins file to contain default binaries, got %q", bins)
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
}
