package runtime

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/neoclaw-ai/neoclaw/internal/logging"
)

const userVisibleHandlerError = "There was an error with your request. Check server logs for details"

// Dispatcher executes queued messages sequentially against a Handler.
type Dispatcher struct {
	handler Handler

	queue chan dispatchItem
	done  chan struct{}

	stateMu    sync.Mutex
	started    bool
	rootCtx    context.Context
	currentRun context.CancelFunc
}

type dispatchItem struct {
	msg    *Message
	writer ResponseWriter
}

// NewDispatcher creates a dispatcher with a fixed-size queue.
func NewDispatcher(handler Handler, queueSize int) *Dispatcher {
	if queueSize <= 0 {
		queueSize = 1
	}
	return &Dispatcher{
		handler: handler,
		queue:   make(chan dispatchItem, queueSize),
		done:    make(chan struct{}),
	}
}

// Start begins the dispatch loop.
func (d *Dispatcher) Start(ctx context.Context) error {
	if d == nil {
		return errors.New("dispatcher is required")
	}
	if d.handler == nil {
		return errors.New("handler is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	d.stateMu.Lock()
	if d.started {
		d.stateMu.Unlock()
		return errors.New("dispatcher already started")
	}
	d.started = true
	d.rootCtx = ctx
	d.stateMu.Unlock()

	go d.run(ctx)
	return nil
}

// Enqueue submits one message for FIFO processing.
func (d *Dispatcher) Enqueue(ctx context.Context, msg *Message, writer ResponseWriter) error {
	if msg == nil {
		return errors.New("message is required")
	}
	if writer == nil {
		return errors.New("response writer is required")
	}
	rootCtx, started := d.dispatchContext()
	if !started {
		return errors.New("dispatcher is not started")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-rootCtx.Done():
		return rootCtx.Err()
	case <-ctx.Done():
		return ctx.Err()
	case d.queue <- dispatchItem{msg: msg, writer: writer}:
		return nil
	}
}

// Stop cancels the in-flight run and drains all queued pending messages.
func (d *Dispatcher) Stop() {
	d.cancelCurrentRun()
	for {
		select {
		case <-d.queue:
		default:
			return
		}
	}
}

// WaitUntilIdle blocks until no message is running and the queue is empty.
func (d *Dispatcher) WaitUntilIdle(ctx context.Context) error {
	if d == nil {
		return errors.New("dispatcher is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		if d.isIdle() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Wait blocks until the dispatch loop exits.
func (d *Dispatcher) Wait() {
	if d == nil {
		return
	}
	<-d.done
}

func (d *Dispatcher) run(ctx context.Context) {
	defer close(d.done)
	for {
		select {
		case <-ctx.Done():
			d.cancelCurrentRun()
			return
		case item := <-d.queue:
			if item.msg == nil || item.writer == nil {
				continue
			}
			runCtx, cancel := context.WithCancel(ctx)
			d.setCurrentRun(cancel)
			err := d.handler.HandleMessage(runCtx, item.writer, item.msg)
			d.clearCurrentRun()
			cancel()
			if err == nil || errors.Is(err, context.Canceled) {
				continue
			}
			logging.Logger().Error("message handling failed", "err", err)
			if writeErr := item.writer.WriteMessage(ctx, userVisibleHandlerError); writeErr != nil {
				logging.Logger().Warn("failed to write handler error message", "err", writeErr)
			}
		}
	}
}

func (d *Dispatcher) dispatchContext() (context.Context, bool) {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	return d.rootCtx, d.started
}

func (d *Dispatcher) setCurrentRun(cancel context.CancelFunc) {
	d.stateMu.Lock()
	d.currentRun = cancel
	d.stateMu.Unlock()
}

func (d *Dispatcher) clearCurrentRun() {
	d.stateMu.Lock()
	d.currentRun = nil
	d.stateMu.Unlock()
}

func (d *Dispatcher) cancelCurrentRun() {
	d.stateMu.Lock()
	cancel := d.currentRun
	d.currentRun = nil
	d.stateMu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (d *Dispatcher) isIdle() bool {
	d.stateMu.Lock()
	running := d.currentRun != nil
	started := d.started
	d.stateMu.Unlock()

	if !started {
		return true
	}
	return !running && len(d.queue) == 0
}
