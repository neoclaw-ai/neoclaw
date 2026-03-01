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

func TestBuildSystemPromptIncludesPersistentFactsBlock(t *testing.T) {
	agentDir := t.TempDir()
	memoryDir := filepath.Join(agentDir, "memory")
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		t.Fatalf("mkdir memory dir: %v", err)
	}
	store := mustNewMemoryStore(t, memoryDir)
	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.Local)
	if err := store.AppendMemory(memory.LogEntry{
		Timestamp: now.Add(-72 * time.Hour),
		Tags:      []string{"location"},
		Text:      "In SF",
		KV:        "-",
	}); err != nil {
		t.Fatalf("append memory fact: %v", err)
	}

	got, err := buildSystemPromptAt(agentDir, store, now, config.ContextConfig{DailyLogLookbackDays: 1})
	if err != nil {
		t.Fatalf("build system prompt: %v", err)
	}
	if !strings.Contains(got, "[Persistent facts]\nage\ttags\ttext\tkv\n3d\tlocation\tIn SF\t-") {
		t.Fatalf("expected persistent facts block, got %q", got)
	}
}

func TestBuildSystemPromptIncludesDailyLogBlockWithTimeColumn(t *testing.T) {
	agentDir := t.TempDir()
	memoryDir := filepath.Join(agentDir, "memory")
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		t.Fatalf("mkdir memory dir: %v", err)
	}
	store := mustNewMemoryStore(t, memoryDir)
	now := time.Date(2026, 2, 17, 15, 0, 0, 0, time.Local)
	if err := store.AppendDailyLog(memory.LogEntry{
		Timestamp: time.Date(2026, 2, 17, 14, 30, 0, 0, time.Local),
		Tags:      []string{"event"},
		Text:      "Worked on API migration",
		KV:        "-",
	}); err != nil {
		t.Fatalf("append daily log: %v", err)
	}

	got, err := buildSystemPromptAt(agentDir, store, now, config.ContextConfig{DailyLogLookbackDays: 1})
	if err != nil {
		t.Fatalf("build system prompt: %v", err)
	}
	if !strings.Contains(got, "[Daily log — 2026-02-17]\ntime\ttags\ttext\tkv\n14:30\tevent\tWorked on API migration\t-") {
		t.Fatalf("expected daily log block, got %q", got)
	}
}

func TestBuildSystemPromptIncludesTodayAndYesterdayOnly(t *testing.T) {
	agentDir := t.TempDir()
	memoryDir := filepath.Join(agentDir, "memory")
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		t.Fatalf("mkdir memory dir: %v", err)
	}
	store := mustNewMemoryStore(t, memoryDir)
	now := time.Date(2026, 2, 17, 15, 0, 0, 0, time.Local)
	for _, entry := range []memory.LogEntry{
		{
			Timestamp: time.Date(2026, 2, 17, 10, 0, 0, 0, time.Local),
			Tags:      []string{"note"},
			Text:      "today",
			KV:        "-",
		},
		{
			Timestamp: time.Date(2026, 2, 16, 10, 0, 0, 0, time.Local),
			Tags:      []string{"note"},
			Text:      "yesterday",
			KV:        "-",
		},
		{
			Timestamp: time.Date(2026, 2, 15, 10, 0, 0, 0, time.Local),
			Tags:      []string{"note"},
			Text:      "older",
			KV:        "-",
		},
	} {
		if err := store.AppendDailyLog(entry); err != nil {
			t.Fatalf("append daily log: %v", err)
		}
	}

	got, err := buildSystemPromptAt(agentDir, store, now, config.ContextConfig{DailyLogLookbackDays: 2})
	if err != nil {
		t.Fatalf("build system prompt: %v", err)
	}
	if !strings.Contains(got, "[Daily log — 2026-02-17]") || !strings.Contains(got, "[Daily log — 2026-02-16]") {
		t.Fatalf("expected today and yesterday blocks, got %q", got)
	}
	if strings.Contains(got, "[Daily log — 2026-02-15]") || strings.Contains(got, "\tolder\t") {
		t.Fatalf("did not expect older daily log block, got %q", got)
	}
}

func TestBuildSystemPromptInjectionOrder(t *testing.T) {
	agentDir := t.TempDir()
	memoryDir := filepath.Join(agentDir, "memory")
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		t.Fatalf("mkdir memory dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "SOUL.md"), []byte("Helpful assistant\n"), 0o644); err != nil {
		t.Fatalf("write soul: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "USER.md"), []byte("Ilya\n"), 0o644); err != nil {
		t.Fatalf("write user: %v", err)
	}
	store := mustNewMemoryStore(t, memoryDir)
	now := time.Date(2026, 2, 17, 15, 0, 0, 0, time.Local)
	if err := store.AppendMemory(memory.LogEntry{
		Timestamp: now.Add(-24 * time.Hour),
		Tags:      []string{"diet"},
		Text:      "Vegetarian",
		KV:        "-",
	}); err != nil {
		t.Fatalf("append memory fact: %v", err)
	}
	if err := store.AppendDailyLog(memory.LogEntry{
		Timestamp: time.Date(2026, 2, 17, 14, 30, 0, 0, time.Local),
		Tags:      []string{"event"},
		Text:      "Worked on API migration",
		KV:        "-",
	}); err != nil {
		t.Fatalf("append daily log: %v", err)
	}

	got, err := buildSystemPromptAt(agentDir, store, now, config.ContextConfig{DailyLogLookbackDays: 1})
	if err != nil {
		t.Fatalf("build system prompt: %v", err)
	}

	soulIndex := strings.Index(got, "[SOUL.md]")
	userIndex := strings.Index(got, "[User profile]")
	factsIndex := strings.Index(got, "[Persistent facts]")
	dailyIndex := strings.Index(got, "[Daily log — 2026-02-17]")
	if soulIndex == -1 || userIndex == -1 || factsIndex == -1 || dailyIndex == -1 {
		t.Fatalf("expected all context blocks, got %q", got)
	}
	if !(soulIndex < userIndex && userIndex < factsIndex && factsIndex < dailyIndex) {
		t.Fatalf("unexpected injection order, got %q", got)
	}
}

func TestCurrentTimeContextLineFormatsRFC3339WithTimezone(t *testing.T) {
	loc := time.FixedZone("America/Los_Angeles", -8*60*60)
	now := time.Date(2026, 2, 24, 15, 4, 5, 0, loc)

	got := currentTimeContextLine(now)
	want := "Current time: 2026-02-24T15:04:05-08:00 (America/Los_Angeles)"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildSystemPromptIncludesCurrentTimeAndDateResolutionInstruction(t *testing.T) {
	agentDir := t.TempDir()
	memoryDir := filepath.Join(agentDir, "memory")
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		t.Fatalf("mkdir memory dir: %v", err)
	}
	store := mustNewMemoryStore(t, memoryDir)
	loc := time.FixedZone("America/Los_Angeles", -8*60*60)
	now := time.Date(2026, 2, 24, 15, 4, 5, 0, loc)

	got, err := buildSystemPromptAt(agentDir, store, now, config.ContextConfig{DailyLogLookbackDays: 1})
	if err != nil {
		t.Fatalf("build system prompt: %v", err)
	}
	if !strings.Contains(got, "Current time: 2026-02-24T15:04:05-08:00 (America/Los_Angeles)") {
		t.Fatalf("expected current time context in prompt, got %q", got)
	}
	if !strings.Contains(got, "Resolve relative date/time phrases") {
		t.Fatalf("expected relative time instruction in prompt, got %q", got)
	}
}

func TestFormatAgeFormatsEachRange(t *testing.T) {
	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		then time.Time
		want string
	}{
		{
			name: "minutes",
			then: now.Add(-45 * time.Minute),
			want: "45m",
		},
		{
			name: "hours",
			then: now.Add(-23 * time.Hour),
			want: "23h",
		},
		{
			name: "days",
			then: now.Add(-29 * 24 * time.Hour),
			want: "29d",
		},
		{
			name: "months",
			then: now.Add(-11 * 30 * 24 * time.Hour),
			want: "11mo",
		},
		{
			name: "years",
			then: now.Add(-3 * 365 * 24 * time.Hour),
			want: "3y",
		},
		{
			name: "future clamps to zero",
			then: now.Add(2 * time.Hour),
			want: "0m",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatAge(now, tc.then)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestFormatAgeZeroNowUsesCurrentTime(t *testing.T) {
	got := formatAge(time.Time{}, time.Now())
	if got != "0m" {
		t.Fatalf("expected %q, got %q", "0m", got)
	}
}

func TestLookbackDates(t *testing.T) {
	now := time.Date(2026, 2, 17, 15, 0, 0, 0, time.Local)

	if got := lookbackDates(now, 0); got != nil {
		t.Fatalf("expected nil for zero days, got %#v", got)
	}

	got := lookbackDates(now, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 dates, got %d", len(got))
	}
	want := []string{"2026-02-17", "2026-02-16", "2026-02-15"}
	for i, date := range got {
		if gotDay := date.In(time.Local).Format("2006-01-02"); gotDay != want[i] {
			t.Fatalf("expected day %q at index %d, got %q", want[i], i, gotDay)
		}
	}
}

func TestTruncateStringByChars(t *testing.T) {
	if got, truncated := truncateStringByChars("hello", 0); got != "" || !truncated {
		t.Fatalf("expected empty+truncated for maxChars=0, got %q %v", got, truncated)
	}

	if got, truncated := truncateStringByChars("hello", 5); got != "hello" || truncated {
		t.Fatalf("expected unmodified string, got %q %v", got, truncated)
	}

	if got, truncated := truncateStringByChars("héllo🙂", 3); got != "hél" || !truncated {
		t.Fatalf("expected unicode-safe truncation, got %q %v", got, truncated)
	}
}

func mustNewMemoryStore(t *testing.T, dir string) *memory.Store {
	t.Helper()

	store, err := memory.New(dir)
	if err != nil {
		t.Fatalf("new memory store: %v", err)
	}
	return store
}
