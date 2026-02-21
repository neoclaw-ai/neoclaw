package scheduler

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/machinae/betterclaw/internal/logging"
)

const schedulerOutputJobIDArg = "job_id"

// ActionRunners holds concrete per-action execution functions used by NewRunner.
type ActionRunners struct {
	SendMessage func(ctx context.Context, writer io.Writer, args map[string]any) (string, error)
	RunCommand  func(ctx context.Context, args map[string]any) (string, error)
	HTTPRequest func(ctx context.Context, args map[string]any) (string, error)
}

// Runner executes scheduler jobs by dispatching to action-specific handlers.
type Runner struct {
	sendMessage func(ctx context.Context, writer io.Writer, args map[string]any) (string, error)
	runCommand  func(ctx context.Context, args map[string]any) (string, error)
	httpRequest func(ctx context.Context, args map[string]any) (string, error)
	writers     map[string]io.Writer
}

// NewRunner constructs a scheduler runner from concrete action execution funcs.
func NewRunner(r ActionRunners, writers map[string]io.Writer) *Runner {
	return &Runner{
		sendMessage: r.SendMessage,
		runCommand:  r.RunCommand,
		httpRequest: r.HTTPRequest,
		writers:     writers,
	}
}

// Run executes one job action and returns tool output text.
func (r *Runner) Run(ctx context.Context, job Job) (string, error) {
	args := cloneArgs(job.Args)
	switch job.Action {
	case ActionSendMessage:
		if r.sendMessage == nil {
			return "", errors.New("send_message runner is not configured")
		}
		if r.writers == nil {
			return "", errors.New("send_message writers registry is not configured")
		}
		writer, ok := r.writers[job.ChannelID]
		if !ok {
			logging.Logger().Warn(
				"scheduled send_message skipped: unknown channel",
				"job_id", job.ID,
				"channel_id", job.ChannelID,
			)
			return "", nil
		}
		return r.sendMessage(ctx, writer, args)
	case ActionRunCommand:
		if r.runCommand == nil {
			return "", errors.New("run_command runner is not configured")
		}
		args[schedulerOutputJobIDArg] = job.ID
		return r.runCommand(ctx, args)
	case ActionHTTPRequest:
		if r.httpRequest == nil {
			return "", errors.New("http_request runner is not configured")
		}
		return r.httpRequest(ctx, args)
	default:
		return "", fmt.Errorf("unsupported action %q", job.Action)
	}
}
