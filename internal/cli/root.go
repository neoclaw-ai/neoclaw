package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/machinae/betterclaw/internal/agent"
	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/bootstrap"
	"github.com/machinae/betterclaw/internal/channels"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/logging"
	providerapi "github.com/machinae/betterclaw/internal/provider"
	runtimeapi "github.com/machinae/betterclaw/internal/runtime"
	"github.com/machinae/betterclaw/internal/tools"
	"github.com/spf13/cobra"
)

var providerFactory = providerapi.NewProviderFromConfig

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
				logging.SetLevel(slog.LevelInfo)
			} else {
				logging.SetLevel(slog.LevelWarn)
			}

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
				// First-run bootstrap is an onboarding path, not a fatal error.
				// Print guidance and exit cleanly so logs do not report failures.
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
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging (info level)")

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

			modelProvider, err := providerFactory(cfg.DefaultLLM())
			if err != nil {
				return err
			}

			registry, err := buildToolRegistry(cfg, cmd.OutOrStdout())
			if err != nil {
				return err
			}

			if strings.TrimSpace(prompt) != "" {
				approver := approval.NewCLIApprover(cmd.InOrStdin(), cmd.OutOrStdout())
				handler := agent.New(modelProvider, registry, approver, agent.DefaultSystemPrompt)
				writer := &singleShotWriter{out: cmd.OutOrStdout()}
				return handler.HandleMessage(cmd.Context(), writer, &runtimeapi.Message{Text: prompt})
			}

			listener := channels.NewCLI(cmd.InOrStdin(), cmd.OutOrStdout())
			handler := agent.New(modelProvider, registry, listener, agent.DefaultSystemPrompt)
			return listener.Listen(cmd.Context(), handler)
		},
	}

	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Prompt message")

	return cmd
}

func buildToolRegistry(cfg *config.Config, out io.Writer) (*tools.Registry, error) {
	registry := tools.NewRegistry()
	coreTools := []tools.Tool{
		tools.ReadFileTool{WorkspaceDir: cfg.WorkspaceDir()},
		tools.ListDirTool{WorkspaceDir: cfg.WorkspaceDir()},
		tools.WriteFileTool{WorkspaceDir: cfg.WorkspaceDir()},
		tools.RunCommandTool{
			WorkspaceDir:    cfg.WorkspaceDir(),
			AllowedBinsPath: filepath.Join(cfg.DataDir, "allowed_bins.json"),
			Timeout:         cfg.Security.CommandTimeout,
		},
		tools.SendMessageTool{Writer: out},
	}
	for _, tool := range coreTools {
		if err := registry.Register(tool); err != nil {
			return nil, fmt.Errorf("register tool %q: %w", tool.Name(), err)
		}
	}
	return registry, nil
}

type singleShotWriter struct {
	out io.Writer
}

// WriteMessage writes one response message for one-shot prompt mode.
func (w *singleShotWriter) WriteMessage(_ context.Context, text string) error {
	_, err := fmt.Fprintln(w.out, text)
	return err
}
