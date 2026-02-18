package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartLoadsDefaultsAndBootstraps(t *testing.T) {
	dataDir := createTestHome(t)
	writeValidConfig(t, dataDir)

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"start"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute start: %v", err)
	}

	if !strings.Contains(out.String(), "starting server...") {
		t.Fatalf("expected start output to include startup message, got %q", out.String())
	}

	soulFile := filepath.Join(dataDir, "agents", "default", "SOUL.md")
	if _, err := os.Stat(soulFile); err != nil {
		t.Fatalf("expected bootstrap file %q to exist: %v", soulFile, err)
	}
}
