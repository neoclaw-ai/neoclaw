package scheduler

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"
)

func TestRunNowValidJobReturnsOutput(t *testing.T) {
	t.Parallel()

	svc := NewService(filepath.Join(t.TempDir(), "jobs.json"), NewRunner(ActionRunners{
		RunCommand: func(_ context.Context, args map[string]any) (string, error) {
			if args["command"] != "echo hello" {
				t.Fatalf("expected command echo hello, got %#v", args["command"])
			}
			return "ok", nil
		},
	}, nil))

	job, err := svc.Create(context.Background(), CreateInput{
		Description: "run now",
		Cron:        "0 9 * * *",
		Action:      ActionRunCommand,
		Args:        map[string]any{"command": "echo hello"},
		ChannelID:   "cli",
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	output, err := svc.RunNow(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("run now: %v", err)
	}
	if output != "ok" {
		t.Fatalf("expected output ok, got %q", output)
	}
}

func TestRunNowMissingJobReturnsError(t *testing.T) {
	t.Parallel()

	svc := NewService(filepath.Join(t.TempDir(), "jobs.json"), NewRunner(ActionRunners{
		RunCommand: func(_ context.Context, _ map[string]any) (string, error) {
			return "", nil
		},
	}, nil))

	_, err := svc.RunNow(context.Background(), "missing")
	if err == nil {
		t.Fatalf("expected missing job error")
	}
}

func TestStartRunNowStopRoundTrip(t *testing.T) {
	t.Parallel()

	called := 0
	svc := NewService(filepath.Join(t.TempDir(), "jobs.json"), NewRunner(ActionRunners{
		SendMessage: func(_ context.Context, _ io.Writer, args map[string]any) (string, error) {
			called++
			if args["message"] != "hello" {
				t.Fatalf("expected message hello, got %#v", args["message"])
			}
			return "sent", nil
		},
	}, map[string]io.Writer{
		"cli": io.Discard,
	}))
	job, err := svc.Create(context.Background(), CreateInput{
		Description: "round trip",
		Cron:        "0 9 * * *",
		Action:      ActionSendMessage,
		Args:        map[string]any{"message": "hello"},
		ChannelID:   "cli",
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	startCtx, startCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer startCancel()
	if err := svc.Start(startCtx); err != nil {
		t.Fatalf("start: %v", err)
	}

	output, err := svc.RunNow(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("run now: %v", err)
	}
	if output != "sent" {
		t.Fatalf("expected sent output, got %q", output)
	}
	if called != 1 {
		t.Fatalf("expected runner called once, got %d", called)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := svc.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestStartTwiceReturnsError(t *testing.T) {
	t.Parallel()

	svc := NewService(filepath.Join(t.TempDir(), "jobs.json"), NewRunner(ActionRunners{}, nil))

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("first start: %v", err)
	}
	err := svc.Start(context.Background())
	if err == nil {
		t.Fatalf("expected second start to fail")
	}
}

func TestStopExpiredContextOnUnstartedServiceReturnsNil(t *testing.T) {
	t.Parallel()

	svc := NewService(filepath.Join(t.TempDir(), "jobs.json"), NewRunner(ActionRunners{}, nil))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := svc.Stop(ctx)
	if err != nil {
		t.Fatalf("expected nil stop error for unstarted service, got %v", err)
	}
}

func TestRegisterAndUnregisterManageEntryMapping(t *testing.T) {
	t.Parallel()

	svc := NewService(filepath.Join(t.TempDir(), "jobs.json"), NewRunner(ActionRunners{
		SendMessage: func(_ context.Context, _ io.Writer, _ map[string]any) (string, error) {
			return "sent", nil
		},
	}, map[string]io.Writer{
		"cli": io.Discard,
	}))

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer svc.Stop(context.Background())

	job, err := svc.Create(context.Background(), CreateInput{
		Description: "dynamic register",
		Cron:        "0 9 * * *",
		Action:      ActionSendMessage,
		Args:        map[string]any{"message": "hello"},
		ChannelID:   "cli",
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	if err := svc.register(context.Background(), job); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := svc.store.entryID(job.ID); !ok {
		t.Fatalf("expected cron entry mapping for job %q", job.ID)
	}

	svc.unregister(job.ID)
	if _, ok := svc.store.entryID(job.ID); ok {
		t.Fatalf("expected cron entry mapping removed for job %q", job.ID)
	}
}
