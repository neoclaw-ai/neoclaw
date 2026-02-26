package agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/neoclaw-ai/neoclaw/internal/logging"
	"github.com/neoclaw-ai/neoclaw/internal/provider"
)

const summaryKind = "summary"

func (a *Agent) compactHistoryIfNeeded(ctx context.Context, systemPrompt string, messages []provider.ChatMessage) ([]provider.ChatMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if a.maxContextTokens <= 0 {
		return append([]provider.ChatMessage{}, messages...), nil
	}
	estimatedTokens := estimateTokens(systemPrompt, messages)
	if estimatedTokens <= a.maxContextTokens {
		return append([]provider.ChatMessage{}, messages...), nil
	}
	if len(messages) == 0 {
		return []provider.ChatMessage{}, nil
	}

	recentCount := a.recentMessages
	if recentCount <= 0 || recentCount > len(messages) {
		recentCount = len(messages)
	}
	initialStart := len(messages) - recentCount
	olderCount := compactionRecentStart(messages, initialStart)
	logging.Logger().Info(
		"history compaction triggered",
		"estimated_tokens", estimatedTokens,
		"max_context_tokens", a.maxContextTokens,
		"message_count", len(messages),
		"requested_recent_count", recentCount,
		"recent_start", olderCount,
		"recent_start_adjusted", olderCount != initialStart,
	)
	recent := append([]provider.ChatMessage{}, messages[olderCount:]...)
	if olderCount <= 0 {
		return recent, nil
	}

	summary, err := a.summarizeMessages(ctx, messages[:olderCount])
	if err != nil {
		logging.Logger().Warn("history compaction summary failed; falling back to recent messages only", "err", err)
		return recent, nil
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		logging.Logger().Warn("history compaction summary empty; falling back to recent messages only")
		return recent, nil
	}

	compacted := make([]provider.ChatMessage, 0, len(recent)+1)
	compacted = append(compacted, provider.ChatMessage{
		Kind:    summaryKind,
		Role:    provider.RoleAssistant,
		Content: summary,
	})
	compacted = append(compacted, recent...)
	return compacted, nil
}

func (a *Agent) summarizeMessages(ctx context.Context, messages []provider.ChatMessage) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}
	transcript := buildSummaryTranscript(messages)
	resp, err := a.provider.Chat(ctx, provider.ChatRequest{
		SystemPrompt: summaryPrompt,
		Messages: []provider.ChatMessage{
			{
				Role:    provider.RoleUser,
				Content: transcript,
			},
		},
		Tools: nil,
	})
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("summary response is nil")
	}
	if err := a.recordUsage(ctx, resp.Usage); err != nil {
		logging.Logger().Warn("failed to record summary usage", "err", err)
	}
	return resp.Content, nil
}

func buildSummaryTranscript(messages []provider.ChatMessage) string {
	var b strings.Builder
	b.WriteString("Summarize this transcript:\n<transcript>\n")
	for i, msg := range messages {
		b.WriteString("[")
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString("] role=")
		b.WriteString(string(msg.Role))
		b.WriteString("\n")
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				b.WriteString("tool_call: ")
				b.WriteString(tc.Name)
				if tc.Arguments != "" {
					b.WriteString(" args=")
					b.WriteString(tc.Arguments)
				}
				b.WriteString("\n")
			}
		}
		if msg.Content != "" {
			b.WriteString("content:\n")
			b.WriteString(msg.Content)
			if !strings.HasSuffix(msg.Content, "\n") {
				b.WriteString("\n")
			}
		}
		b.WriteString("---\n")
	}
	b.WriteString("</transcript>")
	return b.String()
}

func estimateTokens(systemPrompt string, messages []provider.ChatMessage) int {
	charCount := len(systemPrompt)
	for _, msg := range messages {
		charCount += len(msg.Kind)
		charCount += len(msg.Content)
		charCount += len(msg.ToolCallID)
		for _, tc := range msg.ToolCalls {
			charCount += len(tc.ID)
			charCount += len(tc.Name)
			charCount += len(tc.Arguments)
		}
	}
	return charCount / 4
}

func sameMessageSlice(a, b []provider.ChatMessage) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Kind != b[i].Kind || a[i].Role != b[i].Role || a[i].Content != b[i].Content || a[i].ToolCallID != b[i].ToolCallID {
			return false
		}
		if len(a[i].ToolCalls) != len(b[i].ToolCalls) {
			return false
		}
		for j := range a[i].ToolCalls {
			if a[i].ToolCalls[j] != b[i].ToolCalls[j] {
				return false
			}
		}
	}
	return true
}

// compactionRecentStart adjusts the recent-window start so compaction does not
// begin on RoleTool. If the window would start mid tool-turn, it expands
// backward to include the matching assistant tool-call message when available;
// otherwise it skips forward past orphan tool-result blocks.
func compactionRecentStart(messages []provider.ChatMessage, start int) int {
	if start <= 0 || start >= len(messages) || messages[start].Role != provider.RoleTool {
		return start
	}

	toolBlockStart := start
	for toolBlockStart > 0 && messages[toolBlockStart-1].Role == provider.RoleTool {
		toolBlockStart--
	}

	assistantIdx := toolBlockStart - 1
	if assistantIdx >= 0 && messages[assistantIdx].Role == provider.RoleAssistant && len(messages[assistantIdx].ToolCalls) > 0 {
		return assistantIdx
	}

	i := start
	for i < len(messages) && messages[i].Role == provider.RoleTool {
		i++
	}
	return i
}
