package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	runtimeapi "github.com/machinae/betterclaw/internal/runtime"

	"github.com/machinae/betterclaw/internal/approval"
	providerapi "github.com/machinae/betterclaw/internal/provider"
	"github.com/machinae/betterclaw/internal/tools"
)

const defaultAgentMaxIterations = 10

// DefaultSystemPrompt is the default system prompt used by the prompt command.
const DefaultSystemPrompt = "You are BetterClaw, a lightweight personal AI assistant."

// Agent implements the runtime Handler for one conversation.
type Agent struct {
	provider     providerapi.Provider
	registry     *tools.Registry
	approver     approval.Approver
	systemPrompt string
	maxIter      int
	history      []providerapi.ChatMessage
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

	messages := appendUserMessage(a.history, msg.Text)
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
	if err := w.WriteMessage(ctx, resp.Content); err != nil {
		return err
	}
	return nil
}
