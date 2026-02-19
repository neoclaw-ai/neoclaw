package scheduler

import (
	"context"
	"strings"
	"testing"
)

func TestNewRunnerDispatch(t *testing.T) {
	t.Parallel()

	r := NewRunner(ActionRunners{
		SendMessage: func(_ context.Context, args map[string]any) (string, error) {
			if args["message"] != "hello" {
				t.Fatalf("unexpected send args: %#v", args)
			}
			return "sent", nil
		},
		RunCommand: func(_ context.Context, args map[string]any) (string, error) {
			if args["command"] != "pwd" {
				t.Fatalf("unexpected run args: %#v", args)
			}
			return "ran", nil
		},
		HTTPRequest: func(_ context.Context, args map[string]any) (string, error) {
			if args["url"] != "https://example.org" {
				t.Fatalf("unexpected http args: %#v", args)
			}
			return "fetched", nil
		},
	})

	out, err := r.Run(context.Background(), Job{Action: ActionSendMessage, Args: map[string]any{"message": "hello"}})
	if err != nil || out != "sent" {
		t.Fatalf("send: out=%q err=%v", out, err)
	}
	out, err = r.Run(context.Background(), Job{Action: ActionRunCommand, Args: map[string]any{"command": "pwd"}})
	if err != nil || out != "ran" {
		t.Fatalf("run: out=%q err=%v", out, err)
	}
	out, err = r.Run(context.Background(), Job{Action: ActionHTTPRequest, Args: map[string]any{"url": "https://example.org"}})
	if err != nil || out != "fetched" {
		t.Fatalf("http: out=%q err=%v", out, err)
	}
}

func TestNewRunnerMissingActionRunner(t *testing.T) {
	t.Parallel()

	r := NewRunner(ActionRunners{})
	_, err := r.Run(context.Background(), Job{Action: ActionRunCommand, Args: map[string]any{"command": "pwd"}})
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected missing runner error, got %v", err)
	}
}
