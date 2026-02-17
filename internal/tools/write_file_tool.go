package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
