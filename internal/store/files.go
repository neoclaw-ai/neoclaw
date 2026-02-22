// Package store centralizes low-level filesystem reads and writes.
package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// Global files under BETTERCLAW_HOME.
	ConfigFilePath          = "config.toml"
	AllowedDomainsFilePath  = "allowed_domains.json"
	AllowedCommandsFilePath = "allowed_commands.json"
	AllowedBinsFilePath     = "allowed_bins.json"
	AllowedUsersFilePath    = "allowed_users.json"
	CostsFilePath           = "costs.jsonl"

	// Agent directory layout under BETTERCLAW_HOME/agents/{agent}/.
	AgentsDirPath      = "agents"
	WorkspaceDirPath   = "workspace"
	TmpDirPath         = "tmp"
	MemoryDirPath      = "memory"
	DailyDirPath       = "daily"
	SessionsDirPath    = "sessions"
	CLISessionsDirPath = "cli"
	DefaultSessionPath = "default.jsonl"
	JobsFilePath       = "jobs.json"
	SoulFilePath       = "SOUL.md"
	MemoryFilePath     = "memory.md"
)

var (
	pathLocksMu sync.Mutex
	pathLocks   = map[string]*sync.Mutex{}
)

// ReadFile reads a file and returns it as a string.
func ReadFile(path string) (string, error) {
	cleanPath, err := cleanPath(path)
	if err != nil {
		return "", err
	}

	raw, err := os.ReadFile(cleanPath)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// WriteFile atomically replaces a file's contents.
func WriteFile(path string, data []byte) error {
	cleanPath, err := cleanPath(path)
	if err != nil {
		return err
	}

	lock := lockForPath(cleanPath)
	lock.Lock()
	defer lock.Unlock()

	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %q: %w", dir, err)
	}

	tempFile, err := os.CreateTemp(dir, filepath.Base(cleanPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file for %q: %w", cleanPath, err)
	}
	tempPath := tempFile.Name()
	defer func() {
		os.Remove(tempPath)
	}()

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return fmt.Errorf("write temp file for %q: %w", cleanPath, err)
	}
	if err := tempFile.Chmod(0o644); err != nil {
		tempFile.Close()
		return fmt.Errorf("chmod temp file for %q: %w", cleanPath, err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp file for %q: %w", cleanPath, err)
	}
	if err := os.Rename(tempPath, cleanPath); err != nil {
		return fmt.Errorf("replace file %q: %w", cleanPath, err)
	}

	return nil
}

// AppendFile appends bytes to a file, creating it if missing.
func AppendFile(path string, data []byte) error {
	cleanPath, err := cleanPath(path)
	if err != nil {
		return err
	}

	lock := lockForPath(cleanPath)
	lock.Lock()
	defer lock.Unlock()

	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %q: %w", dir, err)
	}

	f, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open file %q for append: %w", cleanPath, err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("append file %q: %w", cleanPath, err)
	}
	return nil
}

func lockForPath(path string) *sync.Mutex {
	pathLocksMu.Lock()
	defer pathLocksMu.Unlock()

	lock, ok := pathLocks[path]
	if ok {
		return lock
	}
	lock = &sync.Mutex{}
	pathLocks[path] = lock
	return lock
}

func cleanPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("path is required")
	}
	return filepath.Clean(trimmed), nil
}
