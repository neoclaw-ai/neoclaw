// Package bootstrap handles first-run initialization of the BetterClaw data directory tree, creating directories, policy files, and a starter config idempotently.
package bootstrap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/machinae/betterclaw/internal/config"
)

var defaultAllowedDomains = []string{
	"api.anthropic.com",
	"api.openrouter.ai",
}

var defaultAllowedBins = []string{
	"cat",
	"cd",
	"cut",
	"echo",
	"expr",
	"find",
	"grep",
	"head",
	"id",
	"ls",
	"paste",
	"pwd",
	"rev",
	"seq",
	"stat",
	"tail",
	"tr",
	"uname",
	"uniq",
	"wc",
	"which",
	"whoami",
}

// Initialize creates the expected BetterClaw data tree if missing.
func Initialize(cfg *config.Config) error {
	agentDir := cfg.AgentDir()
	defaultConfig, err := config.DefaultUserConfigTOML()
	if err != nil {
		return fmt.Errorf("render default config: %w", err)
	}

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
		{path: filepath.Join(cfg.DataDir, "config.toml"), content: defaultConfig},
		{path: filepath.Join(cfg.DataDir, "allowed_domains.json"), content: defaultAllowedDomainsJSON()},
		{path: filepath.Join(cfg.DataDir, "allowed_bins.json"), content: defaultAllowedBinsJSON()},
		{path: filepath.Join(cfg.DataDir, "costs.jsonl"), content: ""},

		{path: filepath.Join(agentDir, "SOUL.md"), content: defaultSoulMarkdown()},
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

func defaultAllowedDomainsJSON() string {
	b, err := json.MarshalIndent(defaultAllowedDomains, "", "  ")
	if err != nil {
		// Static list; this should never fail.
		return "[]\n"
	}
	return string(b) + "\n"
}

func defaultAllowedBinsJSON() string {
	b, err := json.MarshalIndent(defaultAllowedBins, "", "  ")
	if err != nil {
		// Static list; this should never fail.
		return "[]\n"
	}
	return string(b) + "\n"
}

func defaultSoulMarkdown() string {
	return `# Soul

## Persona
You are a helpful personal assistant.

## User


## Preferences


## Tool Conventions
- Use tools when they improve accuracy.
- Keep outputs concise and actionable.
`
}
