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
	if c := findSubcommand(t, cmd, "serve"); c.Name() != "serve" {
		t.Fatalf("serve command not registered")
	}
	if c := findSubcommand(t, cmd, "prompt"); c.Name() != "prompt" {
		t.Fatalf("prompt command not registered")
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
