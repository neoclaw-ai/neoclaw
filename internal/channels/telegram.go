package channels

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/logging"
	"github.com/machinae/betterclaw/internal/runtime"
	"github.com/machinae/betterclaw/internal/store"
)

// ErrWrongCode indicates the entered pairing code does not match the expected code.
var ErrWrongCode = errors.New("wrong pairing code")

const (
	telegramApprovalApprovePrefix = "approval:ok:"
	telegramApprovalDenyPrefix    = "approval:no:"
)

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

type telegramPairCollector struct {
	firstInbound chan telegramInboundMessage
}

type telegramApprovalTarget struct {
	userID   string
	username string
	chatID   int64
}

type telegramPendingApproval struct {
	userID   string
	chatID   int64
	response chan approval.ApprovalDecision
}

type telegramSendMessageFunc func(context.Context, *bot.SendMessageParams) (*models.Message, error)
type telegramAnswerCallbackQueryFunc func(context.Context, *bot.AnswerCallbackQueryParams) (bool, error)
type telegramEditMessageReplyMarkupFunc func(context.Context, *bot.EditMessageReplyMarkupParams) (*models.Message, error)
type telegramSendChatActionFunc func(context.Context, *bot.SendChatActionParams) (bool, error)

// TelegramListener receives Telegram updates and dispatches authorized messages.
type TelegramListener struct {
	token            string
	allowedUsersPath string

	allowedTelegramUsers map[string]struct{}

	sendMessage            telegramSendMessageFunc
	answerCallbackQuery    telegramAnswerCallbackQueryFunc
	editMessageReplyMarkup telegramEditMessageReplyMarkupFunc
	sendChatAction         telegramSendChatActionFunc

	approvalMu           sync.Mutex
	activeApprovalTarget *telegramApprovalTarget
	pendingApprovals     map[string]telegramPendingApproval
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
	collector := &telegramPairCollector{firstInbound: firstInbound}
	b, err := bot.New(trimmedToken, bot.WithDefaultHandler(collector.handleUpdate))
	if err != nil {
		return nil, fmt.Errorf("connect to telegram bot: %w", err)
	}

	me, err := b.GetMe(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch telegram bot profile: %w", err)
	}
	logging.Logger().Info(fmt.Sprintf("Connected to Telegram Bot @%s", strings.TrimSpace(me.Username)))

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

func (c *telegramPairCollector) handleUpdate(_ context.Context, _ *bot.Bot, update *models.Update) {
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
	case c.firstInbound <- msg:
	default:
	}
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
var _ approval.Approver = (*TelegramListener)(nil)

// NewTelegram creates a Telegram listener over one bot token and allowlist path.
func NewTelegram(token, allowedUsersPath string) *TelegramListener {
	return &TelegramListener{
		token:            token,
		allowedUsersPath: allowedUsersPath,
		pendingApprovals: make(map[string]telegramPendingApproval),
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

	dispatchCtx, cancelDispatch := context.WithCancel(ctx)
	dispatcher := runtime.NewDispatcher(&telegramApprovalHandler{listener: t, handler: handler}, defaultDispatchQueue)
	defaultHandler := func(updateCtx context.Context, _ *bot.Bot, update *models.Update) {
		if update == nil || update.Message == nil || update.Message.From == nil {
			return
		}
		t.handleInboundMessage(updateCtx, dispatcher, update.Message)
	}

	b, err := t.createTelegramBot(defaultHandler)
	if err != nil {
		cancelDispatch()
		return fmt.Errorf("create telegram bot: %w", err)
	}

	me, err := b.GetMe(ctx)
	if err != nil {
		cancelDispatch()
		return fmt.Errorf("fetch telegram bot profile: %w", err)
	}
	logging.Logger().Info(fmt.Sprintf("Connected to Telegram Bot @%s", strings.TrimSpace(me.Username)))

	t.sendMessage = b.SendMessage
	t.answerCallbackQuery = b.AnswerCallbackQuery
	t.editMessageReplyMarkup = b.EditMessageReplyMarkup
	t.sendChatAction = b.SendChatAction

	if err := dispatcher.Start(dispatchCtx); err != nil {
		cancelDispatch()
		return err
	}
	defer func() {
		cancelDispatch()
		dispatcher.Wait()
	}()

	go b.Start(ctx)
	<-ctx.Done()
	dispatcher.Stop()
	return nil
}

// RequestApproval prompts the active Telegram user with an inline Approve/Deny keyboard.
func (t *TelegramListener) RequestApproval(ctx context.Context, req approval.ApprovalRequest) (approval.ApprovalDecision, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return approval.Denied, nil
	}

	target, ok := t.activeApprovalTargetSnapshot()
	if !ok {
		return approval.Denied, errors.New("telegram approval target is unavailable")
	}

	token, err := generateTelegramApprovalToken()
	if err != nil {
		return approval.Denied, fmt.Errorf("generate approval token: %w", err)
	}

	prompt := strings.TrimSpace(req.Description)
	if prompt == "" {
		prompt = fmt.Sprintf("Approve %s?", strings.TrimSpace(req.Tool))
	}

	message, err := t.sendTelegramMessage(ctx, &bot.SendMessageParams{
		ChatID: target.chatID,
		Text:   prompt,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{
						Text:         "✅ Approve",
						CallbackData: telegramApprovalApprovePrefix + token,
					},
					{
						Text:         "❌ Deny",
						CallbackData: telegramApprovalDenyPrefix + token,
					},
				},
			},
		},
	})
	if err != nil {
		return approval.Denied, fmt.Errorf("send approval prompt: %w", err)
	}

	pending := telegramPendingApproval{
		userID:   target.userID,
		chatID:   target.chatID,
		response: make(chan approval.ApprovalDecision, 1),
	}
	t.storePendingApproval(token, pending)
	defer t.deletePendingApproval(token)

	select {
	case decision := <-pending.response:
		return decision, nil
	case <-ctx.Done():
		if message != nil {
			if _, err := t.editTelegramReplyMarkup(context.Background(), &bot.EditMessageReplyMarkupParams{
				ChatID:      target.chatID,
				MessageID:   message.ID,
				ReplyMarkup: nil,
			}); err != nil {
				logging.Logger().Warn("failed to clear approval keyboard", "chat_id", target.chatID, "message_id", message.ID, "err", err)
			}
		}
		return approval.Denied, nil
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

func (t *TelegramListener) handleApprovalCallback(
	ctx context.Context,
	callback *models.CallbackQuery,
	callbackPrefix string,
	decision approval.ApprovalDecision,
) {
	if callback == nil {
		return
	}

	if _, err := t.answerTelegramCallback(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
	}); err != nil {
		logging.Logger().Warn("failed to answer approval callback", "err", err)
	}

	token := parseApprovalToken(callback.Data, callbackPrefix)
	if token == "" {
		return
	}

	pending, found := t.pendingApproval(token)
	if !found {
		return
	}

	userID := strconv.FormatInt(callback.From.ID, 10)
	if userID != pending.userID {
		return
	}

	chatID, messageID, ok := callbackMessageLocation(callback)
	if !ok || chatID != pending.chatID {
		return
	}

	t.deletePendingApproval(token)
	if _, err := t.editTelegramReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      chatID,
		MessageID:   messageID,
		ReplyMarkup: nil,
	}); err != nil {
		logging.Logger().Warn("failed to clear approval keyboard", "chat_id", chatID, "message_id", messageID, "err", err)
	}

	select {
	case pending.response <- decision:
	default:
	}
}

func (t *TelegramListener) handleInboundMessage(
	ctx context.Context,
	dispatcher *runtime.Dispatcher,
	msg *models.Message,
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

	writer := &telegramWriter{
		listener: t,
		chatID:   msg.Chat.ID,
		userID:   userID,
		username: username,
	}
	trimmedText := strings.TrimSpace(text)
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
	listener *TelegramListener
	chatID   int64
	userID   string
	username string
}

func (w *telegramWriter) WriteMessage(ctx context.Context, text string) error {
	if w == nil || w.listener == nil {
		return errors.New("telegram sender is not configured")
	}
	return w.listener.sendChatMessage(ctx, w.chatID, text)
}

type telegramApprovalHandler struct {
	listener *TelegramListener
	handler  runtime.Handler
}

func (h *telegramApprovalHandler) HandleMessage(ctx context.Context, w runtime.ResponseWriter, msg *runtime.Message) error {
	if h.listener != nil {
		if writer, ok := w.(*telegramWriter); ok {
			h.listener.setActiveApprovalTarget(writer.userID, writer.username, writer.chatID)
			defer h.listener.clearActiveApprovalTarget()
			if msg != nil && !strings.HasPrefix(strings.TrimSpace(msg.Text), "/") {
				go h.listener.runTypingIndicator(ctx, writer.chatID)
			}
		}
	}
	return h.handler.HandleMessage(ctx, w, msg)
}

func (t *TelegramListener) setActiveApprovalTarget(userID, username string, chatID int64) {
	t.approvalMu.Lock()
	defer t.approvalMu.Unlock()
	t.activeApprovalTarget = &telegramApprovalTarget{
		userID:   strings.TrimSpace(userID),
		username: strings.TrimSpace(username),
		chatID:   chatID,
	}
}

func (t *TelegramListener) clearActiveApprovalTarget() {
	t.approvalMu.Lock()
	defer t.approvalMu.Unlock()
	t.activeApprovalTarget = nil
}

func (t *TelegramListener) activeApprovalTargetSnapshot() (telegramApprovalTarget, bool) {
	t.approvalMu.Lock()
	defer t.approvalMu.Unlock()
	if t.activeApprovalTarget == nil {
		return telegramApprovalTarget{}, false
	}
	return *t.activeApprovalTarget, true
}

func (t *TelegramListener) storePendingApproval(token string, pending telegramPendingApproval) {
	t.approvalMu.Lock()
	defer t.approvalMu.Unlock()
	t.pendingApprovals[token] = pending
}

func (t *TelegramListener) pendingApproval(token string) (telegramPendingApproval, bool) {
	t.approvalMu.Lock()
	defer t.approvalMu.Unlock()
	pending, ok := t.pendingApprovals[token]
	return pending, ok
}

func (t *TelegramListener) deletePendingApproval(token string) {
	t.approvalMu.Lock()
	defer t.approvalMu.Unlock()
	delete(t.pendingApprovals, token)
}

func (t *TelegramListener) sendTelegramMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	send := t.sendMessage
	if send == nil {
		return nil, errors.New("telegram bot is not connected")
	}
	return send(ctx, params)
}

func (t *TelegramListener) sendChatMessage(ctx context.Context, chatID int64, text string) error {
	_, err := t.sendTelegramMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	return err
}

// Send delivers a channel message to the active Telegram chat for the current request.
func (t *TelegramListener) Send(ctx context.Context, message string) error {
	target, ok := t.activeApprovalTargetSnapshot()
	if !ok {
		return errors.New("telegram chat target is unavailable")
	}
	return t.sendChatMessage(ctx, target.chatID, message)
}

func (t *TelegramListener) answerTelegramCallback(ctx context.Context, params *bot.AnswerCallbackQueryParams) (bool, error) {
	answer := t.answerCallbackQuery
	if answer == nil {
		return false, errors.New("telegram bot is not connected")
	}
	return answer(ctx, params)
}

func (t *TelegramListener) editTelegramReplyMarkup(ctx context.Context, params *bot.EditMessageReplyMarkupParams) (*models.Message, error) {
	edit := t.editMessageReplyMarkup
	if edit == nil {
		return nil, errors.New("telegram bot is not connected")
	}
	return edit(ctx, params)
}

func (t *TelegramListener) runTypingIndicator(ctx context.Context, chatID int64) {
	t.sendTypingAction(ctx, chatID)

	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.sendTypingAction(ctx, chatID)
		}
	}
}

func (t *TelegramListener) sendTypingAction(ctx context.Context, chatID int64) {
	send := t.sendChatAction
	if send == nil {
		return
	}
	send(ctx, &bot.SendChatActionParams{
		ChatID: chatID,
		Action: models.ChatActionTyping,
	})
}

func generateTelegramApprovalToken() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func parseApprovalToken(data, prefix string) string {
	if !strings.HasPrefix(data, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(data, prefix))
}

func callbackMessageLocation(callback *models.CallbackQuery) (int64, int, bool) {
	if callback == nil {
		return 0, 0, false
	}
	if callback.Message.Message != nil {
		return callback.Message.Message.Chat.ID, callback.Message.Message.ID, true
	}
	if callback.Message.InaccessibleMessage != nil {
		return callback.Message.InaccessibleMessage.Chat.ID, callback.Message.InaccessibleMessage.MessageID, true
	}
	return 0, 0, false
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

func (t *TelegramListener) createTelegramBot(defaultHandler bot.HandlerFunc) (*bot.Bot, error) {
	options := []bot.Option{
		bot.WithDefaultHandler(defaultHandler),
		bot.WithCallbackQueryDataHandler(telegramApprovalApprovePrefix, bot.MatchTypePrefix, t.onApprovalApproveCallback),
		bot.WithCallbackQueryDataHandler(telegramApprovalDenyPrefix, bot.MatchTypePrefix, t.onApprovalDenyCallback),
	}
	return bot.New(strings.TrimSpace(t.token), options...)
}

func (t *TelegramListener) onApprovalApproveCallback(ctx context.Context, _ *bot.Bot, update *models.Update) {
	if update == nil || update.CallbackQuery == nil {
		return
	}
	t.handleApprovalCallback(ctx, update.CallbackQuery, telegramApprovalApprovePrefix, approval.Approved)
}

func (t *TelegramListener) onApprovalDenyCallback(ctx context.Context, _ *bot.Bot, update *models.Update) {
	if update == nil || update.CallbackQuery == nil {
		return
	}
	t.handleApprovalCallback(ctx, update.CallbackQuery, telegramApprovalDenyPrefix, approval.Denied)
}
