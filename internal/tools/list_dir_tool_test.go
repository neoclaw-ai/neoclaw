package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListDir_ExistingPath(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.Mkdir(filepath.Join(workspace, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}

	tool := ListDirTool{WorkspaceDir: workspace}
	res, err := tool.Execute(context.Background(), map[string]any{"path": "."})
	if err != nil {
		t.Fatalf("list dir: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(res.Output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d (%q)", len(lines), res.Output)
	}
	if lines[0] != "a.txt" {
		t.Fatalf("unexpected first entry: %q", lines[0])
	}
	if lines[1] != "sub" {
		t.Fatalf("unexpected second entry: %q", lines[1])
	}
}

func TestListDir_MissingPathErrors(t *testing.T) {
	tool := ListDirTool{WorkspaceDir: t.TempDir()}
	_, err := tool.Execute(context.Background(), map[string]any{"path": "missing"})
	if err == nil {
		t.Fatalf("expected error for missing directory")
	}
}
