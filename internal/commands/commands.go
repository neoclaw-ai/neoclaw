// Package commands provides channel-agnostic slash command handling.
package commands

import (
	"context"
	"errors"
	"strings"

	"github.com/machinae/betterclaw/internal/runtime"
)

const helpText = "Commands: /help, /commands, /new, /reset, /stop"

// Resetter resets the active conversation/session state.
type Resetter interface {
	Reset(ctx context.Context) error
}

// Handler dispatches supported slash commands.
type Handler struct {
	resetter Resetter
}

// New creates a new slash command handler.
func New(resetter Resetter) *Handler {
	return &Handler{resetter: resetter}
}

// Handle executes one command and reports whether it was handled.
func (h *Handler) Handle(ctx context.Context, cmd string, w runtime.ResponseWriter) (handled bool, err error) {
	if w == nil {
		return false, errors.New("response writer is required")
	}

	switch normalize(cmd) {
	case "/help", "/commands":
		return true, w.WriteMessage(ctx, helpText)
	case "/new", "/reset":
		if h.resetter == nil {
			return true, errors.New("reset command is unavailable")
		}
		if err := h.resetter.Reset(ctx); err != nil {
			return true, err
		}
		return true, w.WriteMessage(ctx, "Session cleared.")
	default:
		return false, nil
	}
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
