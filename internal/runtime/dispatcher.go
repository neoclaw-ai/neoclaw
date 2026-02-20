package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Dispatcher executes queued messages sequentially against a Handler.
type Dispatcher struct {
	handler Handler
	writer  ResponseWriter

	queue chan *Message
	done  chan struct{}

	stateMu    sync.Mutex
	started    bool
	rootCtx    context.Context
	currentRun context.CancelFunc
}

// NewDispatcher creates a dispatcher with a fixed-size queue.
func NewDispatcher(handler Handler, writer ResponseWriter, queueSize int) *Dispatcher {
	if queueSize <= 0 {
		queueSize = 1
	}
	return &Dispatcher{
		handler: handler,
		writer:  writer,
		queue:   make(chan *Message, queueSize),
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
	if d.writer == nil {
		return errors.New("response writer is required")
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
func (d *Dispatcher) Enqueue(ctx context.Context, msg *Message) error {
	if msg == nil {
		return errors.New("message is required")
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
	case d.queue <- msg:
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
		case msg := <-d.queue:
			if msg == nil {
				continue
			}
			runCtx, cancel := context.WithCancel(ctx)
			d.setCurrentRun(cancel)
			err := d.handler.HandleMessage(runCtx, d.writer, msg)
			d.clearCurrentRun()
			cancel()
			if err == nil || errors.Is(err, context.Canceled) {
				continue
			}
			d.writer.WriteMessage(ctx, fmt.Sprintf("error: %v", err))
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
