package llm

import "context"

// Provider sends chat requests to an LLM backend.
type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// Role is the author role for a chat message.
type Role string

const (
	// RoleUser is a user-authored message.
	RoleUser      Role = "user"
	// RoleAssistant is an assistant-authored message.
	RoleAssistant Role = "assistant"
	// RoleTool is a tool-result message addressed to the model.
	RoleTool      Role = "tool"
)

// ChatMessage is a single message in model conversation history.
type ChatMessage struct {
	Role       Role
	Content    string
	ToolCallID string
	ToolCalls  []ToolCall
}

// ToolDefinition describes a callable tool exposed to the model.
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// ToolCall is a model request to execute a tool.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// TokenUsage reports provider token accounting for one response.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// ChatRequest is the provider-agnostic request payload.
type ChatRequest struct {
	SystemPrompt string
	Messages     []ChatMessage
	Tools        []ToolDefinition
	MaxTokens    int
}

// ChatResponse is the provider-agnostic response payload.
type ChatResponse struct {
	Content   string
	ToolCalls []ToolCall
	Usage     TokenUsage
}
