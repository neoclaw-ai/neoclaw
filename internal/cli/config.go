package cli

import (
	"github.com/machinae/betterclaw/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Print merged configuration as TOML",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return config.Write(cmd.OutOrStdout())
		},
	}
}
