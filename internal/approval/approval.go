package approval

import (
	"context"
	"fmt"

	"github.com/machinae/betterclaw/internal/logging"
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
	// Approved indicates approval for this invocation only.
	Approved ApprovalDecision = iota
	// AlwaysApproved indicates the action is approved and the decision should be persisted
	// so future invocations of the same binary/domain are auto-approved.
	AlwaysApproved
	// Denied indicates the action was explicitly rejected.
	Denied
)

// ExecuteTool enforces permission checks and executes the tool when allowed.
func ExecuteTool(ctx context.Context, approver Approver, tool tools.Tool, args map[string]any, description string) (*tools.ToolResult, error) {
	permission := tool.Permission()
	if permission == tools.RequiresApproval {
		if conditional, ok := tool.(tools.ConditionalApprover); ok {
			requiresApproval, err := conditional.RequiresApprovalForArgs(args)
			if err != nil {
				return nil, err
			}
			if !requiresApproval {
				permission = tools.AutoApprove
			}
		}
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
			// Return guidance text that helps the model recover on the next turn.
			return nil, fmt.Errorf(
				"user denied tool %q. User denied this action. Try a different approach or ask the user for guidance",
				tool.Name(),
			)
		}
		if decision == AlwaysApproved {
			if persister, ok := tool.(tools.ApprovalPersister); ok {
				if err := persister.PersistAlwaysApproval(args); err != nil {
					logging.Logger().Warn(
						"failed to persist always approval",
						"tool", tool.Name(),
						"err", err,
					)
				}
			}
		}
	}

	return tool.Execute(ctx, args)
}
