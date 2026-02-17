package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfigPrintsMergedConfig(t *testing.T) {
	dataDir := createTestHome(t)
	writeValidConfig(t, dataDir)

	cmd := NewRootCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"config"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute config: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "[llm.default]") {
		t.Fatalf("expected llm.default section, got %q", got)
	}
	if !strings.Contains(got, "provider = 'anthropic'") {
		t.Fatalf("expected merged provider in output, got %q", got)
	}
	if !strings.Contains(got, "[costs]") {
		t.Fatalf("expected defaults in output, got %q", got)
	}
}
