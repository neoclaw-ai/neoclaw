package tools

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/neoclaw-ai/neoclaw/internal/memory"
)

func TestDailyLogAppendToolExecute(t *testing.T) {
	memoryDir := t.TempDir()
	store := mustNewMemoryStore(t, memoryDir)
	tool := DailyLogAppendTool{Store: store}

	res, err := tool.Execute(context.Background(), map[string]any{
		"tags": "event,meeting",
		"text": "Met with Sarah",
	})
	if err != nil {
		t.Fatalf("daily log append: %v", err)
	}
	if res.Output != "ok" {
		t.Fatalf("expected ok output, got %q", res.Output)
	}

	entries := store.DailyLogsByDate([]time.Time{time.Now()})
	if len(entries) != 1 {
		t.Fatalf("expected 1 daily log entry, got %d", len(entries))
	}
	if got := strings.Join(entries[0].Tags, ","); got != "event,meeting" {
		t.Fatalf("unexpected tags %q", got)
	}
	if entries[0].Text != "Met with Sarah" {
		t.Fatalf("unexpected text %q", entries[0].Text)
	}

	path := filepath.Join(memoryDir, "daily", entries[0].Timestamp.Format("2006-01-02")+".tsv")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read daily log: %v", err)
	}
	if !strings.Contains(string(raw), "event,meeting\tMet with Sarah\t-") {
		t.Fatalf("expected tsv row, got %q", string(raw))
	}
}

func TestDailyLogAppendToolRejectsSummaryTag(t *testing.T) {
	memoryDir := t.TempDir()
	store := mustNewMemoryStore(t, memoryDir)
	tool := DailyLogAppendTool{Store: store}

	_, err := tool.Execute(context.Background(), map[string]any{
		"tags": "summary,session",
		"text": "Should fail",
	})
	if err == nil || !strings.Contains(err.Error(), "cannot write summary") {
		t.Fatalf("expected summary-tag rejection, got %v", err)
	}
}

func TestMemoryAppendToolAddsExpiresEpoch(t *testing.T) {
	memoryDir := t.TempDir()
	store := mustNewMemoryStore(t, memoryDir)
	tool := MemoryAppendTool{Store: store}

	start := time.Now()
	res, err := tool.Execute(context.Background(), map[string]any{
		"tags":    "location",
		"text":    "In SF",
		"expires": "2d",
	})
	end := time.Now()
	if err != nil {
		t.Fatalf("memory append: %v", err)
	}
	if !strings.Contains(res.Output, "location\tIn SF") {
		t.Fatalf("unexpected output %q", res.Output)
	}

	raw, err := os.ReadFile(filepath.Join(memoryDir, "memory.tsv"))
	if err != nil {
		t.Fatalf("read memory.tsv: %v", err)
	}
	matches := regexp.MustCompile(`expires=(\d+)`).FindStringSubmatch(string(raw))
	if len(matches) != 2 {
		t.Fatalf("expected expires token in %q", string(raw))
	}
	value, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		t.Fatalf("parse expires epoch: %v", err)
	}
	min := start.Add(48 * time.Hour).Unix()
	max := end.Add(48 * time.Hour).Unix()
	if value < min || value > max {
		t.Fatalf("expected expires in [%d, %d], got %d", min, max, value)
	}
}

func TestMemoryTagsToolFormatsSortedCounts(t *testing.T) {
	memoryDir := t.TempDir()
	store := mustNewMemoryStore(t, memoryDir)
	if err := store.AppendMemory(memory.LogEntry{Tags: []string{"location"}, Text: "In SF", KV: "-"}); err != nil {
		t.Fatalf("append first memory fact: %v", err)
	}
	if err := store.AppendMemory(memory.LogEntry{Tags: []string{"diet"}, Text: "Vegetarian", KV: "-"}); err != nil {
		t.Fatalf("append second memory fact: %v", err)
	}
	if err := store.AppendMemory(memory.LogEntry{Tags: []string{"location"}, Text: "In LA", KV: "-"}); err != nil {
		t.Fatalf("append third memory fact: %v", err)
	}

	tool := MemoryTagsTool{Store: store}
	res, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("memory tags: %v", err)
	}
	if res.Output != "tag\tcount\nlocation\t2\ndiet\t1" {
		t.Fatalf("unexpected output %q", res.Output)
	}
}

func TestSearchLogsToolExecuteFormatsTSVAndAppliesTimeBounds(t *testing.T) {
	memoryDir := t.TempDir()
	store := mustNewMemoryStore(t, memoryDir)
	if err := store.AppendDailyLog(memory.LogEntry{
		Timestamp: time.Date(2026, 2, 16, 11, 0, 0, 0, time.UTC),
		Tags:      []string{"event"},
		Text:      "Discussed migration timeline",
		KV:        "-",
	}); err != nil {
		t.Fatalf("append first daily log: %v", err)
	}
	if err := store.AppendDailyLog(memory.LogEntry{
		Timestamp: time.Date(2026, 2, 17, 9, 0, 0, 0, time.UTC),
		Tags:      []string{"event"},
		Text:      "API migration work",
		KV:        "-",
	}); err != nil {
		t.Fatalf("append second daily log: %v", err)
	}

	tool := SearchLogsTool{Store: store}
	res, err := tool.Execute(context.Background(), map[string]any{
		"query":     "migration (timeline|work)",
		"from_time": "2026-02-16T00:00:00Z",
		"to_time":   "2026-02-16T23:59:59Z",
	})
	if err != nil {
		t.Fatalf("search logs: %v", err)
	}

	want := strings.Join([]string{
		"ts\ttags\ttext\tkv",
		"2026-02-16T11:00:00Z\tevent\tDiscussed migration timeline\t-",
	}, "\n")
	if res.Output != want {
		t.Fatalf("expected %q, got %q", want, res.Output)
	}
}

func TestParseExpiryTimeFormatsAndInvalid(t *testing.T) {
	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)

	got, err := parseExpiryTime("2h", now)
	if err != nil {
		t.Fatalf("parse 2h expiry: %v", err)
	}
	if !got.Equal(now.Add(2 * time.Hour)) {
		t.Fatalf("expected %v, got %v", now.Add(2*time.Hour), got)
	}

	got, err = parseExpiryTime("1w", now)
	if err != nil {
		t.Fatalf("parse 1w expiry: %v", err)
	}
	if !got.Equal(now.Add(7 * 24 * time.Hour)) {
		t.Fatalf("expected %v, got %v", now.Add(7*24*time.Hour), got)
	}

	got, err = parseExpiryTime("2026-02-28", now)
	if err != nil {
		t.Fatalf("parse date expiry: %v", err)
	}
	wantDate := time.Date(2026, 2, 28, 0, 0, 0, 0, time.Local)
	if !got.Equal(wantDate) {
		t.Fatalf("expected %v, got %v", wantDate, got)
	}

	got, err = parseExpiryTime("2026-02-28T15:00", now)
	if err != nil {
		t.Fatalf("parse datetime expiry: %v", err)
	}
	wantDateTime := time.Date(2026, 2, 28, 15, 0, 0, 0, time.Local)
	if !got.Equal(wantDateTime) {
		t.Fatalf("expected %v, got %v", wantDateTime, got)
	}

	if _, err := parseExpiryTime("not-a-date", now); err == nil {
		t.Fatal("expected error for invalid expiry format")
	}
}

func TestParseTagsArg(t *testing.T) {
	got, err := parseTagsArg(map[string]any{"tags": " Follow Up , api , follow up , Project X "}, "tags")
	if err != nil {
		t.Fatalf("parse tags: %v", err)
	}
	want := []string{"follow_up", "api", "project_x"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("expected %#v, got %#v", want, got)
	}

	if _, err := parseTagsArg(map[string]any{"tags": " ,  "}, "tags"); err == nil {
		t.Fatal("expected error for empty normalized tags")
	}
}

func TestOptionalRFC3339Arg(t *testing.T) {
	def := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)

	got, err := optionalRFC3339Arg(map[string]any{}, "from_time", def)
	if err != nil {
		t.Fatalf("missing key should use default: %v", err)
	}
	if !got.Equal(def) {
		t.Fatalf("expected default %v, got %v", def, got)
	}

	got, err = optionalRFC3339Arg(map[string]any{"from_time": "   "}, "from_time", def)
	if err != nil {
		t.Fatalf("blank value should use default: %v", err)
	}
	if !got.Equal(def) {
		t.Fatalf("expected default %v, got %v", def, got)
	}

	got, err = optionalRFC3339Arg(map[string]any{"from_time": "2026-02-16T00:00:00Z"}, "from_time", def)
	if err != nil {
		t.Fatalf("valid rfc3339 should parse: %v", err)
	}
	want := time.Date(2026, 2, 16, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}

	if _, err := optionalRFC3339Arg(map[string]any{"from_time": "not-a-time"}, "from_time", def); err == nil {
		t.Fatal("expected error for invalid rfc3339")
	}
}

func TestAppendKVToken(t *testing.T) {
	if got := appendKVToken("-", "expires=1"); got != "expires=1" {
		t.Fatalf("expected placeholder to be replaced, got %q", got)
	}
	if got := appendKVToken("", "expires=1"); got != "expires=1" {
		t.Fatalf("expected empty kv to become token, got %q", got)
	}
	if got := appendKVToken("status=active", "expires=1"); got != "status=active expires=1" {
		t.Fatalf("expected appended token, got %q", got)
	}
	if got := appendKVToken("status=active", "   "); got != "status=active" {
		t.Fatalf("expected blank token to be ignored, got %q", got)
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
