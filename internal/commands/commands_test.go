package commands

import (
	"context"
	"errors"
	"testing"

	"github.com/machinae/betterclaw/internal/runtime"
)

func TestHelpCommand(t *testing.T) {
	h := New(nil)
	w := &captureWriter{}

	handled, err := h.Handle(context.Background(), "/help", w)
	if err != nil {
		t.Fatalf("handle /help: %v", err)
	}
	if !handled {
		t.Fatalf("expected /help handled")
	}
	if len(w.messages) != 1 || w.messages[0] != helpText {
		t.Fatalf("unexpected help output: %#v", w.messages)
	}
}

func TestResetAlias(t *testing.T) {
	resetter := &fakeResetter{}
	h := New(resetter)
	w := &captureWriter{}

	handled, err := h.Handle(context.Background(), "/reset", w)
	if err != nil {
		t.Fatalf("handle /reset: %v", err)
	}
	if !handled {
		t.Fatalf("expected /reset handled")
	}
	if resetter.calls != 1 {
		t.Fatalf("expected resetter call, got %d", resetter.calls)
	}
	if len(w.messages) != 1 || w.messages[0] != "Session cleared." {
		t.Fatalf("unexpected reset output: %#v", w.messages)
	}
}

func TestUnknownCommand(t *testing.T) {
	h := New(nil)
	w := &captureWriter{}

	handled, err := h.Handle(context.Background(), "/unknown", w)
	if err != nil {
		t.Fatalf("handle unknown: %v", err)
	}
	if handled {
		t.Fatalf("expected unknown handled=false")
	}
	if len(w.messages) != 0 {
		t.Fatalf("expected no output, got %#v", w.messages)
	}
}

func TestRouterForwardsNonCommands(t *testing.T) {
	next := &fakeRuntimeHandler{}
	router := Router{
		Commands: New(nil),
		Next:     next,
	}

	if err := router.HandleMessage(context.Background(), &captureWriter{}, &runtime.Message{Text: "hello"}); err != nil {
		t.Fatalf("router forward: %v", err)
	}
	if next.calls != 1 {
		t.Fatalf("expected Next called once, got %d", next.calls)
	}
}

func TestRouterHandlesSlashCommand(t *testing.T) {
	next := &fakeRuntimeHandler{}
	router := Router{
		Commands: New(nil),
		Next:     next,
	}
	w := &captureWriter{}

	if err := router.HandleMessage(context.Background(), w, &runtime.Message{Text: "/help"}); err != nil {
		t.Fatalf("router /help: %v", err)
	}
	if next.calls != 0 {
		t.Fatalf("expected Next not called for command, got %d", next.calls)
	}
}

func TestResetErrorReturned(t *testing.T) {
	resetter := &fakeResetter{err: errors.New("boom")}
	h := New(resetter)

	handled, err := h.Handle(context.Background(), "/new", &captureWriter{})
	if !handled {
		t.Fatalf("expected handled=true")
	}
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected reset error, got %v", err)
	}
}

type fakeResetter struct {
	calls int
	err   error
}

func (r *fakeResetter) Reset(_ context.Context) error {
	r.calls++
	return r.err
}

type fakeRuntimeHandler struct {
	calls int
}

func (h *fakeRuntimeHandler) HandleMessage(_ context.Context, _ runtime.ResponseWriter, _ *runtime.Message) error {
	h.calls++
	return nil
}

type captureWriter struct {
	messages []string
}

func (w *captureWriter) WriteMessage(_ context.Context, text string) error {
	w.messages = append(w.messages, text)
	return nil
}
