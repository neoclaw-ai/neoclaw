package cli

import (
	"fmt"

	"github.com/machinae/betterclaw/internal/bootstrap"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root command and registers all subcommands.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "claw",
		Short: "BetterClaw CLI",
	}

	root.AddCommand(newServeCmd())
	root.AddCommand(newPromptCmd())

	return root
}

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := bootstrap.Initialize(cfg); err != nil {
				return err
			}

			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"starting server... agent=%s provider=%s model=%s data_dir=%s\n",
				cfg.Agent,
				cfg.LLM.Provider,
				cfg.LLM.Model,
				cfg.DataDir,
			)
			return err
		},
	}
}

func newPromptCmd() *cobra.Command {
	var prompt string

	cmd := &cobra.Command{
		Use:   "prompt",
		Short: "Send a prompt message",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), prompt)
			return err
		},
	}

	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Prompt message")

	return cmd
}
