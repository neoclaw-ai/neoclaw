package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestDispatcherFIFO(t *testing.T) {
	handler := &recordingHandler{}
	writer := &recordingWriter{}
	d := NewDispatcher(handler, 20)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("start dispatcher: %v", err)
	}

	if err := d.Enqueue(context.Background(), &Message{Text: "first"}, writer); err != nil {
		t.Fatalf("enqueue first: %v", err)
	}
	if err := d.Enqueue(context.Background(), &Message{Text: "second"}, writer); err != nil {
		t.Fatalf("enqueue second: %v", err)
	}

	waitFor(t, time.Second, func() bool {
		handler.mu.Lock()
		defer handler.mu.Unlock()
		return len(handler.messages) == 2
	})

	cancel()
	d.Wait()

	handler.mu.Lock()
	defer handler.mu.Unlock()
	if got := handler.messages; len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Fatalf("expected FIFO order [first second], got %#v", got)
	}
}

func TestDispatcherQueuesBehindRunningMessage(t *testing.T) {
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondStarted := make(chan struct{}, 1)
	handler := &queueingHandler{
		firstStarted:  firstStarted,
		releaseFirst:  releaseFirst,
		secondStarted: secondStarted,
	}
	writer := &recordingWriter{}
	d := NewDispatcher(handler, 20)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("start dispatcher: %v", err)
	}
	if err := d.Enqueue(context.Background(), &Message{Text: "first"}, writer); err != nil {
		t.Fatalf("enqueue first: %v", err)
	}
	<-firstStarted
	if err := d.Enqueue(context.Background(), &Message{Text: "second"}, writer); err != nil {
		t.Fatalf("enqueue second: %v", err)
	}

	select {
	case <-secondStarted:
		t.Fatalf("second message started before first completed")
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseFirst)
	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		t.Fatalf("second message did not start after first completed")
	}

	cancel()
	d.Wait()
}

func TestDispatcherStopCancelsInFlightAndDrainsQueue(t *testing.T) {
	firstCanceled := make(chan struct{}, 1)
	handler := &stopHandler{
		firstCanceled: firstCanceled,
	}
	writer := &recordingWriter{}
	d := NewDispatcher(handler, 20)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("start dispatcher: %v", err)
	}

	if err := d.Enqueue(context.Background(), &Message{Text: "first"}, writer); err != nil {
		t.Fatalf("enqueue first: %v", err)
	}
	waitFor(t, time.Second, func() bool {
		handler.mu.Lock()
		defer handler.mu.Unlock()
		return handler.startedFirst
	})
	if err := d.Enqueue(context.Background(), &Message{Text: "second"}, writer); err != nil {
		t.Fatalf("enqueue second: %v", err)
	}
	if err := d.Enqueue(context.Background(), &Message{Text: "third"}, writer); err != nil {
		t.Fatalf("enqueue third: %v", err)
	}

	d.Stop()

	select {
	case <-firstCanceled:
	case <-time.After(time.Second):
		t.Fatalf("expected in-flight first message to be canceled")
	}

	time.Sleep(50 * time.Millisecond)
	handler.mu.Lock()
	defer handler.mu.Unlock()
	if handler.otherCalls != 0 {
		t.Fatalf("expected queued messages to be drained, got %d extra calls", handler.otherCalls)
	}
}

func TestDispatcherStopWithoutInFlightIsNoop(t *testing.T) {
	d := NewDispatcher(&recordingHandler{}, 20)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("start dispatcher: %v", err)
	}
	d.Stop()
	cancel()
	d.Wait()
}

func TestDispatcherWritesHandlerErrors(t *testing.T) {
	handler := &errorHandler{err: errors.New("boom")}
	writer := &recordingWriter{}
	d := NewDispatcher(handler, 20)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("start dispatcher: %v", err)
	}
	if err := d.Enqueue(context.Background(), &Message{Text: "x"}, writer); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	waitFor(t, time.Second, func() bool {
		writer.mu.Lock()
		defer writer.mu.Unlock()
		return len(writer.messages) > 0
	})
	cancel()
	d.Wait()

	writer.mu.Lock()
	defer writer.mu.Unlock()
	if len(writer.messages) != 1 || writer.messages[0] != userVisibleHandlerError {
		t.Fatalf("expected one error write, got %#v", writer.messages)
	}
}

func TestDispatcherWaitUntilIdle(t *testing.T) {
	handler := &recordingHandler{}
	writer := &recordingWriter{}
	d := NewDispatcher(handler, 20)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("start dispatcher: %v", err)
	}
	if err := d.Enqueue(context.Background(), &Message{Text: "x"}, writer); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	if err := d.WaitUntilIdle(waitCtx); err != nil {
		t.Fatalf("wait until idle: %v", err)
	}

	cancel()
	d.Wait()

	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.messages) != 1 || handler.messages[0] != "x" {
		t.Fatalf("expected processed message before idle, got %#v", handler.messages)
	}
}

func TestDispatcherWaitUntilIdleDeadline(t *testing.T) {
	handler := &stopHandler{firstCanceled: make(chan struct{}, 1)}
	writer := &recordingWriter{}
	d := NewDispatcher(handler, 20)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("start dispatcher: %v", err)
	}
	if err := d.Enqueue(context.Background(), &Message{Text: "first"}, writer); err != nil {
		t.Fatalf("enqueue first: %v", err)
	}
	waitFor(t, time.Second, func() bool {
		handler.mu.Lock()
		defer handler.mu.Unlock()
		return handler.startedFirst
	})

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer waitCancel()
	if err := d.WaitUntilIdle(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}

	d.Stop()
	cancel()
	d.Wait()
}

func TestDispatcherSuppressesContextCanceledError(t *testing.T) {
	handler := &errorHandler{err: context.Canceled}
	writer := &recordingWriter{}
	d := NewDispatcher(handler, 20)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := d.Start(ctx); err != nil {
		t.Fatalf("start dispatcher: %v", err)
	}
	if err := d.Enqueue(context.Background(), &Message{Text: "x"}, writer); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	cancel()
	d.Wait()

	writer.mu.Lock()
	defer writer.mu.Unlock()
	if len(writer.messages) != 0 {
		t.Fatalf("expected no error write for context canceled, got %#v", writer.messages)
	}
}

type recordingHandler struct {
	mu       sync.Mutex
	messages []string
}

func (h *recordingHandler) HandleMessage(_ context.Context, _ ResponseWriter, msg *Message) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, msg.Text)
	return nil
}

type queueingHandler struct {
	firstStarted  chan struct{}
	releaseFirst  chan struct{}
	secondStarted chan struct{}
}

func (h *queueingHandler) HandleMessage(_ context.Context, _ ResponseWriter, msg *Message) error {
	switch msg.Text {
	case "first":
		close(h.firstStarted)
		<-h.releaseFirst
	case "second":
		h.secondStarted <- struct{}{}
	}
	return nil
}

type stopHandler struct {
	mu           sync.Mutex
	startedFirst bool
	otherCalls   int

	firstCanceled chan struct{}
}

func (h *stopHandler) HandleMessage(ctx context.Context, _ ResponseWriter, msg *Message) error {
	if msg.Text == "first" {
		h.mu.Lock()
		h.startedFirst = true
		h.mu.Unlock()
		<-ctx.Done()
		h.firstCanceled <- struct{}{}
		return ctx.Err()
	}
	h.mu.Lock()
	h.otherCalls++
	h.mu.Unlock()
	return nil
}

type errorHandler struct {
	err error
}

func (h *errorHandler) HandleMessage(_ context.Context, _ ResponseWriter, _ *Message) error {
	return h.err
}

type recordingWriter struct {
	mu       sync.Mutex
	messages []string
}

func (w *recordingWriter) WriteMessage(_ context.Context, text string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.messages = append(w.messages, text)
	return nil
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}
