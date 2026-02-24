package agent

import (
	"testing"

	"github.com/neoclaw-ai/neoclaw/internal/provider"
)

func TestCompactionRecentStart_NoAdjustmentWhenNotTool(t *testing.T) {
	messages := []provider.ChatMessage{
		{Role: provider.RoleUser, Content: "u1"},
		{Role: provider.RoleAssistant, Content: "a1"},
		{Role: provider.RoleUser, Content: "u2"},
	}

	start := compactionRecentStart(messages, 1)
	if start != 1 {
		t.Fatalf("expected start=1, got %d", start)
	}
}

func TestCompactionRecentStart_ShiftsToAssistantForToolTurn(t *testing.T) {
	messages := []provider.ChatMessage{
		{Role: provider.RoleUser, Content: "u1"},
		{
			Role: provider.RoleAssistant,
			ToolCalls: []provider.ToolCall{
				{ID: "toolu_1", Name: "memory_append", Arguments: `{"x":"y"}`},
			},
		},
		{Role: provider.RoleTool, ToolCallID: "toolu_1", Content: "ok1"},
		{Role: provider.RoleTool, ToolCallID: "toolu_1", Content: "ok2"},
		{Role: provider.RoleUser, Content: "u2"},
	}

	start := compactionRecentStart(messages, 3)
	if start != 1 {
		t.Fatalf("expected start to shift to assistant index 1, got %d", start)
	}
}

func TestCompactionRecentStart_SkipsOrphanToolBlock(t *testing.T) {
	messages := []provider.ChatMessage{
		{Role: provider.RoleUser, Content: "u1"},
		{Role: provider.RoleAssistant, Content: "a1"},
		{Role: provider.RoleTool, ToolCallID: "orphan", Content: "bad1"},
		{Role: provider.RoleTool, ToolCallID: "orphan", Content: "bad2"},
		{Role: provider.RoleUser, Content: "u2"},
	}

	start := compactionRecentStart(messages, 2)
	if start != 4 {
		t.Fatalf("expected start to skip orphan tool block to index 4, got %d", start)
	}
}

