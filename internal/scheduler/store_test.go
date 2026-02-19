package scheduler

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestStoreListMissingFileReturnsEmpty(t *testing.T) {
	t.Parallel()

	store := NewStore(filepath.Join(t.TempDir(), "jobs.json"))
	jobs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected no jobs, got %d", len(jobs))
	}
}

func TestStoreCreateListGetDelete(t *testing.T) {
	t.Parallel()

	store := NewStore(filepath.Join(t.TempDir(), "jobs.json"))
	created, err := store.Create(context.Background(), CreateInput{
		Description: "daily check-in",
		Cron:        "0 9 * * *",
		Action:      ActionSendMessage,
		Args: map[string]any{
			"message": "hello",
		},
		ChannelID: "cli",
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected non-empty id")
	}
	if created.ChannelID != "cli" {
		t.Fatalf("unexpected channel id: %q", created.ChannelID)
	}
	if !created.Enabled {
		t.Fatalf("expected default enabled=true")
	}

	jobs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	got, err := store.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("expected id %q, got %q", created.ID, got.ID)
	}

	if err := store.Delete(context.Background(), created.ID); err != nil {
		t.Fatalf("delete job: %v", err)
	}
	jobs, err = store.List(context.Background())
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected no jobs, got %d", len(jobs))
	}
}

func TestStoreEmptyArgsRoundTrip(t *testing.T) {
	t.Parallel()

	store := NewStore(filepath.Join(t.TempDir(), "jobs.json"))
	created, err := store.Create(context.Background(), CreateInput{
		Description: "empty args",
		Cron:        "0 9 * * *",
		Action:      ActionSendMessage,
		Args:        map[string]any{},
		ChannelID:   "cli",
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	listed, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 job, got %d", len(listed))
	}
	if listed[0].ID != created.ID {
		t.Fatalf("expected job id %q, got %q", created.ID, listed[0].ID)
	}
	if listed[0].Args == nil {
		t.Fatalf("expected args map, got nil")
	}
}

func TestStoreCreateValidation(t *testing.T) {
	t.Parallel()

	store := NewStore(filepath.Join(t.TempDir(), "jobs.json"))
	_, err := store.Create(context.Background(), CreateInput{
		Description: "bad cron",
		Cron:        "invalid",
		Action:      ActionSendMessage,
		Args:        map[string]any{"message": "x"},
		ChannelID:   "cli",
	})
	if err == nil {
		t.Fatalf("expected cron validation error")
	}

	_, err = store.Create(context.Background(), CreateInput{
		Description: "bad action",
		Cron:        "0 9 * * *",
		Action:      Action("weird"),
		Args:        map[string]any{"message": "x"},
		ChannelID:   "cli",
	})
	if err == nil {
		t.Fatalf("expected action validation error")
	}

	_, err = store.Create(context.Background(), CreateInput{
		Description: "missing channel",
		Cron:        "0 9 * * *",
		Action:      ActionSendMessage,
		Args:        map[string]any{"message": "x"},
		ChannelID:   "",
	})
	if err == nil {
		t.Fatalf("expected channel validation error")
	}
}

func TestStoreGetNotFound(t *testing.T) {
	t.Parallel()

	store := NewStore(filepath.Join(t.TempDir(), "jobs.json"))
	_, err := store.Get(context.Background(), "missing")
	if err == nil {
		t.Fatalf("expected not found error")
	}
}

func TestStoreContextCancel(t *testing.T) {
	t.Parallel()

	store := NewStore(filepath.Join(t.TempDir(), "jobs.json"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.List(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}

	_, err = store.Create(ctx, CreateInput{
		Description: "x",
		Cron:        "0 9 * * *",
		Action:      ActionSendMessage,
		Args:        map[string]any{"message": "x"},
		ChannelID:   "cli",
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}
