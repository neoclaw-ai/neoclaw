package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Set at build time via ldflags. See .goreleaser.yaml.
var (
	Version = "dev"
	Commit  = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version and build info",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "NeoClaw %s (%s)\n", Version, Commit)
			return nil
		},
	}
}
