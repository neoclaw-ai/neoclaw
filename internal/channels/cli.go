// Package channels provides runtime.Listener implementations for each supported input channel (CLI, and future Telegram, Discord, etc.).
package channels

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chzyer/readline"
	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/runtime"
	"golang.org/x/term"
)

const (
	defaultReplPrompt    = "you> "
	defaultDispatchQueue = 20
	// Allow queued input to finish when stdin closes before shutting down the dispatcher.
	dispatchDrainTimeout = 5 * time.Second
)

var (
	_ runtime.Listener  = (*CLIListener)(nil)
	_ approval.Approver = (*CLIListener)(nil)
)

// CLIWriter writes assistant responses to terminal output.
type CLIWriter struct {
	out io.Writer
}

// WriteMessage writes one assistant message line.
func (w *CLIWriter) WriteMessage(_ context.Context, text string) error {
	_, err := fmt.Fprintf(w.out, "assistant> %s\n\n", text)
	return err
}

// CLIListener listens for interactive terminal input and dispatches messages.
type CLIListener struct {
	in  io.Reader
	out io.Writer

	rl       *readline.Instance
	fallback *bufio.Reader

	stateMu      sync.Mutex
	approvalReq  chan approvalInputRequest
	listenDoneCh chan struct{}
}

// NewCLI creates a new CLI listener over stdin/stdout style streams.
func NewCLI(in io.Reader, out io.Writer) *CLIListener {
	return &CLIListener{in: in, out: out}
}

// Listen runs the interactive loop until EOF, /quit, /exit, or fatal handler error.
func (c *CLIListener) Listen(ctx context.Context, handler runtime.Handler) error {
	if handler == nil {
		return fmt.Errorf("handler is required")
	}
	if err := c.ensureInputReady(); err != nil {
		return err
	}
	if c.rl != nil {
		defer c.rl.Close()
	}

	if _, err := fmt.Fprintln(c.out, "Interactive mode. Type /quit or /exit to stop."); err != nil {
		return err
	}

	writer := &CLIWriter{out: c.out}
	dispatchCtx, cancelDispatch := context.WithCancel(ctx)

	dispatcher := runtime.NewDispatcher(handler, writer, defaultDispatchQueue)
	if err := dispatcher.Start(dispatchCtx); err != nil {
		cancelDispatch()
		return err
	}
	defer func() {
		cancelDispatch()
		dispatcher.Wait()
	}()

	reqCh, doneCh := c.setupApprovalChannels()
	defer c.teardownApprovalChannels(reqCh, doneCh)

	inputCh := make(chan inputEvent)
	go c.readInputLoop(ctx, inputCh)

	var pendingApproval *approvalInputRequest
	for {
		select {
		case <-ctx.Done():
			dispatcher.Stop()
			return nil
		case req := <-reqCh:
			if pendingApproval != nil {
				req.response <- approvalInputResponse{err: errors.New("another approval is already pending")}
				continue
			}
			pendingApproval = &req
			if _, err := fmt.Fprint(c.out, req.prompt); err != nil {
				pendingApproval.response <- approvalInputResponse{err: err}
				pendingApproval = nil
			}
			case event, ok := <-inputCh:
				if !ok {
					c.drainDispatcher(dispatcher)
					return nil
				}
				if event.err != nil {
					if pendingApproval != nil {
						pendingApproval.response <- approvalInputResponse{err: event.err}
						pendingApproval = nil
					}
					if errors.Is(event.err, io.EOF) {
						c.drainDispatcher(dispatcher)
						return nil
					}
					if errors.Is(event.err, context.Canceled) {
						dispatcher.Stop()
						return nil
					}
					return event.err
				}

			line := strings.TrimSpace(event.line)
			if pendingApproval != nil {
				pendingApproval.response <- approvalInputResponse{line: line}
				pendingApproval = nil
				continue
			}
			if line == "" {
				continue
			}

			switch strings.ToLower(line) {
			case "/stop", "stop":
				dispatcher.Stop()
				writer.WriteMessage(ctx, "Stopped.")
				continue
			case "/quit", "quit", "/exit", "exit":
				dispatcher.Stop()
				writer.WriteMessage(ctx, "Stopped.")
				return nil
			}

			if err := dispatcher.Enqueue(ctx, &runtime.Message{Text: line}); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return err
			}
		}
	}
}

func (c *CLIListener) drainDispatcher(dispatcher *runtime.Dispatcher) {
	drainCtx, cancel := context.WithTimeout(context.Background(), dispatchDrainTimeout)
	defer cancel()
	if err := dispatcher.WaitUntilIdle(drainCtx); err != nil {
		dispatcher.Stop()
	}
}

// RequestApproval prompts the user for tool approval decision.
func (c *CLIListener) RequestApproval(ctx context.Context, req approval.ApprovalRequest) (approval.ApprovalDecision, error) {
	if err := ctx.Err(); err != nil {
		return approval.Denied, err
	}
	if err := c.ensureInputReady(); err != nil {
		return approval.Denied, err
	}

	prompt := fmt.Sprintf("approve tool %s? %s [y/N]: ", req.Tool, req.Description)
	reqCh, doneCh := c.approvalChannels()
	if reqCh == nil || doneCh == nil {
		return c.requestApprovalDirect(prompt)
	}

	pending := approvalInputRequest{
		prompt:   prompt,
		response: make(chan approvalInputResponse, 1),
	}
	select {
	case reqCh <- pending:
	case <-doneCh:
		return approval.Denied, errors.New("approval unavailable: listener stopped")
	case <-ctx.Done():
		return approval.Denied, ctx.Err()
	}

	var answer string
	select {
	case resp := <-pending.response:
		if resp.err != nil {
			return approval.Denied, resp.err
		}
		answer = resp.line
	case <-doneCh:
		return approval.Denied, errors.New("approval unavailable: listener stopped")
	case <-ctx.Done():
		return approval.Denied, ctx.Err()
	}
	return parseApprovalAnswer(answer), nil
}

func (c *CLIListener) requestApprovalDirect(prompt string) (approval.ApprovalDecision, error) {
	var answer string
	if c.rl != nil {
		line, err := c.readApprovalLineReadline(prompt)
		if err != nil {
			return approval.Denied, err
		}
		answer = line
	} else {
		if _, err := fmt.Fprint(c.out, prompt); err != nil {
			return approval.Denied, err
		}
		line, err := c.fallback.ReadString('\n')
		if err != nil {
			return approval.Denied, err
		}
		answer = line
	}

	return parseApprovalAnswer(answer), nil
}

func parseApprovalAnswer(answer string) approval.ApprovalDecision {
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return approval.Approved
	default:
		return approval.Denied
	}
}

func (c *CLIListener) ensureInputReady() error {
	if c.rl != nil || c.fallback != nil {
		return nil
	}

	rl, err := newReadline(c.in, c.out)
	if err == nil {
		c.rl = rl
		return nil
	}

	c.fallback = bufio.NewReader(c.in)
	return nil
}

func (c *CLIListener) readLine(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if c.rl != nil {
		line, err := c.rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt || err == io.EOF {
				return "", io.EOF
			}
			return "", err
		}
		return line, nil
	}

	if _, err := fmt.Fprint(c.out, defaultReplPrompt); err != nil {
		return "", err
	}
	line, err := c.fallback.ReadString('\n')
	if err != nil {
		if len(line) > 0 {
			return line, nil
		}
		return "", err
	}
	return line, nil
}

func (c *CLIListener) readApprovalLineReadline(prompt string) (string, error) {
	c.rl.SetPrompt(prompt)
	c.rl.Refresh()
	defer func() {
		c.rl.SetPrompt(defaultReplPrompt)
		c.rl.Refresh()
	}()

	line, err := c.rl.Readline()
	if err != nil {
		return "", err
	}
	return line, nil
}

func (c *CLIListener) readInputLoop(ctx context.Context, out chan<- inputEvent) {
	defer close(out)
	for {
		line, err := c.readLine(ctx)
		select {
		case out <- inputEvent{line: line, err: err}:
		case <-ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

func (c *CLIListener) setupApprovalChannels() (chan approvalInputRequest, chan struct{}) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	reqCh := make(chan approvalInputRequest)
	doneCh := make(chan struct{})
	c.approvalReq = reqCh
	c.listenDoneCh = doneCh
	return reqCh, doneCh
}

func (c *CLIListener) teardownApprovalChannels(reqCh chan approvalInputRequest, doneCh chan struct{}) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	if c.approvalReq == reqCh {
		c.approvalReq = nil
	}
	if c.listenDoneCh == doneCh {
		close(doneCh)
		c.listenDoneCh = nil
	}
}

func (c *CLIListener) approvalChannels() (chan approvalInputRequest, chan struct{}) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	return c.approvalReq, c.listenDoneCh
}

type approvalInputRequest struct {
	prompt   string
	response chan approvalInputResponse
}

type approvalInputResponse struct {
	line string
	err  error
}

type inputEvent struct {
	line string
	err  error
}

func newReadline(in io.Reader, out io.Writer) (*readline.Instance, error) {
	stdin, ok := in.(io.ReadCloser)
	if !ok {
		return nil, fmt.Errorf("stdin is not read-closer")
	}
	inFile, ok := in.(*os.File)
	if !ok || !term.IsTerminal(int(inFile.Fd())) {
		return nil, fmt.Errorf("stdin is not terminal")
	}
	outFile, ok := out.(*os.File)
	if !ok || !term.IsTerminal(int(outFile.Fd())) {
		return nil, fmt.Errorf("stdout is not terminal")
	}

	return readline.NewEx(&readline.Config{
		Prompt:          defaultReplPrompt,
		HistoryFile:     filepath.Join(os.TempDir(), ".betterclaw_history"),
		HistoryLimit:    200,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		Stdin:           stdin,
		Stdout:          out,
		Stderr:          out,
	})
}
