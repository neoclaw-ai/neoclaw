package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandRegistersSubcommands(t *testing.T) {
	cmd := NewRootCmd()

	serve, _, err := cmd.Find([]string{"serve"})
	if err != nil {
		t.Fatalf("find serve command: %v", err)
	}
	if serve == nil || serve.Name() != "serve" {
		t.Fatalf("serve command not registered")
	}

	prompt, _, err := cmd.Find([]string{"prompt"})
	if err != nil {
		t.Fatalf("find prompt command: %v", err)
	}
	if prompt == nil || prompt.Name() != "prompt" {
		t.Fatalf("prompt command not registered")
	}
}

func TestPromptFlagParsing(t *testing.T) {
	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"prompt", "-p", "hello"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute prompt command: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "hello" {
		t.Fatalf("expected output %q, got %q", "hello", got)
	}
}
