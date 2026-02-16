package approval

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// CLIApprover prompts for y/n approvals on stdin/stdout.
type CLIApprover struct {
	in  *bufio.Reader
	out io.Writer
}

func NewCLIApprover(in io.Reader, out io.Writer) *CLIApprover {
	return &CLIApprover{
		in:  bufio.NewReader(in),
		out: out,
	}
}

func (a *CLIApprover) RequestApproval(_ context.Context, req ApprovalRequest) (ApprovalDecision, error) {
	if _, err := fmt.Fprintf(a.out, "approve tool %s? %s [y/N]: ", req.Tool, req.Description); err != nil {
		return Denied, err
	}

	answer, err := a.in.ReadString('\n')
	if err != nil {
		return Denied, err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))

	switch answer {
	case "y", "yes":
		return Approved, nil
	case "a", "always":
		return AlwaysApproved, nil
	default:
		return Denied, nil
	}
}
