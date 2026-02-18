package cli

import (
	"os"
	"os/signal"
	"syscall"

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

			runCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			<-runCtx.Done()
			logging.Logger().Info("server stopped")
			return nil
		},
	}
}
