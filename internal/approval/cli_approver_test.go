package approval

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestCLIApprover_Approved(t *testing.T) {
	in := strings.NewReader("y\n")
	var out bytes.Buffer
	approver := NewCLIApprover(in, &out)

	decision, err := approver.RequestApproval(context.Background(), ApprovalRequest{
		Tool:        "run_command",
		Description: "run: ls",
	})
	if err != nil {
		t.Fatalf("request approval: %v", err)
	}
	if decision != Approved {
		t.Fatalf("expected Approved, got %v", decision)
	}
	if !strings.Contains(out.String(), "approve tool run_command?") {
		t.Fatalf("expected prompt text, got %q", out.String())
	}
	if !strings.Contains(out.String(), "[y/N]") {
		t.Fatalf("expected explicit y/N prompt, got %q", out.String())
	}
}

func TestCLIApprover_DeniedOnUnknownAnswer(t *testing.T) {
	in := strings.NewReader("always\n")
	var out bytes.Buffer
	approver := NewCLIApprover(in, &out)

	decision, err := approver.RequestApproval(context.Background(), ApprovalRequest{
		Tool:        "run_command",
		Description: "run: ls",
	})
	if err != nil {
		t.Fatalf("request approval: %v", err)
	}
	if decision != Denied {
		t.Fatalf("expected Denied, got %v", decision)
	}
}

func TestCLIApprover_Denied(t *testing.T) {
	in := strings.NewReader("n\n")
	var out bytes.Buffer
	approver := NewCLIApprover(in, &out)

	decision, err := approver.RequestApproval(context.Background(), ApprovalRequest{
		Tool:        "write_file",
		Description: "write file",
	})
	if err != nil {
		t.Fatalf("request approval: %v", err)
	}
	if decision != Denied {
		t.Fatalf("expected Denied, got %v", decision)
	}
}
