package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/machinae/betterclaw/internal/agent"
	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/bootstrap"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/llm"
	"github.com/machinae/betterclaw/internal/tools"
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
		Short: "Send a prompt message",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(prompt) == "" {
				return errors.New("prompt cannot be empty")
			}

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
				tools.SendMessageTool{Writer: cmd.OutOrStdout()},
			}
			for _, tool := range coreTools {
				if err := registry.Register(tool); err != nil {
					return err
				}
			}

			approver := approval.NewCLIApprover(cmd.InOrStdin(), cmd.OutOrStdout())
			resp, _, err := agent.Run(
				cmd.Context(),
				provider,
				registry,
				approver,
				"You are BetterClaw, a lightweight personal AI assistant.",
				[]llm.ChatMessage{
					{
						Role:    llm.RoleUser,
						Content: prompt,
					},
				},
				10,
			)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), resp.Content)
			return err
		},
	}

	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Prompt message")

	return cmd
}
