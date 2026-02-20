package channels

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/runtime"
)

func TestCLIListenerListenDispatchesMessages(t *testing.T) {
	out := &bytes.Buffer{}
	listener := NewCLI(strings.NewReader("hello\n"), out)

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
	out := &bytes.Buffer{}
	listener := NewCLI(strings.NewReader("/exit\n"), out)
	handler := &testHandler{response: "unused"}

	err := listener.Listen(context.Background(), handler)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	if len(handler.messages) != 0 {
		t.Fatalf("expected no handler calls, got %#v", handler.messages)
	}
	if got := out.String(); !strings.Contains(got, "assistant> Stopped.") {
		t.Fatalf("expected stop output, got %q", got)
	}
}

func TestCLIListenerListenHandlesStopWithoutDispatch(t *testing.T) {
	out := &bytes.Buffer{}
	listener := NewCLI(strings.NewReader("/stop\n/quit\n"), out)
	handler := &testHandler{response: "unused"}

	err := listener.Listen(context.Background(), handler)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	if len(handler.messages) != 0 {
		t.Fatalf("expected no handler calls, got %#v", handler.messages)
	}
	if got := out.String(); strings.Count(got, "assistant> Stopped.") < 2 {
		t.Fatalf("expected stop output for /stop and /quit, got %q", got)
	}
}

func TestCLIListenerListenWritesFatalHandlerError(t *testing.T) {
	out := &bytes.Buffer{}
	listener := NewCLI(strings.NewReader("hello\n"), out)
	handler := &testHandler{err: errors.New("fatal")}

	err := listener.Listen(context.Background(), handler)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "assistant> error: fatal") {
		t.Fatalf("expected error output, got %q", got)
	}
}

func TestCLIListenerRequestApproval(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected approval.ApprovalDecision
	}{
		{name: "approved", input: "y\n", expected: approval.Approved},
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
			if got := out.String(); !strings.Contains(got, "[y/N]") {
				t.Fatalf("expected explicit y/N prompt, got %q", got)
			}
		})
	}
}

func TestTelegramPairSessionSubmitCodeWrongReturnsErrWrongCode(t *testing.T) {
	session := &TelegramPairSession{
		expectedCode: "123456",
	}

	err := session.SubmitCode(context.Background(), "000000")
	if !errors.Is(err, ErrWrongCode) {
		t.Fatalf("expected ErrWrongCode, got %v", err)
	}
}

func TestGenerateTelegramPairCode_IsSixDigits(t *testing.T) {
	re := regexp.MustCompile(`^\d{6}$`)

	for range 20 {
		code, err := generateTelegramPairCode()
		if err != nil {
			t.Fatalf("generate code: %v", err)
		}
		if !re.MatchString(code) {
			t.Fatalf("expected 6-digit code, got %q", code)
		}
	}
}

type testHandler struct {
	messages []string
	response string
	err      error
}

func (h *testHandler) HandleMessage(ctx context.Context, w runtime.ResponseWriter, msg *runtime.Message) error {
	h.messages = append(h.messages, msg.Text)
	if h.err != nil {
		return h.err
	}
	return w.WriteMessage(ctx, h.response)
}
