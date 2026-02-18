package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommandRegistersSubcommands(t *testing.T) {
	cmd := NewRootCmd()

	if c := findSubcommand(t, cmd, "config"); c.Name() != "config" {
		t.Fatalf("config command not registered")
	}
	if c := findSubcommand(t, cmd, "start"); c.Name() != "start" {
		t.Fatalf("start command not registered")
	}
	if c := findSubcommand(t, cmd, "cli"); c.Name() != "cli" {
		t.Fatalf("cli command not registered")
	}
}

func findSubcommand(t *testing.T, root *cobra.Command, name string) *cobra.Command {
	t.Helper()
	c, _, err := root.Find([]string{name})
	if err != nil {
		t.Fatalf("find %s command: %v", name, err)
	}
	if c == nil {
		t.Fatalf("%s command not found", name)
	}
	return c
}
