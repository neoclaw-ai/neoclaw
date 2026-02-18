// Package channels provides runtime.Listener implementations for each supported input channel (CLI, and future Telegram, Discord, etc.).
package channels

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/runtime"
	"golang.org/x/term"
)

const defaultReplPrompt = "you> "

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
	for {
		line, err := c.readLine(ctx)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}
		switch strings.ToLower(input) {
		case "/quit", "quit", "/exit", "exit":
			return nil
		}

		if err := handler.HandleMessage(ctx, writer, &runtime.Message{Text: input}); err != nil {
			return err
		}
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

	prompt := fmt.Sprintf("approve tool %s? %s [y]es/[n]o/[a]lways: ", req.Tool, req.Description)

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

	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return approval.Approved, nil
	case "a", "always":
		return approval.AlwaysApproved, nil
	default:
		return approval.Denied, nil
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
