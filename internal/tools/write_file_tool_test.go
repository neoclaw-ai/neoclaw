package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFile_WithinWorkspaceOK(t *testing.T) {
	workspace := t.TempDir()
	tool := WriteFileTool{WorkspaceDir: workspace}
	res, err := tool.Execute(context.Background(), map[string]any{
		"path":    "nested/file.txt",
		"content": "hello",
	})
	if err != nil {
		t.Fatalf("write file: %v", err)
	}
	if res.Output != "ok" {
		t.Fatalf("expected ok output, got %q", res.Output)
	}

	content, err := os.ReadFile(filepath.Join(workspace, "nested/file.txt"))
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("expected hello, got %q", string(content))
	}
}

func TestWriteFile_OutsideWorkspaceErrors(t *testing.T) {
	workspace := t.TempDir()
	tool := WriteFileTool{WorkspaceDir: workspace}

	outside := filepath.Join(filepath.Dir(workspace), "outside.txt")
	_, err := tool.Execute(context.Background(), map[string]any{
		"path":    outside,
		"content": "nope",
	})
	if err == nil || !strings.Contains(err.Error(), "outside workspace") {
		t.Fatalf("expected outside workspace error, got %v", err)
	}
}

func TestWriteFileSummarizeArgs(t *testing.T) {
	tool := WriteFileTool{}
	s, ok := any(tool).(Summarizer)
	if !ok {
		t.Fatalf("write file tool should implement Summarizer")
	}

	content := strings.Repeat("a", 1243)
	got := s.SummarizeArgs(map[string]any{
		"path":    "notes.md",
		"content": content,
	})
	want := `write_file: path="notes.md" (1,243 bytes)`
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
