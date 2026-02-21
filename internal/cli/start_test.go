package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/scheduler"
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
		*scheduler.Store,
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

	soulFile := filepath.Join(dataDir, "agents", "default", "SOUL.md")
	if _, err := os.Stat(soulFile); err != nil {
		t.Fatalf("expected bootstrap file %q to exist: %v", soulFile, err)
	}
}
