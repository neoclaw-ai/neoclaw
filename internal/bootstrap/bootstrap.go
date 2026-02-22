// Package bootstrap handles first-run initialization of the BetterClaw data directory tree, creating directories, policy files, and a starter config idempotently.
package bootstrap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/store"
)

var defaultAllowedDomains = []string{
	"api.anthropic.com",
	"api.openrouter.ai",
}

var defaultAllowedCommands = []string{
	"cat *",
	"cd *",
	"curl *",
	"cut *",
	"echo *",
	"expr *",
	"find *",
	"grep *",
	"head *",
	"id *",
	"ls *",
	"paste *",
	"pwd *",
	"rev *",
	"seq *",
	"stat *",
	"tail *",
	"tr *",
	"uname *",
	"uniq *",
	"wc *",
	"which *",
	"whoami *",
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
		filepath.Join(cfg.DataDir, store.AgentsDirPath),
		agentDir,
		filepath.Join(agentDir, store.WorkspaceDirPath),
		filepath.Join(agentDir, store.MemoryDirPath),
		filepath.Join(agentDir, store.MemoryDirPath, store.DailyDirPath),
		filepath.Join(agentDir, store.SessionsDirPath),
		filepath.Join(agentDir, store.SessionsDirPath, store.CLISessionsDirPath),
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
		{path: filepath.Join(cfg.DataDir, store.ConfigFilePath), content: defaultConfig},
		{path: filepath.Join(cfg.DataDir, store.AllowedDomainsFilePath), content: defaultAllowedDomainsJSON()},
		{path: filepath.Join(cfg.DataDir, store.AllowedCommandsFilePath), content: defaultAllowedCommandsJSON()},
		{path: filepath.Join(cfg.DataDir, store.AllowedUsersFilePath), content: defaultAllowedUsersJSON()},
		{path: filepath.Join(cfg.DataDir, store.CostsFilePath), content: ""},

		{path: filepath.Join(agentDir, store.SoulFilePath), content: defaultSoulMarkdown()},
		{path: filepath.Join(agentDir, store.JobsFilePath), content: "[]\n"},
		{path: filepath.Join(agentDir, store.MemoryDirPath, store.MemoryFilePath), content: "# Memory\n"},
		{path: filepath.Join(agentDir, store.SessionsDirPath, store.CLISessionsDirPath, store.DefaultSessionPath), content: ""},
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

	if err := store.WriteFile(path, []byte(content)); err != nil {
		return fmt.Errorf("write file %q: %w", path, err)
	}
	return nil
}

func defaultAllowedDomainsJSON() string {
	payload := map[string][]string{
		"allow": defaultAllowedDomains,
		"deny":  []string{},
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "{\n  \"allow\": [],\n  \"deny\": []\n}\n"
	}
	return string(b) + "\n"
}

func defaultAllowedCommandsJSON() string {
	payload := map[string][]string{
		"allow": defaultAllowedCommands,
		"deny":  []string{},
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "{\n  \"allow\": [],\n  \"deny\": []\n}\n"
	}
	return string(b) + "\n"
}

func defaultAllowedUsersJSON() string {
	return "{\n  \"users\": []\n}\n"
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
