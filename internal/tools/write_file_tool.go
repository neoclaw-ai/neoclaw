package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// WriteFileTool writes text files under the workspace root.
type WriteFileTool struct {
	WorkspaceDir string
}

// Name returns the tool name.
func (t WriteFileTool) Name() string {
	return "write_file"
}

// Description returns the tool description for the model.
func (t WriteFileTool) Description() string {
	return "Write content to a file in the workspace"
}

// Schema returns the JSON schema for write_file args.
func (t WriteFileTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path relative to workspace or absolute path under workspace",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "File contents",
			},
		},
		"required": []string{"path", "content"},
	}
}

// Permission declares default permission behavior for this tool.
func (t WriteFileTool) Permission() Permission {
	return RequiresApproval
}

// SummarizeArgs returns a concise approval prompt summary for write_file.
func (t WriteFileTool) SummarizeArgs(args map[string]any) string {
	path := "<unknown>"
	if raw, ok := args["path"]; ok {
		if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
			path = s
		}
	}

	byteCount := 0
	if raw, ok := args["content"]; ok {
		if s, ok := raw.(string); ok {
			byteCount = len([]byte(s))
		}
	}

	return fmt.Sprintf(`write_file: path=%q (%s bytes)`, path, formatWithCommas(byteCount))
}

// Execute writes content to a workspace-scoped file path.
func (t WriteFileTool) Execute(_ context.Context, args map[string]any) (*ToolResult, error) {
	pathArg, err := stringArg(args, "path")
	if err != nil {
		return nil, err
	}
	content, err := stringArg(args, "content")
	if err != nil {
		return nil, err
	}

	path, err := resolveWorkspacePath(t.WorkspaceDir, pathArg)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create parent directories: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	return &ToolResult{Output: "ok"}, nil
}

func formatWithCommas(n int) string {
	if n == 0 {
		return "0"
	}
	s := strconv.Itoa(n)
	var b strings.Builder
	lead := len(s) % 3
	if lead == 0 {
		lead = 3
	}
	b.WriteString(s[:lead])
	for i := lead; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
