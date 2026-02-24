package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/neoclaw-ai/neoclaw/internal/channels"
	"github.com/neoclaw-ai/neoclaw/internal/config"
	"github.com/neoclaw-ai/neoclaw/internal/scheduler"
	"github.com/neoclaw-ai/neoclaw/internal/store"
)

func TestStartLoadsDefaultsAndBootstraps(t *testing.T) {
	dataDir := createTestHome(t)
	writeValidConfig(t, dataDir)
	origStartTelegram := startTelegramFunc
	defer func() {
		startTelegramFunc = origStartTelegram
	}()
	startTelegramFunc = func(
		context.Context,
		*config.Config,
		io.Writer,
		map[string]io.Writer,
		*scheduler.Service,
	) (<-chan error, error) {
		return nil, nil
	}

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"start"})
	ctx, cancel := context.WithCancel(context.Background())
	cmd.SetContext(ctx)
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute start: %v", err)
	}

	cfg := &config.Config{HomeDir: dataDir, Agent: "default"}
	soulFile := cfg.SoulPath()
	if _, err := os.Stat(soulFile); err != nil {
		t.Fatalf("expected bootstrap file %q to exist: %v", soulFile, err)
	}
}

func TestRegisterTelegramChannelWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allowed_users.json")
	if err := store.WriteFile(path, []byte(`{
  "users": [
    {"id":"111","channel":"telegram","username":"alice","name":"Alice","added_at":"2026-02-21T00:00:00Z"},
    {"id":"not-a-number","channel":"telegram","username":"broken","name":"Broken","added_at":"2026-02-21T00:00:00Z"},
    {"id":"222","channel":"slack","username":"bob","name":"Bob","added_at":"2026-02-21T00:00:00Z"}
  ]
}
`)); err != nil {
		t.Fatalf("write users file: %v", err)
	}

	listener := channels.NewTelegram("token", path)
	channelWriters := map[string]io.Writer{}
	if err := registerTelegramChannelWriters(channelWriters, path, listener); err != nil {
		t.Fatalf("register telegram channel writers: %v", err)
	}

	if _, ok := channelWriters["telegram-111"]; !ok {
		t.Fatalf("expected telegram writer for channel telegram-111")
	}
	if len(channelWriters) != 1 {
		t.Fatalf("expected one valid telegram writer entry, got %d", len(channelWriters))
	}
}
