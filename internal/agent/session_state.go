package agent

import (
	"context"
	"strings"

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
