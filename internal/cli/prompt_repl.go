package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
	"github.com/machinae/betterclaw/internal/agent"
	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/llm"
	"github.com/machinae/betterclaw/internal/tools"
	"golang.org/x/term"
)

const (
	defaultSystemPrompt = "You are BetterClaw, a lightweight personal AI assistant."
	defaultReplPrompt   = "you> "
)

type promptRunner struct {
	provider     llm.Provider
	registry     *tools.Registry
	approver     approval.Approver
	systemPrompt string
	maxIter      int
	history      []llm.ChatMessage
}

func newPromptRunner(cfg *config.Config, provider llm.Provider, approver approval.Approver, out io.Writer) (*promptRunner, error) {
	registry := tools.NewRegistry()
	coreTools := []tools.Tool{
		tools.ReadFileTool{WorkspaceDir: cfg.WorkspaceDir()},
		tools.ListDirTool{WorkspaceDir: cfg.WorkspaceDir()},
		tools.WriteFileTool{WorkspaceDir: cfg.WorkspaceDir()},
		tools.RunCommandTool{
			WorkspaceDir:    cfg.WorkspaceDir(),
			AllowedBinsPath: filepath.Join(cfg.DataDir, "allowed_bins.json"),
			Timeout:         cfg.Security.CommandTimeout,
		},
		tools.SendMessageTool{Writer: out},
	}
	for _, tool := range coreTools {
		if err := registry.Register(tool); err != nil {
			return nil, err
		}
	}

	return &promptRunner{
		provider:     provider,
		registry:     registry,
		approver:     approver,
		systemPrompt: defaultSystemPrompt,
		maxIter:      10,
	}, nil
}

func (r *promptRunner) Send(ctx context.Context, prompt string) (string, error) {
	messages := append(append([]llm.ChatMessage{}, r.history...), llm.ChatMessage{
		Role:    llm.RoleUser,
		Content: prompt,
	})

	resp, history, err := agent.Run(
		ctx,
		r.provider,
		r.registry,
		r.approver,
		r.systemPrompt,
		messages,
		r.maxIter,
	)
	if err != nil {
		return "", err
	}
	r.history = history
	return resp.Content, nil
}

type promptChannel interface {
	Read(ctx context.Context) (string, error)
	Write(ctx context.Context, text string) error
	WriteMeta(ctx context.Context, text string) error
}

type readlinePromptChannel struct {
	rl  *readline.Instance
	out io.Writer
}

func newReadlinePromptChannel(in io.Reader, out io.Writer) (*readlinePromptChannel, error) {
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

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          defaultReplPrompt,
		HistoryFile:     filepath.Join(os.TempDir(), ".betterclaw_history"),
		HistoryLimit:    200,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		Stdin:           stdin,
		Stdout:          out,
		Stderr:          out,
	})
	if err != nil {
		return nil, err
	}
	return &readlinePromptChannel{rl: rl, out: out}, nil
}

func (c *readlinePromptChannel) Read(_ context.Context) (string, error) {
	line, err := c.rl.Readline()
	if err != nil {
		if err == readline.ErrInterrupt || err == io.EOF {
			return "", io.EOF
		}
		return "", err
	}
	return line, nil
}

func (c *readlinePromptChannel) Write(_ context.Context, text string) error {
	_, err := fmt.Fprintf(c.out, "assistant> %s\n\n", text)
	return err
}

func (c *readlinePromptChannel) WriteMeta(_ context.Context, text string) error {
	_, err := fmt.Fprintf(c.out, "%s\n", text)
	return err
}

func (c *readlinePromptChannel) Close() error {
	return c.rl.Close()
}

type stdioPromptChannel struct {
	in     *bufio.Reader
	out    io.Writer
	prompt string
}

func newStdioPromptChannel(in *bufio.Reader, out io.Writer) *stdioPromptChannel {
	return &stdioPromptChannel{
		in:     in,
		out:    out,
		prompt: defaultReplPrompt,
	}
}

func (c *stdioPromptChannel) Read(_ context.Context) (string, error) {
	if _, err := fmt.Fprint(c.out, c.prompt); err != nil {
		return "", err
	}
	line, err := c.in.ReadString('\n')
	if err != nil {
		if len(line) > 0 {
			return line, nil
		}
		return "", err
	}
	return line, nil
}

func (c *stdioPromptChannel) Write(_ context.Context, text string) error {
	_, err := fmt.Fprintf(c.out, "assistant> %s\n\n", text)
	return err
}

func (c *stdioPromptChannel) WriteMeta(_ context.Context, text string) error {
	_, err := fmt.Fprintf(c.out, "%s\n", text)
	return err
}

func runPromptREPL(ctx context.Context, runner *promptRunner, in io.Reader, fallbackReader *bufio.Reader, out io.Writer) error {
	var channel promptChannel
	readlineChannel, err := newReadlinePromptChannel(in, out)
	if err == nil {
		channel = readlineChannel
	}
	if channel == nil {
		channel = newStdioPromptChannel(fallbackReader, out)
	}
	if closer, ok := any(channel).(io.Closer); ok {
		defer closer.Close()
	}

	return runPromptLoop(ctx, runner, channel)
}

func runPromptLoop(ctx context.Context, runner *promptRunner, channel promptChannel) error {
	if err := channel.WriteMeta(ctx, "Interactive mode. Type /quit or /exit to stop."); err != nil {
		return err
	}

	for {
		raw, err := channel.Read(ctx)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		input := strings.TrimSpace(raw)
		if input == "" {
			continue
		}
		switch strings.ToLower(input) {
		case "/quit", "quit", "/exit", "exit":
			return nil
		}

		resp, err := runner.Send(ctx, input)
		if err != nil {
			if writeErr := channel.WriteMeta(ctx, fmt.Sprintf("error: %v", err)); writeErr != nil {
				return writeErr
			}
			continue
		}
		if err := channel.Write(ctx, resp); err != nil {
			return err
		}
	}
}
