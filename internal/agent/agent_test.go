package agent

import (
	"context"
	"errors"
	"testing"

	runtimeapi "github.com/machinae/betterclaw/internal/runtime"

	"github.com/machinae/betterclaw/internal/approval"
	providerapi "github.com/machinae/betterclaw/internal/provider"
	"github.com/machinae/betterclaw/internal/tools"
)

func TestAgentHandleMessageWritesResponse(t *testing.T) {
	registry := tools.NewRegistry()
	modelProvider := &recordingProvider{
		responses: []*providerapi.ChatResponse{{Content: "hello"}},
	}
	ag := New(modelProvider, registry, noopApprover{}, DefaultSystemPrompt)
	writer := &captureWriter{}

	err := ag.HandleMessage(context.Background(), writer, &runtimeapi.Message{Text: "hi"})
	if err != nil {
		t.Fatalf("handle message: %v", err)
	}
	if len(writer.messages) != 1 || writer.messages[0] != "hello" {
		t.Fatalf("expected one assistant message, got %#v", writer.messages)
	}
}

func TestAgentHandleMessageAccumulatesHistory(t *testing.T) {
	registry := tools.NewRegistry()
	modelProvider := &recordingProvider{
		responses: []*providerapi.ChatResponse{
			{Content: "r1"},
			{Content: "r2"},
		},
	}
	ag := New(modelProvider, registry, noopApprover{}, DefaultSystemPrompt)
	writer := &captureWriter{}

	if err := ag.HandleMessage(context.Background(), writer, &runtimeapi.Message{Text: "one"}); err != nil {
		t.Fatalf("first handle message: %v", err)
	}
	if err := ag.HandleMessage(context.Background(), writer, &runtimeapi.Message{Text: "two"}); err != nil {
		t.Fatalf("second handle message: %v", err)
	}

	if len(modelProvider.requests) != 2 {
		t.Fatalf("expected 2 provider requests, got %d", len(modelProvider.requests))
	}
	if got := len(modelProvider.requests[1].Messages); got != 3 {
		t.Fatalf("expected second request to include prior history, got %d messages", got)
	}
}

func TestAgentHandleMessageProviderErrorIsFatal(t *testing.T) {
	registry := tools.NewRegistry()
	modelProvider := &recordingProvider{
		err: errors.New("provider unavailable"),
	}
	ag := New(modelProvider, registry, noopApprover{}, DefaultSystemPrompt)
	writer := &captureWriter{}

	err := ag.HandleMessage(context.Background(), writer, &runtimeapi.Message{Text: "hi"})
	if err == nil {
		t.Fatalf("expected fatal error")
	}
	if len(writer.messages) != 0 {
		t.Fatalf("expected no user-facing response, got %#v", writer.messages)
	}
}

func TestAgentHandleMessageCanceledContextIsFatal(t *testing.T) {
	registry := tools.NewRegistry()
	modelProvider := &recordingProvider{
		requireLiveContext: true,
	}
	ag := New(modelProvider, registry, noopApprover{}, DefaultSystemPrompt)
	writer := &captureWriter{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ag.HandleMessage(ctx, writer, &runtimeapi.Message{Text: "hi"})
	if err == nil {
		t.Fatalf("expected fatal cancellation error")
	}
}

type noopApprover struct{}

func (noopApprover) RequestApproval(context.Context, approval.ApprovalRequest) (approval.ApprovalDecision, error) {
	return approval.Approved, nil
}

type captureWriter struct {
	messages []string
}

func (w *captureWriter) WriteMessage(_ context.Context, text string) error {
	w.messages = append(w.messages, text)
	return nil
}

type recordingProvider struct {
	requests           []providerapi.ChatRequest
	responses          []*providerapi.ChatResponse
	err                error
	requireLiveContext bool
}

func (p *recordingProvider) Chat(ctx context.Context, req providerapi.ChatRequest) (*providerapi.ChatResponse, error) {
	p.requests = append(p.requests, req)
	if p.requireLiveContext && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if p.err != nil {
		return nil, p.err
	}
	if len(p.responses) == 0 {
		return &providerapi.ChatResponse{Content: ""}, nil
	}
	resp := p.responses[0]
	p.responses = p.responses[1:]
	return resp, nil
}
