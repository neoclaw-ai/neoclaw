package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/machinae/betterclaw/internal/agent"
	"github.com/machinae/betterclaw/internal/channels"
	"github.com/machinae/betterclaw/internal/commands"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/costs"
	"github.com/machinae/betterclaw/internal/logging"
	"github.com/machinae/betterclaw/internal/memory"
	"github.com/machinae/betterclaw/internal/scheduler"
	"github.com/machinae/betterclaw/internal/session"
	"github.com/machinae/betterclaw/internal/store"
	"github.com/spf13/cobra"
)

var startTelegramFunc = startTelegram

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
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
			logging.Logger().Info(
				"starting server",
				"agent", cfg.Agent,
				"provider", llm.Provider,
				"model", llm.Model,
				"data_dir", cfg.DataDir,
			)

			pidFilePath := filepath.Join(cfg.DataDir, "claw.pid")
			if err := os.WriteFile(pidFilePath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644); err != nil {
				return fmt.Errorf("write pid file %q: %w", pidFilePath, err)
			}
			defer func() {
				os.Remove(pidFilePath)
			}()

			jobsStore := newSchedulerStore(cfg)
			service := newSchedulerService(cfg, cmd.OutOrStdout(), jobsStore)

			runCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if err := service.Start(runCtx); err != nil {
				return err
			}

			telegramErrCh, err := startTelegramFunc(runCtx, cfg, cmd.OutOrStdout(), jobsStore, service)
			if err != nil {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				service.Stop(shutdownCtx)
				return err
			}

			var listenerErr error
			if telegramErrCh == nil {
				<-runCtx.Done()
			} else {
				listenerErrCh := telegramErrCh
				for {
					select {
					case <-runCtx.Done():
						listenerErrCh = nil
					case err, ok := <-listenerErrCh:
						if !ok {
							listenerErrCh = nil
							continue
						}
						listenerErr = err
						stop()
						listenerErrCh = nil
					}
					if listenerErrCh == nil {
						break
					}
				}
			}

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := service.Stop(shutdownCtx); err != nil {
				return err
			}
			if listenerErr != nil {
				return listenerErr
			}
			logging.Logger().Info("server stopped")
			return nil
		},
	}
}

func startTelegram(
	ctx context.Context,
	cfg *config.Config,
	out io.Writer,
	jobsStore *scheduler.Store,
	schedulerService *scheduler.Service,
) (<-chan error, error) {
	telegramCfg := cfg.TelegramChannel()
	if !telegramCfg.Enabled {
		return nil, nil
	}

	token := strings.TrimSpace(telegramCfg.Token)
	if token == "" {
		return nil, errors.New("telegram is enabled but token is empty")
	}

	logging.Logger().Info("Starting Telegram listener")
	listener := channels.NewTelegram(token, filepath.Join(cfg.DataDir, store.AllowedUsersFilePath))

	llmCfg := cfg.DefaultLLM()
	modelProvider, err := providerFactory(llmCfg)
	if err != nil {
		return nil, err
	}

	memoryStore := memory.New(filepath.Join(cfg.AgentDir(), store.MemoryDirPath))
	registry, err := buildToolRegistry(cfg, out, memoryStore, listener, jobsStore, schedulerService, listener)
	if err != nil {
		return nil, err
	}

	systemPrompt, err := agent.BuildSystemPrompt(cfg.AgentDir(), memoryStore)
	if err != nil {
		return nil, err
	}

	costTracker := costs.New(filepath.Join(cfg.DataDir, store.CostsFilePath))
	sessionStore := session.New(filepath.Join(cfg.AgentDir(), store.SessionsDirPath, "telegram", store.DefaultSessionPath))
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

	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		if err := listener.Listen(ctx, router); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- err
		}
	}()
	return errCh, nil
}
