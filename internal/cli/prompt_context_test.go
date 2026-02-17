package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/machinae/betterclaw/internal/memory"
)

func TestBuildSystemPromptIncludesMemoryAndTodayLog(t *testing.T) {
	memoryDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(memoryDir, "daily"), 0o755); err != nil {
		t.Fatalf("mkdir memory dirs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memoryDir, "memory.md"), []byte("# Memory\n\n## Preferences\n- Vegetarian\n"), 0o644); err != nil {
		t.Fatalf("write memory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memoryDir, "daily", "2026-02-17.md"), []byte("# 2026-02-17\n\n- 10:00:00: Worked on API migration\n"), 0o644); err != nil {
		t.Fatalf("write daily log: %v", err)
	}

	got, err := buildSystemPrompt(memory.New(memoryDir), time.Date(2026, 2, 17, 12, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("build system prompt: %v", err)
	}
	if !strings.Contains(got, "memory_append") {
		t.Fatalf("expected auto-remember instruction, got %q", got)
	}
	if !strings.Contains(got, "[Long-term memory]") || !strings.Contains(got, "Vegetarian") {
		t.Fatalf("expected long-term memory context, got %q", got)
	}
	if !strings.Contains(got, "[Today's daily log]") || !strings.Contains(got, "Worked on API migration") {
		t.Fatalf("expected daily log context, got %q", got)
	}
}
