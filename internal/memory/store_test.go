package memory

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNewExistingDirectoryNoFiles(t *testing.T) {
	store := mustNewStore(t, t.TempDir())
	if len(store.dailyLog) != 0 {
		t.Fatalf("expected empty dailyLog, got %d entries", len(store.dailyLog))
	}
	if len(store.memoryFacts) != 0 {
		t.Fatalf("expected empty memoryFacts, got %d entries", len(store.memoryFacts))
	}
}

func TestNewMissingDirectoryReturnsError(t *testing.T) {
	_, err := New(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestAppendDailyLogZeroTimestampWritesTSVAndUpdatesCache(t *testing.T) {
	store := mustNewStore(t, t.TempDir())

	if err := store.AppendDailyLog(LogEntry{Tags: []string{"Note"}, Text: "Met with Sarah", KV: ""}); err != nil {
		t.Fatalf("append daily log: %v", err)
	}

	if len(store.dailyLog) != 1 {
		t.Fatalf("expected 1 cached daily log entry, got %d", len(store.dailyLog))
	}
	entry := store.dailyLog[0]
	if entry.Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}
	if got := entry.Tags; len(got) != 1 || got[0] != "note" {
		t.Fatalf("expected normalized tags, got %#v", got)
	}
	if entry.KV != "-" {
		t.Fatalf("expected normalized kv '-', got %q", entry.KV)
	}

	path := filepath.Join(store.dir, "daily", entry.Timestamp.Format("2006-01-02")+".tsv")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read daily log: %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, "ts\ttags\ttext\tkv") {
		t.Fatalf("expected tsv header, got %q", content)
	}
	if !strings.Contains(content, "note\tMet with Sarah\t-") {
		t.Fatalf("expected tsv row, got %q", content)
	}
}

func TestAppendDailyLogUsesProvidedTimestamp(t *testing.T) {
	store := mustNewStore(t, t.TempDir())
	ts := time.Date(2026, 2, 17, 10, 30, 0, 123456789, time.UTC)

	if err := store.AppendDailyLog(LogEntry{
		Timestamp: ts,
		Tags:      []string{"event"},
		Text:      "Met with Sarah",
		KV:        "-",
	}); err != nil {
		t.Fatalf("append daily log: %v", err)
	}

	if len(store.dailyLog) != 1 {
		t.Fatalf("expected 1 cached daily log entry, got %d", len(store.dailyLog))
	}
	if !store.dailyLog[0].Timestamp.Equal(ts) {
		t.Fatalf("expected timestamp %v, got %v", ts, store.dailyLog[0].Timestamp)
	}
	path := filepath.Join(store.dir, "daily", "2026-02-17.tsv")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read daily log: %v", err)
	}
	if !strings.Contains(string(raw), ts.Format(time.RFC3339Nano)) {
		t.Fatalf("expected timestamp in file, got %q", string(raw))
	}
}

func TestNewLoadsPreexistingTSVFiles(t *testing.T) {
	dir := t.TempDir()
	writeTSVTestFile(t, filepath.Join(dir, "memory.tsv"), [][]string{
		{"2026-02-16T09:00:00Z", "preference", "Vegetarian", "-"},
	})
	writeTSVTestFile(t, filepath.Join(dir, "daily", "2026-02-16.tsv"), [][]string{
		{"2026-02-16T09:00:00Z", "event", "first", "-"},
	})
	writeTSVTestFile(t, filepath.Join(dir, "daily", "2026-02-17.tsv"), [][]string{
		{"2026-02-17T10:00:00Z", "event", "second", "-"},
	})

	store := mustNewStore(t, dir)
	if len(store.memoryFacts) != 1 {
		t.Fatalf("expected 1 memory fact, got %d", len(store.memoryFacts))
	}
	if len(store.dailyLog) != 2 {
		t.Fatalf("expected 2 daily log entries, got %d", len(store.dailyLog))
	}
	if store.dailyLog[0].Text != "first" || store.dailyLog[1].Text != "second" {
		t.Fatalf("unexpected daily log entries: %#v", store.dailyLog)
	}
}

func TestLoadContextReturnsMemoryOnly(t *testing.T) {
	dir := t.TempDir()
	memoryText := "# Memory\n\n## Preferences\n- Vegetarian\n"
	if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte(memoryText), 0o644); err != nil {
		t.Fatalf("write memory: %v", err)
	}
	writeTSVTestFile(t, filepath.Join(dir, "daily", "2026-02-17.tsv"), [][]string{
		{"2026-02-17T09:00:00Z", "event", "API migration work", "-"},
	})

	store := mustNewStore(t, dir)
	mem, err := store.LoadContext()
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if mem != memoryText {
		t.Fatalf("expected memory text %q, got %q", memoryText, mem)
	}
}

func TestGetDailyLogsSkipsMalformedRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daily", "2026-02-17.tsv")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir daily dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.Join([]string{
		"ts\ttags\ttext\tkv",
		"2026-02-17T09:00:00Z\tevent\tvalid entry\t-",
		"bad\trow",
		"not-a-time\tevent\tbad timestamp\t-",
		"2026-02-17T11:00:00Z\tevent\tanother valid entry\t-",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatalf("write daily log: %v", err)
	}

	store := mustNewStore(t, dir)
	fromTime := time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC)
	toTime := time.Date(2026, 2, 17, 23, 59, 59, 0, time.UTC)
	entries, err := store.GetDailyLogs(fromTime, toTime)
	if err != nil {
		t.Fatalf("get daily logs: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 valid log entries, got %d", len(entries))
	}
	if entries[0].Text != "valid entry" || entries[1].Text != "another valid entry" {
		t.Fatalf("unexpected parsed entries: %#v", entries)
	}
}

func TestGetDailyLogsFromAfterToReturnsError(t *testing.T) {
	store := mustNewStore(t, t.TempDir())
	fromTime := time.Date(2026, 2, 18, 0, 0, 0, 0, time.UTC)
	toTime := time.Date(2026, 2, 17, 0, 0, 0, 0, time.UTC)
	_, err := store.GetDailyLogs(fromTime, toTime)
	if err == nil {
		t.Fatalf("expected error when fromTime is after toTime")
	}
}

func TestGetAllDailyLogsReturnsAllEntries(t *testing.T) {
	store := mustNewStore(t, t.TempDir())
	if err := store.AppendDailyLog(LogEntry{
		Timestamp: time.Date(2026, 2, 16, 9, 0, 0, 0, time.UTC),
		Tags:      []string{"event"},
		Text:      "first",
		KV:        "-",
	}); err != nil {
		t.Fatalf("append first daily log: %v", err)
	}
	if err := store.AppendDailyLog(LogEntry{
		Timestamp: time.Date(2026, 2, 17, 10, 0, 0, 0, time.UTC),
		Tags:      []string{"event"},
		Text:      "second",
		KV:        "-",
	}); err != nil {
		t.Fatalf("append second daily log: %v", err)
	}

	entries, err := store.GetAllDailyLogs()
	if err != nil {
		t.Fatalf("get all daily logs: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Text != "first" || entries[1].Text != "second" {
		t.Fatalf("unexpected entries order/content: %#v", entries)
	}
}

func TestSearchReturnsMatchingDailyLogs(t *testing.T) {
	dir := t.TempDir()
	writeTSVTestFile(t, filepath.Join(dir, "daily", "2026-02-16.tsv"), [][]string{
		{"2026-02-16T11:00:00Z", "event", "Discussed migration timeline", "-"},
	})
	writeTSVTestFile(t, filepath.Join(dir, "daily", "2026-02-17.tsv"), [][]string{
		{"2026-02-17T09:00:00Z", "event", "API migration work", "-"},
	})

	store := mustNewStore(t, dir)
	got, err := store.Search("migration", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(got))
	}
	if got[0].Text != "Discussed migration timeline" || got[1].Text != "API migration work" {
		t.Fatalf("unexpected matches: %#v", got)
	}
}

func TestSearchReturnsMatchingMemoryFacts(t *testing.T) {
	dir := t.TempDir()
	writeTSVTestFile(t, filepath.Join(dir, "memory.tsv"), [][]string{
		{"2026-02-16T11:00:00Z", "diet", "Vegetarian", "-"},
		{"2026-02-17T09:00:00Z", "tool", "Uses ripgrep", "-"},
	})

	store := mustNewStore(t, dir)
	got, err := store.Search("Vegetarian|ripgrep", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(got))
	}
	if got[0].Text != "Vegetarian" || got[1].Text != "Uses ripgrep" {
		t.Fatalf("unexpected matches: %#v", got)
	}
}

func TestSearchTimeBounds(t *testing.T) {
	dir := t.TempDir()
	writeTSVTestFile(t, filepath.Join(dir, "daily", "2026-02-15.tsv"), [][]string{
		{"2026-02-15T08:00:00Z", "event", "migration kickoff", "-"},
	})
	writeTSVTestFile(t, filepath.Join(dir, "daily", "2026-02-17.tsv"), [][]string{
		{"2026-02-17T09:00:00Z", "event", "migration followup", "-"},
	})

	store := mustNewStore(t, dir)
	fromTime := time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC)
	got, err := store.Search("migration", fromTime, time.Time{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
	if got[0].Text != "migration followup" {
		t.Fatalf("unexpected match: %#v", got)
	}
}

func TestActiveFactsDedupesAndFallsBackFromExpired(t *testing.T) {
	dir := t.TempDir()
	futureExpires := time.Date(2036, 2, 17, 0, 0, 0, 0, time.UTC).Unix()
	pastExpires := time.Date(2025, 11, 14, 0, 0, 0, 0, time.UTC).Unix()
	writeTSVTestFile(t, filepath.Join(dir, "memory.tsv"), [][]string{
		{"2026-02-15T09:00:00Z", "location", "In SF", "expires=" + strconv.FormatInt(futureExpires, 10)},
		{"2026-02-16T09:00:00Z", "location", "In LA", "expires=" + strconv.FormatInt(pastExpires, 10)},
		{"2026-02-14T09:00:00Z", "diet", "Vegetarian", "-"},
		{"2026-02-17T09:00:00Z", "diet", "Pescatarian", "-"},
	})

	store := mustNewStore(t, dir)
	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	got := store.ActiveFacts(now)
	if len(got) != 2 {
		t.Fatalf("expected 2 active facts, got %d", len(got))
	}
	if got[0].Text != "In SF" {
		t.Fatalf("expected fallback non-expired location fact first, got %#v", got[0])
	}
	if got[1].Text != "Pescatarian" {
		t.Fatalf("expected latest diet fact second, got %#v", got[1])
	}
}

func TestFactTagsCountsHistoricalEntries(t *testing.T) {
	dir := t.TempDir()
	writeTSVTestFile(t, filepath.Join(dir, "memory.tsv"), [][]string{
		{"2026-02-15T09:00:00Z", "location", "In SF", "-"},
		{"2026-02-16T09:00:00Z", "location", "In LA", "-"},
		{"2026-02-17T09:00:00Z", "diet", "Pescatarian", "-"},
		{"2026-02-18T09:00:00Z", "", "untagged", "-"},
	})

	store := mustNewStore(t, dir)
	got := store.FactTags()
	if got["location"] != 2 {
		t.Fatalf("expected location count 2, got %d", got["location"])
	}
	if got["diet"] != 1 {
		t.Fatalf("expected diet count 1, got %d", got["diet"])
	}
	if _, ok := got[""]; ok {
		t.Fatalf("did not expect empty-tag entry in counts: %#v", got)
	}
}

func TestDailyLogsByDateMatchesLocalCalendarDays(t *testing.T) {
	dir := t.TempDir()
	first := time.Date(2026, 2, 16, 23, 0, 0, 0, time.Local)
	second := time.Date(2026, 2, 17, 9, 0, 0, 0, time.Local)
	writeTSVTestFile(t, filepath.Join(dir, "daily", "2026-02-16.tsv"), [][]string{
		{first.Format(time.RFC3339Nano), "event", "first", "-"},
	})
	writeTSVTestFile(t, filepath.Join(dir, "daily", "2026-02-17.tsv"), [][]string{
		{second.Format(time.RFC3339Nano), "event", "second", "-"},
	})

	store := mustNewStore(t, dir)
	got := store.DailyLogsByDate([]time.Time{
		time.Date(2026, 2, 17, 12, 0, 0, 0, time.Local),
	})
	if len(got) != 1 {
		t.Fatalf("expected 1 daily log entry, got %d", len(got))
	}
	if got[0].Text != "second" {
		t.Fatalf("unexpected daily log entry: %#v", got[0])
	}
}

func mustNewStore(t *testing.T, dir string) *Store {
	t.Helper()

	store, err := New(dir)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return store
}

func writeTSVTestFile(t *testing.T, path string, rows [][]string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for tsv file: %v", err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tsv file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = '\t'
	if err := writer.Write([]string{"ts", "tags", "text", "kv"}); err != nil {
		t.Fatalf("write tsv header: %v", err)
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			t.Fatalf("write tsv row: %v", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatalf("flush tsv file: %v", err)
	}
}
