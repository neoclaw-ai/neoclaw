// Package tools defines the Tool interface, Registry, and ToolResult used by the agent loop, with optional interfaces for conditional approval and result summarization.
package tools

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/neoclaw-ai/neoclaw/internal/config"
	"github.com/neoclaw-ai/neoclaw/internal/provider"
)

// Permission classifies whether a tool can run automatically or needs user approval.
type Permission int

const (
	// AutoApprove indicates the tool can run without prompting the user.
	AutoApprove Permission = iota
	// RequiresApproval indicates the tool must be explicitly approved.
	RequiresApproval
)

// Tool is the core executable action exposed to the LLM.
type Tool interface {
	Name() string
	Description() string
	Schema() map[string]any
	Permission() Permission
	Execute(ctx context.Context, args map[string]any) (*ToolResult, error)
}

// Summarizer is an optional interface tools can implement to provide
// human-readable descriptions for approval prompts.
type Summarizer interface {
	SummarizeArgs(args map[string]any) string
}

// ConditionalApprover is an optional interface for tools that can decide
// approval requirements from per-call arguments.
type ConditionalApprover interface {
	RequiresApprovalForArgs(args map[string]any) (bool, error)
}

// ApprovalPersister is an optional interface for tools that can persist an
// approved decision for future invocations.
type ApprovalPersister interface {
	PersistApproval(args map[string]any) error
}

// ToolResult is the normalized output returned by tools.
type ToolResult struct {
	Output string
}

// TruncateOutput truncates large output to the configured inline limit.
func TruncateOutput(output string) (*ToolResult, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	limit := cfg.Context.ToolOutputLength
	if len(output) <= limit {
		return &ToolResult{Output: output}, nil
	}
	return &ToolResult{Output: output[:limit]}, nil
}

// Registry stores tools by unique name.
type Registry struct {
	byName map[string]Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{byName: make(map[string]Tool)}
}

// Register adds a tool by unique name.
func (r *Registry) Register(tool Tool) error {
	if tool == nil {
		return errors.New("tool cannot be nil")
	}
	name := tool.Name()
	if name == "" {
		return errors.New("tool name cannot be empty")
	}
	if _, exists := r.byName[name]; exists {
		return fmt.Errorf("tool %s already registered", name)
	}
	r.byName[name] = tool
	return nil
}

// Lookup returns a tool by name.
func (r *Registry) Lookup(name string) (Tool, bool) {
	tool, ok := r.byName[name]
	return tool, ok
}

// Tools returns all registered tools in stable name order.
func (r *Registry) Tools() []Tool {
	keys := make([]string, 0, len(r.byName))
	for name := range r.byName {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	out := make([]Tool, 0, len(keys))
	for _, name := range keys {
		out = append(out, r.byName[name])
	}
	return out
}

// ToolDefinitions converts registered tools into LLM request tool definitions.
func (r *Registry) ToolDefinitions() []provider.ToolDefinition {
	tools := r.Tools()
	defs := make([]provider.ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		defs = append(defs, provider.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  tool.Schema(),
		})
	}
	return defs
}
