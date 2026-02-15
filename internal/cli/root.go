package cli

import (
	"fmt"

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
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "starting server...")
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
