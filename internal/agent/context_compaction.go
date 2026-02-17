package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/machinae/betterclaw/internal/logging"
	providerapi "github.com/machinae/betterclaw/internal/provider"
)

const summaryKind = "summary"
const summaryPrompt = "Summarize the earlier conversation for future context. Include user preferences, constraints, decisions, and unresolved tasks. Be concise and factual."

func (a *Agent) compactHistoryIfNeeded(ctx context.Context, messages []providerapi.ChatMessage) ([]providerapi.ChatMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if a.maxContextTokens <= 0 {
		return append([]providerapi.ChatMessage{}, messages...), nil
	}
	if estimateTokens(a.systemPrompt, messages) <= a.maxContextTokens {
		return append([]providerapi.ChatMessage{}, messages...), nil
	}
	if len(messages) == 0 {
		return []providerapi.ChatMessage{}, nil
	}

	recentCount := a.recentMessages
	if recentCount <= 0 || recentCount > len(messages) {
		recentCount = len(messages)
	}
	olderCount := len(messages) - recentCount
	recent := append([]providerapi.ChatMessage{}, messages[olderCount:]...)
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

	compacted := make([]providerapi.ChatMessage, 0, len(recent)+1)
	compacted = append(compacted, providerapi.ChatMessage{
		Kind:    summaryKind,
		Role:    providerapi.RoleAssistant,
		Content: summary,
	})
	compacted = append(compacted, recent...)
	return compacted, nil
}

func (a *Agent) summarizeMessages(ctx context.Context, messages []providerapi.ChatMessage) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}
	resp, err := a.provider.Chat(ctx, providerapi.ChatRequest{
		SystemPrompt: summaryPrompt,
		Messages:     messages,
		Tools:        nil,
	})
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("summary response is nil")
	}
	return resp.Content, nil
}

func estimateTokens(systemPrompt string, messages []providerapi.ChatMessage) int {
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

func sameMessageSlice(a, b []providerapi.ChatMessage) bool {
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
