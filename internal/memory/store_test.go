package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendFactCreatesSections(t *testing.T) {
	store := New(t.TempDir())

	if err := store.AppendFact("Preferences", "Vegetarian"); err != nil {
		t.Fatalf("append fact: %v", err)
	}
	if err := store.AppendFact("Preferences", "Prefers concise answers"); err != nil {
		t.Fatalf("append second fact: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(store.dir, "memory.md"))
	if err != nil {
		t.Fatalf("read memory file: %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, "## Preferences") {
		t.Fatalf("expected section header, got %q", content)
	}
	if !strings.Contains(content, "- Vegetarian") || !strings.Contains(content, "- Prefers concise answers") {
		t.Fatalf("expected both facts, got %q", content)
	}
}

func TestAppendFactDeduplicates(t *testing.T) {
	store := New(t.TempDir())
	if err := store.AppendFact("User", "Name: Alex"); err != nil {
		t.Fatalf("append first: %v", err)
	}
	if err := store.AppendFact("User", "Name: Alex"); err != nil {
		t.Fatalf("append duplicate: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(store.dir, "memory.md"))
	if err != nil {
		t.Fatalf("read memory file: %v", err)
	}
	if strings.Count(string(raw), "- Name: Alex") != 1 {
		t.Fatalf("expected one matching bullet, got %q", string(raw))
	}
}

func TestRemoveFactFindsAndDeletes(t *testing.T) {
	store := New(t.TempDir())
	path := filepath.Join(store.dir, "memory.md")
	initial := "# Memory\n\n## User\n- Name: Alex\n- Vegetarian\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial memory: %v", err)
	}

	removed, err := store.RemoveFact("Vegetarian")
	if err != nil {
		t.Fatalf("remove fact: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed line, got %d", removed)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read memory file: %v", err)
	}
	if strings.Contains(string(raw), "Vegetarian") {
		t.Fatalf("expected fact removed, got %q", string(raw))
	}
}

func TestAppendDailyLogCreatesDatedFile(t *testing.T) {
	store := New(t.TempDir())
	now := time.Date(2026, 2, 17, 10, 30, 0, 0, time.Local)

	if err := store.AppendDailyLog(now, "Met with Sarah"); err != nil {
		t.Fatalf("append daily log: %v", err)
	}

	path := filepath.Join(store.dir, "daily", "2026-02-17.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read daily log: %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, "# 2026-02-17") {
		t.Fatalf("expected header, got %q", content)
	}
	if !strings.Contains(content, "- 10:30:00: Met with Sarah") {
		t.Fatalf("expected timestamped entry, got %q", content)
	}
}

func TestSearchLogsAcrossMultipleDays(t *testing.T) {
	store := New(t.TempDir())
	dailyDir := filepath.Join(store.dir, "daily")
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

	got, err := store.SearchLogs(time.Date(2026, 2, 17, 12, 0, 0, 0, time.Local), "migration", 2)
	if err != nil {
		t.Fatalf("search logs: %v", err)
	}
	if !strings.Contains(got, "2026-02-17") || !strings.Contains(got, "2026-02-16") {
		t.Fatalf("expected matches for both days, got %q", got)
	}
}

func TestLoadContextReturnsMemoryOnly(t *testing.T) {
	store := New(t.TempDir())
	dailyDir := filepath.Join(store.dir, "daily")
	if err := os.MkdirAll(dailyDir, 0o755); err != nil {
		t.Fatalf("mkdir daily dir: %v", err)
	}

	memoryText := "# Memory\n\n## Preferences\n- Vegetarian\n"
	if err := os.WriteFile(filepath.Join(store.dir, "memory.md"), []byte(memoryText), 0o644); err != nil {
		t.Fatalf("write memory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dailyDir, "2026-02-17.md"), []byte("# 2026-02-17\n\n- 09:00:00: API migration work\n"), 0o644); err != nil {
		t.Fatalf("write daily: %v", err)
	}

	mem, err := store.LoadContext()
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if mem != memoryText {
		t.Fatalf("expected memory text %q, got %q", memoryText, mem)
	}
}
