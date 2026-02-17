package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/machinae/betterclaw/internal/llm"
)

const maxInlineOutputChars = 2000

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

// ToolResult is the normalized output returned by tools.
type ToolResult struct {
	Output         string
	Truncated      bool
	FullOutputPath string
}

// TruncateOutput stores very large output in a temp file and returns a compact result.
func TruncateOutput(output string) (*ToolResult, error) {
	if len(output) <= maxInlineOutputChars {
		return &ToolResult{Output: output}, nil
	}

	tempFile, err := os.CreateTemp("", "betterclaw-tool-output-*.txt")
	if err != nil {
		return nil, fmt.Errorf("create temp output file: %w", err)
	}
	defer tempFile.Close()

	if _, err := tempFile.WriteString(output); err != nil {
		return nil, fmt.Errorf("write temp output file: %w", err)
	}

	return &ToolResult{
		Output:         output[:maxInlineOutputChars],
		Truncated:      true,
		FullOutputPath: tempFile.Name(),
	}, nil
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
		return fmt.Errorf("tool %q already registered", name)
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
func (r *Registry) ToolDefinitions() []llm.ToolDefinition {
	tools := r.Tools()
	defs := make([]llm.ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		defs = append(defs, llm.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  tool.Schema(),
		})
	}
	return defs
}
