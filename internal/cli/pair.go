package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/machinae/betterclaw/internal/channels"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/logging"
	"github.com/spf13/cobra"
)

const pairTimeout = 15 * time.Minute

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

			pidFilePath := cfg.PIDPath()
			if _, err := os.Stat(pidFilePath); err == nil {
				return errors.New("server is already running. Stop it first, then run claw pair")
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("stat pid file %s: %w", pidFilePath, err)
			}

			pairingCtx, cancel := context.WithTimeout(cmd.Context(), pairTimeout)
			defer cancel()

			logging.Logger().Info(
				"connecting to telegram and waiting for first inbound message",
				"timeout", pairTimeout.String(),
			)

			session, err := channels.BeginTelegramPairing(pairingCtx, token)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					fmt.Fprintln(cmd.OutOrStdout(), "Pairing timed out.")
				}
				return err
			}
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Bot connected: @%s. Code sent to Telegram. Enter the pairing code:\n",
				session.BotUsername(),
			)

			reader := bufio.NewReader(cmd.InOrStdin())
			for {
				if err := pairingCtx.Err(); err != nil {
					if errors.Is(err, context.DeadlineExceeded) {
						fmt.Fprintln(cmd.OutOrStdout(), "Pairing timed out.")
					}
					return err
				}

				fmt.Fprint(cmd.OutOrStdout(), "Code: ")
				line, err := reader.ReadString('\n')
				if err != nil {
					if errors.Is(err, io.EOF) {
						return io.EOF
					}
					return err
				}
				entered := strings.TrimSpace(line)

				err = session.SubmitCode(pairingCtx, entered)
				if errors.Is(err, channels.ErrWrongCode) {
					fmt.Fprintln(cmd.OutOrStdout(), "Incorrect code. Try again:")
					continue
				}
				if err != nil {
					if errors.Is(err, context.DeadlineExceeded) {
						fmt.Fprintln(cmd.OutOrStdout(), "Pairing timed out.")
					}
					return err
				}

				name := session.Name()
				if name == "" {
					name = "Unknown"
				}
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"Paired: %s (@%s | ID %s)\n",
					name,
					session.Username(),
					session.UserID(),
				)
				return nil
			}
		},
	}
}
