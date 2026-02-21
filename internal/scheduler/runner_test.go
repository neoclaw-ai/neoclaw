package scheduler

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestNewRunnerDispatch(t *testing.T) {
	t.Parallel()

	telegramWriter := &bytes.Buffer{}
	r := NewRunner(ActionRunners{
		SendMessage: func(_ context.Context, writer io.Writer, args map[string]any) (string, error) {
			if writer != telegramWriter {
				t.Fatalf("unexpected send writer: %#v", writer)
			}
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
	}, map[string]io.Writer{
		"telegram-123": telegramWriter,
	})

	out, err := r.Run(context.Background(), Job{
		Action:    ActionSendMessage,
		ChannelID: "telegram-123",
		Args:      map[string]any{"message": "hello"},
	})
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

	r := NewRunner(ActionRunners{}, nil)
	_, err := r.Run(context.Background(), Job{Action: ActionRunCommand, Args: map[string]any{"command": "pwd"}})
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected missing runner error, got %v", err)
	}
}

func TestNewRunnerUnknownSendMessageChannelIsSkipped(t *testing.T) {
	t.Parallel()

	called := 0
	r := NewRunner(ActionRunners{
		SendMessage: func(_ context.Context, _ io.Writer, _ map[string]any) (string, error) {
			called++
			return "sent", nil
		},
	}, map[string]io.Writer{
		"telegram-123": &bytes.Buffer{},
	})

	out, err := r.Run(context.Background(), Job{
		Action:    ActionSendMessage,
		ChannelID: "telegram-999",
		Args:      map[string]any{"message": "hello"},
	})
	if err != nil {
		t.Fatalf("send: err=%v", err)
	}
	if out != "" {
		t.Fatalf("expected empty output for unknown channel, got %q", out)
	}
	if called != 0 {
		t.Fatalf("expected send runner not to be called for unknown channel, got %d", called)
	}
}
