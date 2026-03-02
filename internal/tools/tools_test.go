package tools

import (
	"context"
	"testing"
)

func TestRegistryRegisterAndLookup(t *testing.T) {
	r := NewRegistry()
	tool := staticTool{name: "read_file", description: "read a file", schema: map[string]any{"type": "object"}}

	if err := r.Register(tool); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	got, ok := r.Lookup("read_file")
	if !ok {
		t.Fatalf("expected tool lookup to succeed")
	}
	if got.Name() != "read_file" {
		t.Fatalf("expected tool name read_file, got %q", got.Name())
	}
}

func TestRegistryRegisterRejectsDuplicate(t *testing.T) {
	r := NewRegistry()
	tool := staticTool{name: "read_file"}
	if err := r.Register(tool); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := r.Register(tool); err == nil {
		t.Fatalf("expected duplicate registration error")
	}
}

func TestToolDefinitionsSerializesSchema(t *testing.T) {
	r := NewRegistry()
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string"},
		},
	}
	if err := r.Register(staticTool{name: "read_file", description: "Read file", schema: schema}); err != nil {
		t.Fatalf("register: %v", err)
	}

	defs := r.ToolDefinitions()
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}
	if defs[0].Name != "read_file" {
		t.Fatalf("expected name read_file, got %q", defs[0].Name)
	}
	if defs[0].Description != "Read file" {
		t.Fatalf("expected description to round trip")
	}
	if got := defs[0].Parameters["type"]; got != "object" {
		t.Fatalf("expected schema type object, got %#v", got)
	}
}

func TestTruncateOutput_NoTruncationForSmallOutput(t *testing.T) {
	res, err := TruncateOutput("hello")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if res.Output != "hello" {
		t.Fatalf("expected output hello, got %q", res.Output)
	}
}

func TestTruncateOutput_TruncatesLargeOutput(t *testing.T) {
	long := make([]byte, 13000)
	for i := range long {
		long[i] = 'a'
	}

	res, err := TruncateOutput(string(long))
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if len(res.Output) != 12000 {
		t.Fatalf("expected inline output to be 12000 chars, got %d", len(res.Output))
	}
}

type staticTool struct {
	name        string
	description string
	schema      map[string]any
	permission  Permission
	result      *ToolResult
	err         error
}

func (t staticTool) Name() string        { return t.name }
func (t staticTool) Description() string { return t.description }
func (t staticTool) Schema() map[string]any {
	if t.schema == nil {
		return map[string]any{"type": "object"}
	}
	return t.schema
}
func (t staticTool) Permission() Permission { return t.permission }
func (t staticTool) Execute(_ context.Context, _ map[string]any) (*ToolResult, error) {
	if t.err != nil {
		return nil, t.err
	}
	if t.result != nil {
		return t.result, nil
	}
	return &ToolResult{Output: "ok"}, nil
}
