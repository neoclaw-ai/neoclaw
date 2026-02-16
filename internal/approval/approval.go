package approval

import (
	"context"
	"fmt"

	"github.com/machinae/betterclaw/internal/tools"
)

// Approver requests and returns user approval decisions.
type Approver interface {
	RequestApproval(ctx context.Context, req ApprovalRequest) (ApprovalDecision, error)
}

type ApprovalRequest struct {
	Tool        string
	Description string
	Args        map[string]any
}

type ApprovalDecision int

const (
	Approved ApprovalDecision = iota
	AlwaysApproved
	Denied
)

// ExecuteTool enforces permission checks and executes the tool when allowed.
func ExecuteTool(ctx context.Context, approver Approver, tool tools.Tool, args map[string]any, description string) (*tools.ToolResult, error) {
	permission := tool.Permission()
	if permission == tools.RequiresApproval {
		switch t := tool.(type) {
		case tools.RunCommandTool:
			requiresApproval, err := t.RequiresApprovalForArgs(args)
			if err != nil {
				return nil, err
			}
			if !requiresApproval {
				permission = tools.AutoApprove
			}
		case *tools.RunCommandTool:
			requiresApproval, err := t.RequiresApprovalForArgs(args)
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
			return nil, fmt.Errorf(
				"user denied tool %q. User denied this action. Try a different approach or ask the user for guidance",
				tool.Name(),
			)
		}
	}

	return tool.Execute(ctx, args)
}
