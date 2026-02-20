package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/machinae/betterclaw/internal/provider"
)

func createTestHome(t *testing.T) string {
	t.Helper()
	dataDir := filepath.Join(t.TempDir(), ".betterclaw")
	t.Setenv("BETTERCLAW_HOME", dataDir)
	return dataDir
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
`
	if err := os.WriteFile(filepath.Join(dataDir, "config.toml"), []byte(configBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

type fakeProvider struct {
	resp *provider.ChatResponse
	err  error
}

func (p fakeProvider) Chat(_ context.Context, _ provider.ChatRequest) (*provider.ChatResponse, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.resp, nil
}
