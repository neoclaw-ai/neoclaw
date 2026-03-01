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

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{"Follow Up", "API", "follow up", " api ", "", "Project X"})
	want := []string{"follow_up", "api", "project_x"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}
