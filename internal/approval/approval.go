// Package approval defines the Approver interface and enforces permission checks before tool execution.
package approval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/neoclaw-ai/neoclaw/internal/config"
	"github.com/neoclaw-ai/neoclaw/internal/logging"
	"github.com/neoclaw-ai/neoclaw/internal/store"
	"github.com/neoclaw-ai/neoclaw/internal/tools"
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

type policyPaths struct {
	commands string
	domains  string
	users    string
}

var (
	policyCacheMu      sync.Mutex
	commandPolicyCache = map[string]commandPolicy{}
	domainPolicyCache  = map[string]domainPolicy{}
	usersPolicyCache   = map[string]UsersFile{}
)

// ExecuteTool enforces permission checks and executes the tool when allowed.
func ExecuteTool(ctx context.Context, approver Approver, tool tools.Tool, args map[string]any, description string) (*tools.ToolResult, error) {
	// In danger mode we bypass all approval and policy checks for tool execution.
	if isDangerMode() {
		return tool.Execute(ctx, args)
	}

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
			return nil, fmt.Errorf("tool %s requires approval but no approver is configured", tool.Name())
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

	result, execErr := tool.Execute(ctx, args)
	if tool.Name() != "run_command" || !shouldFlushPolicies() {
		return result, execErr
	}

	if flushErr := FlushPolicies(); flushErr != nil {
		if execErr != nil {
			return result, errors.Join(execErr, flushErr)
		}
		return result, flushErr
	}
	return result, execErr
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

	paths, err := currentPolicyPaths()
	if err != nil {
		return tools.RequiresApproval, err
	}

	if err := ensurePolicyCacheLoaded(paths); err != nil {
		return tools.RequiresApproval, err
	}

	policy, err := loadCachedCommandPolicy(paths.commands)
	if err != nil {
		return tools.RequiresApproval, err
	}

	switch evaluateCommandPatterns(command, policy.Allow, policy.Deny) {
	case commandAllowed:
		return tools.AutoApprove, nil
	case commandDenied:
		return tools.RequiresApproval, toolDeniedError(tool.Name())
	case commandNoMatch:
		return promptForRunCommandPolicy(ctx, approver, tool.Name(), args, description, paths.commands, policy, command)
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
		return tools.RequiresApproval, fmt.Errorf("tool %s requires approval but no approver is configured", toolName)
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
			if err := saveCachedCommandPolicy(path, policy); err != nil {
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
			if err := saveCachedCommandPolicy(path, policy); err != nil {
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

// FlushPolicies rewrites in-memory policy state back to disk.
func FlushPolicies() error {
	paths, err := currentPolicyPaths()
	if err != nil {
		return err
	}
	if err := ensurePolicyCacheLoaded(paths); err != nil {
		return err
	}

	commandPolicy, err := loadCachedCommandPolicy(paths.commands)
	if err != nil {
		return err
	}
	domainPolicy, err := loadCachedDomainPolicy(paths.domains)
	if err != nil {
		return err
	}
	usersPolicy, err := loadCachedUsersFile(paths.users)
	if err != nil {
		return err
	}

	flushErr := saveCommandPolicy(paths.commands, commandPolicy)
	flushErr = errors.Join(flushErr, saveDomainPolicy(paths.domains, domainPolicy))
	flushErr = errors.Join(flushErr, saveUsers(paths.users, usersPolicy))
	if flushErr != nil {
		return fmt.Errorf("flush policies: %w", flushErr)
	}
	return nil
}

// Resolve policy file paths from config.
func currentPolicyPaths() (policyPaths, error) {
	cfg, err := config.Load()
	if err != nil {
		return policyPaths{}, fmt.Errorf("load config: %w", err)
	}
	return policyPaths{
		commands: cfg.AllowedCommandsPath(),
		domains:  cfg.AllowedDomainsPath(),
		users:    cfg.AllowedUsersPath(),
	}, nil
}

// Determine whether run_command policy flush should run.
func shouldFlushPolicies() bool {
	return !isDangerMode()
}

// isDangerMode reports whether security.mode is configured as danger.
func isDangerMode() bool {
	cfg, err := config.Load()
	if err != nil {
		logging.Logger().Warn("failed to load config for security mode check", "err", err)
		return false
	}
	return strings.EqualFold(strings.TrimSpace(cfg.Security.Mode), config.SecurityModeDanger)
}

// Ensure both command and domain policy are loaded into in-memory cache.
func ensurePolicyCacheLoaded(paths policyPaths) error {
	if _, err := loadCachedCommandPolicy(paths.commands); err != nil {
		return err
	}
	if _, err := loadCachedDomainPolicy(paths.domains); err != nil {
		return err
	}
	if _, err := loadCachedUsersFile(paths.users); err != nil {
		return err
	}
	return nil
}

// Load command policy from in-memory cache, lazy-loading from disk once.
func loadCachedCommandPolicy(path string) (commandPolicy, error) {
	policyCacheMu.Lock()
	defer policyCacheMu.Unlock()

	if policy, ok := commandPolicyCache[path]; ok {
		return cloneCommandPolicy(policy), nil
	}

	policy, err := loadCommandPolicy(path)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		policy = commandPolicy{}
	default:
		return commandPolicy{}, err
	}
	commandPolicyCache[path] = cloneCommandPolicy(policy)
	return cloneCommandPolicy(policy), nil
}

// Persist command policy and update in-memory cache.
func saveCachedCommandPolicy(path string, policy commandPolicy) error {
	copied := cloneCommandPolicy(policy)

	policyCacheMu.Lock()
	commandPolicyCache[path] = copied
	policyCacheMu.Unlock()

	if err := saveCommandPolicy(path, copied); err != nil {
		return err
	}
	return nil
}

// Load domain policy from in-memory cache, lazy-loading from disk once.
func loadCachedDomainPolicy(path string) (domainPolicy, error) {
	policyCacheMu.Lock()
	defer policyCacheMu.Unlock()

	if policy, ok := domainPolicyCache[path]; ok {
		return cloneDomainPolicy(policy), nil
	}

	policy, err := loadDomainPolicy(path)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		policy = domainPolicy{}
	default:
		return domainPolicy{}, err
	}
	domainPolicyCache[path] = cloneDomainPolicy(policy)
	return cloneDomainPolicy(policy), nil
}

// Persist domain policy and update in-memory cache.
func saveCachedDomainPolicy(path string, policy domainPolicy) error {
	copied := cloneDomainPolicy(policy)

	policyCacheMu.Lock()
	domainPolicyCache[path] = copied
	policyCacheMu.Unlock()

	if err := saveDomainPolicy(path, copied); err != nil {
		return err
	}
	return nil
}

// Load allowed users from in-memory cache, lazy-loading from disk once.
func loadCachedUsersFile(path string) (UsersFile, error) {
	policyCacheMu.Lock()
	defer policyCacheMu.Unlock()

	if usersFile, ok := usersPolicyCache[path]; ok {
		return cloneUsersFile(usersFile), nil
	}

	usersFile, err := LoadUsers(path)
	if err != nil {
		return UsersFile{}, err
	}
	usersPolicyCache[path] = cloneUsersFile(usersFile)
	return cloneUsersFile(usersFile), nil
}

// Persist allowed users and update in-memory cache.
func saveCachedUsersFile(path string, usersFile UsersFile) error {
	copied := cloneUsersFile(usersFile)

	policyCacheMu.Lock()
	usersPolicyCache[path] = copied
	policyCacheMu.Unlock()

	if err := saveUsers(path, copied); err != nil {
		return err
	}
	return nil
}

// Copy command policy slices before returning/storing.
func cloneCommandPolicy(policy commandPolicy) commandPolicy {
	return commandPolicy{
		Allow: append([]string(nil), policy.Allow...),
		Deny:  append([]string(nil), policy.Deny...),
	}
}

// Copy domain policy slices before returning/storing.
func cloneDomainPolicy(policy domainPolicy) domainPolicy {
	return domainPolicy{
		Allow: append([]string(nil), policy.Allow...),
		Deny:  append([]string(nil), policy.Deny...),
	}
}

// Copy users list before returning/storing.
func cloneUsersFile(usersFile UsersFile) UsersFile {
	return UsersFile{
		Users: append([]User(nil), usersFile.Users...),
	}
}

// Reset in-memory policy cache state.
func resetPolicyCache() {
	policyCacheMu.Lock()
	defer policyCacheMu.Unlock()
	commandPolicyCache = map[string]commandPolicy{}
	domainPolicyCache = map[string]domainPolicy{}
	usersPolicyCache = map[string]UsersFile{}
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
		return commandPolicy{}, fmt.Errorf("decode command policy %s: %w", path, err)
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
		return "", fmt.Errorf("missing required argument %s", "command")
	}
	command, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("argument %s must be a string", "command")
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("argument %s cannot be empty", "command")
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
		"user denied tool %s. User denied this action. Try a different approach or ask the user for guidance",
		toolName,
	)
}
