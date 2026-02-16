package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFile_ReadExistingFile(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "note.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	tool := ReadFileTool{WorkspaceDir: workspace}
	res, err := tool.Execute(context.Background(), map[string]any{"path": "note.txt"})
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if res.Output != "hello" {
		t.Fatalf("expected hello, got %q", res.Output)
	}
	if res.Truncated {
		t.Fatalf("expected non-truncated read")
	}
}

func TestReadFile_BinaryFileErrors(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "blob.bin")
	if err := os.WriteFile(path, []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	tool := ReadFileTool{WorkspaceDir: workspace}
	_, err := tool.Execute(context.Background(), map[string]any{"path": "blob.bin"})
	if err == nil || !strings.Contains(err.Error(), "appears to be binary") {
		t.Fatalf("expected binary file error, got %v", err)
	}
}

func TestReadFile_LargeFileTruncated(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "large.txt")
	data := strings.Repeat("a", maxReadFileBytes+500)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	tool := ReadFileTool{WorkspaceDir: workspace}
	res, err := tool.Execute(context.Background(), map[string]any{"path": "large.txt"})
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !res.Truncated {
		t.Fatalf("expected truncated result")
	}
	if len(res.Output) != maxReadFileBytes {
		t.Fatalf("expected %d bytes, got %d", maxReadFileBytes, len(res.Output))
	}
}
