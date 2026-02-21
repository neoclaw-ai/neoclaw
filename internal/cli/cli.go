package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/machinae/betterclaw/internal/agent"
	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/channels"
	"github.com/machinae/betterclaw/internal/commands"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/costs"
	"github.com/machinae/betterclaw/internal/memory"
	"github.com/machinae/betterclaw/internal/runtime"
	"github.com/machinae/betterclaw/internal/scheduler"
	"github.com/machinae/betterclaw/internal/session"
	"github.com/machinae/betterclaw/internal/store"
	"github.com/machinae/betterclaw/internal/tools"
	"github.com/spf13/cobra"
)

func newCLICmd() *cobra.Command {
	var prompt string

	cmd := &cobra.Command{
		Use:   "cli",
		Short: "Send a message (or start interactive chat without -p)",
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
			jobsStore := newSchedulerStore(cfg)
			schedulerService := newSchedulerService(cfg, cmd.OutOrStdout(), jobsStore)
			trimmedPrompt := strings.TrimSpace(prompt)
			var (
				approver approval.Approver
				listener *channels.CLIListener
			)
			if trimmedPrompt != "" {
				if strings.HasPrefix(trimmedPrompt, "/") {
					return fmt.Errorf("slash commands are not supported in one-shot -p mode")
				}
				approver = approval.NewCLIApprover(cmd.InOrStdin(), cmd.OutOrStdout())
			} else {
				listener = channels.NewCLI(cmd.InOrStdin(), cmd.OutOrStdout())
				approver = listener
			}

			registry, err := buildToolRegistry(cfg, cmd.OutOrStdout(), memoryStore, approver, jobsStore, schedulerService, nil)
			if err != nil {
				return err
			}
			systemPrompt, err := agent.BuildSystemPrompt(cfg.AgentDir(), memoryStore)
			if err != nil {
				return err
			}
			costTracker := costs.New(filepath.Join(cfg.DataDir, store.CostsFilePath))

			if trimmedPrompt != "" {
				handler := agent.New(modelProvider, registry, approver, systemPrompt)
				handler.ConfigureCosts(
					costTracker,
					llmCfg.Provider,
					llmCfg.Model,
					cfg.Costs.DailyLimit,
					cfg.Costs.MonthlyLimit,
				)
				writer := &singleShotWriter{out: cmd.OutOrStdout()}
				return handler.HandleMessage(cmd.Context(), writer, &runtime.Message{Text: trimmedPrompt})
			}

			sessionStore := session.New(filepath.Join(cfg.AgentDir(), store.SessionsDirPath, store.CLISessionsDirPath, store.DefaultSessionPath))
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
			handler.ConfigureCosts(
				costTracker,
				llmCfg.Provider,
				llmCfg.Model,
				cfg.Costs.DailyLimit,
				cfg.Costs.MonthlyLimit,
			)
			router := commands.Router{
				Commands: commands.New(handler, jobsStore, costTracker, cfg.Costs.DailyLimit, cfg.Costs.MonthlyLimit),
				Next:     handler,
			}
			return listener.Listen(cmd.Context(), router)
		},
	}

	cmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Prompt message")

	return cmd
}

func buildToolRegistry(
	cfg *config.Config,
	out io.Writer,
	memoryStore *memory.Store,
	approver approval.Approver,
	jobsStore *scheduler.Store,
	schedulerService *scheduler.Service,
	channelSender tools.ChannelMessageSender,
) (*tools.Registry, error) {
	registry := tools.NewRegistry()
	httpClient := &http.Client{
		Transport: approval.RoundTripper{
			Checker: approval.Checker{
				AllowedDomainsPath: filepath.Join(cfg.DataDir, store.AllowedDomainsFilePath),
				Approver:           approver,
			},
		},
	}
	coreTools := []tools.Tool{
		tools.ReadFileTool{WorkspaceDir: cfg.WorkspaceDir()},
		tools.ListDirTool{WorkspaceDir: cfg.WorkspaceDir()},
		tools.WriteFileTool{WorkspaceDir: cfg.WorkspaceDir()},
		tools.MemoryReadTool{Store: memoryStore},
		tools.MemoryAppendTool{Store: memoryStore},
		tools.MemoryRemoveTool{Store: memoryStore},
		tools.DailyLogTool{Store: memoryStore},
		tools.SearchLogsTool{Store: memoryStore},
		tools.JobListTool{Store: jobsStore},
		tools.JobCreateTool{Store: jobsStore, ChannelID: "cli"},
		tools.JobDeleteTool{Store: jobsStore},
		tools.JobRunTool{Service: schedulerService},
		tools.RunCommandTool{
			WorkspaceDir:    cfg.WorkspaceDir(),
			AllowedBinsPath: filepath.Join(cfg.DataDir, store.AllowedBinsFilePath),
			Timeout:         cfg.Security.CommandTimeout,
		},
		tools.SendMessageTool{
			Sender: channelSender,
			Writer: out,
		},
		tools.WebSearchTool{
			Client:   httpClient,
			Provider: cfg.Web.Search.Provider,
			APIKey:   cfg.Web.Search.APIKey,
		},
		tools.HTTPRequestTool{Client: httpClient},
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
	fmt.Fprintln(w.out, text)
	return nil
}
