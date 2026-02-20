package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/spf13/cobra"
)

func newPairCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pair",
		Short: "Authorize a Telegram user for bot access",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			token := strings.TrimSpace(cfg.TelegramChannel().Token)
			if token == "" {
				return errors.New("telegram bot token is not configured. Set [channels.telegram] token in config.toml")
			}

			pidFilePath := filepath.Join(cfg.DataDir, "claw.pid")
			if _, err := os.Stat(pidFilePath); err == nil {
				return errors.New("server appears to be running. Stop it first, then run claw pair")
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("stat pid file %q: %w", pidFilePath, err)
			}

			b, err := bot.New(token)
			if err != nil {
				return fmt.Errorf("connect to telegram bot: %w", err)
			}

			me, err := b.GetMe(cmd.Context())
			if err != nil {
				return fmt.Errorf("fetch telegram bot profile: %w", err)
			}

			username := strings.TrimSpace(me.Username)
			if username == "" {
				username = "unknown"
			}
			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"Bot connected: @%s. Pairing mode active for 15 minutes. Message your bot in Telegram to receive a pairing code.\n",
				username,
			)
			return err
		},
	}
}
