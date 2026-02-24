package tools

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neoclaw-ai/neoclaw/internal/scheduler"
)

func TestJobCreateListDeleteTools(t *testing.T) {
	t.Parallel()

	svc := scheduler.NewService(filepath.Join(t.TempDir(), "jobs.json"), scheduler.NewRunner(scheduler.ActionRunners{}, nil))
	createTool := JobCreateTool{Service: svc, ChannelID: "cli"}
	listTool := JobListTool{Service: svc}
	deleteTool := JobDeleteTool{Service: svc}

	created, err := createTool.Execute(context.Background(), map[string]any{
		"description": "daily ping",
		"cron":        "0 9 * * *",
		"action":      "send_message",
		"args": map[string]any{
			"message": "hello",
		},
	})
	if err != nil {
		t.Fatalf("create tool execute: %v", err)
	}
	if !strings.Contains(created.Output, "created job") {
		t.Fatalf("expected create output, got %q", created.Output)
	}

	listed, err := listTool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tool execute: %v", err)
	}
	if !strings.Contains(listed.Output, "daily ping") {
		t.Fatalf("expected list output to contain description, got %q", listed.Output)
	}

	jobs, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("store list: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	deleted, err := deleteTool.Execute(context.Background(), map[string]any{"id": jobs[0].ID})
	if err != nil {
		t.Fatalf("delete tool execute: %v", err)
	}
	if deleted.Output != "deleted" {
		t.Fatalf("expected deleted output, got %q", deleted.Output)
	}

	listed, err = listTool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tool execute after delete: %v", err)
	}
	if listed.Output != "No scheduled jobs." {
		t.Fatalf("expected empty list output, got %q", listed.Output)
	}
}

func TestJobCreateToolDefaultsChannelID(t *testing.T) {
	t.Parallel()

	svc := scheduler.NewService(filepath.Join(t.TempDir(), "jobs.json"), scheduler.NewRunner(scheduler.ActionRunners{}, nil))
	createTool := JobCreateTool{Service: svc}

	_, err := createTool.Execute(context.Background(), map[string]any{
		"description": "daily ping",
		"cron":        "0 9 * * *",
		"action":      "send_message",
		"args": map[string]any{
			"message": "hello",
		},
	})
	if err != nil {
		t.Fatalf("create tool execute: %v", err)
	}

	jobs, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("store list: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].ChannelID != "cli" {
		t.Fatalf("expected default channel cli, got %q", jobs[0].ChannelID)
	}
}

func TestJobCreateToolUsesResolvedChannelID(t *testing.T) {
	t.Parallel()

	svc := scheduler.NewService(filepath.Join(t.TempDir(), "jobs.json"), scheduler.NewRunner(scheduler.ActionRunners{}, nil))
	createTool := JobCreateTool{
		Service:          svc,
		ChannelID:        "cli",
		ResolveChannelID: func() string { return "telegram-12345" },
	}

	_, err := createTool.Execute(context.Background(), map[string]any{
		"description": "daily ping",
		"cron":        "0 9 * * *",
		"action":      "send_message",
		"args": map[string]any{
			"message": "hello",
		},
	})
	if err != nil {
		t.Fatalf("create tool execute: %v", err)
	}

	jobs, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("store list: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].ChannelID != "telegram-12345" {
		t.Fatalf("expected resolved channel id telegram-12345, got %q", jobs[0].ChannelID)
	}
}

func TestJobRunToolRunsService(t *testing.T) {
	t.Parallel()

	svc := scheduler.NewService(filepath.Join(t.TempDir(), "jobs.json"), scheduler.NewRunner(scheduler.ActionRunners{
		SendMessage: func(_ context.Context, _ io.Writer, args map[string]any) (string, error) {
			if args["message"] != "hello" {
				t.Fatalf("expected message hello, got %#v", args["message"])
			}
			return "run ok", nil
		},
	}, map[string]io.Writer{
		"cli": io.Discard,
	}))
	job, err := svc.Create(context.Background(), scheduler.CreateInput{
		Description: "run now",
		Cron:        "0 9 * * *",
		Action:      scheduler.ActionSendMessage,
		Args: map[string]any{
			"message": "hello",
		},
		ChannelID: "cli",
	})
	if err != nil {
		t.Fatalf("store create: %v", err)
	}

	runTool := JobRunTool{Service: svc}
	result, err := runTool.Execute(context.Background(), map[string]any{"id": job.ID})
	if err != nil {
		t.Fatalf("job_run execute: %v", err)
	}
	if result.Output != "run ok" {
		t.Fatalf("expected run output, got %q", result.Output)
	}
}
