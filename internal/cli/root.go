// Package cli wires Cobra subcommands to application dependencies; it is a thin controller with no business logic.
package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/machinae/betterclaw/internal/bootstrap"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/logging"
	"github.com/machinae/betterclaw/internal/provider"
	"github.com/machinae/betterclaw/internal/store"
	"github.com/spf13/cobra"
)

var providerFactory = provider.NewProviderFromConfig

// NewRootCmd creates the root command and registers all subcommands.
func NewRootCmd() *cobra.Command {
	var verbose bool

	root := &cobra.Command{
		Use:   "claw",
		Short: "BetterClaw CLI",
		// Let main handle fatal error rendering through structured logs.
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if verbose {
				logging.SetLevel(slog.LevelDebug)
			} else {
				logging.SetLevel(slog.LevelInfo)
			}

			// The config command only reads and prints merged config and should not
			// trigger bootstrap/first-run onboarding behavior.
			if cmd.Name() == "config" {
				return nil
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			configPath := filepath.Join(cfg.DataDir, store.ConfigFilePath)
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
				// First-run bootstrap is an onboarding path, not a fatal error.
				// Print guidance and exit cleanly so logs do not report failures.
				fmt.Fprintf(
					cmd.ErrOrStderr(),
					"First run setup complete.\nEdit config file: %s\nRestart BetterClaw.\n",
					configPath,
				)
				os.Exit(0)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to `claw start` when no subcommand is provided.
			startCmd, _, err := cmd.Find([]string{"start"})
			if err != nil {
				return err
			}
			startCmd.SetContext(cmd.Context())
			return startCmd.RunE(startCmd, args)
		},
	}

	root.AddCommand(newConfigCmd())
	root.AddCommand(newStartCmd())
	root.AddCommand(newCLICmd())
	root.AddCommand(newPairCmd())
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging (debug level)")

	return root
}
