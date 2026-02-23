package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/machinae/betterclaw/internal/config"
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

func TestReadFile_LargeFileReturnsFullContent(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "large.txt")
	data := strings.Repeat("a", 50*1024+500)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	tool := ReadFileTool{WorkspaceDir: workspace}
	res, err := tool.Execute(context.Background(), map[string]any{"path": "large.txt"})
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if res.Truncated {
		t.Fatalf("expected non-truncated read")
	}
	if res.Output != data {
		t.Fatalf("expected full content, got %d bytes", len(res.Output))
	}
}

func TestReadFileDoesNotImplementSummarizer(t *testing.T) {
	tool := ReadFileTool{}
	if _, ok := any(tool).(Summarizer); ok {
		t.Fatalf("read file tool should not implement Summarizer")
	}
}

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

func TestWriteFile_SymlinkEscapeErrors(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	linkPath := filepath.Join(workspace, "link")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	tool := WriteFileTool{WorkspaceDir: workspace}
	_, err := tool.Execute(context.Background(), map[string]any{
		"path":    "link/escape.txt",
		"content": "nope",
	})
	if err == nil || !strings.Contains(err.Error(), "outside workspace") {
		t.Fatalf("expected outside workspace error, got %v", err)
	}
}

func TestWriteFile_DangerModeAllowsOutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	tool := WriteFileTool{
		WorkspaceDir: workspace,
		SecurityMode: config.SecurityModeDanger,
	}

	outside := filepath.Join(filepath.Dir(workspace), "outside-danger.txt")
	res, err := tool.Execute(context.Background(), map[string]any{
		"path":    outside,
		"content": "danger-ok",
	})
	if err != nil {
		t.Fatalf("write file in danger mode: %v", err)
	}
	if res.Output != "ok" {
		t.Fatalf("expected ok output, got %q", res.Output)
	}

	content, err := os.ReadFile(outside)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(content) != "danger-ok" {
		t.Fatalf("expected danger-ok, got %q", string(content))
	}
}

func TestWriteFile_OutsideWorkspaceBehaviorBySecurityMode(t *testing.T) {
	workspace := t.TempDir()
	testCases := []struct {
		name      string
		mode      string
		expectErr bool
	}{
		{
			name:      "standard mode blocks outside workspace",
			mode:      config.SecurityModeStandard,
			expectErr: true,
		},
		{
			name:      "strict mode blocks outside workspace",
			mode:      config.SecurityModeStrict,
			expectErr: true,
		},
		{
			name:      "danger mode allows outside workspace",
			mode:      config.SecurityModeDanger,
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outside := filepath.Join(filepath.Dir(workspace), tc.mode+"-outside.txt")
			tool := WriteFileTool{
				WorkspaceDir: workspace,
				SecurityMode: tc.mode,
			}
			_, err := tool.Execute(context.Background(), map[string]any{
				"path":    outside,
				"content": tc.mode,
			})
			if tc.expectErr {
				if err == nil || !strings.Contains(err.Error(), "outside workspace") {
					t.Fatalf("expected outside workspace error in mode %q, got %v", tc.mode, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected success in mode %q, got %v", tc.mode, err)
			}
		})
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
