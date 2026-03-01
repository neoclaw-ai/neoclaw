package memory

import (
	"reflect"
	"testing"
	"time"
)

func TestLogEntryUnmarshalTSVValid(t *testing.T) {
	var entry LogEntry
	err := entry.UnmarshalTSV([]string{
		"2026-02-28T09:30:00.123456789Z",
		"event, followup , project_x",
		"Discussed API migration",
		"status=active ref=api_migration",
	})
	if err != nil {
		t.Fatalf("unmarshal tsv: %v", err)
	}

	wantTime := time.Date(2026, 2, 28, 9, 30, 0, 123456789, time.UTC)
	if !entry.Timestamp.Equal(wantTime) {
		t.Fatalf("expected timestamp %v, got %v", wantTime, entry.Timestamp)
	}
	wantTags := []string{"event", "followup", "project_x"}
	if !reflect.DeepEqual(entry.Tags, wantTags) {
		t.Fatalf("expected tags %#v, got %#v", wantTags, entry.Tags)
	}
	if entry.Text != "Discussed API migration" {
		t.Fatalf("expected text %q, got %q", "Discussed API migration", entry.Text)
	}
	if entry.KV != "status=active ref=api_migration" {
		t.Fatalf("expected kv %q, got %q", "status=active ref=api_migration", entry.KV)
	}
}

func TestLogEntryUnmarshalTSVWrongFieldCount(t *testing.T) {
	var entry LogEntry
	if err := entry.UnmarshalTSV([]string{"a", "b", "c"}); err == nil {
		t.Fatal("expected error for wrong field count")
	}
}

func TestLogEntryUnmarshalTSVBadTimestamp(t *testing.T) {
	var entry LogEntry
	if err := entry.UnmarshalTSV([]string{"not-a-time", "event", "Discussed API migration", "-"}); err == nil {
		t.Fatal("expected error for invalid timestamp")
	}
}

func TestLogEntryMarshalTSVRoundTrip(t *testing.T) {
	want := LogEntry{
		Timestamp: time.Date(2026, 2, 28, 9, 30, 0, 123456789, time.UTC),
		Tags:      []string{"event", "project_x"},
		Text:      "Discussed API migration",
		KV:        "status=active ref=api_migration",
	}

	fields := want.MarshalTSV()
	var got LogEntry
	if err := got.UnmarshalTSV(fields); err != nil {
		t.Fatalf("unmarshal marshaled fields: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestLogEntryMarshalTSVSanitizesFieldsAndDefaultsKV(t *testing.T) {
	entry := LogEntry{
		Timestamp: time.Date(2026, 2, 28, 9, 30, 0, 123456789, time.UTC),
		Tags:      []string{"Event", "Project X"},
		Text:      "  Discussed\tAPI\nmigration\r  ",
		KV:        "  status=active\tref=api_migration\n  ",
	}

	got := entry.MarshalTSV()
	want := []string{
		"2026-02-28T09:30:00.123456789Z",
		"event,project_x",
		"DiscussedAPImigration",
		"status=activeref=api_migration",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}

	entry.KV = " \n\t "
	got = entry.MarshalTSV()
	if got[3] != "-" {
		t.Fatalf("expected blank kv to default to '-', got %q", got[3])
	}
}

func TestLogEntryFormatLLM(t *testing.T) {
	entry := LogEntry{
		Tags: []string{"event", "project_x"},
		Text: "Discussed API migration",
		KV:   "status=active",
	}

	got := entry.FormatLLM()
	want := "event,project_x\tDiscussed API migration\tstatus=active"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{"Follow Up", "API", "follow up", " api ", "", "Project X"})
	want := []string{"follow_up", "api", "project_x"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestParseKVMalformedTokensAndDuplicateKeys(t *testing.T) {
	got := ParseKV("status=active badtoken expires=1 status=done invalid-key=x empty= ok=alpha=beta -")
	want := map[string]string{
		"status":  "done",
		"expires": "1",
		"empty":   "",
		"ok":      "alpha=beta",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}
