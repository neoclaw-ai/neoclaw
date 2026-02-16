package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const defaultCommandTimeout = 5 * time.Minute

type RunCommandTool struct {
	WorkspaceDir    string
	AllowedBinsPath string
	Timeout         time.Duration
}

func (t RunCommandTool) Name() string {
	return "run_command"
}

func (t RunCommandTool) Description() string {
	return "Run a shell command"
}

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

func (t RunCommandTool) Permission() Permission {
	return RequiresApproval
}

func (t RunCommandTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	command, err := stringArg(args, "command")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(t.WorkspaceDir) == "" {
		return nil, errors.New("workspace directory is required")
	}

	bin, err := firstCommandToken(command)
	if err != nil {
		return nil, err
	}
	if err := checkAllowedBinary(t.AllowedBinsPath, bin); err != nil {
		return nil, err
	}

	workdir := t.WorkspaceDir
	if raw, ok := args["workdir"]; ok {
		value, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("argument %q must be a string", "workdir")
		}
		value = strings.TrimSpace(value)
		if value != "" {
			wd, err := resolveWorkspacePath(t.WorkspaceDir, value)
			if err != nil {
				return nil, err
			}
			workdir = wd
		}
	}

	timeout := t.Timeout
	if timeout <= 0 {
		timeout = defaultCommandTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "bash", "-lc", command)
	cmd.Dir = workdir
	combinedOut, runErr := cmd.CombinedOutput()
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		switch {
		case errors.Is(runCtx.Err(), context.DeadlineExceeded):
			// Use 124 for timeout to match common shell timeout conventions.
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

	result := &ToolResult{Output: output}
	truncated, err := TruncateOutput(result.Output)
	if err != nil {
		return nil, err
	}
	result.Output = truncated.Output
	result.Truncated = truncated.Truncated
	result.FullOutputPath = truncated.FullOutputPath

	return result, nil
}

func firstCommandToken(command string) (string, error) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "", fmt.Errorf("argument %q cannot be empty", "command")
	}

	for _, token := range fields {
		if isEnvAssignment(token) {
			continue
		}
		return token, nil
	}

	return "", fmt.Errorf("argument %q cannot be empty", "command")
}

func isEnvAssignment(token string) bool {
	idx := strings.Index(token, "=")
	if idx <= 0 {
		return false
	}
	name := token[:idx]
	for i, r := range name {
		if r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			continue
		}
		if i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func checkAllowedBinary(allowedBinsPath, bin string) error {
	if strings.TrimSpace(allowedBinsPath) == "" {
		return fmt.Errorf("allowed bins path is required")
	}

	raw, err := os.ReadFile(allowedBinsPath)
	if err != nil {
		return fmt.Errorf("read allowed bins file: %w", err)
	}

	var allowed []string
	if err := json.Unmarshal(raw, &allowed); err != nil {
		return fmt.Errorf("parse allowed bins file: %w", err)
	}

	target := filepath.Base(strings.TrimSpace(bin))
	for _, candidate := range allowed {
		if filepath.Base(strings.TrimSpace(candidate)) == target {
			return nil
		}
	}

	return fmt.Errorf("binary %q is not allowed", target)
}
