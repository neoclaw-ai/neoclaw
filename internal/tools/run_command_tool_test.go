package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunCommand_AllowlistEnforced(t *testing.T) {
	workspace := t.TempDir()
	allowedPath := filepath.Join(t.TempDir(), "allowed_bins.json")
	if err := os.WriteFile(allowedPath, []byte("[\"echo\"]\n"), 0o644); err != nil {
		t.Fatalf("write allowed bins: %v", err)
	}

	tool := RunCommandTool{
		WorkspaceDir:    workspace,
		AllowedBinsPath: allowedPath,
	}
	_, err := tool.Execute(context.Background(), map[string]any{"command": "cat /etc/hosts"})
	if err == nil || !strings.Contains(err.Error(), "is not allowed") {
		t.Fatalf("expected allowlist rejection, got %v", err)
	}
}

func TestRunCommand_AllowedBinaryOK(t *testing.T) {
	workspace := t.TempDir()
	allowedPath := filepath.Join(t.TempDir(), "allowed_bins.json")
	if err := os.WriteFile(allowedPath, []byte("[\"echo\"]\n"), 0o644); err != nil {
		t.Fatalf("write allowed bins: %v", err)
	}

	tool := RunCommandTool{
		WorkspaceDir:    workspace,
		AllowedBinsPath: allowedPath,
	}
	res, err := tool.Execute(context.Background(), map[string]any{"command": "echo hello"})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if strings.TrimSpace(res.Output) != "hello" {
		t.Fatalf("expected output hello, got %q", res.Output)
	}
	if strings.Contains(res.Output, "exit code") {
		t.Fatalf("did not expect exit code on success, got %q", res.Output)
	}
}

func TestRunCommand_TimeoutEnforced(t *testing.T) {
	workspace := t.TempDir()
	allowedPath := filepath.Join(t.TempDir(), "allowed_bins.json")
	if err := os.WriteFile(allowedPath, []byte("[\"sleep\"]\n"), 0o644); err != nil {
		t.Fatalf("write allowed bins: %v", err)
	}

	tool := RunCommandTool{
		WorkspaceDir:    workspace,
		AllowedBinsPath: allowedPath,
		Timeout:         10 * time.Millisecond,
	}
	res, err := tool.Execute(context.Background(), map[string]any{"command": "sleep 1"})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if !strings.Contains(res.Output, "[exit code: 124]") {
		t.Fatalf("expected timeout exit code marker, got %q", res.Output)
	}
}

func TestRunCommand_CombinedOutputOnFailure(t *testing.T) {
	workspace := t.TempDir()
	allowedPath := filepath.Join(t.TempDir(), "allowed_bins.json")
	if err := os.WriteFile(allowedPath, []byte("[\"bash\"]\n"), 0o644); err != nil {
		t.Fatalf("write allowed bins: %v", err)
	}

	tool := RunCommandTool{
		WorkspaceDir:    workspace,
		AllowedBinsPath: allowedPath,
	}
	res, err := tool.Execute(context.Background(), map[string]any{"command": "bash -lc 'echo out; echo err 1>&2; exit 7'"})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if !strings.Contains(res.Output, "out") {
		t.Fatalf("expected combined output to contain stdout, got %q", res.Output)
	}
	if !strings.Contains(res.Output, "err") {
		t.Fatalf("expected combined output to contain stderr, got %q", res.Output)
	}
	if !strings.Contains(res.Output, "[exit code: 7]") {
		t.Fatalf("expected exit code marker on failure, got %q", res.Output)
	}
}

func TestRunCommand_TruncationMetadata(t *testing.T) {
	workspace := t.TempDir()
	allowedPath := filepath.Join(t.TempDir(), "allowed_bins.json")
	if err := os.WriteFile(allowedPath, []byte("[\"bash\"]\n"), 0o644); err != nil {
		t.Fatalf("write allowed bins: %v", err)
	}

	tool := RunCommandTool{
		WorkspaceDir:    workspace,
		AllowedBinsPath: allowedPath,
	}
	res, err := tool.Execute(context.Background(), map[string]any{
		"command": "bash -lc 'head -c 2101 /dev/zero | tr \"\\000\" a'",
	})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if !res.Truncated {
		t.Fatalf("expected truncated output")
	}
	if res.FullOutputPath == "" {
		t.Fatalf("expected full output path")
	}
}

func TestFirstCommandToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "simple command",
			input: "git status",
			want:  "git",
		},
		{
			name:  "single command token",
			input: "ls",
			want:  "ls",
		},
		{
			name:  "leading env assignments are skipped",
			input: "FOO=bar PATH=/tmp go test ./...",
			want:  "go",
		},
		{
			name:    "empty string errors",
			input:   "",
			wantErr: true,
		},
		{
			name:    "whitespace string errors",
			input:   "   \t\n ",
			wantErr: true,
		},
		{
			name:    "env assignment only errors",
			input:   "FOO=bar",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := firstCommandToken(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got token %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected token %q, got %q", tc.want, got)
			}
		})
	}
}

func TestIsEnvAssignment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{input: "FOO=bar", want: true},
		{input: "A1=bar", want: true},
		{input: "_PATH=/tmp", want: true},
		{input: "NAME=value=with=equals", want: true},
		{input: "=bar", want: false},
		{input: "1FOO=bar", want: false},
		{input: "FOO", want: false},
		{input: "FOO-BAR=baz", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			got := isEnvAssignment(tc.input)
			if got != tc.want {
				t.Fatalf("isEnvAssignment(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
