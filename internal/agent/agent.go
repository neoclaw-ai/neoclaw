package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	runtimeapi "github.com/machinae/betterclaw/internal/runtime"

	"github.com/machinae/betterclaw/internal/approval"
	providerapi "github.com/machinae/betterclaw/internal/provider"
	"github.com/machinae/betterclaw/internal/session"
	"github.com/machinae/betterclaw/internal/tools"
)

const defaultAgentMaxIterations = 10

// DefaultSystemPrompt is the default system prompt used by the prompt command.
const DefaultSystemPrompt = "You are BetterClaw, a lightweight personal AI assistant."

// Agent implements the runtime Handler for one conversation.
type Agent struct {
	provider          providerapi.Provider
	registry          *tools.Registry
	approver          approval.Approver
	systemPrompt      string
	maxIter           int
	maxContextTokens  int
	recentMessages    int
	history           []providerapi.ChatMessage
	sessionStore      *session.Store
	historyLoadedOnce bool
}

// New creates a conversation-scoped Agent.
func New(provider providerapi.Provider, registry *tools.Registry, approver approval.Approver, systemPrompt string) *Agent {
	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = DefaultSystemPrompt
	}
	return &Agent{
		provider:     provider,
		registry:     registry,
		approver:     approver,
		systemPrompt: systemPrompt,
		maxIter:      defaultAgentMaxIterations,
	}
}

// NewWithSession creates a conversation-scoped Agent with session persistence.
func NewWithSession(
	provider providerapi.Provider,
	registry *tools.Registry,
	approver approval.Approver,
	systemPrompt string,
	sessionStore *session.Store,
	maxContextTokens int,
	recentMessages int,
) *Agent {
	ag := New(provider, registry, approver, systemPrompt)
	ag.sessionStore = sessionStore
	ag.maxContextTokens = maxContextTokens
	ag.recentMessages = recentMessages
	return ag
}

// HandleMessage processes one inbound message and writes the assistant response.
func (a *Agent) HandleMessage(ctx context.Context, w runtimeapi.ResponseWriter, msg *runtimeapi.Message) error {
	if w == nil {
		return errors.New("response writer is required")
	}
	if msg == nil {
		return errors.New("message is required")
	}
	if strings.TrimSpace(msg.Text) == "" {
		return nil
	}
	if err := a.ensureHistoryLoaded(ctx); err != nil {
		return err
	}

	if isResetCommand(msg.Text) {
		if err := a.resetSession(ctx); err != nil {
			return err
		}
		return w.WriteMessage(ctx, "Session cleared.")
	}

	baseHistory := append([]providerapi.ChatMessage{}, a.history...)
	messages := appendUserMessage(baseHistory, msg.Text)
	uncompactedMessages := append([]providerapi.ChatMessage{}, messages...)
	messages, err := a.compactHistoryIfNeeded(ctx, messages)
	if err != nil {
		return err
	}
	resp, history, err := Run(
		ctx,
		a.provider,
		a.registry,
		a.approver,
		a.systemPrompt,
		messages,
		a.maxIter,
	)
	if err != nil {
		// Option 2 policy: return runtime/infrastructure errors so transports
		// can own retry/backoff/exit behavior.
		return err
	}
	if resp == nil {
		return fmt.Errorf("agent run returned nil response")
	}

	a.history = history
	if sameMessageSlice(messages, uncompactedMessages) {
		err = a.appendSessionDelta(ctx, baseHistory, history)
	} else {
		err = a.rewriteSessionIfNeeded(ctx, history)
	}
	if err != nil {
		return err
	}
	if err := w.WriteMessage(ctx, resp.Content); err != nil {
		return err
	}
	return nil
}
