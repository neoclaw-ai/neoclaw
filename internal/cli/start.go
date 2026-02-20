package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/logging"
	"github.com/spf13/cobra"
)

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

			store := newSchedulerStore(cfg)
			service := newSchedulerService(cfg, cmd.OutOrStdout(), store)

			runCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if err := service.Start(runCtx); err != nil {
				return err
			}

			<-runCtx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := service.Stop(shutdownCtx); err != nil {
				return err
			}
			logging.Logger().Info("server stopped")
			return nil
		},
	}
}
