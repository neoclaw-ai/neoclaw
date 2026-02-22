package tools

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunCommand_ExecutesCommand(t *testing.T) {
	workspace := t.TempDir()

	tool := RunCommandTool{
		WorkspaceDir: workspace,
		Timeout:      5 * time.Minute,
	}
	res, err := tool.Execute(context.Background(), map[string]any{"command": "pwd"})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if strings.TrimSpace(res.Output) == "" {
		t.Fatalf("expected command output")
	}
}

func TestRunCommand_AllowedBinaryOK(t *testing.T) {
	workspace := t.TempDir()

	tool := RunCommandTool{
		WorkspaceDir: workspace,
		Timeout:      5 * time.Minute,
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

	tool := RunCommandTool{
		WorkspaceDir: workspace,
		Timeout:      10 * time.Millisecond,
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

	tool := RunCommandTool{
		WorkspaceDir: workspace,
		Timeout:      5 * time.Minute,
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

func TestRunCommand_ContextCanceledStopsCommand(t *testing.T) {
	workspace := t.TempDir()

	tool := RunCommandTool{
		WorkspaceDir: workspace,
		Timeout:      5 * time.Minute,
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	startedAt := time.Now()
	res, err := tool.Execute(ctx, map[string]any{"command": "sleep 5 && echo done"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got result=%#v err=%v", res, err)
	}
	if elapsed := time.Since(startedAt); elapsed > 2*time.Second {
		t.Fatalf("expected fast cancellation, took %s", elapsed)
	}
}

func TestRunCommand_TruncationMetadata(t *testing.T) {
	workspace := t.TempDir()

	tool := RunCommandTool{
		WorkspaceDir: workspace,
		Timeout:      5 * time.Minute,
	}
	res, err := tool.Execute(context.Background(), map[string]any{
		"command": "bash -lc 'head -c 2101 /dev/zero | tr \"\\000\" a'",
	})
	if err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if res.FullOutputPath == "" {
		t.Fatalf("expected full output path")
	}
	if res.Truncated {
		t.Fatalf("expected non-truncated tool result")
	}
}

func TestRunCommand_WriteOutputUsesSchedulerJobIDInFilename(t *testing.T) {
	workspace := t.TempDir()
	tool := RunCommandTool{WorkspaceDir: workspace}

	path, err := tool.WriteOutput(map[string]any{
		schedulerOutputJobIDArg: "job_123",
	}, "ok")
	if err != nil {
		t.Fatalf("write output: %v", err)
	}
	if filepath.Dir(path) != filepath.Join(workspace, "tmp") {
		t.Fatalf("expected output under workspace/tmp, got %q", path)
	}
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "job_123-tool-output-") || !strings.HasSuffix(base, ".txt") {
		t.Fatalf("unexpected scheduler output filename %q", base)
	}
}

func TestRunCommandSummarizeArgs(t *testing.T) {
	tool := RunCommandTool{}
	s, ok := any(tool).(Summarizer)
	if !ok {
		t.Fatalf("run command tool should implement Summarizer")
	}

	got := s.SummarizeArgs(map[string]any{"command": "git status"})
	want := "run_command: git status"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
