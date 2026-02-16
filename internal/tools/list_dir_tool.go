package tools

import (
	"context"
	"os"
	"sort"
	"strings"
)

type dirEntry struct {
	Name string `json:"name"`
}

type ListDirTool struct {
	WorkspaceDir string
}

func (t ListDirTool) Name() string {
	return "list_dir"
}

func (t ListDirTool) Description() string {
	return "List directory entries"
}

func (t ListDirTool) Schema() map[string]any {
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

func (t ListDirTool) Permission() Permission {
	return AutoApprove
}

func (t ListDirTool) Execute(_ context.Context, args map[string]any) (*ToolResult, error) {
	pathArg, err := stringArg(args, "path")
	if err != nil {
		return nil, err
	}

	path, err := resolveInputPath(t.WorkspaceDir, pathArg)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	items := make([]dirEntry, 0, len(entries))
	for _, entry := range entries {
		items = append(items, dirEntry{
			Name: entry.Name(),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	var b strings.Builder
	for i, item := range items {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(item.Name)
	}

	return &ToolResult{Output: b.String()}, nil
}
