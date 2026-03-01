package tools

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/neoclaw-ai/neoclaw/internal/memory"
)

// DailyLogAppendTool appends structured entries to the daily log.
type DailyLogAppendTool struct {
	Store *memory.Store
}

// Name returns the tool name.
func (t DailyLogAppendTool) Name() string {
	return "daily_log_append"
}

// Description returns the tool description for the model.
func (t DailyLogAppendTool) Description() string {
	return "Append a structured entry to the daily log"
}

// Schema returns the JSON schema for daily_log_append args.
func (t DailyLogAppendTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tags": map[string]any{
				"type":        "string",
				"description": "Comma-separated tags. First tag is the primary type.",
			},
			"text": map[string]any{
				"type":        "string",
				"description": "Daily log entry text",
			},
			"kv": map[string]any{
				"type":        "string",
				"description": "Optional key=value metadata string",
			},
		},
		"required": []string{"tags", "text"},
	}
}

// Permission declares default permission behavior for this tool.
func (t DailyLogAppendTool) Permission() Permission {
	return AutoApprove
}

// Execute appends a structured entry into the daily log.
func (t DailyLogAppendTool) Execute(_ context.Context, args map[string]any) (*ToolResult, error) {
	if t.Store == nil {
		return nil, errors.New("memory store is required")
	}
	tags, err := parseTagsArg(args, "tags")
	if err != nil {
		return nil, err
	}
	if tags[0] == "summary" {
		return nil, errors.New("daily_log_append cannot write summary entries")
	}
	text, err := stringArg(args, "text")
	if err != nil {
		return nil, err
	}
	kv, err := optionalStringArg(args, "kv", "-")
	if err != nil {
		return nil, err
	}
	if err := t.Store.AppendDailyLog(memory.LogEntry{
		Tags: tags,
		Text: text,
		KV:   kv,
	}); err != nil {
		return nil, err
	}
	return &ToolResult{Output: "ok"}, nil
}

// MemoryAppendTool appends structured facts to long-term memory.
type MemoryAppendTool struct {
	Store *memory.Store
}

// Name returns the tool name.
func (t MemoryAppendTool) Name() string {
	return "memory_append"
}

// Description returns the tool description for the model.
func (t MemoryAppendTool) Description() string {
	return "Add a structured fact to long-term memory"
}

// Schema returns the JSON schema for memory_append args.
func (t MemoryAppendTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tags": map[string]any{
				"type":        "string",
				"description": "Comma-separated tags. First tag is the primary topic.",
			},
			"text": map[string]any{
				"type":        "string",
				"description": "Fact text to remember",
			},
			"kv": map[string]any{
				"type":        "string",
				"description": "Optional key=value metadata string",
			},
			"expires": map[string]any{
				"type":        "string",
				"description": "Optional expiry like 2h, 3d, 1w, 2026-02-28, or 2026-02-28T15:00",
			},
		},
		"required": []string{"tags", "text"},
	}
}

// Permission declares default permission behavior for this tool.
func (t MemoryAppendTool) Permission() Permission {
	return AutoApprove
}

// Execute appends a structured fact to memory.tsv.
func (t MemoryAppendTool) Execute(_ context.Context, args map[string]any) (*ToolResult, error) {
	if t.Store == nil {
		return nil, errors.New("memory store is required")
	}
	tags, err := parseTagsArg(args, "tags")
	if err != nil {
		return nil, err
	}
	text, err := stringArg(args, "text")
	if err != nil {
		return nil, err
	}
	kv, err := optionalStringArg(args, "kv", "-")
	if err != nil {
		return nil, err
	}
	expires, err := optionalStringArg(args, "expires", "")
	if err != nil {
		return nil, err
	}
	if expires != "" {
		expiresAt, err := parseExpiryTime(expires, time.Now())
		if err != nil {
			return nil, err
		}
		kv = appendKVToken(kv, "expires="+strconv.FormatInt(expiresAt.Unix(), 10))
	}
	entry := memory.LogEntry{
		Tags: tags,
		Text: text,
		KV:   kv,
	}
	if err := t.Store.AppendMemory(entry); err != nil {
		return nil, err
	}
	return &ToolResult{Output: fmt.Sprintf("%s\t%s", strings.Join(entry.Tags, ","), entry.Text)}, nil
}

// MemoryTagsTool lists first-tag counts across memory facts.
type MemoryTagsTool struct {
	Store *memory.Store
}

// Name returns the tool name.
func (t MemoryTagsTool) Name() string {
	return "memory_tags"
}

// Description returns the tool description for the model.
func (t MemoryTagsTool) Description() string {
	return "List memory fact tags and how many entries each has"
}

// Schema returns the JSON schema for memory_tags args.
func (t MemoryTagsTool) Schema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

// Permission declares default permission behavior for this tool.
func (t MemoryTagsTool) Permission() Permission {
	return AutoApprove
}

// Execute returns tag counts sorted by count descending, then tag ascending.
func (t MemoryTagsTool) Execute(_ context.Context, _ map[string]any) (*ToolResult, error) {
	if t.Store == nil {
		return nil, errors.New("memory store is required")
	}
	counts := t.Store.FactTags()
	if len(counts) == 0 {
		return &ToolResult{Output: ""}, nil
	}
	tags := make([]string, 0, len(counts))
	for tag := range counts {
		tags = append(tags, tag)
	}
	sort.Slice(tags, func(i, j int) bool {
		left := counts[tags[i]]
		right := counts[tags[j]]
		if left != right {
			return left > right
		}
		return tags[i] < tags[j]
	})

	var out strings.Builder
	out.WriteString("tag\tcount")
	for _, tag := range tags {
		out.WriteByte('\n')
		out.WriteString(tag)
		out.WriteByte('\t')
		out.WriteString(strconv.Itoa(counts[tag]))
	}
	return &ToolResult{Output: out.String()}, nil
}

// SearchLogsTool searches memory entries and daily logs using regex matching.
type SearchLogsTool struct {
	Store *memory.Store
}

// Name returns the tool name.
func (t SearchLogsTool) Name() string {
	return "search_logs"
}

// Description returns the tool description for the model.
func (t SearchLogsTool) Description() string {
	return "Search daily logs and memory facts using regex matching"
}

// Schema returns the JSON schema for search_logs args.
func (t SearchLogsTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Regex pattern to search for",
			},
			"from_time": map[string]any{
				"type":        "string",
				"description": "Optional RFC3339 timestamp lower bound (inclusive)",
			},
			"to_time": map[string]any{
				"type":        "string",
				"description": "Optional RFC3339 timestamp upper bound (inclusive, default: now)",
			},
		},
		"required": []string{"query"},
	}
}

// Permission declares default permission behavior for this tool.
func (t SearchLogsTool) Permission() Permission {
	return AutoApprove
}

// Execute searches logs and returns TSV output with a header row.
func (t SearchLogsTool) Execute(_ context.Context, args map[string]any) (*ToolResult, error) {
	if t.Store == nil {
		return nil, errors.New("memory store is required")
	}
	query, err := stringArg(args, "query")
	if err != nil {
		return nil, err
	}
	fromTime, err := optionalRFC3339Arg(args, "from_time", time.Time{})
	if err != nil {
		return nil, err
	}
	toTime, err := optionalRFC3339Arg(args, "to_time", time.Now())
	if err != nil {
		return nil, err
	}
	entries, err := t.Store.Search(query, fromTime, toTime)
	if err != nil {
		return nil, err
	}

	lines := make([]string, 0, len(entries)+1)
	lines = append(lines, "ts\ttags\ttext\tkv")
	for _, entry := range entries {
		lines = append(lines, strings.Join(entry.MarshalTSV(), "\t"))
	}
	return &ToolResult{Output: strings.Join(lines, "\n")}, nil
}

// optionalRFC3339Arg parses an optional RFC3339 timestamp argument or returns the default.
func optionalRFC3339Arg(args map[string]any, key string, def time.Time) (time.Time, error) {
	raw, ok := args[key]
	if !ok {
		return def, nil
	}
	s, ok := raw.(string)
	if !ok {
		return time.Time{}, fmt.Errorf("argument %s must be an RFC3339 timestamp string", key)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return def, nil
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("argument %s must be RFC3339 format", key)
	}
	return parsed, nil
}

// optionalStringArg returns an optional string argument, treating blank values as the default.
func optionalStringArg(args map[string]any, key, def string) (string, error) {
	raw, ok := args[key]
	if !ok {
		return def, nil
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("argument %s must be a string", key)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return def, nil
	}
	return s, nil
}

// parseTagsArg parses, trims, and normalizes a required comma-separated tags argument.
func parseTagsArg(args map[string]any, key string) ([]string, error) {
	raw, err := stringArg(args, key)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tags = append(tags, strings.TrimSpace(part))
	}
	tags = memory.NormalizeTags(tags)
	if len(tags) == 0 {
		return nil, fmt.Errorf("argument %s must include at least one tag", key)
	}
	return tags, nil
}

// parseExpiryTime converts a human-readable expiry string into an absolute time.
func parseExpiryTime(input string, now time.Time) (time.Time, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return time.Time{}, errors.New("expires is required")
	}
	if now.IsZero() {
		now = time.Now()
	}
	if duration, err := time.ParseDuration(input); err == nil {
		return now.Add(duration), nil
	}
	if len(input) > 1 {
		unit := input[len(input)-1]
		valueText := strings.TrimSpace(input[:len(input)-1])
		if unit == 'd' || unit == 'w' {
			value, err := strconv.Atoi(valueText)
			if err != nil {
				return time.Time{}, fmt.Errorf("invalid expires value %q", input)
			}
			multiplier := 24 * time.Hour
			if unit == 'w' {
				multiplier = 7 * 24 * time.Hour
			}
			return now.Add(time.Duration(value) * multiplier), nil
		}
	}
	if parsed, err := time.ParseInLocation("2006-01-02T15:04", input, time.Local); err == nil {
		return parsed, nil
	}
	if parsed, err := time.ParseInLocation("2006-01-02", input, time.Local); err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("unsupported expires format %q", input)
}

// appendKVToken appends one key=value token to the KV string, handling empty placeholders.
func appendKVToken(kv, token string) string {
	kv = strings.TrimSpace(kv)
	token = strings.TrimSpace(token)
	if token == "" {
		return kv
	}
	if kv == "" || kv == "-" {
		return token
	}
	return kv + " " + token
}
