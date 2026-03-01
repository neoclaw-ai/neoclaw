package agent

import (
	"testing"

	"github.com/neoclaw-ai/neoclaw/internal/provider"
)

func TestSanitizeToolTurns_DropsOrphanToolResult(t *testing.T) {
	in := []provider.ChatMessage{
		{Role: provider.RoleUser, Content: "hi"},
		{Role: provider.RoleTool, ToolCallID: "toolu_orphan", Content: "orphan"},
		{Role: provider.RoleAssistant, Content: "hello"},
	}

	out, changed := sanitizeToolTurns(in)
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(out))
	}
	if out[1].Role != provider.RoleAssistant {
		t.Fatalf("expected assistant at index 1, got %q", out[1].Role)
	}
}

func TestSanitizeToolTurns_StripsAssistantToolCallsWithoutResults(t *testing.T) {
	in := []provider.ChatMessage{
		{
			Role:    provider.RoleAssistant,
			Content: "calling tool",
			ToolCalls: []provider.ToolCall{
				{ID: "toolu_1", Name: "memory_append", Arguments: `{}`},
			},
		},
		{Role: provider.RoleUser, Content: "next"},
	}

	out, changed := sanitizeToolTurns(in)
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(out))
	}
	if len(out[0].ToolCalls) != 0 {
		t.Fatalf("expected assistant tool calls to be stripped")
	}
}

func TestSanitizeToolTurns_RequiresMatchingIDs(t *testing.T) {
	in := []provider.ChatMessage{
		{
			Role: provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{
				{ID: "toolu_1", Name: "a"},
				{ID: "toolu_2", Name: "b"},
			},
		},
		{Role: provider.RoleTool, ToolCallID: "toolu_1", Content: "ok"},
		{Role: provider.RoleTool, ToolCallID: "toolu_missing", Content: "bad"},
		{Role: provider.RoleUser, Content: "next"},
	}

	out, changed := sanitizeToolTurns(in)
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(out))
	}
	if out[0].Role != provider.RoleAssistant || len(out[0].ToolCalls) != 1 || out[0].ToolCalls[0].ID != "toolu_1" {
		t.Fatalf("assistant tool calls not filtered as expected: %+v", out[0].ToolCalls)
	}
	if out[1].Role != provider.RoleTool || out[1].ToolCallID != "toolu_1" {
		t.Fatalf("unexpected tool result kept: %+v", out[1])
	}
}

func TestSanitizeToolTurns_PreservesValidTurnUnchanged(t *testing.T) {
	in := []provider.ChatMessage{
		{Role: provider.RoleUser, Content: "hi"},
		{
			Role: provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{
				{ID: "toolu_1", Name: "memory_append", Arguments: `{}`},
			},
		},
		{Role: provider.RoleTool, ToolCallID: "toolu_1", Content: "ok"},
		{Role: provider.RoleAssistant, Content: "done"},
	}

	out, changed := sanitizeToolTurns(in)
	if changed {
		t.Fatalf("expected changed=false for valid history")
	}
	if len(out) != len(in) {
		t.Fatalf("expected same length, got %d vs %d", len(out), len(in))
	}
}

func TestSanitizeToolTurns_DropsToolResultWithoutID(t *testing.T) {
	in := []provider.ChatMessage{
		{
			Role: provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{
				{ID: "toolu_1", Name: "memory_append", Arguments: `{}`},
			},
		},
		{Role: provider.RoleTool, ToolCallID: "", Content: "bad"},
		{Role: provider.RoleTool, ToolCallID: "toolu_1", Content: "ok"},
	}

	out, changed := sanitizeToolTurns(in)
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if len(out) != 2 {
		t.Fatalf("expected assistant + one valid tool result, got %d", len(out))
	}
	if out[1].ToolCallID != "toolu_1" {
		t.Fatalf("expected only valid tool result to remain, got %#v", out[1])
	}
}

func TestSanitizeToolTurns_AssistantWithPartialResults(t *testing.T) {
	in := []provider.ChatMessage{
		{
			Role: provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{
				{ID: "toolu_1", Name: "a"},
				{ID: "toolu_2", Name: "b"},
			},
		},
		{Role: provider.RoleTool, ToolCallID: "toolu_2", Content: "ok-b"},
	}

	out, changed := sanitizeToolTurns(in)
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if len(out) != 2 {
		t.Fatalf("expected assistant + one tool result, got %d", len(out))
	}
	if len(out[0].ToolCalls) != 1 || out[0].ToolCalls[0].ID != "toolu_2" {
		t.Fatalf("expected assistant tool calls filtered to toolu_2, got %+v", out[0].ToolCalls)
	}
}
