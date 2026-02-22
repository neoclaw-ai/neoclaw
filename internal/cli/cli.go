package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/machinae/betterclaw/internal/agent"
	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/channels"
	"github.com/machinae/betterclaw/internal/commands"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/costs"
	"github.com/machinae/betterclaw/internal/memory"
	"github.com/machinae/betterclaw/internal/runtime"
	"github.com/machinae/betterclaw/internal/sandbox"
	"github.com/machinae/betterclaw/internal/scheduler"
	"github.com/machinae/betterclaw/internal/session"
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
			if err := cfg.Validate(); err != nil {
				return err
			}
			if cfg.Security.Mode == config.SecurityModeStrict && !sandbox.IsSandboxSupported() {
				return fmt.Errorf("security.mode strict requires sandbox support on this platform")
			}
			warnStartupConditions(cfg)

			llmCfg := cfg.DefaultLLM()
			modelProvider, err := providerFactory(llmCfg)
			if err != nil {
				return err
			}

			memoryStore := memory.New(cfg.MemoryDir())
			channelWriters := map[string]io.Writer{"cli": cmd.OutOrStdout()}
			schedulerService := newSchedulerService(cfg, channelWriters)
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

			registry, err := buildToolRegistry(cfg, cmd.OutOrStdout(), memoryStore, approver, schedulerService, nil, nil)
			if err != nil {
				return err
			}
			systemPrompt, err := agent.BuildSystemPrompt(cfg.AgentDir(), memoryStore)
			if err != nil {
				return err
			}
			costTracker := costs.New(cfg.CostsPath())

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

			sessionStore := session.New(cfg.CLIContextPath())
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
				Commands: commands.New(handler, schedulerService, costTracker, cfg.Costs.DailyLimit, cfg.Costs.MonthlyLimit),
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
	schedulerService *scheduler.Service,
	channelSender tools.ChannelMessageSender,
	resolveChannelID func() string,
) (*tools.Registry, error) {
	registry := tools.NewRegistry()
	httpClient := &http.Client{
		Transport: approval.RoundTripper{
			Checker: approval.Checker{
				AllowedDomainsPath: cfg.AllowedDomainsPath(),
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
		tools.JobListTool{Service: schedulerService},
		tools.JobCreateTool{
			Service:          schedulerService,
			ChannelID:        "cli",
			ResolveChannelID: resolveChannelID,
		},
		tools.JobDeleteTool{Service: schedulerService},
		tools.JobRunTool{Service: schedulerService},
		tools.RunCommandTool{
			WorkspaceDir: cfg.WorkspaceDir(),
			Timeout:      cfg.Security.CommandTimeout,
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
