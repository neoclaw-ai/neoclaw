package agent

import (
	"context"
	"encoding/csv"
	"io"
	"strings"
	"time"

	"github.com/neoclaw-ai/neoclaw/internal/logging"
	"github.com/neoclaw-ai/neoclaw/internal/memory"
	"github.com/neoclaw-ai/neoclaw/internal/provider"
)

func (a *Agent) ensureHistoryLoaded(ctx context.Context) error {
	if a.historyLoadedOnce || a.sessionStore == nil {
		return nil
	}
	history, err := a.sessionStore.Load(ctx)
	if err != nil {
		return err
	}
	history, _ = sanitizeToolTurns(history)
	a.history = history
	a.historyLoadedOnce = true
	return nil
}

// Reset clears conversation history and persisted session state.
func (a *Agent) Reset(ctx context.Context) error {
	if err := a.ensureHistoryLoaded(ctx); err != nil {
		return err
	}
	historySnapshot := append([]provider.ChatMessage{}, a.history...)
	a.summarizeSessionToDailyLogAsync(ctx, historySnapshot)
	return a.resetSession(ctx)
}

func (a *Agent) resetSession(ctx context.Context) error {
	a.history = nil
	a.historyLoadedOnce = true
	if a.sessionStore == nil {
		return nil
	}
	return a.sessionStore.Reset(ctx)
}

func (a *Agent) rewriteSessionIfNeeded(ctx context.Context, history []provider.ChatMessage) error {
	if a.sessionStore == nil {
		return nil
	}
	return a.sessionStore.Rewrite(ctx, history)
}

func (a *Agent) appendSessionDelta(ctx context.Context, base, history []provider.ChatMessage) error {
	if a.sessionStore == nil {
		return nil
	}
	if len(history) < len(base) {
		return a.sessionStore.Rewrite(ctx, history)
	}
	return a.sessionStore.Append(ctx, history[len(base):])
}

func (a *Agent) summarizeSessionToDailyLogAsync(ctx context.Context, history []provider.ChatMessage) {
	if a == nil || a.memoryStore == nil || len(history) == 0 {
		return
	}
	timeout := a.requestTimeout
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}

	snapshot := append([]provider.ChatMessage{}, history...)
	go a.runSessionSummary(context.WithoutCancel(ctx), timeout, snapshot)
}

func (a *Agent) runSessionSummary(ctx context.Context, timeout time.Duration, snapshot []provider.ChatMessage) {
	if len(snapshot) == 0 {
		return
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	transcript := buildSummaryTranscript(snapshot)
	resp, err := a.provider.Chat(reqCtx, provider.ChatRequest{
		SystemPrompt: sessionSummaryPrompt,
		Messages: []provider.ChatMessage{
			{
				Role:    provider.RoleUser,
				Content: transcript,
			},
		},
		Tools: nil,
	})
	if err != nil {
		logging.Logger().Warn("session reset summary failed", "err", err)
		return
	}
	if resp == nil {
		logging.Logger().Warn("session reset summary failed", "err", "summary response is nil")
		return
	}
	if err := a.recordUsage(reqCtx, resp.Usage); err != nil {
		logging.Logger().Warn("failed to record summary usage", "err", err)
	}
	summary := strings.TrimSpace(resp.Content)
	if summary == "" {
		logging.Logger().Warn("session reset summary is empty; skipping daily log append")
		return
	}

	baseTime := time.Now()
	reader := csv.NewReader(strings.NewReader(summary))
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1

	entryIndex := 0
	for {
		fields, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			logging.Logger().Warn("drop malformed session summary line", "err", err)
			continue
		}
		if len(fields) == 0 {
			continue
		}
		blank := true
		for _, field := range fields {
			if strings.TrimSpace(field) != "" {
				blank = false
				break
			}
		}
		if blank {
			continue
		}
		if len(fields) != 3 {
			logging.Logger().Warn("drop malformed session summary line", "fields", len(fields))
			continue
		}

		entry := memory.LogEntry{
			Timestamp: baseTime.Add(time.Duration(entryIndex) * time.Nanosecond),
			Tags:      memory.NormalizeTags(strings.Split(fields[0], ",")),
			Text:      fields[1],
			KV:        fields[2],
		}
		if err := a.memoryStore.AppendDailyLog(entry); err != nil {
			logging.Logger().Warn("append session summary to daily log failed", "err", err)
			return
		}
		entryIndex++
	}
}
