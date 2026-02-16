package cli

import (
	"errors"
	"fmt"
	"strings"

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
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			return bootstrap.Initialize(cfg)
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

			resp, err := provider.Chat(cmd.Context(), llm.ChatRequest{
				SystemPrompt: "You are BetterClaw, a lightweight personal AI assistant.",
				Messages: []llm.ChatMessage{
					{
						Role:    llm.RoleUser,
						Content: prompt,
					},
				},
			})
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
