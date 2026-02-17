package tools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

// ReadFileTool reads text files from the workspace.
type ReadFileTool struct {
	WorkspaceDir string
}

// Name returns the tool name.
func (t ReadFileTool) Name() string {
	return "read_file"
}

// Description returns the tool description for the model.
func (t ReadFileTool) Description() string {
	return "Read a text file from disk"
}

// Schema returns the JSON schema for read_file args.
func (t ReadFileTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Absolute path or path relative to workspace",
			},
		},
		"required": []string{"path"},
	}
}

// Permission declares default permission behavior for this tool.
func (t ReadFileTool) Permission() Permission {
	return AutoApprove
}

// Execute reads text content from a workspace-scoped path.
func (t ReadFileTool) Execute(_ context.Context, args map[string]any) (*ToolResult, error) {
	pathArg, err := stringArg(args, "path")
	if err != nil {
		return nil, err
	}

	path, err := resolveInputPath(t.WorkspaceDir, pathArg)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, maxReadFileBytes+1)
	n, readErr := io.ReadFull(f, buf)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("read file: %w", readErr)
	}
	data := buf[:n]

	if isBinary(data) {
		return nil, fmt.Errorf("file %q appears to be binary", path)
	}

	truncated := n > maxReadFileBytes
	if truncated {
		data = data[:maxReadFileBytes]
	}

	return &ToolResult{Output: string(data), Truncated: truncated}, nil
}
