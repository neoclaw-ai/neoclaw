package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/machinae/betterclaw/internal/llm"
	"github.com/machinae/betterclaw/internal/tools"
)

func TestRun_DispatchesToolAndReturnsFinalResponse(t *testing.T) {
	registry := tools.NewRegistry()
	if err := registry.Register(fakeTool{name: "read_file", out: "hello from file"}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	provider := &scriptProvider{responses: []*llm.ChatResponse{
		{
			ToolCalls: []llm.ToolCall{{
				ID:        "call_1",
				Name:      "read_file",
				Arguments: `{"path":"README.md"}`,
			}},
		},
		{Content: "done"},
	}}

	resp, history, err := Run(
		context.Background(),
		provider,
		registry,
		nil,
		"system",
		[]llm.ChatMessage{{Role: llm.RoleUser, Content: "read it"}},
		10,
	)
	if err != nil {
		t.Fatalf("run loop: %v", err)
	}
	if resp.Content != "done" {
		t.Fatalf("expected final response done, got %q", resp.Content)
	}
	if provider.calls != 2 {
		t.Fatalf("expected 2 provider calls, got %d", provider.calls)
	}

	var foundToolResult bool
	for _, msg := range history {
		if msg.Role == llm.RoleTool && msg.ToolCallID == "call_1" && msg.Content == "hello from file" {
			foundToolResult = true
		}
	}
	if !foundToolResult {
		t.Fatalf("expected tool result to be appended to history")
	}
}

func TestRun_MaxIterationsGuard(t *testing.T) {
	registry := tools.NewRegistry()
	if err := registry.Register(fakeTool{name: "read_file", out: "x"}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	provider := &scriptProvider{responses: []*llm.ChatResponse{
		{
			ToolCalls: []llm.ToolCall{{
				ID:        "1",
				Name:      "read_file",
				Arguments: `{"path":"a"}`,
			}},
		},
		{
			ToolCalls: []llm.ToolCall{{
				ID:        "2",
				Name:      "read_file",
				Arguments: `{"path":"b"}`,
			}},
		},
	}}

	_, _, err := Run(
		context.Background(),
		provider,
		registry,
		nil,
		"system",
		[]llm.ChatMessage{{Role: llm.RoleUser, Content: "loop"}},
		1,
	)
	if err == nil || !strings.Contains(err.Error(), "max iterations exceeded") {
		t.Fatalf("expected max iterations error, got %v", err)
	}
}

func TestRun_UnknownToolAppendsErrorAndContinues(t *testing.T) {
	registry := tools.NewRegistry()
	if err := registry.Register(fakeTool{name: "read_file", out: "ok"}); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	provider := &scriptProvider{responses: []*llm.ChatResponse{
		{
			ToolCalls: []llm.ToolCall{{
				ID:        "call_1",
				Name:      "does_not_exist",
				Arguments: `{}`,
			}},
		},
		{Content: "fallback complete"},
	}}

	resp, history, err := Run(
		context.Background(),
		provider,
		registry,
		nil,
		"system",
		[]llm.ChatMessage{{Role: llm.RoleUser, Content: "do it"}},
		2,
	)
	if err != nil {
		t.Fatalf("expected loop to continue after unknown tool, got %v", err)
	}
	if resp.Content != "fallback complete" {
		t.Fatalf("expected fallback response, got %q", resp.Content)
	}

	var foundUnknownToolMessage bool
	for _, msg := range history {
		if msg.Role == llm.RoleTool && msg.ToolCallID == "call_1" && strings.Contains(msg.Content, "unknown tool") {
			foundUnknownToolMessage = true
		}
	}
	if !foundUnknownToolMessage {
		t.Fatalf("expected unknown tool message in history")
	}
}

type scriptProvider struct {
	responses []*llm.ChatResponse
	calls     int
}

func (p *scriptProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	if p.calls >= len(p.responses) {
		return nil, fmt.Errorf("unexpected extra call")
	}
	resp := p.responses[p.calls]
	p.calls++
	return resp, nil
}

type fakeTool struct {
	name string
	out  string
}

func (t fakeTool) Name() string                 { return t.name }
func (t fakeTool) Description() string          { return t.name }
func (t fakeTool) Schema() map[string]any       { return map[string]any{"type": "object"} }
func (t fakeTool) Permission() tools.Permission { return tools.AutoApprove }
func (t fakeTool) Execute(_ context.Context, _ map[string]any) (*tools.ToolResult, error) {
	return &tools.ToolResult{Output: t.out}, nil
}
