package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxReadFileBytes = 50 * 1024

func stringArg(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing required argument %q", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("argument %q must be a string", key)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("argument %q cannot be empty", key)
	}
	return s, nil
}

func resolveInputPath(workspaceDir, input string) (string, error) {
	if filepath.IsAbs(input) {
		return filepath.Clean(input), nil
	}
	if strings.TrimSpace(workspaceDir) == "" {
		return "", errors.New("workspace directory is required for relative paths")
	}
	return filepath.Clean(filepath.Join(workspaceDir, input)), nil
}

func resolveWorkspacePath(workspaceDir, input string) (string, error) {
	if strings.TrimSpace(workspaceDir) == "" {
		return "", errors.New("workspace directory is required")
	}
	workspaceAbs, err := filepath.Abs(workspaceDir)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path: %w", err)
	}

	candidate := input
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(workspaceAbs, candidate)
	}
	candidate = filepath.Clean(candidate)

	rel, err := filepath.Rel(workspaceAbs, candidate)
	if err != nil {
		return "", fmt.Errorf("resolve relative path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q is outside workspace", input)
	}
	return candidate, nil
}

func isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return true
	}
	return !utf8.Valid(data)
}

type dirEntry struct {
	Name string `json:"name"`
}

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

// ListDirTool lists directory entries from a workspace-scoped path.
type ListDirTool struct {
	WorkspaceDir string
}

// Name returns the tool name.
func (t ListDirTool) Name() string {
	return "list_dir"
}

// Description returns the tool description for the model.
func (t ListDirTool) Description() string {
	return "List directory entries"
}

// Schema returns the JSON schema for list_dir args.
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

// Permission declares default permission behavior for this tool.
func (t ListDirTool) Permission() Permission {
	return AutoApprove
}

// Execute lists entries in the resolved directory path.
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
		items = append(items, dirEntry{Name: entry.Name()})
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
