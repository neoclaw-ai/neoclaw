package cli

import (
	"fmt"

	"github.com/machinae/betterclaw/internal/config"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := config.ValidateStartup(cfg); err != nil {
				return err
			}

			llm := cfg.DefaultLLM()
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"starting server... agent=%s provider=%s model=%s data_dir=%s\n",
				cfg.Agent,
				llm.Provider,
				llm.Model,
				cfg.DataDir,
			)
			return err
		},
	}
}
