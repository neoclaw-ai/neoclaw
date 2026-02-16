package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/bootstrap"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/llm"
	"github.com/spf13/cobra"
)

var providerFactory = llm.NewProviderFromConfig

// NewRootCmd creates the root command and registers all subcommands.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "claw",
		Short: "BetterClaw CLI",
		// Let main handle fatal error rendering through structured logs.
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			configPath := filepath.Join(cfg.DataDir, "config.toml")
			firstRun := false
			if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
				firstRun = true
			} else if err != nil {
				return fmt.Errorf("stat BetterClaw config file %q: %w", configPath, err)
			}

			if err := bootstrap.Initialize(cfg); err != nil {
				return err
			}

			if firstRun {
				if _, err := fmt.Fprintf(
					cmd.ErrOrStderr(),
					"First run setup complete.\nEdit config file: %s\nRestart BetterClaw.\n",
					configPath,
				); err != nil {
					return err
				}
				os.Exit(0)
			}

			return nil
		},
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

func newPromptCmd() *cobra.Command {
	var prompt string

	cmd := &cobra.Command{
		Use:   "prompt",
		Short: "Send a prompt message (or start interactive chat without -p)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := config.ValidateStartup(cfg); err != nil {
				return err
			}

			provider, err := providerFactory(cfg.DefaultLLM())
			if err != nil {
				return err
			}

			inputReader := bufio.NewReader(cmd.InOrStdin())
			approver := approval.NewCLIApproverFromReader(inputReader, cmd.OutOrStdout())
			runner, err := newPromptRunner(cfg, provider, approver, cmd.OutOrStdout())
			if err != nil {
				return err
			}

			if strings.TrimSpace(prompt) == "" {
				return runPromptREPL(cmd.Context(), runner, cmd.InOrStdin(), inputReader, cmd.OutOrStdout())
			}

			resp, err := runner.Send(cmd.Context(), prompt)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), resp)
			return err
		},
	}

	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Prompt message")

	return cmd
}
