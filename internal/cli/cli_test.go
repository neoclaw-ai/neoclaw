package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/provider"
)

func TestCLIFlagParsing(t *testing.T) {
	dataDir := createTestHome(t)
	writeValidConfig(t, dataDir)

	origFactory := providerFactory
	defer func() { providerFactory = origFactory }()
	providerFactory = func(_ config.LLMProviderConfig) (provider.Provider, error) {
		return fakeProvider{
			resp: &provider.ChatResponse{Content: "hello from llm"},
		}, nil
	}

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"cli", "-p", "hello"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute cli command: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "hello from llm" {
		t.Fatalf("expected output %q, got %q", "hello from llm", got)
	}
}

func TestCLIInteractiveMode(t *testing.T) {
	dataDir := createTestHome(t)
	writeValidConfig(t, dataDir)

	origFactory := providerFactory
	defer func() { providerFactory = origFactory }()
	providerFactory = func(_ config.LLMProviderConfig) (provider.Provider, error) {
		return fakeProvider{
			resp: &provider.ChatResponse{Content: "hello from llm"},
		}, nil
	}

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetIn(strings.NewReader("/help\n/quit\n"))
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"cli"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute interactive cli command: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Interactive mode. Type /quit or /exit to stop.") {
		t.Fatalf("expected interactive mode header, got %q", got)
	}
	if !strings.Contains(got, "assistant> Stopped.") {
		t.Fatalf("expected stop output in interactive mode, got %q", got)
	}
}

func TestCLIOneShotRejectsSlashCommands(t *testing.T) {
	dataDir := createTestHome(t)
	writeValidConfig(t, dataDir)

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"cli", "-p", "/new"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected slash command rejection in one-shot mode")
	}
	if !strings.Contains(err.Error(), "slash commands are not supported in one-shot -p mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}
