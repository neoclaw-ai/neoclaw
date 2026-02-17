package agent

import (
	"context"
	"strings"
	"time"

	"github.com/machinae/betterclaw/internal/logging"
	providerapi "github.com/machinae/betterclaw/internal/provider"
)

func (a *Agent) ensureHistoryLoaded(ctx context.Context) error {
	if a.historyLoadedOnce || a.sessionStore == nil {
		return nil
	}
	history, err := a.sessionStore.Load(ctx)
	if err != nil {
		return err
	}
	a.history = history
	a.historyLoadedOnce = true
	return nil
}

func (a *Agent) resetSession(ctx context.Context) error {
	a.history = nil
	a.historyLoadedOnce = true
	if a.sessionStore == nil {
		return nil
	}
	return a.sessionStore.Reset(ctx)
}

func (a *Agent) rewriteSessionIfNeeded(ctx context.Context, history []providerapi.ChatMessage) error {
	if a.sessionStore == nil {
		return nil
	}
	return a.sessionStore.Rewrite(ctx, history)
}

func (a *Agent) appendSessionDelta(ctx context.Context, base, history []providerapi.ChatMessage) error {
	if a.sessionStore == nil {
		return nil
	}
	if len(history) < len(base) {
		return a.sessionStore.Rewrite(ctx, history)
	}
	return a.sessionStore.Append(ctx, history[len(base):])
}

func isResetCommand(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	return normalized == "/new" || normalized == "/reset"
}

func (a *Agent) summarizeSessionToDailyLogAsync(ctx context.Context, history []providerapi.ChatMessage) {
	if a == nil || a.memoryStore == nil || len(history) == 0 {
		return
	}
	timeout := a.requestTimeout
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}

	go func(snapshot []providerapi.ChatMessage) {
		reqCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		summary, err := a.summarizeMessages(reqCtx, snapshot)
		if err != nil {
			logging.Logger().Warn("session reset summary failed", "err", err)
			return
		}
		summary = strings.TrimSpace(summary)
		if summary == "" {
			logging.Logger().Warn("session reset summary is empty; skipping daily log append")
			return
		}
		if err := a.memoryStore.AppendDailyLog(time.Now(), "Summary: "+summary); err != nil {
			logging.Logger().Warn("append session summary to daily log failed", "err", err)
		}
	}(append([]providerapi.ChatMessage{}, history...))
}
