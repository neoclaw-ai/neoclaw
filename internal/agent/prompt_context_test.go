package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neoclaw-ai/neoclaw/internal/config"
	"github.com/neoclaw-ai/neoclaw/internal/memory"
)

func TestBuildSystemPromptIncludesSoulMemoryAndRecentDailyLogs(t *testing.T) {
	agentDir := t.TempDir()
	memoryDir := filepath.Join(agentDir, "memory")
	if err := os.MkdirAll(filepath.Join(memoryDir, "daily"), 0o755); err != nil {
		t.Fatalf("mkdir memory dirs: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(agentDir, "SOUL.md"),
		[]byte("# Soul\n\n## Persona\nHelpful assistant\n"),
		0o644,
	); err != nil {
		t.Fatalf("write soul: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memoryDir, "memory.md"), []byte("# Memory\n\n## Preferences\n- Vegetarian\n"), 0o644); err != nil {
		t.Fatalf("write memory: %v", err)
	}
	store := memory.New(memoryDir)
	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.Local)
	if err := store.AppendDailyLog(now.Add(-1*time.Hour), "Worked on API migration"); err != nil {
		t.Fatalf("append recent daily log: %v", err)
	}

	got, err := buildSystemPromptAt(agentDir, store, now, config.ContextConfig{DailyLogLookback: 24 * time.Hour})
	if err != nil {
		t.Fatalf("build system prompt: %v", err)
	}
	if !strings.Contains(got, "memory_append") {
		t.Fatalf("expected auto-remember instruction, got %q", got)
	}
	if !strings.Contains(got, "[SOUL.md]") || !strings.Contains(got, "Helpful assistant") {
		t.Fatalf("expected SOUL context, got %q", got)
	}
	if !strings.Contains(got, "[Long-term memory]") || !strings.Contains(got, "Vegetarian") {
		t.Fatalf("expected long-term memory context, got %q", got)
	}
	if !strings.Contains(got, "[Recent daily log]") || !strings.Contains(got, "Worked on API migration") {
		t.Fatalf("expected recent daily log context, got %q", got)
	}
}

func TestBuildSystemPromptDailyLogLookbackWindow(t *testing.T) {
	agentDir := t.TempDir()
	memoryDir := filepath.Join(agentDir, "memory")
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		t.Fatalf("mkdir memory dir: %v", err)
	}
	store := memory.New(memoryDir)
	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.Local)
	if err := store.AppendDailyLog(now.Add(-23*time.Hour), "inside lookback"); err != nil {
		t.Fatalf("append inside lookback: %v", err)
	}
	if err := store.AppendDailyLog(now.Add(-25*time.Hour), "outside lookback"); err != nil {
		t.Fatalf("append outside lookback: %v", err)
	}

	got, err := buildSystemPromptAt(agentDir, store, now, config.ContextConfig{DailyLogLookback: 24 * time.Hour})
	if err != nil {
		t.Fatalf("build system prompt: %v", err)
	}
	if !strings.Contains(got, "inside lookback") {
		t.Fatalf("expected inside-lookback entry in prompt, got %q", got)
	}
	if strings.Contains(got, "outside lookback") {
		t.Fatalf("did not expect outside-lookback entry in prompt, got %q", got)
	}
}
