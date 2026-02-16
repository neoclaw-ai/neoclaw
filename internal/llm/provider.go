package llm

import "context"

// Provider sends chat requests to an LLM backend.
type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type ChatMessage struct {
	Role       Role
	Content    string
	ToolCallID string
	ToolCalls  []ToolCall
}

type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

type ChatRequest struct {
	SystemPrompt string
	Messages     []ChatMessage
	Tools        []ToolDefinition
	MaxTokens    int
}

type ChatResponse struct {
	Content   string
	ToolCalls []ToolCall
	Usage     TokenUsage
}
