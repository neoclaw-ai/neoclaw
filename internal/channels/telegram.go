package channels

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/commands"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/logging"
	"github.com/machinae/betterclaw/internal/runtime"
	"github.com/machinae/betterclaw/internal/store"
)

// ErrWrongCode indicates the entered pairing code does not match the expected code.
var ErrWrongCode = errors.New("wrong pairing code")

type telegramPairUser struct {
	id       string
	username string
	name     string
}

// TelegramPairSession represents one active Telegram pairing session.
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

// TelegramListener receives Telegram updates and dispatches authorized messages.
type TelegramListener struct {
	token            string
	allowedUsersPath string
	commands         *commands.Handler

	allowedTelegramUsers map[string]struct{}
}

// BeginTelegramPairing starts Telegram pairing and waits for the first inbound user message.
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

// BotUsername returns the connected bot username discovered via getMe.
func (s *TelegramPairSession) BotUsername() string {
	return s.botUsername
}

// UserID returns the paired user's Telegram ID as a string.
func (s *TelegramPairSession) UserID() string {
	return s.user.id
}

// Username returns the paired user's Telegram username.
func (s *TelegramPairSession) Username() string {
	return s.user.username
}

// Name returns the paired user's display name.
func (s *TelegramPairSession) Name() string {
	return s.user.name
}

// SubmitCode validates an entered code and persists the paired Telegram user on success.
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
	logging.Logger().Info(
		"telegram user paired",
		"user_id", s.user.id,
		"username", s.user.username,
		"channel", "telegram",
	)

	return nil
}

func generateTelegramPairCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

var _ runtime.Listener = (*TelegramListener)(nil)

// NewTelegram creates a Telegram listener over one bot token and allowlist path.
func NewTelegram(token, allowedUsersPath string, commandsHandler *commands.Handler) *TelegramListener {
	return &TelegramListener{
		token:            token,
		allowedUsersPath: allowedUsersPath,
		commands:         commandsHandler,
	}
}

// Listen starts long-polling Telegram and dispatches authorized messages.
func (t *TelegramListener) Listen(ctx context.Context, handler runtime.Handler) error {
	if handler == nil {
		return errors.New("handler is required")
	}
	if strings.TrimSpace(t.token) == "" {
		return errors.New("telegram token is required")
	}
	if err := t.loadAllowedUsers(); err != nil {
		return err
	}
	if len(t.allowedTelegramUsers) == 0 {
		logging.Logger().Warn("No authorized Telegram users. Run claw pair to authorize your account.")
	}

	updateCh := make(chan *models.Update, defaultDispatchQueue)
	b, err := bot.New(
		strings.TrimSpace(t.token),
		bot.WithDefaultHandler(func(_ context.Context, _ *bot.Bot, update *models.Update) {
			select {
			case updateCh <- update:
			default:
				logging.Logger().Warn("telegram update queue is full; dropping update")
			}
		}),
	)
	if err != nil {
		return fmt.Errorf("create telegram bot: %w", err)
	}

	dispatchCtx, cancelDispatch := context.WithCancel(ctx)
	dispatcher := runtime.NewDispatcher(handler, defaultDispatchQueue)
	if err := dispatcher.Start(dispatchCtx); err != nil {
		cancelDispatch()
		return err
	}
	defer func() {
		cancelDispatch()
		dispatcher.Wait()
	}()

	go b.Start(ctx)

	for {
		select {
		case <-ctx.Done():
			dispatcher.Stop()
			return nil
		case update := <-updateCh:
			t.handleUpdate(ctx, dispatcher, b, update)
		}
	}
}

func (t *TelegramListener) loadAllowedUsers() error {
	usersFile, err := approval.LoadUsers(t.allowedUsersPath)
	if err != nil {
		return fmt.Errorf("load allowed users %q: %w", t.allowedUsersPath, err)
	}

	allowed := make(map[string]struct{}, len(usersFile.Users))
	for _, user := range usersFile.Users {
		if strings.EqualFold(strings.TrimSpace(user.Channel), "telegram") {
			id := strings.TrimSpace(user.ID)
			if id == "" {
				continue
			}
			allowed[id] = struct{}{}
		}
	}
	t.allowedTelegramUsers = allowed
	return nil
}

func (t *TelegramListener) handleUpdate(ctx context.Context, dispatcher *runtime.Dispatcher, b *bot.Bot, update *models.Update) {
	if update == nil || update.Message == nil || update.Message.From == nil {
		return
	}
	t.handleInboundMessage(ctx, dispatcher, update.Message, func(ctx context.Context, text string) error {
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   text,
		})
		return err
	})
}

func (t *TelegramListener) handleInboundMessage(
	ctx context.Context,
	dispatcher *runtime.Dispatcher,
	msg *models.Message,
	send func(context.Context, string) error,
) {
	if msg == nil || msg.From == nil {
		return
	}

	userID := strconv.FormatInt(msg.From.ID, 10)
	username := strings.TrimSpace(msg.From.Username)
	text := msg.Text
	logging.Logger().Info(
		"telegram inbound message",
		"user_id", userID,
		"username", username,
		"text", messagePreview(text, 100),
	)

	if !t.isAllowedUser(userID) {
		return
	}

	writer := &telegramWriter{send: send}
	trimmedText := strings.TrimSpace(text)
	if strings.HasPrefix(trimmedText, "/") && t.commands != nil {
		handled, err := t.commands.Handle(ctx, trimmedText, writer)
		if err != nil {
			logging.Logger().Warn("telegram command failed", "user_id", userID, "username", username, "err", err)
			writer.WriteMessage(ctx, fmt.Sprintf("error: %v", err))
			return
		}
		if handled {
			return
		}
	}

	if err := dispatcher.Enqueue(ctx, &runtime.Message{Text: trimmedText}, writer); err != nil {
		logging.Logger().Warn("telegram enqueue failed", "user_id", userID, "username", username, "err", err)
	}
}

func (t *TelegramListener) isAllowedUser(userID string) bool {
	if t.allowedTelegramUsers == nil {
		return false
	}
	_, ok := t.allowedTelegramUsers[strings.TrimSpace(userID)]
	return ok
}

type telegramWriter struct {
	send func(ctx context.Context, text string) error
}

func (w *telegramWriter) WriteMessage(ctx context.Context, text string) error {
	if w == nil || w.send == nil {
		return errors.New("telegram sender is not configured")
	}
	return w.send(ctx, text)
}

func messagePreview(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}
