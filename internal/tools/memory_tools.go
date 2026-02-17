package tools

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/machinae/betterclaw/internal/memory"
)

// MemoryReadTool reads the long-term memory file.
type MemoryReadTool struct {
	Store *memory.Store
}

// Name returns the tool name.
func (t MemoryReadTool) Name() string {
	return "memory_read"
}

// Description returns the tool description for the model.
func (t MemoryReadTool) Description() string {
	return "Read the full long-term memory file (memory.md)"
}

// Schema returns the JSON schema for memory_read args.
func (t MemoryReadTool) Schema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

// Permission declares default permission behavior for this tool.
func (t MemoryReadTool) Permission() Permission {
	return AutoApprove
}

// Execute reads memory.md and returns its contents.
func (t MemoryReadTool) Execute(_ context.Context, _ map[string]any) (*ToolResult, error) {
	if t.Store == nil {
		return nil, errors.New("memory store is required")
	}
	text, err := t.Store.ReadMemory()
	if err != nil {
		return nil, err
	}
	return TruncateOutput(text)
}

// MemoryAppendTool appends a fact to a memory section.
type MemoryAppendTool struct {
	Store *memory.Store
}

// Name returns the tool name.
func (t MemoryAppendTool) Name() string {
	return "memory_append"
}

// Description returns the tool description for the model.
func (t MemoryAppendTool) Description() string {
	return "Add a fact to long-term memory"
}

// Schema returns the JSON schema for memory_append args.
func (t MemoryAppendTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"section": map[string]any{
				"type":        "string",
				"description": "Examples: User, Preferences, People, Ongoing",
			},
			"fact": map[string]any{
				"type":        "string",
				"description": "The fact to remember",
			},
		},
		"required": []string{"section", "fact"},
	}
}

// Permission declares default permission behavior for this tool.
func (t MemoryAppendTool) Permission() Permission {
	return AutoApprove
}

// Execute appends a fact to the requested memory section.
func (t MemoryAppendTool) Execute(_ context.Context, args map[string]any) (*ToolResult, error) {
	if t.Store == nil {
		return nil, errors.New("memory store is required")
	}
	section, err := stringArg(args, "section")
	if err != nil {
		return nil, err
	}
	fact, err := stringArg(args, "fact")
	if err != nil {
		return nil, err
	}
	if err := t.Store.AppendFact(section, fact); err != nil {
		return nil, err
	}
	return &ToolResult{Output: "ok"}, nil
}

// MemoryRemoveTool removes matching facts from memory.
type MemoryRemoveTool struct {
	Store *memory.Store
}

// Name returns the tool name.
func (t MemoryRemoveTool) Name() string {
	return "memory_remove"
}

// Description returns the tool description for the model.
func (t MemoryRemoveTool) Description() string {
	return "Remove a fact from long-term memory"
}

// Schema returns the JSON schema for memory_remove args.
func (t MemoryRemoveTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"fact": map[string]any{
				"type":        "string",
				"description": "Fact text to match and remove",
			},
		},
		"required": []string{"fact"},
	}
}

// Permission declares default permission behavior for this tool.
func (t MemoryRemoveTool) Permission() Permission {
	return AutoApprove
}

// Execute removes all exact matching fact bullet lines from memory.md.
func (t MemoryRemoveTool) Execute(_ context.Context, args map[string]any) (*ToolResult, error) {
	if t.Store == nil {
		return nil, errors.New("memory store is required")
	}
	fact, err := stringArg(args, "fact")
	if err != nil {
		return nil, err
	}
	removed, err := t.Store.RemoveFact(fact)
	if err != nil {
		return nil, err
	}
	if removed == 0 {
		return &ToolResult{Output: "not found"}, nil
	}
	return &ToolResult{Output: fmt.Sprintf("removed %d", removed)}, nil
}

// DailyLogTool appends entries to today's daily memory log.
type DailyLogTool struct {
	Store *memory.Store
	Now   func() time.Time
}

// Name returns the tool name.
func (t DailyLogTool) Name() string {
	return "daily_log"
}

// Description returns the tool description for the model.
func (t DailyLogTool) Description() string {
	return "Append an entry to today's daily log"
}

// Schema returns the JSON schema for daily_log args.
func (t DailyLogTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"entry": map[string]any{
				"type":        "string",
				"description": "Daily log entry text",
			},
		},
		"required": []string{"entry"},
	}
}

// Permission declares default permission behavior for this tool.
func (t DailyLogTool) Permission() Permission {
	return AutoApprove
}

// Execute appends a timestamped line into today's log file.
func (t DailyLogTool) Execute(_ context.Context, args map[string]any) (*ToolResult, error) {
	if t.Store == nil {
		return nil, errors.New("memory store is required")
	}
	entry, err := stringArg(args, "entry")
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if t.Now != nil {
		now = t.Now()
	}
	if err := t.Store.AppendDailyLog(now, entry); err != nil {
		return nil, err
	}
	return &ToolResult{Output: "ok"}, nil
}

// SearchLogsTool searches daily log files for matching lines.
type SearchLogsTool struct {
	Store *memory.Store
	Now   func() time.Time
}

// Name returns the tool name.
func (t SearchLogsTool) Name() string {
	return "search_logs"
}

// Description returns the tool description for the model.
func (t SearchLogsTool) Description() string {
	return "Search past daily logs"
}

// Schema returns the JSON schema for search_logs args.
func (t SearchLogsTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Query substring to search for",
			},
			"days_back": map[string]any{
				"type":        "integer",
				"description": "Number of days to search (default: 7)",
			},
		},
		"required": []string{"query"},
	}
}

// Permission declares default permission behavior for this tool.
func (t SearchLogsTool) Permission() Permission {
	return AutoApprove
}

// Execute searches logs over the requested day range and returns matching lines.
func (t SearchLogsTool) Execute(_ context.Context, args map[string]any) (*ToolResult, error) {
	if t.Store == nil {
		return nil, errors.New("memory store is required")
	}
	query, err := stringArg(args, "query")
	if err != nil {
		return nil, err
	}
	daysBack, err := memory.OptionalIntArg(args, "days_back", 7)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if t.Now != nil {
		now = t.Now()
	}

	text, err := t.Store.SearchLogs(now, query, daysBack)
	if err != nil {
		return nil, err
	}
	return TruncateOutput(text)
}
