package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/machinae/betterclaw/internal/agent"
	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/channels"
	"github.com/machinae/betterclaw/internal/commands"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/costs"
	"github.com/machinae/betterclaw/internal/logging"
	"github.com/machinae/betterclaw/internal/memory"
	"github.com/machinae/betterclaw/internal/sandbox"
	"github.com/machinae/betterclaw/internal/scheduler"
	"github.com/machinae/betterclaw/internal/session"
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
			if err := cfg.Validate(); err != nil {
				return err
			}
			if cfg.Security.Mode == config.SecurityModeStrict && !sandbox.IsSandboxSupported() {
				return errors.New("security.mode strict requires sandbox support on this platform")
			}
			warnStartupConditions(cfg)

			llm := cfg.DefaultLLM()
			logging.Logger().Info(
				"starting server",
				"agent", cfg.Agent,
				"provider", llm.Provider,
				"model", llm.Model,
				"security_mode", cfg.Security.Mode,
				"data_dir", cfg.DataDir(),
			)

			pidFilePath := cfg.PIDPath()
			if err := os.WriteFile(pidFilePath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644); err != nil {
				return fmt.Errorf("write pid file %q: %w", pidFilePath, err)
			}
			defer func() {
				os.Remove(pidFilePath)
			}()

			channelWriters := map[string]io.Writer{
				"cli": cmd.OutOrStdout(),
			}
			service, err := newSchedulerService(cfg, channelWriters)
			if err != nil {
				return err
			}

			runCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			telegramErrCh, err := startTelegramFunc(runCtx, cfg, cmd.OutOrStdout(), channelWriters, service)
			if err != nil {
				return err
			}
			if err := service.Start(runCtx); err != nil {
				stop()
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
	channelWriters map[string]io.Writer,
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
	allowedUsersPath := cfg.AllowedUsersPath()
	listener := channels.NewTelegram(token, allowedUsersPath)
	if err := registerTelegramChannelWriters(channelWriters, allowedUsersPath, listener); err != nil {
		return nil, err
	}

	llmCfg := cfg.DefaultLLM()
	modelProvider, err := providerFactory(llmCfg)
	if err != nil {
		return nil, err
	}

	memoryStore := memory.New(cfg.MemoryDir())
	registry, err := buildToolRegistry(cfg, out, memoryStore, listener, schedulerService, listener, listener.CurrentChannelID)
	if err != nil {
		return nil, err
	}

	systemPrompt, err := agent.BuildSystemPrompt(cfg.AgentDir(), memoryStore)
	if err != nil {
		return nil, err
	}

	costTracker := costs.New(cfg.CostsPath())
	sessionStore := session.New(cfg.TelegramContextPath())
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

	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		if err := listener.Listen(ctx, router); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- err
		}
	}()
	return errCh, nil
}

func registerTelegramChannelWriters(channelWriters map[string]io.Writer, allowedUsersPath string, listener *channels.TelegramListener) error {
	usersFile, err := approval.LoadUsers(allowedUsersPath)
	if err != nil {
		return fmt.Errorf("load allowed users %q: %w", allowedUsersPath, err)
	}

	for _, user := range usersFile.Users {
		if !strings.EqualFold(strings.TrimSpace(user.Channel), "telegram") {
			continue
		}
		id := strings.TrimSpace(user.ID)
		if id == "" {
			continue
		}

		chatID, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			logging.Logger().Warn(
				"skipping telegram channel writer registration with invalid user id",
				"user_id", id,
				"err", err,
			)
			continue
		}

		channelID := fmt.Sprintf("telegram-%d", chatID)
		channelWriters[channelID] = listener.ChannelWriter(chatID)
	}
	return nil
}
