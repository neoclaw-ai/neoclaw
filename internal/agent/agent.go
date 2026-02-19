// Package agent implements the conversation handler, driving the LLM tool-use loop with per-conversation history and session persistence.
package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/machinae/betterclaw/internal/runtime"

	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/costs"
	"github.com/machinae/betterclaw/internal/logging"
	"github.com/machinae/betterclaw/internal/memory"
	"github.com/machinae/betterclaw/internal/provider"
	"github.com/machinae/betterclaw/internal/session"
	"github.com/machinae/betterclaw/internal/tools"
)

const defaultAgentMaxIterations = 10
const defaultRequestTimeout = 30 * time.Second

// DefaultSystemPrompt is the default system prompt used by the cli command.
const DefaultSystemPrompt = "You are BetterClaw, a lightweight personal AI assistant."

// Agent implements the runtime Handler for one conversation.
type Agent struct {
	provider          provider.Provider
	registry          *tools.Registry
	approver          approval.Approver
	systemPrompt      string
	maxIter           int
	maxContextTokens  int
	recentMessages    int
	history           []provider.ChatMessage
	sessionStore      *session.Store
	memoryStore       *memory.Store
	requestTimeout    time.Duration
	historyLoadedOnce bool
	costTracker       *costs.Tracker
	costProvider      string
	costModel         string
	dailySpendLimit   float64
	monthlySpendLimit float64
}

// New creates a conversation-scoped Agent.
func New(provider provider.Provider, registry *tools.Registry, approver approval.Approver, systemPrompt string) *Agent {
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
	provider provider.Provider,
	registry *tools.Registry,
	approver approval.Approver,
	systemPrompt string,
	sessionStore *session.Store,
	memoryStore *memory.Store,
	maxContextTokens int,
	recentMessages int,
	requestTimeout time.Duration,
) *Agent {
	ag := New(provider, registry, approver, systemPrompt)
	ag.sessionStore = sessionStore
	ag.memoryStore = memoryStore
	ag.maxContextTokens = maxContextTokens
	ag.recentMessages = recentMessages
	ag.requestTimeout = requestTimeout
	if ag.requestTimeout <= 0 {
		ag.requestTimeout = defaultRequestTimeout
	}
	return ag
}

// ConfigureCosts enables cost tracking and optional daily/monthly spend limits.
func (a *Agent) ConfigureCosts(
	tracker *costs.Tracker,
	providerName string,
	model string,
	dailyLimit float64,
	monthlyLimit float64,
) {
	a.costTracker = tracker
	a.costProvider = providerName
	a.costModel = model
	a.dailySpendLimit = dailyLimit
	a.monthlySpendLimit = monthlyLimit
}

// HandleMessage processes one inbound message and writes the assistant response.
func (a *Agent) HandleMessage(ctx context.Context, w runtime.ResponseWriter, msg *runtime.Message) error {
	if w == nil {
		return errors.New("response writer is required")
	}
	if msg == nil {
		return errors.New("message is required")
	}
	if strings.TrimSpace(msg.Text) == "" {
		return nil
	}

	blocked, err := a.enforceSpendLimits(ctx, w, time.Now())
	if err != nil {
		return err
	}
	if blocked {
		return nil
	}

	if err := a.ensureHistoryLoaded(ctx); err != nil {
		return err
	}

	baseHistory := append([]provider.ChatMessage{}, a.history...)
	messages := appendUserMessage(baseHistory, msg.Text)
	uncompactedMessages := append([]provider.ChatMessage{}, messages...)
	messages, err = a.compactHistoryIfNeeded(ctx, messages)
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
		func(usage provider.TokenUsage) error {
			if err := a.recordUsage(ctx, usage); err != nil {
				logging.Logger().Warn("failed to record llm usage", "err", err)
			}
			return nil
		},
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

func (a *Agent) enforceSpendLimits(ctx context.Context, w runtime.ResponseWriter, now time.Time) (bool, error) {
	if a.costTracker == nil {
		return false, nil
	}
	if a.dailySpendLimit <= 0 && a.monthlySpendLimit <= 0 {
		return false, nil
	}

	spend, err := a.costTracker.Spend(ctx, now)
	if err != nil {
		return false, err
	}

	if a.dailySpendLimit > 0 && spend.TodayUSD >= a.dailySpendLimit {
		if err := w.WriteMessage(ctx, fmt.Sprintf("Daily spend limit reached: $%.4f / $%.4f", spend.TodayUSD, a.dailySpendLimit)); err != nil {
			return false, err
		}
		return true, nil
	}
	if a.monthlySpendLimit > 0 && spend.MonthUSD >= a.monthlySpendLimit {
		if err := w.WriteMessage(ctx, fmt.Sprintf("Monthly spend limit reached: $%.4f / $%.4f", spend.MonthUSD, a.monthlySpendLimit)); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (a *Agent) recordUsage(ctx context.Context, usage provider.TokenUsage) error {
	if a.costTracker == nil {
		return nil
	}

	costUSD := 0.0
	if usage.CostUSD != nil {
		costUSD = *usage.CostUSD
	} else if estimated, ok := costs.EstimateUSD(
		a.costProvider,
		a.costModel,
		usage.InputTokens,
		usage.OutputTokens,
	); ok {
		costUSD = estimated
	}

	return a.costTracker.Append(ctx, costs.Record{
		Timestamp:    time.Now(),
		Provider:     a.costProvider,
		Model:        a.costModel,
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		TotalTokens:  usage.TotalTokens,
		CostUSD:      costUSD,
	})
}
