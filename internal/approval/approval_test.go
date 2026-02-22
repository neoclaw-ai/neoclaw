package approval

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/machinae/betterclaw/internal/store"
	"github.com/machinae/betterclaw/internal/tools"
)

func TestExecuteTool_AutoApproveSkipsApprover(t *testing.T) {
	tool := fakeTool{name: "read_file", permission: tools.AutoApprove, output: "done"}
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
	tool := fakeTool{name: "write_file", permission: tools.RequiresApproval, output: "done"}
	res, err := ExecuteTool(context.Background(), appr, tool, map[string]any{"path": "notes.md"}, "Write file")
	if err != nil {
		t.Fatalf("execute tool: %v", err)
	}
	if res.Output != "done" {
		t.Fatalf("expected output done, got %q", res.Output)
	}
	if appr.lastReq.Tool != "write_file" {
		t.Fatalf("expected approval request for write_file, got %q", appr.lastReq.Tool)
	}
	if appr.lastReq.Description != "Write file" {
		t.Fatalf("expected description to round trip")
	}
}

func TestExecuteTool_RequiresApprovalDeniedPath(t *testing.T) {
	appr := &fakeApprover{decision: Denied}
	tool := fakeTool{name: "write_file", permission: tools.RequiresApproval, output: "done"}
	_, err := ExecuteTool(context.Background(), appr, tool, nil, "Write file")
	if err == nil {
		t.Fatalf("expected denial error")
	}
	if !strings.Contains(err.Error(), "User denied this action") {
		t.Fatalf("expected recovery guidance in denial error, got %v", err)
	}
}

func TestExecuteTool_RequiresApprovalApproverError(t *testing.T) {
	appr := &fakeApprover{err: errors.New("timeout")}
	tool := fakeTool{name: "write_file", permission: tools.RequiresApproval, output: "done"}
	_, err := ExecuteTool(context.Background(), appr, tool, nil, "Write file")
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected approver error, got %v", err)
	}
}

func TestExecuteTool_RequiresApprovalMissingApprover(t *testing.T) {
	tool := fakeTool{name: "write_file", permission: tools.RequiresApproval, output: "done"}
	_, err := ExecuteTool(context.Background(), nil, tool, nil, "Write file")
	if err == nil || !strings.Contains(err.Error(), "requires approval") {
		t.Fatalf("expected missing approver error, got %v", err)
	}
}

func TestExecuteTool_RunCommandAllowPatternAutoApproves(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("BETTERCLAW_HOME", dataDir)
	writeCommandPolicyFile(t, dataDir, commandPolicy{
		Allow: []string{"git status"},
		Deny:  nil,
	})

	appr := &fakeApprover{decision: Denied}
	tool := fakeTool{name: "run_command", permission: tools.RequiresApproval, output: "done"}
	res, err := ExecuteTool(context.Background(), appr, tool, map[string]any{"command": "git status"}, "Run: git status")
	if err != nil {
		t.Fatalf("execute tool: %v", err)
	}
	if res.Output != "done" {
		t.Fatalf("expected output done, got %q", res.Output)
	}
	if appr.calls != 0 {
		t.Fatalf("expected no prompt for allowlisted command, got %d prompts", appr.calls)
	}
}

func TestExecuteTool_RunCommandDenyPatternAutoDenies(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("BETTERCLAW_HOME", dataDir)
	writeCommandPolicyFile(t, dataDir, commandPolicy{
		Allow: []string{"git *"},
		Deny:  []string{"git push *"},
	})

	appr := &fakeApprover{decision: Approved}
	tool := fakeTool{name: "run_command", permission: tools.RequiresApproval, output: "done"}
	_, err := ExecuteTool(context.Background(), appr, tool, map[string]any{"command": "git push origin main"}, "Run: git push")
	if err == nil {
		t.Fatalf("expected deny error")
	}
	if appr.calls != 0 {
		t.Fatalf("expected no prompt for denylisted command, got %d prompts", appr.calls)
	}
}

func TestExecuteTool_RunCommandNoMatchPromptsWithPatternAndPersistsAllow(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("BETTERCLAW_HOME", dataDir)

	appr := &fakeApprover{decision: Approved}
	tool := fakeTool{name: "run_command", permission: tools.RequiresApproval, output: "done"}

	res, err := ExecuteTool(
		context.Background(),
		appr,
		tool,
		map[string]any{"command": `git commit -m "x"`},
		"Run: git commit -m \"x\"",
	)
	if err != nil {
		t.Fatalf("execute tool: %v", err)
	}
	if res.Output != "done" {
		t.Fatalf("expected output done, got %q", res.Output)
	}
	if appr.calls != 1 {
		t.Fatalf("expected one prompt, got %d", appr.calls)
	}
	if appr.lastReq.Description != "Allow Command: git commit *" {
		t.Fatalf("expected generated pattern prompt, got %q", appr.lastReq.Description)
	}

	policy := readCommandPolicyFile(t, dataDir)
	if !containsString(policy.Allow, "git commit *") {
		t.Fatalf("expected allow list to contain generated pattern, got %#v", policy.Allow)
	}
}

func TestExecuteTool_RunCommandNoMatchPromptsAndPersistsDeny(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("BETTERCLAW_HOME", dataDir)

	appr := &fakeApprover{decision: Denied}
	tool := fakeTool{name: "run_command", permission: tools.RequiresApproval, output: "done"}

	_, err := ExecuteTool(
		context.Background(),
		appr,
		tool,
		map[string]any{"command": `git commit -m "x"`},
		"Run: git commit -m \"x\"",
	)
	if err == nil {
		t.Fatalf("expected deny error")
	}
	if appr.calls != 1 {
		t.Fatalf("expected one prompt, got %d", appr.calls)
	}

	policy := readCommandPolicyFile(t, dataDir)
	if !containsString(policy.Deny, "git commit *") {
		t.Fatalf("expected deny list to contain generated pattern, got %#v", policy.Deny)
	}
}

func TestExecuteTool_RunCommandSubsequentInvocationUsesPersistedAllow(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("BETTERCLAW_HOME", dataDir)

	firstApprover := &fakeApprover{decision: Approved}
	tool := fakeTool{name: "run_command", permission: tools.RequiresApproval, output: "done"}
	_, err := ExecuteTool(
		context.Background(),
		firstApprover,
		tool,
		map[string]any{"command": "npm run build --watch"},
		"Run: npm run build --watch",
	)
	if err != nil {
		t.Fatalf("first execute tool: %v", err)
	}
	if firstApprover.calls != 1 {
		t.Fatalf("expected initial prompt")
	}

	secondApprover := &fakeApprover{decision: Denied}
	_, err = ExecuteTool(
		context.Background(),
		secondApprover,
		tool,
		map[string]any{"command": "npm run build --watch"},
		"Run: npm run build --watch",
	)
	if err != nil {
		t.Fatalf("second execute should auto-approve via persisted pattern, got %v", err)
	}
	if secondApprover.calls != 0 {
		t.Fatalf("expected no second prompt, got %d", secondApprover.calls)
	}
}

func TestExecuteTool_RunCommandNoMatchMissingApprover(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("BETTERCLAW_HOME", dataDir)

	tool := fakeTool{name: "run_command", permission: tools.RequiresApproval, output: "done"}
	_, err := ExecuteTool(context.Background(), nil, tool, map[string]any{"command": "python3 script.py"}, "Run: python3 script.py")
	if err == nil || !strings.Contains(err.Error(), "requires approval") {
		t.Fatalf("expected missing approver error, got %v", err)
	}
}

type fakeTool struct {
	name       string
	permission tools.Permission
	output     string
}

func (t fakeTool) Name() string           { return t.name }
func (t fakeTool) Description() string    { return "tool" }
func (t fakeTool) Schema() map[string]any { return map[string]any{"type": "object"} }
func (t fakeTool) Permission() tools.Permission {
	return t.permission
}
func (t fakeTool) Execute(_ context.Context, _ map[string]any) (*tools.ToolResult, error) {
	return &tools.ToolResult{Output: t.output}, nil
}

type fakeApprover struct {
	decision ApprovalDecision
	err      error
	calls    int
	lastReq  ApprovalRequest
}

func (f *fakeApprover) RequestApproval(_ context.Context, req ApprovalRequest) (ApprovalDecision, error) {
	f.calls++
	f.lastReq = req
	if f.err != nil {
		return Denied, f.err
	}
	return f.decision, nil
}

func writeCommandPolicyFile(t *testing.T, homeDir string, policy commandPolicy) {
	t.Helper()

	raw, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		t.Fatalf("marshal policy: %v", err)
	}
	raw = append(raw, '\n')
	path := filepath.Join(homeDir, store.DataDirPath, store.AllowedCommandsFilePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir policy dir: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}
}

func readCommandPolicyFile(t *testing.T, homeDir string) commandPolicy {
	t.Helper()

	path := filepath.Join(homeDir, store.DataDirPath, store.AllowedCommandsFilePath)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read policy: %v", err)
	}
	var policy commandPolicy
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatalf("unmarshal policy: %v", err)
	}
	return policy
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
