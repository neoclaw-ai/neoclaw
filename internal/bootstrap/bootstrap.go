package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/machinae/betterclaw/internal/config"
)

// Initialize creates the expected BetterClaw data tree if missing.
func Initialize(cfg *config.Config) error {
	agentDir := cfg.AgentDir()
	dirs := []string{
		cfg.DataDir,
		filepath.Join(cfg.DataDir, "agents"),
		agentDir,
		filepath.Join(agentDir, "workspace"),
		filepath.Join(agentDir, "memory"),
		filepath.Join(agentDir, "memory", "daily"),
		filepath.Join(agentDir, "sessions"),
		filepath.Join(agentDir, "sessions", "cli"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %q: %w", dir, err)
		}
	}

	files := []struct {
		path    string
		content string
	}{
		{path: filepath.Join(cfg.DataDir, "allowed_domains.json"), content: "[]\n"},
		{path: filepath.Join(cfg.DataDir, "allowed_bins.json"), content: "[]\n"},
		{path: filepath.Join(cfg.DataDir, "costs.jsonl"), content: ""},

		{path: filepath.Join(agentDir, "AGENT.md"), content: "# AGENT\n"},
		{path: filepath.Join(agentDir, "SOUL.md"), content: "# SOUL\n"},
		{path: filepath.Join(agentDir, "USER.md"), content: "# USER\n"},
		{path: filepath.Join(agentDir, "IDENTITY.md"), content: "# IDENTITY\n"},
		{path: filepath.Join(agentDir, "TOOLS.md"), content: "# TOOLS\n"},
		{path: filepath.Join(agentDir, "jobs.json"), content: "[]\n"},
		{path: filepath.Join(agentDir, "memory", "memory.md"), content: "# Memory\n"},
		{path: filepath.Join(agentDir, "sessions", "cli", "default.jsonl"), content: ""},
	}

	for _, file := range files {
		if err := writeFileIfMissing(file.path, file.content); err != nil {
			return err
		}
	}

	return nil
}

func writeFileIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %q: %w", path, err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write file %q: %w", path, err)
	}
	return nil
}
