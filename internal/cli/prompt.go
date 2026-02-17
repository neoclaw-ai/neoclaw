package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/machinae/betterclaw/internal/agent"
	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/channels"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/memory"
	"github.com/machinae/betterclaw/internal/runtime"
	"github.com/machinae/betterclaw/internal/session"
	"github.com/machinae/betterclaw/internal/tools"
	"github.com/spf13/cobra"
)

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

			llmCfg := cfg.DefaultLLM()
			modelProvider, err := providerFactory(llmCfg)
			if err != nil {
				return err
			}

			memoryStore := memory.New(filepath.Join(cfg.AgentDir(), "memory"))

			registry, err := buildToolRegistry(cfg, cmd.OutOrStdout(), memoryStore)
			if err != nil {
				return err
			}
			systemPrompt, err := agent.BuildSystemPrompt(cfg.AgentDir(), memoryStore)
			if err != nil {
				return err
			}

			if strings.TrimSpace(prompt) != "" {
				if strings.HasPrefix(strings.TrimSpace(prompt), "/") {
					return fmt.Errorf("slash commands are not supported in one-shot -p mode")
				}
				approver := approval.NewCLIApprover(cmd.InOrStdin(), cmd.OutOrStdout())
				handler := agent.New(modelProvider, registry, approver, systemPrompt)
				writer := &singleShotWriter{out: cmd.OutOrStdout()}
				return handler.HandleMessage(cmd.Context(), writer, &runtime.Message{Text: prompt})
			}

			listener := channels.NewCLI(cmd.InOrStdin(), cmd.OutOrStdout())
			sessionStore := session.New(filepath.Join(cfg.AgentDir(), "sessions", "cli", "default.jsonl"))
			handler := agent.NewWithSession(
				modelProvider,
				registry,
				listener,
				systemPrompt,
				sessionStore,
				memoryStore,
				cfg.Costs.MaxContextTokens,
				cfg.Costs.RecentMessages,
				llmCfg.RequestTimeout,
			)
			return listener.Listen(cmd.Context(), handler)
		},
	}

	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Prompt message")

	return cmd
}

func buildToolRegistry(cfg *config.Config, out io.Writer, memoryStore *memory.Store) (*tools.Registry, error) {
	registry := tools.NewRegistry()
	coreTools := []tools.Tool{
		tools.ReadFileTool{WorkspaceDir: cfg.WorkspaceDir()},
		tools.ListDirTool{WorkspaceDir: cfg.WorkspaceDir()},
		tools.WriteFileTool{WorkspaceDir: cfg.WorkspaceDir()},
		tools.MemoryReadTool{Store: memoryStore},
		tools.MemoryAppendTool{Store: memoryStore},
		tools.MemoryRemoveTool{Store: memoryStore},
		tools.DailyLogTool{Store: memoryStore},
		tools.SearchLogsTool{Store: memoryStore},
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
