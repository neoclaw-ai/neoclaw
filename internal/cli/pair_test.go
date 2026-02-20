package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/store"
)

func TestPair_MissingTokenFails(t *testing.T) {
	dataDir := createTestHome(t)
	writePairConfig(t, dataDir, "")

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"pair"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing token error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "token") {
		t.Fatalf("expected token error, got: %v", err)
	}
}

func TestPair_PIDFilePresentFails(t *testing.T) {
	dataDir := createTestHome(t)
	writePairConfig(t, dataDir, "telegram-token")
	pidPath := filepath.Join(dataDir, "claw.pid")
	if err := store.WriteFile(pidPath, []byte("12345\n")); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"pair"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected running server error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "running") {
		t.Fatalf("expected running-server error, got: %v", err)
	}
}

func TestPair_TimeoutPrintsMessageAndDoesNotWriteUsers(t *testing.T) {
	dataDir := createTestHome(t)
	writePairConfig(t, dataDir, "telegram-token")

	deadlineCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"pair"})
	cmd.SetContext(deadlineCtx)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(out.String(), "Pairing timed out.") {
		t.Fatalf("expected timeout output, got %q", out.String())
	}

	usersPath := filepath.Join(dataDir, store.AllowedUsersFilePath)
	users, loadErr := approval.LoadUsers(usersPath)
	if loadErr != nil {
		t.Fatalf("load users: %v", loadErr)
	}
	if len(users.Users) != 0 {
		t.Fatalf("expected no users to be written on timeout, got %d", len(users.Users))
	}
}

func writePairConfig(t *testing.T, dataDir, token string) {
	t.Helper()
	configBody := `
[llm.default]
api_key = "test-key"
provider = "anthropic"
model = "claude-sonnet-4-6"

[channels.telegram]
enabled = true
token = "` + token + `"
`
	if err := store.WriteFile(filepath.Join(dataDir, store.ConfigFilePath), []byte(configBody)); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
