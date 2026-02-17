package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/machinae/betterclaw/internal/provider"
)

func TestStoreAppendLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions", "cli", "default.jsonl")
	store := New(path)

	input := []provider.ChatMessage{
		{Role: provider.RoleUser, Content: "hello"},
		{
			Kind:    "summary",
			Role:    provider.RoleAssistant,
			Content: "summary text",
		},
		{
			Role:    provider.RoleAssistant,
			Content: "calling tool",
			ToolCalls: []provider.ToolCall{
				{ID: "1", Name: "list_dir", Arguments: `{"path":"."}`},
			},
		},
		{
			Role:       provider.RoleTool,
			ToolCallID: "1",
			Content:    "file1\nfile2",
		},
	}

	if err := store.Append(context.Background(), input); err != nil {
		t.Fatalf("append: %v", err)
	}

	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != len(input) {
		t.Fatalf("expected %d messages, got %d", len(input), len(got))
	}
	if got[1].Kind != "summary" {
		t.Fatalf("expected summary kind to round-trip, got %q", got[1].Kind)
	}
	if got[2].ToolCalls[0].Name != "list_dir" {
		t.Fatalf("expected tool call to round-trip, got %#v", got[2].ToolCalls)
	}
}

func TestStoreLoadSkipsMalformedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions", "cli", "default.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := []byte("{bad json}\n{\"role\":\"user\",\"content\":\"ok\"}\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	store := New(path)
	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 || got[0].Content != "ok" {
		t.Fatalf("expected only valid record, got %#v", got)
	}
}

func TestStoreResetClearsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions", "cli", "default.jsonl")
	store := New(path)
	if err := store.Append(context.Background(), []provider.ChatMessage{
		{Role: provider.RoleUser, Content: "hello"},
	}); err != nil {
		t.Fatalf("append: %v", err)
	}

	if err := store.Reset(context.Background()); err != nil {
		t.Fatalf("reset: %v", err)
	}
	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty history, got %#v", got)
	}
}
