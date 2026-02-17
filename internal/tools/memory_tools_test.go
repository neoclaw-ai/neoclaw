package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/machinae/betterclaw/internal/memory"
)

func TestMemoryAppendCreatesAndUpdatesSections(t *testing.T) {
	memoryDir := t.TempDir()
	store := memory.New(memoryDir)
	tool := MemoryAppendTool{Store: store}

	_, err := tool.Execute(context.Background(), map[string]any{
		"section": "Preferences",
		"fact":    "Vegetarian",
	})
	if err != nil {
		t.Fatalf("append fact: %v", err)
	}
	_, err = tool.Execute(context.Background(), map[string]any{
		"section": "Preferences",
		"fact":    "Prefers concise answers",
	})
	if err != nil {
		t.Fatalf("append second fact: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(memoryDir, "memory.md"))
	if err != nil {
		t.Fatalf("read memory file: %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, "## Preferences") {
		t.Fatalf("expected preferences section, got %q", content)
	}
	if !strings.Contains(content, "- Vegetarian") || !strings.Contains(content, "- Prefers concise answers") {
		t.Fatalf("expected both facts in section, got %q", content)
	}
}

func TestMemoryRemoveFindsAndDeletes(t *testing.T) {
	memoryDir := t.TempDir()
	path := filepath.Join(memoryDir, "memory.md")
	initial := "# Memory\n\n## User\n- Name: Alex\n- Vegetarian\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial memory: %v", err)
	}

	store := memory.New(memoryDir)
	tool := MemoryRemoveTool{Store: store}
	res, err := tool.Execute(context.Background(), map[string]any{"fact": "Vegetarian"})
	if err != nil {
		t.Fatalf("remove fact: %v", err)
	}
	if !strings.Contains(res.Output, "removed 1") {
		t.Fatalf("expected removed count, got %q", res.Output)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read memory file: %v", err)
	}
	if strings.Contains(string(raw), "Vegetarian") {
		t.Fatalf("expected fact to be removed, got %q", string(raw))
	}
}

func TestDailyLogCreatesDatedFile(t *testing.T) {
	memoryDir := t.TempDir()
	fixed := time.Date(2026, 2, 17, 10, 30, 0, 0, time.Local)
	store := memory.New(memoryDir)
	tool := DailyLogTool{
		Store: store,
		Now:   func() time.Time { return fixed },
	}

	_, err := tool.Execute(context.Background(), map[string]any{"entry": "Met with Sarah"})
	if err != nil {
		t.Fatalf("daily log: %v", err)
	}

	path := filepath.Join(memoryDir, "daily", "2026-02-17.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read daily log: %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, "# 2026-02-17") {
		t.Fatalf("expected daily log header, got %q", content)
	}
	if !strings.Contains(content, "- 10:30:00: Met with Sarah") {
		t.Fatalf("expected timestamped entry, got %q", content)
	}
}

func TestSearchLogsFindsAcrossMultipleDays(t *testing.T) {
	memoryDir := t.TempDir()
	dailyDir := filepath.Join(memoryDir, "daily")
	if err := os.MkdirAll(dailyDir, 0o755); err != nil {
		t.Fatalf("mkdir daily dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(dailyDir, "2026-02-17.md"),
		[]byte("# 2026-02-17\n\n- 09:00:00: API migration work\n"),
		0o644,
	); err != nil {
		t.Fatalf("write day 1: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(dailyDir, "2026-02-16.md"),
		[]byte("# 2026-02-16\n\n- 11:00:00: Discussed migration timeline\n"),
		0o644,
	); err != nil {
		t.Fatalf("write day 2: %v", err)
	}

	store := memory.New(memoryDir)
	tool := SearchLogsTool{
		Store: store,
		Now:   func() time.Time { return time.Date(2026, 2, 17, 12, 0, 0, 0, time.Local) },
	}
	res, err := tool.Execute(context.Background(), map[string]any{
		"query":     "migration",
		"days_back": 2,
	})
	if err != nil {
		t.Fatalf("search logs: %v", err)
	}
	if !strings.Contains(res.Output, "2026-02-17") || !strings.Contains(res.Output, "2026-02-16") {
		t.Fatalf("expected matches from both days, got %q", res.Output)
	}
}
