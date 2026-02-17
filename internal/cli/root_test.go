package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/machinae/betterclaw/internal/config"
	providerapi "github.com/machinae/betterclaw/internal/provider"
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
	dataDir := filepath.Join(t.TempDir(), ".betterclaw")
	t.Setenv("BETTERCLAW_HOME", dataDir)
	writeValidConfig(t, dataDir)

	origFactory := providerFactory
	defer func() { providerFactory = origFactory }()
	providerFactory = func(_ config.LLMProviderConfig) (providerapi.Provider, error) {
		return fakeProvider{
			resp: &providerapi.ChatResponse{Content: "hello from llm"},
		}, nil
	}

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"prompt", "-p", "hello"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute prompt command: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "hello from llm" {
		t.Fatalf("expected output %q, got %q", "hello from llm", got)
	}
}

func TestPromptInteractiveMode(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), ".betterclaw")
	t.Setenv("BETTERCLAW_HOME", dataDir)
	writeValidConfig(t, dataDir)

	origFactory := providerFactory
	defer func() { providerFactory = origFactory }()
	providerFactory = func(_ config.LLMProviderConfig) (providerapi.Provider, error) {
		return fakeProvider{
			resp: &providerapi.ChatResponse{Content: "hello from llm"},
		}, nil
	}

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetIn(strings.NewReader("hello\nquit\n"))
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"prompt"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute interactive prompt command: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Interactive mode. Type /quit or /exit to stop.") {
		t.Fatalf("expected interactive mode header, got %q", got)
	}
	if !strings.Contains(got, "assistant> hello from llm") {
		t.Fatalf("expected assistant response in interactive mode, got %q", got)
	}
}

func TestPromptOneShotRejectsSlashCommands(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), ".betterclaw")
	t.Setenv("BETTERCLAW_HOME", dataDir)
	writeValidConfig(t, dataDir)

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"prompt", "-p", "/new"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected slash command rejection in one-shot mode")
	}
	if !strings.Contains(err.Error(), "slash commands are not supported in one-shot -p mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServeLoadsDefaultsAndBootstraps(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), ".betterclaw")
	t.Setenv("BETTERCLAW_HOME", dataDir)
	writeValidConfig(t, dataDir)

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"serve"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute serve: %v", err)
	}

	if !strings.Contains(out.String(), "starting server...") {
		t.Fatalf("expected serve output to include startup message, got %q", out.String())
	}

	soulFile := filepath.Join(dataDir, "agents", "default", "SOUL.md")
	if _, err := os.Stat(soulFile); err != nil {
		t.Fatalf("expected bootstrap file %q to exist: %v", soulFile, err)
	}
}

func writeValidConfig(t *testing.T, dataDir string) {
	t.Helper()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	configBody := `
[llm.default]
api_key = "test-key"
provider = "anthropic"
model = "claude-sonnet-4-6"

[channels.telegram]
enabled = true
token = "telegram-token"
allowed_users = [123456789]
`
	if err := os.WriteFile(filepath.Join(dataDir, "config.toml"), []byte(configBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

type fakeProvider struct {
	resp *providerapi.ChatResponse
	err  error
}

func (p fakeProvider) Chat(_ context.Context, _ providerapi.ChatRequest) (*providerapi.ChatResponse, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.resp, nil
}
