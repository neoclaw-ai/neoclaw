package channels

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/store"
)

var ErrWrongCode = errors.New("wrong pairing code")

type telegramPairUser struct {
	id       string
	username string
	name     string
}

type TelegramPairSession struct {
	bot              *bot.Bot
	botUsername      string
	chatID           int64
	expectedCode     string
	user             telegramPairUser
	allowedUsersPath string
}

type telegramInboundMessage struct {
	userID   string
	username string
	name     string
	chatID   int64
}

func BeginTelegramPairing(ctx context.Context, token string) (*TelegramPairSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	trimmedToken := strings.TrimSpace(token)
	if trimmedToken == "" {
		return nil, errors.New("telegram token is required")
	}

	firstInbound := make(chan telegramInboundMessage, 1)
	b, err := bot.New(trimmedToken, bot.WithDefaultHandler(func(_ context.Context, _ *bot.Bot, update *models.Update) {
		if update == nil || update.Message == nil || update.Message.From == nil {
			return
		}

		msg := telegramInboundMessage{
			userID:   fmt.Sprintf("%d", update.Message.From.ID),
			username: strings.TrimSpace(update.Message.From.Username),
			name:     strings.TrimSpace(update.Message.From.FirstName),
			chatID:   update.Message.Chat.ID,
		}
		select {
		case firstInbound <- msg:
		default:
		}
	}))
	if err != nil {
		return nil, fmt.Errorf("connect to telegram bot: %w", err)
	}

	me, err := b.GetMe(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch telegram bot profile: %w", err)
	}

	go b.Start(ctx)

	var inbound telegramInboundMessage
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case inbound = <-firstInbound:
	}

	code, err := generateTelegramPairCode()
	if err != nil {
		return nil, fmt.Errorf("generate pairing code: %w", err)
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: inbound.chatID,
		Text:   fmt.Sprintf("Pairing mode active. Your code is: %s - enter this in your terminal.", code),
	})
	if err != nil {
		return nil, fmt.Errorf("send pairing code: %w", err)
	}

	dataDir, err := config.HomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve data dir for users store: %w", err)
	}

	return &TelegramPairSession{
		bot:          b,
		botUsername:  strings.TrimSpace(me.Username),
		chatID:       inbound.chatID,
		expectedCode: code,
		user: telegramPairUser{
			id:       inbound.userID,
			username: inbound.username,
			name:     inbound.name,
		},
		allowedUsersPath: filepath.Join(dataDir, store.AllowedUsersFilePath),
	}, nil
}

func (s *TelegramPairSession) BotUsername() string {
	return s.botUsername
}

func (s *TelegramPairSession) UserID() string {
	return s.user.id
}

func (s *TelegramPairSession) Username() string {
	return s.user.username
}

func (s *TelegramPairSession) Name() string {
	return s.user.name
}

func (s *TelegramPairSession) SubmitCode(ctx context.Context, entered string) error {
	if strings.TrimSpace(entered) != s.expectedCode {
		return ErrWrongCode
	}

	if _, err := s.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: s.chatID,
		Text:   "You are now authorized. Restart the bot server to activate.",
	}); err != nil {
		return fmt.Errorf("send pairing confirmation: %w", err)
	}

	if err := approval.AddUser(s.allowedUsersPath, approval.User{
		ID:       s.user.id,
		Channel:  "telegram",
		Username: s.user.username,
		Name:     s.user.name,
	}); err != nil {
		return fmt.Errorf("persist paired user: %w", err)
	}

	return nil
}

func generateTelegramPairCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
