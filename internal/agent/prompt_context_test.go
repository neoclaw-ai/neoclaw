package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/machinae/betterclaw/internal/memory"
)

func TestBuildSystemPromptIncludesSoulAndMemory(t *testing.T) {
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

	got, err := BuildSystemPrompt(agentDir, memory.New(memoryDir))
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
	if strings.Contains(got, "[Today's daily log]") {
		t.Fatalf("did not expect daily log auto-injection, got %q", got)
	}
}

func TestBuildSystemPromptTruncatesSoulContent(t *testing.T) {
	agentDir := t.TempDir()
	memoryDir := filepath.Join(agentDir, "memory")
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		t.Fatalf("mkdir memory dir: %v", err)
	}
	longSoul := strings.Repeat("a", maxSoulChars+25)
	if err := os.WriteFile(filepath.Join(agentDir, "SOUL.md"), []byte(longSoul), 0o644); err != nil {
		t.Fatalf("write soul: %v", err)
	}

	got, err := BuildSystemPrompt(agentDir, memory.New(memoryDir))
	if err != nil {
		t.Fatalf("build system prompt: %v", err)
	}
	if !strings.Contains(got, strings.Repeat("a", maxSoulChars)) {
		t.Fatalf("expected truncated SOUL content in prompt")
	}
	if strings.Contains(got, strings.Repeat("a", maxSoulChars+1)) {
		t.Fatalf("expected SOUL content to be capped at %d chars", maxSoulChars)
	}
}
