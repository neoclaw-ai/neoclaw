package approval

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/machinae/betterclaw/internal/tools"
)

func TestExecuteTool_AutoApproveSkipsApprover(t *testing.T) {
	tool := fakeTool{permission: tools.AutoApprove, output: "done"}
	res, err := ExecuteTool(context.Background(), nil, tool, map[string]any{"k": "v"}, "")
	if err != nil {
		t.Fatalf("execute tool: %v", err)
	}
	if res.Output != "done" {
		t.Fatalf("expected output done, got %q", res.Output)
	}
}

func TestExecuteTool_RequiresApprovalApprovedPath(t *testing.T) {
	appr := &fakeApprover{decision: Approved}
	tool := fakeTool{permission: tools.RequiresApproval, output: "done"}
	res, err := ExecuteTool(context.Background(), appr, tool, map[string]any{"cmd": "ls"}, "Run: ls")
	if err != nil {
		t.Fatalf("execute tool: %v", err)
	}
	if res.Output != "done" {
		t.Fatalf("expected output done, got %q", res.Output)
	}
	if appr.lastReq.Tool != "run_command" {
		t.Fatalf("expected approval request for run_command, got %q", appr.lastReq.Tool)
	}
	if appr.lastReq.Description != "Run: ls" {
		t.Fatalf("expected description to round trip")
	}
}

func TestExecuteTool_RequiresApprovalDeniedPath(t *testing.T) {
	appr := &fakeApprover{decision: Denied}
	tool := fakeTool{permission: tools.RequiresApproval, output: "done"}
	_, err := ExecuteTool(context.Background(), appr, tool, nil, "Run: rm -rf /")
	if err == nil {
		t.Fatalf("expected denial error")
	}
	if !strings.Contains(err.Error(), "denied by user") {
		t.Fatalf("expected denied by user error, got %v", err)
	}
}

func TestExecuteTool_RequiresApprovalApproverError(t *testing.T) {
	appr := &fakeApprover{err: errors.New("timeout")}
	tool := fakeTool{permission: tools.RequiresApproval, output: "done"}
	_, err := ExecuteTool(context.Background(), appr, tool, nil, "Run: ls")
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected approver error, got %v", err)
	}
}

func TestExecuteTool_RequiresApprovalMissingApprover(t *testing.T) {
	tool := fakeTool{permission: tools.RequiresApproval, output: "done"}
	_, err := ExecuteTool(context.Background(), nil, tool, nil, "Run: ls")
	if err == nil || !strings.Contains(err.Error(), "requires approval") {
		t.Fatalf("expected missing approver error, got %v", err)
	}
}

type fakeTool struct {
	permission tools.Permission
	output     string
}

func (t fakeTool) Name() string                 { return "run_command" }
func (t fakeTool) Description() string          { return "run command" }
func (t fakeTool) Schema() map[string]any       { return map[string]any{"type": "object"} }
func (t fakeTool) Permission() tools.Permission { return t.permission }
func (t fakeTool) Execute(_ context.Context, _ map[string]any) (*tools.ToolResult, error) {
	return &tools.ToolResult{Output: t.output}, nil
}

type fakeApprover struct {
	decision ApprovalDecision
	err      error
	lastReq  ApprovalRequest
}

func (f *fakeApprover) RequestApproval(_ context.Context, req ApprovalRequest) (ApprovalDecision, error) {
	f.lastReq = req
	if f.err != nil {
		return Denied, f.err
	}
	return f.decision, nil
}
