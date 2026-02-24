// Package commands provides channel-agnostic slash command handling.
package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/neoclaw-ai/neoclaw/internal/costs"
	"github.com/neoclaw-ai/neoclaw/internal/runtime"
	"github.com/neoclaw-ai/neoclaw/internal/scheduler"
)

const helpText = `Available commands:
/help - Show available commands
/new, /reset - Clear the current session
/jobs - List scheduled jobs
/usage - Show cost usage`

// Resetter resets the active conversation/session state.
type Resetter interface {
	Reset(ctx context.Context) error
}

// Handler dispatches supported slash commands.
type Handler struct {
	resetter Resetter
	jobs     *scheduler.Service
	costs    *costs.Tracker
	daily    float64
	monthly  float64
}

// New creates a new slash command handler.
func New(resetter Resetter, jobs *scheduler.Service, costTracker *costs.Tracker, dailyLimit, monthlyLimit float64) *Handler {
	return &Handler{
		resetter: resetter,
		jobs:     jobs,
		costs:    costTracker,
		daily:    dailyLimit,
		monthly:  monthlyLimit,
	}
}

// Handle executes one command and reports whether it was handled.
func (h *Handler) Handle(ctx context.Context, cmd string, w runtime.ResponseWriter) (handled bool, err error) {
	if w == nil {
		return false, errors.New("response writer is required")
	}

	switch normalize(cmd) {
	case "/help":
		return true, h.handleHelp(ctx, w)
	case "/new", "/reset":
		return true, h.handleReset(ctx, w)
	case "/jobs":
		return true, h.handleJobs(ctx, w)
	case "/usage":
		return true, h.handleUsage(ctx, w)
	default:
		return false, nil
	}
}

func (h *Handler) handleHelp(ctx context.Context, w runtime.ResponseWriter) error {
	return w.WriteMessage(ctx, helpText)
}

func (h *Handler) handleReset(ctx context.Context, w runtime.ResponseWriter) error {
	if h.resetter == nil {
		return errors.New("reset command is unavailable")
	}
	if err := h.resetter.Reset(ctx); err != nil {
		return err
	}
	return w.WriteMessage(ctx, "Session cleared.")
}

func (h *Handler) handleJobs(ctx context.Context, w runtime.ResponseWriter) error {
	if h.jobs == nil {
		return errors.New("jobs command is unavailable")
	}
	jobs, err := h.jobs.List(ctx)
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		return w.WriteMessage(ctx, "No scheduled jobs.")
	}
	var b strings.Builder
	b.WriteString("Scheduled jobs:\n")
	for i, job := range jobs {
		status := "disabled"
		if job.Enabled {
			status = "enabled"
		}
		fmt.Fprintf(&b, "%d. %s (%s) - %s\n", i+1, job.Description, job.Cron, status)
		fmt.Fprintf(&b, "   id: %s, action: %s", job.ID, job.Action)
		if i < len(jobs)-1 {
			b.WriteByte('\n')
		}
	}
	return w.WriteMessage(ctx, b.String())
}

func (h *Handler) handleUsage(ctx context.Context, w runtime.ResponseWriter) error {
	if h.costs == nil {
		return errors.New("usage command is unavailable")
	}

	spend, err := h.costs.Spend(ctx, time.Now())
	if err != nil {
		return err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Today: $%.4f", spend.TodayUSD)
	if h.daily > 0 {
		fmt.Fprintf(&b, " / $%.4f", h.daily)
	}
	b.WriteByte('\n')

	fmt.Fprintf(&b, "This month: $%.4f", spend.MonthUSD)
	if h.monthly > 0 {
		fmt.Fprintf(&b, " / $%.4f", h.monthly)
	}

	return w.WriteMessage(ctx, b.String())
}

// Router dispatches slash commands before delegating to the next runtime.Handler.
type Router struct {
	Commands *Handler
	Next     runtime.Handler
}

// HandleMessage runs command dispatch first, then forwards non-command input.
func (r Router) HandleMessage(ctx context.Context, w runtime.ResponseWriter, msg *runtime.Message) error {
	if msg == nil {
		return errors.New("message is required")
	}
	if r.Next == nil {
		return errors.New("next handler is required")
	}
	if r.Commands != nil {
		handled, err := r.Commands.Handle(ctx, msg.Text, w)
		if handled || err != nil {
			return err
		}
	}
	return r.Next.HandleMessage(ctx, w, msg)
}

func normalize(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}
