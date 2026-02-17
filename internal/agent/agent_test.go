package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/machinae/betterclaw/internal/memory"
	"github.com/machinae/betterclaw/internal/runtime"

	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/provider"
	"github.com/machinae/betterclaw/internal/session"
	"github.com/machinae/betterclaw/internal/tools"
)

func TestAgentHandleMessageWritesResponse(t *testing.T) {
	registry := tools.NewRegistry()
	modelProvider := &recordingProvider{
		responses: []*provider.ChatResponse{{Content: "hello"}},
	}
	ag := New(modelProvider, registry, noopApprover{}, DefaultSystemPrompt)
	writer := &captureWriter{}

	err := ag.HandleMessage(context.Background(), writer, &runtime.Message{Text: "hi"})
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
		responses: []*provider.ChatResponse{
			{Content: "r1"},
			{Content: "r2"},
		},
	}
	ag := New(modelProvider, registry, noopApprover{}, DefaultSystemPrompt)
	writer := &captureWriter{}

	if err := ag.HandleMessage(context.Background(), writer, &runtime.Message{Text: "one"}); err != nil {
		t.Fatalf("first handle message: %v", err)
	}
	if err := ag.HandleMessage(context.Background(), writer, &runtime.Message{Text: "two"}); err != nil {
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

	err := ag.HandleMessage(context.Background(), writer, &runtime.Message{Text: "hi"})
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

	err := ag.HandleMessage(ctx, writer, &runtime.Message{Text: "hi"})
	if err == nil {
		t.Fatalf("expected fatal cancellation error")
	}
}

func TestAgentWithSessionLoadsHistoryAndAppendsTurn(t *testing.T) {
	registry := tools.NewRegistry()
	modelProvider := &recordingProvider{
		responses: []*provider.ChatResponse{{Content: "new response"}},
	}
	sessionPath := filepath.Join(t.TempDir(), "sessions", "cli", "default.jsonl")
	sessionStore := session.New(sessionPath)
	if err := sessionStore.Append(context.Background(), []provider.ChatMessage{
		{Role: provider.RoleUser, Content: "old user"},
		{Role: provider.RoleAssistant, Content: "old assistant"},
	}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	ag := NewWithSession(modelProvider, registry, noopApprover{}, DefaultSystemPrompt, sessionStore, nil, 4000, 10, time.Second)
	writer := &captureWriter{}

	if err := ag.HandleMessage(context.Background(), writer, &runtime.Message{Text: "next"}); err != nil {
		t.Fatalf("handle message: %v", err)
	}
	if len(modelProvider.requests) != 1 {
		t.Fatalf("expected one provider request, got %d", len(modelProvider.requests))
	}
	if got := len(modelProvider.requests[0].Messages); got != 3 {
		t.Fatalf("expected loaded history + new user (3 messages), got %d", got)
	}

	loaded, err := sessionStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if len(loaded) != 4 {
		t.Fatalf("expected 4 persisted messages, got %d", len(loaded))
	}
	if loaded[2].Role != provider.RoleUser || loaded[2].Content != "next" {
		t.Fatalf("expected persisted user message, got %#v", loaded[2])
	}
	if loaded[3].Role != provider.RoleAssistant || loaded[3].Content != "new response" {
		t.Fatalf("expected persisted assistant message, got %#v", loaded[3])
	}
}

func TestAgentHandleMessageNewResetsSession(t *testing.T) {
	registry := tools.NewRegistry()
	modelProvider := &recordingProvider{
		responses: []*provider.ChatResponse{{Content: "after reset"}},
	}
	sessionPath := filepath.Join(t.TempDir(), "sessions", "cli", "default.jsonl")
	sessionStore := session.New(sessionPath)
	if err := sessionStore.Append(context.Background(), []provider.ChatMessage{
		{Role: provider.RoleUser, Content: "old user"},
		{Role: provider.RoleAssistant, Content: "old assistant"},
	}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	ag := NewWithSession(modelProvider, registry, noopApprover{}, DefaultSystemPrompt, sessionStore, nil, 4000, 10, time.Second)
	writer := &captureWriter{}

	if err := ag.HandleMessage(context.Background(), writer, &runtime.Message{Text: "/new"}); err != nil {
		t.Fatalf("handle /new: %v", err)
	}
	if len(modelProvider.requests) != 0 {
		t.Fatalf("expected no provider call for /new")
	}
	if len(writer.messages) != 1 || writer.messages[0] != "Session cleared." {
		t.Fatalf("expected session cleared response, got %#v", writer.messages)
	}

	loaded, err := sessionStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected empty session after /new, got %#v", loaded)
	}

	if err := ag.HandleMessage(context.Background(), writer, &runtime.Message{Text: "fresh"}); err != nil {
		t.Fatalf("handle post-reset message: %v", err)
	}
	if len(modelProvider.requests) != 1 {
		t.Fatalf("expected one provider request after reset, got %d", len(modelProvider.requests))
	}
	if got := len(modelProvider.requests[0].Messages); got != 1 {
		t.Fatalf("expected only fresh user message after reset, got %d", got)
	}
}

func TestAgentHandleMessageNewWritesSummaryToDailyLog(t *testing.T) {
	registry := tools.NewRegistry()
	modelProvider := &recordingProvider{
		responses: []*provider.ChatResponse{{Content: "session summary"}},
	}
	sessionPath := filepath.Join(t.TempDir(), "sessions", "cli", "default.jsonl")
	sessionStore := session.New(sessionPath)
	if err := sessionStore.Append(context.Background(), []provider.ChatMessage{
		{Role: provider.RoleUser, Content: "old user"},
		{Role: provider.RoleAssistant, Content: "old assistant"},
	}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	memoryDir := filepath.Join(t.TempDir(), "memory")
	memoryStore := memory.New(memoryDir)
	ag := NewWithSession(modelProvider, registry, noopApprover{}, DefaultSystemPrompt, sessionStore, memoryStore, 4000, 10, time.Second)
	writer := &captureWriter{}

	if err := ag.HandleMessage(context.Background(), writer, &runtime.Message{Text: "/new"}); err != nil {
		t.Fatalf("handle /new: %v", err)
	}
	if len(writer.messages) != 1 || writer.messages[0] != "Session cleared." {
		t.Fatalf("expected session cleared response, got %#v", writer.messages)
	}

	var dailyContent string
	waitFor(t, 2*time.Second, func() bool {
		path := filepath.Join(memoryDir, "daily", time.Now().Format("2006-01-02")+".md")
		raw, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		dailyContent = string(raw)
		return strings.Contains(dailyContent, "Summary: session summary")
	})

	if !strings.Contains(dailyContent, "Summary: session summary") {
		t.Fatalf("expected summary line in daily log, got %q", dailyContent)
	}
}

func TestAgentHandleMessageNewSkipsSummaryOnEmptyHistory(t *testing.T) {
	registry := tools.NewRegistry()
	modelProvider := &recordingProvider{
		responses: []*provider.ChatResponse{{Content: "unexpected summary call"}},
	}
	sessionPath := filepath.Join(t.TempDir(), "sessions", "cli", "default.jsonl")
	sessionStore := session.New(sessionPath)
	memoryDir := filepath.Join(t.TempDir(), "memory")
	memoryStore := memory.New(memoryDir)
	ag := NewWithSession(modelProvider, registry, noopApprover{}, DefaultSystemPrompt, sessionStore, memoryStore, 4000, 10, time.Second)
	writer := &captureWriter{}

	if err := ag.HandleMessage(context.Background(), writer, &runtime.Message{Text: "/new"}); err != nil {
		t.Fatalf("handle /new: %v", err)
	}
	if len(writer.messages) != 1 || writer.messages[0] != "Session cleared." {
		t.Fatalf("expected session cleared response, got %#v", writer.messages)
	}

	time.Sleep(50 * time.Millisecond)
	if len(modelProvider.requests) != 0 {
		t.Fatalf("expected no summary provider call for empty history, got %d", len(modelProvider.requests))
	}
	if _, err := os.Stat(filepath.Join(memoryDir, "daily")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no daily log directory, stat err=%v", err)
	}
}

func TestCompactHistoryIfNeededAddsSummaryMessage(t *testing.T) {
	modelProvider := &recordingProvider{
		responses: []*provider.ChatResponse{{Content: "summary output"}},
	}
	ag := New(modelProvider, tools.NewRegistry(), noopApprover{}, DefaultSystemPrompt)
	ag.maxContextTokens = 10
	ag.recentMessages = 2
	messages := []provider.ChatMessage{
		{Role: provider.RoleUser, Content: "1111111111"},
		{Role: provider.RoleAssistant, Content: "2222222222"},
		{Role: provider.RoleUser, Content: "3333333333"},
		{Role: provider.RoleAssistant, Content: "4444444444"},
	}

	compacted, err := ag.compactHistoryIfNeeded(context.Background(), messages)
	if err != nil {
		t.Fatalf("compact history: %v", err)
	}
	if len(compacted) != 3 {
		t.Fatalf("expected summary + 2 recent messages, got %d", len(compacted))
	}
	if compacted[0].Kind != summaryKind || compacted[0].Role != provider.RoleAssistant || compacted[0].Content != "summary output" {
		t.Fatalf("expected summary message, got %#v", compacted[0])
	}
	if len(modelProvider.requests) != 1 {
		t.Fatalf("expected one summary provider request, got %d", len(modelProvider.requests))
	}
}

func TestCompactHistoryIfNeededFallbackRecentOnlyOnSummaryFailure(t *testing.T) {
	modelProvider := &recordingProvider{
		errs: []error{errors.New("summary failed")},
	}
	ag := New(modelProvider, tools.NewRegistry(), noopApprover{}, DefaultSystemPrompt)
	ag.maxContextTokens = 10
	ag.recentMessages = 2
	messages := []provider.ChatMessage{
		{Role: provider.RoleUser, Content: "1111111111"},
		{Role: provider.RoleAssistant, Content: "2222222222"},
		{Role: provider.RoleUser, Content: "3333333333"},
		{Role: provider.RoleAssistant, Content: "4444444444"},
	}

	compacted, err := ag.compactHistoryIfNeeded(context.Background(), messages)
	if err != nil {
		t.Fatalf("compact history: %v", err)
	}
	if len(compacted) != 2 {
		t.Fatalf("expected recent-only fallback of 2 messages, got %d", len(compacted))
	}
	if compacted[0].Content != "3333333333" || compacted[1].Content != "4444444444" {
		t.Fatalf("unexpected recent-only fallback messages: %#v", compacted)
	}
}

func TestAgentSessionStoresTruncatedToolOutput(t *testing.T) {
	registry := tools.NewRegistry()
	if err := registry.Register(truncatingTool{}); err != nil {
		t.Fatalf("register truncating tool: %v", err)
	}

	modelProvider := &recordingProvider{
		responses: []*provider.ChatResponse{
			{
				ToolCalls: []provider.ToolCall{
					{ID: "call_1", Name: "truncating_tool", Arguments: "{}"},
				},
			},
			{Content: "done"},
		},
	}

	sessionPath := filepath.Join(t.TempDir(), "sessions", "cli", "default.jsonl")
	sessionStore := session.New(sessionPath)
	ag := NewWithSession(modelProvider, registry, noopApprover{}, DefaultSystemPrompt, sessionStore, nil, 4000, 10, time.Second)
	writer := &captureWriter{}

	if err := ag.HandleMessage(context.Background(), writer, &runtime.Message{Text: "run the tool"}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	loaded, err := sessionStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load session: %v", err)
	}

	var toolMsg *provider.ChatMessage
	for i := range loaded {
		if loaded[i].Role == provider.RoleTool {
			toolMsg = &loaded[i]
			break
		}
	}
	if toolMsg == nil {
		t.Fatalf("expected persisted tool message, got %#v", loaded)
	}
	if len(toolMsg.Content) != 2000 {
		t.Fatalf("expected truncated tool output length 2000, got %d", len(toolMsg.Content))
	}
	if toolMsg.Content != strings.Repeat("x", 2000) {
		t.Fatalf("expected stored tool output to be truncated 2000-byte prefix")
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
	requests           []provider.ChatRequest
	responses          []*provider.ChatResponse
	err                error
	errs               []error
	requireLiveContext bool
}

func (p *recordingProvider) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	p.requests = append(p.requests, req)
	if p.requireLiveContext && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if p.err != nil {
		return nil, p.err
	}
	if len(p.errs) > 0 {
		err := p.errs[0]
		p.errs = p.errs[1:]
		if err != nil {
			return nil, err
		}
	}
	if len(p.responses) == 0 {
		return &provider.ChatResponse{Content: ""}, nil
	}
	resp := p.responses[0]
	p.responses = p.responses[1:]
	return resp, nil
}

type truncatingTool struct{}

func (truncatingTool) Name() string        { return "truncating_tool" }
func (truncatingTool) Description() string { return "returns long output for truncation tests" }
func (truncatingTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
	}
}

func waitFor(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}
func (truncatingTool) Permission() tools.Permission { return tools.AutoApprove }
func (truncatingTool) Execute(_ context.Context, _ map[string]any) (*tools.ToolResult, error) {
	return tools.TruncateOutput(strings.Repeat("x", 2500))
}
