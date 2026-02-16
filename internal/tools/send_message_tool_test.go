package tools

import (
	"bytes"
	"context"
	"testing"
)

func TestSendMessageStubWritesToWriter(t *testing.T) {
	var out bytes.Buffer
	tool := SendMessageTool{Writer: &out}

	res, err := tool.Execute(context.Background(), map[string]any{"message": "hello"})
	if err != nil {
		t.Fatalf("execute send_message: %v", err)
	}
	if res.Output != "sent" {
		t.Fatalf("expected sent output, got %q", res.Output)
	}
	if got := out.String(); got != "hello\n" {
		t.Fatalf("expected writer output %q, got %q", "hello\n", got)
	}
}

func TestSendMessageWithSender(t *testing.T) {
	sender := &fakeChannelSender{}
	tool := SendMessageTool{Sender: sender}

	res, err := tool.Execute(context.Background(), map[string]any{"message": "hello"})
	if err != nil {
		t.Fatalf("execute send_message: %v", err)
	}
	if res.Output != "sent" {
		t.Fatalf("expected sent output, got %q", res.Output)
	}
	if sender.last != "hello" {
		t.Fatalf("expected sender to receive message")
	}
}

type fakeChannelSender struct {
	last string
}

func (s *fakeChannelSender) Send(_ context.Context, message string) error {
	s.last = message
	return nil
}
