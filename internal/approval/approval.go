// Package approval defines the Approver interface and enforces permission checks before tool execution.
package approval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/logging"
	"github.com/machinae/betterclaw/internal/store"
	"github.com/machinae/betterclaw/internal/tools"
)

// Approver requests and returns user approval decisions.
type Approver interface {
	RequestApproval(ctx context.Context, req ApprovalRequest) (ApprovalDecision, error)
}

// ApprovalRequest describes a single approval prompt request.
type ApprovalRequest struct {
	Tool        string
	Description string
	Args        map[string]any
}

// ApprovalDecision is the user's decision for an approval request.
type ApprovalDecision int

const (
	// Approved indicates the action is approved and may be persisted by policy.
	Approved ApprovalDecision = iota
	// Denied indicates the action was explicitly rejected.
	Denied
)

type commandPolicy struct {
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
}

// ExecuteTool enforces permission checks and executes the tool when allowed.
func ExecuteTool(ctx context.Context, approver Approver, tool tools.Tool, args map[string]any, description string) (*tools.ToolResult, error) {
	permission := tool.Permission()

	if permission == tools.RequiresApproval && tool.Name() == "run_command" {
		permissionForRunCommand, err := resolveRunCommandPermission(ctx, approver, tool, args, description)
		if err != nil {
			return nil, err
		}
		permission = permissionForRunCommand
	}

	if permission == tools.RequiresApproval {
		if approver == nil {
			return nil, fmt.Errorf("tool %q requires approval but no approver is configured", tool.Name())
		}
		decision, err := approver.RequestApproval(ctx, ApprovalRequest{
			Tool:        tool.Name(),
			Description: description,
			Args:        args,
		})
		if err != nil {
			return nil, err
		}
		if decision == Denied {
			return nil, toolDeniedError(tool.Name())
		}
	}

	return tool.Execute(ctx, args)
}

// Resolve run_command permission by matching against persisted allow/deny patterns.
func resolveRunCommandPermission(
	ctx context.Context,
	approver Approver,
	tool tools.Tool,
	args map[string]any,
	description string,
) (tools.Permission, error) {
	command, err := commandArg(args)
	if err != nil {
		return tools.RequiresApproval, err
	}

	path, err := allowedCommandsPath()
	if err != nil {
		return tools.RequiresApproval, err
	}

	policy, err := loadCommandPolicy(path)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		policy = commandPolicy{}
	default:
		return tools.RequiresApproval, err
	}

	switch evaluateCommandPatterns(command, policy.Allow, policy.Deny) {
	case commandAllowed:
		return tools.AutoApprove, nil
	case commandDenied:
		return tools.RequiresApproval, toolDeniedError(tool.Name())
	case commandNoMatch:
		return promptForRunCommandPolicy(ctx, approver, tool.Name(), args, description, path, policy, command)
	default:
		return tools.RequiresApproval, nil
	}
}

// Prompt for run_command policy decision and persist allow/deny pattern.
func promptForRunCommandPolicy(
	ctx context.Context,
	approver Approver,
	toolName string,
	args map[string]any,
	description string,
	path string,
	policy commandPolicy,
	command string,
) (tools.Permission, error) {
	if approver == nil {
		return tools.RequiresApproval, fmt.Errorf("tool %q requires approval but no approver is configured", toolName)
	}

	pattern, ok := generateCommandPattern(command)
	if !ok {
		pattern = strings.TrimSpace(command)
	}

	prompt := description
	if pattern != "" {
		prompt = fmt.Sprintf("Allow Command: %s", pattern)
	}

	decision, err := approver.RequestApproval(ctx, ApprovalRequest{
		Tool:        toolName,
		Description: prompt,
		Args:        args,
	})
	if err != nil {
		return tools.RequiresApproval, err
	}

	switch decision {
	case Approved:
		if pattern != "" {
			policy.Allow = appendUnique(policy.Allow, pattern)
			if err := saveCommandPolicy(path, policy); err != nil {
				logging.Logger().Warn(
					"failed to persist command allow pattern",
					"pattern", pattern,
					"err", err,
				)
			}
		}
		return tools.AutoApprove, nil
	case Denied:
		if pattern != "" {
			policy.Deny = appendUnique(policy.Deny, pattern)
			if err := saveCommandPolicy(path, policy); err != nil {
				logging.Logger().Warn(
					"failed to persist command deny pattern",
					"pattern", pattern,
					"err", err,
				)
			}
		}
		return tools.RequiresApproval, toolDeniedError(toolName)
	default:
		return tools.RequiresApproval, toolDeniedError(toolName)
	}
}

// Resolve the allowed_commands.json path from config.
func allowedCommandsPath() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	return cfg.AllowedCommandsPath(), nil
}

// Load command policy from disk.
func loadCommandPolicy(path string) (commandPolicy, error) {
	raw, err := store.ReadFile(path)
	if err != nil {
		return commandPolicy{}, err
	}
	if strings.TrimSpace(raw) == "" {
		return commandPolicy{}, nil
	}

	var policy commandPolicy
	if err := json.Unmarshal([]byte(raw), &policy); err != nil {
		return commandPolicy{}, fmt.Errorf("decode command policy %q: %w", path, err)
	}
	return policy, nil
}

// Save command policy to disk.
func saveCommandPolicy(path string, policy commandPolicy) error {
	encoded, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return fmt.Errorf("encode command policy: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := store.WriteFile(path, encoded); err != nil {
		return fmt.Errorf("write command policy: %w", err)
	}
	return nil
}

// Extract a non-empty command argument from tool args.
func commandArg(args map[string]any) (string, error) {
	raw, ok := args["command"]
	if !ok {
		return "", fmt.Errorf("missing required argument %q", "command")
	}
	command, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("argument %q must be a string", "command")
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("argument %q cannot be empty", "command")
	}
	return command, nil
}

// Append only values that are not already present.
func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// Build standard user-facing denial guidance.
func toolDeniedError(toolName string) error {
	return fmt.Errorf(
		"user denied tool %q. User denied this action. Try a different approach or ask the user for guidance",
		toolName,
	)
}
