package channels

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/machinae/betterclaw/internal/approval"
	runtimeapi "github.com/machinae/betterclaw/internal/runtime"
)

func TestCLIListenerListenDispatchesMessages(t *testing.T) {
	out := &bytes.Buffer{}
	listener := NewCLI(strings.NewReader("hello\nquit\n"), out)

	handler := &testHandler{response: "ok"}
	err := listener.Listen(context.Background(), handler)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	if len(handler.messages) != 1 || handler.messages[0] != "hello" {
		t.Fatalf("expected one dispatched message, got %#v", handler.messages)
	}
	if got := out.String(); !strings.Contains(got, "assistant> ok") {
		t.Fatalf("expected assistant output, got %q", got)
	}
}

func TestCLIListenerListenExitsOnExitCommands(t *testing.T) {
	listener := NewCLI(strings.NewReader("/exit\n"), &bytes.Buffer{})
	handler := &testHandler{response: "unused"}

	err := listener.Listen(context.Background(), handler)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	if len(handler.messages) != 0 {
		t.Fatalf("expected no handler calls, got %#v", handler.messages)
	}
}

func TestCLIListenerListenPropagatesFatalHandlerError(t *testing.T) {
	listener := NewCLI(strings.NewReader("hello\n"), &bytes.Buffer{})
	handler := &testHandler{err: errors.New("fatal")}

	err := listener.Listen(context.Background(), handler)
	if err == nil || !strings.Contains(err.Error(), "fatal") {
		t.Fatalf("expected fatal error, got %v", err)
	}
}

func TestCLIListenerRequestApproval(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected approval.ApprovalDecision
	}{
		{name: "approved", input: "y\n", expected: approval.Approved},
		{name: "always approved", input: "a\n", expected: approval.AlwaysApproved},
		{name: "denied", input: "n\n", expected: approval.Denied},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			listener := NewCLI(strings.NewReader(tc.input), out)

			decision, err := listener.RequestApproval(context.Background(), approval.ApprovalRequest{
				Tool:        "run_command",
				Description: `run_command {"command":"pwd"}`,
			})
			if err != nil {
				t.Fatalf("request approval: %v", err)
			}
			if decision != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, decision)
			}
			if got := out.String(); !strings.Contains(got, "approve tool run_command?") {
				t.Fatalf("expected prompt output, got %q", got)
			}
		})
	}
}

type testHandler struct {
	messages []string
	response string
	err      error
}

func (h *testHandler) HandleMessage(ctx context.Context, w runtimeapi.ResponseWriter, msg *runtimeapi.Message) error {
	h.messages = append(h.messages, msg.Text)
	if h.err != nil {
		return h.err
	}
	return w.WriteMessage(ctx, h.response)
}
