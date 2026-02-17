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

// RunCommandTool executes shell commands within the configured workspace.
type RunCommandTool struct {
	WorkspaceDir    string
	AllowedBinsPath string
	Timeout         time.Duration
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

// RequiresApprovalForArgs resolves approval behavior for this specific command.
// Allowlisted binaries are auto-approved; all others require an approval prompt.
func (t RunCommandTool) RequiresApprovalForArgs(args map[string]any) (bool, error) {
	command, _, err := t.validateArgs(args)
	if err != nil {
		return true, err
	}

	bin, err := firstCommandToken(command)
	if err != nil {
		return true, err
	}

	if isAllowedBinary(t.AllowedBinsPath, bin) {
		return false, nil
	}
	return true, nil
}

// Execute runs the command and returns combined output, appending exit code on failures.
func (t RunCommandTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	command, workdir, err := t.validateArgs(args)
	if err != nil {
		return nil, err
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
			return "", "", fmt.Errorf("argument %q must be a string", "workdir")
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

func firstCommandToken(command string) (string, error) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "", fmt.Errorf("argument %q cannot be empty", "command")
	}

	for _, token := range fields {
		// Allow leading VAR=value prefixes and return the first actual command token.
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

func isAllowedBinary(allowedBinsPath, bin string) bool {
	if strings.TrimSpace(allowedBinsPath) == "" {
		return false
	}

	// Load on each check so edits to allowed_bins.json take effect immediately
	// without restarting the process.
	raw, err := os.ReadFile(allowedBinsPath)
	if err != nil {
		return false
	}

	var allowed []string
	if err := json.Unmarshal(raw, &allowed); err != nil {
		return false
	}

	target := filepath.Base(strings.TrimSpace(bin))
	for _, candidate := range allowed {
		if filepath.Base(strings.TrimSpace(candidate)) == target {
			return true
		}
	}

	return false
}
