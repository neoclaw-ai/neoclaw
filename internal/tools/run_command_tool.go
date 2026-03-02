package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/neoclaw-ai/neoclaw/internal/config"
	"github.com/neoclaw-ai/neoclaw/internal/logging"
)

// RunCommandTool executes shell commands within the configured workspace.
type RunCommandTool struct {
	WorkspaceDir string
	Timeout      time.Duration
	SecurityMode string
	ProxyAddress string
}

// Name returns the tool name.
func (t RunCommandTool) Name() string {
	return "run_command"
}

// Description returns the tool description for the model.
func (t RunCommandTool) Description() string {
	return "Run a shell command"
}

// Schema returns the JSON schema for run_command args.
func (t RunCommandTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "Shell command to execute",
			},
			"workdir": map[string]any{
				"type":        "string",
				"description": "Optional working directory relative to workspace or absolute under workspace",
			},
		},
		"required": []string{"command"},
	}
}

// Permission declares default permission behavior for this tool.
func (t RunCommandTool) Permission() Permission {
	return RequiresApproval
}

// SummarizeArgs returns a concise approval prompt summary for run_command.
func (t RunCommandTool) SummarizeArgs(args map[string]any) string {
	raw, ok := args["command"]
	if !ok {
		return "run_command: <empty>"
	}
	command, ok := raw.(string)
	if !ok || strings.TrimSpace(command) == "" {
		return "run_command: <empty>"
	}
	return fmt.Sprintf("run_command: %s", command)
}

// Execute runs the command and returns combined output, appending exit code on failures.
func (t RunCommandTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	command, workdir, err := t.validateArgs(args)
	if err != nil {
		return nil, err
	}

	timeout := t.Timeout
	if timeout <= 0 {
		return nil, errors.New("command timeout must be greater than zero")
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "sh", "-c", command)
	cmd.Dir = workdir
	cmd.Env = t.commandEnv()
	configureCommandForCancellation(cmd)
	combinedOut, runErr := cmd.CombinedOutput()
	// Critical security control: kill the process group after Wait so background children cannot outlive the shell and race policy flush.
	if err := killCommandProcessGroup(cmd); err != nil {
		logging.Logger().Warn("failed to kill command process group after command exit", "err", err)
	}
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		switch {
		case errors.Is(runCtx.Err(), context.Canceled):
			return nil, context.Canceled
		case errors.Is(runCtx.Err(), context.DeadlineExceeded):
			exitCode = 124
		case errors.As(runErr, &exitErr):
			exitCode = exitErr.ExitCode()
		default:
			return nil, fmt.Errorf("execute command: %w", runErr)
		}
	}

	output := string(combinedOut)
	if exitCode != 0 {
		if strings.TrimSpace(output) == "" {
			output = fmt.Sprintf("[exit code: %d]", exitCode)
		} else {
			if !strings.HasSuffix(output, "\n") {
				output += "\n"
			}
			output += fmt.Sprintf("[exit code: %d]", exitCode)
		}
	}

	return &ToolResult{Output: output}, nil
}

// commandEnv returns subprocess environment with proxy settings for non-danger modes.
func (t RunCommandTool) commandEnv() []string {
	env := os.Environ()
	if strings.EqualFold(strings.TrimSpace(t.SecurityMode), config.SecurityModeDanger) {
		return env
	}
	proxyAddress := strings.TrimSpace(t.ProxyAddress)
	if proxyAddress == "" {
		return env
	}
	env = append(env,
		"HTTP_PROXY="+proxyAddress,
		"HTTPS_PROXY="+proxyAddress,
		"http_proxy="+proxyAddress,
		"https_proxy="+proxyAddress,
		"NO_PROXY=",
		"no_proxy=",
	)
	return env
}

// validateArgs validates command args and resolves working directory.
func (t RunCommandTool) validateArgs(args map[string]any) (string, string, error) {
	command, err := stringArg(args, "command")
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(t.WorkspaceDir) == "" {
		return "", "", errors.New("workspace directory is required")
	}

	workdir := t.WorkspaceDir
	if raw, ok := args["workdir"]; ok {
		value, ok := raw.(string)
		if !ok {
			return "", "", fmt.Errorf("argument %s must be a string", "workdir")
		}
		value = strings.TrimSpace(value)
		if value != "" {
			wd, err := resolveWorkspacePath(t.WorkspaceDir, value)
			if err != nil {
				return "", "", err
			}
			workdir = wd
		}
	}

	return command, workdir, nil
}
