package tools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

type ReadFileTool struct {
	WorkspaceDir string
}

func (t ReadFileTool) Name() string {
	return "read_file"
}

func (t ReadFileTool) Description() string {
	return "Read a text file from disk"
}

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

func (t ReadFileTool) Permission() Permission {
	return AutoApprove
}

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
