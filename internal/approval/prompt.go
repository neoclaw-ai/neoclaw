package approval

import (
	"fmt"
	"strings"
)

// FormatApprovalPrompt builds the CLI prompt text for an approval request.
func FormatApprovalPrompt(req ApprovalRequest) string {
	description := strings.TrimSpace(req.Description)
	if req.Tool == "network_domain" && description != "" {
		return fmt.Sprintf("%s [y/N]: ", description)
	}

	if description == "" {
		return fmt.Sprintf("approve tool %s? [y/N]: ", req.Tool)
	}
	return fmt.Sprintf("approve tool %s? %s [y/N]: ", req.Tool, description)
}

