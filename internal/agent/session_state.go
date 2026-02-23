package agent

import (
	"context"
	"strings"
	"time"

	"github.com/machinae/betterclaw/internal/logging"
	"github.com/machinae/betterclaw/internal/provider"
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
}
