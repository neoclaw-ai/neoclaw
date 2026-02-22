package channels

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/commands"
	"github.com/machinae/betterclaw/internal/runtime"
	"github.com/machinae/betterclaw/internal/store"
)

func TestFormatTelegramMappings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bold",
			input:    "**bold**",
			expected: "<b>bold</b>",
		},
		{
			name:     "italic",
			input:    "*italic*",
			expected: "<i>italic</i>",
		},
		{
			name:     "strikethrough",
			input:    "~~gone~~",
			expected: "<s>gone</s>",
		},
		{
			name:     "heading",
			input:    "# Title",
			expected: "<b>Title</b>\n",
		},
		{
			name:     "inline code",
			input:    "`echo hi`",
			expected: "<code>echo hi</code>",
		},
		{
			name:     "fenced code",
			input:    "```go\nfmt.Println(\"hi\")\n```",
			expected: "<pre><code>fmt.Println(&#34;hi&#34;)\n</code></pre>",
		},
		{
			name:     "link",
			input:    "[site](https://example.com)",
			expected: `<a href="https://example.com">site</a>`,
		},
		{
			name:     "list item",
			input:    "- one\n- two",
			expected: "- one\n- two\n",
		},
		{
			name:     "plain passthrough",
			input:    "hello world",
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := formatTelegram(tt.input)
			if !ok {
				t.Fatalf("expected format success for input %q", tt.input)
			}
			if got != tt.expected {
				t.Fatalf("unexpected format output\ninput: %q\ngot: %q\nexpected: %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatTelegram_OmitsImagesAndRawHTML(t *testing.T) {
	got, ok := formatTelegram(`<b>raw</b> ![img](https://example.com/a.png)`)
	if !ok {
		t.Fatal("expected format success")
	}
	if strings.Contains(got, "<b>") || strings.Contains(got, "</b>") {
		t.Fatalf("expected raw html tags to be omitted, got %q", got)
	}
	if strings.Contains(got, "<img") {
		t.Fatalf("expected image tags to be omitted, got %q", got)
	}
}

func TestFormatTelegram_RenderErrorFallback(t *testing.T) {
	formatted, err := renderTelegram("hello", nil)
	if err == nil {
		t.Fatal("expected render error for nil parser")
	}
	if formatted != "" {
		t.Fatalf("expected empty formatted output on render failure, got %q", formatted)
	}

	got, ok := formatTelegram("hello")
	if !ok {
		t.Fatal("expected standard formatter to succeed")
	}
	if got != "hello" {
		t.Fatalf("expected passthrough text, got %q", got)
	}
}

func TestTelegramWriterWriteMessage_UsesHTMLParseMode(t *testing.T) {
	listener := NewTelegram("token", "")

	var sent *bot.SendMessageParams
	listener.sendMessage = func(_ context.Context, params *bot.SendMessageParams) (*models.Message, error) {
		sent = params
		return &models.Message{ID: 1, Chat: models.Chat{ID: chatIDFromAny(params.ChatID)}}, nil
	}

	writer := &telegramWriter{
		listener: listener,
		chatID:   42,
		userID:   "111",
		username: "alice",
	}
	if err := writer.WriteMessage(context.Background(), "**ok**"); err != nil {
		t.Fatalf("write message: %v", err)
	}

	if sent == nil {
		t.Fatal("expected send message call")
	}
	if sent.ParseMode != models.ParseModeHTML {
		t.Fatalf("expected ParseModeHTML, got %q", sent.ParseMode)
	}
	if sent.Text != "<b>ok</b>" {
		t.Fatalf("unexpected formatted text: %q", sent.Text)
	}
}

func TestTelegramWriterWriteMessage_FormatterFailureFallsBackToPlain(t *testing.T) {
	original := telegramMarkdown
	telegramMarkdown = nil
	defer func() {
		telegramMarkdown = original
	}()

	listener := NewTelegram("token", "")
	var sent *bot.SendMessageParams
	listener.sendMessage = func(_ context.Context, params *bot.SendMessageParams) (*models.Message, error) {
		sent = params
		return &models.Message{ID: 1, Chat: models.Chat{ID: chatIDFromAny(params.ChatID)}}, nil
	}

	writer := &telegramWriter{
		listener: listener,
		chatID:   42,
	}
	if err := writer.WriteMessage(context.Background(), "**ok**"); err != nil {
		t.Fatalf("write message: %v", err)
	}

	if sent == nil {
		t.Fatal("expected send message call")
	}
	if sent.ParseMode != "" {
		t.Fatalf("expected empty parse mode on formatter failure, got %q", sent.ParseMode)
	}
	if sent.Text != "**ok**" {
		t.Fatalf("expected plain fallback text, got %q", sent.Text)
	}
}

func TestTelegramSendChatMessage_DoesNotSetParseMode(t *testing.T) {
	listener := NewTelegram("token", "")

	var sent *bot.SendMessageParams
	listener.sendMessage = func(_ context.Context, params *bot.SendMessageParams) (*models.Message, error) {
		sent = params
		return &models.Message{ID: 1, Chat: models.Chat{ID: chatIDFromAny(params.ChatID)}}, nil
	}

	if err := listener.sendChatMessage(context.Background(), 42, "**ok**"); err != nil {
		t.Fatalf("send chat message: %v", err)
	}
	if sent == nil {
		t.Fatal("expected send message call")
	}
	if sent.ParseMode != "" {
		t.Fatalf("expected empty parse mode for plain chat send, got %q", sent.ParseMode)
	}
	if sent.Text != "**ok**" {
		t.Fatalf("unexpected plain text send content: %q", sent.Text)
	}
}

func TestTelegramListenerSend_UsesHTMLParseMode(t *testing.T) {
	listener := NewTelegram("token", "")
	listener.setActiveApprovalTarget("111", "alice", 42)

	var sent *bot.SendMessageParams
	listener.sendMessage = func(_ context.Context, params *bot.SendMessageParams) (*models.Message, error) {
		sent = params
		return &models.Message{ID: 1, Chat: models.Chat{ID: chatIDFromAny(params.ChatID)}}, nil
	}

	if err := listener.Send(context.Background(), "**ok**"); err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if sent == nil {
		t.Fatal("expected send call")
	}
	if sent.ParseMode != models.ParseModeHTML {
		t.Fatalf("expected ParseModeHTML, got %q", sent.ParseMode)
	}
	if sent.Text != "<b>ok</b>" {
		t.Fatalf("unexpected formatted text: %q", sent.Text)
	}
}

func TestTelegramChannelWriterWrite_UsesHTMLParseMode(t *testing.T) {
	listener := NewTelegram("token", "")

	var sent *bot.SendMessageParams
	listener.sendMessage = func(_ context.Context, params *bot.SendMessageParams) (*models.Message, error) {
		sent = params
		return &models.Message{ID: 1, Chat: models.Chat{ID: chatIDFromAny(params.ChatID)}}, nil
	}

	writer := listener.ChannelWriter(42)
	if _, err := writer.Write([]byte("**ok**")); err != nil {
		t.Fatalf("channel writer write failed: %v", err)
	}
	if sent == nil {
		t.Fatal("expected send call")
	}
	if sent.ParseMode != models.ParseModeHTML {
		t.Fatalf("expected ParseModeHTML, got %q", sent.ParseMode)
	}
	if sent.Text != "<b>ok</b>" {
		t.Fatalf("unexpected formatted text: %q", sent.Text)
	}
}

func TestTelegramPairSessionSubmitCodeWrongReturnsErrWrongCode(t *testing.T) {
	session := &TelegramPairSession{
		expectedCode: "123456",
	}

	err := session.SubmitCode(context.Background(), "000000")
	if !errors.Is(err, ErrWrongCode) {
		t.Fatalf("expected ErrWrongCode, got %v", err)
	}
}

func TestGenerateTelegramPairCode_IsSixDigits(t *testing.T) {
	re := regexp.MustCompile(`^\d{6}$`)
	for range 20 {
		code, err := generateTelegramPairCode()
		if err != nil {
			t.Fatalf("generate code: %v", err)
		}
		if !re.MatchString(code) {
			t.Fatalf("expected 6-digit code, got %q", code)
		}
	}
}

func TestTelegramListener_LoadAllowedUsersOnce(t *testing.T) {
	path := writeAllowedUsersFile(t, `{
  "users": [
    {"id":"111","channel":"telegram","username":"alice","name":"Alice","added_at":"2026-02-19T14:30:00Z"}
  ]
}
`)

	listener := NewTelegram("token", path)
	if err := listener.loadAllowedUsers(); err != nil {
		t.Fatalf("load users: %v", err)
	}

	// If per-message reads were happening, this replacement would revoke access.
	if err := store.WriteFile(path, []byte("{\"users\": []}\n")); err != nil {
		t.Fatalf("replace users file: %v", err)
	}

	handler := &telegramTestHandler{done: make(chan *runtime.Message, 2)}
	dispatcher, stop := startTestDispatcher(t, handler)
	defer stop()

	outbound := &outboundMessages{}
	configureTelegramSendCapture(listener, outbound)
	listener.handleInboundMessage(
		context.Background(),
		dispatcher,
		&models.Message{
			From: &models.User{ID: 111, Username: "alice"},
			Chat: models.Chat{ID: 10},
			Text: "hello",
		},
	)

	select {
	case <-handler.done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected authorized message to be dispatched")
	}
}

func TestTelegramListener_UnauthorizedUserIsDropped(t *testing.T) {
	path := writeAllowedUsersFile(t, `{
  "users": [
    {"id":"222","channel":"telegram","username":"bob","name":"Bob","added_at":"2026-02-19T14:30:00Z"}
  ]
}
`)

	listener := NewTelegram("token", path)
	if err := listener.loadAllowedUsers(); err != nil {
		t.Fatalf("load users: %v", err)
	}

	handler := &telegramTestHandler{done: make(chan *runtime.Message, 2)}
	dispatcher, stop := startTestDispatcher(t, handler)
	defer stop()

	outbound := &outboundMessages{}
	configureTelegramSendCapture(listener, outbound)
	listener.handleInboundMessage(
		context.Background(),
		dispatcher,
		&models.Message{
			From: &models.User{ID: 111, Username: "alice"},
			Chat: models.Chat{ID: 10},
			Text: "hello",
		},
	)

	select {
	case msg := <-handler.done:
		t.Fatalf("expected no handler call for unauthorized user, got %#v", msg)
	case <-time.After(80 * time.Millisecond):
	}
	if len(outbound.messages) != 0 {
		t.Fatalf("expected no outbound messages, got %#v", outbound.messages)
	}
}

func TestTelegramListener_HelpCommandHandledByCommandsHandler(t *testing.T) {
	path := writeAllowedUsersFile(t, `{
  "users": [
    {"id":"111","channel":"telegram","username":"alice","name":"Alice","added_at":"2026-02-19T14:30:00Z"}
  ]
}
`)

	listener := NewTelegram("token", path)
	if err := listener.loadAllowedUsers(); err != nil {
		t.Fatalf("load users: %v", err)
	}

	next := &telegramTestHandler{done: make(chan *runtime.Message, 2)}
	router := commands.Router{
		Commands: commands.New(nil, nil, nil, 0, 0),
		Next:     next,
	}
	dispatcher, stop := startTestDispatcher(t, router)
	defer stop()

	outbound := &outboundMessages{}
	configureTelegramSendCapture(listener, outbound)
	listener.handleInboundMessage(
		context.Background(),
		dispatcher,
		&models.Message{
			From: &models.User{ID: 111, Username: "alice"},
			Chat: models.Chat{ID: 10},
			Text: "/help",
		},
	)

	select {
	case <-next.done:
		t.Fatal("expected /help to be handled before agent dispatch")
	case <-time.After(80 * time.Millisecond):
	}
	if len(outbound.messages) != 1 {
		t.Fatalf("expected one command response, got %#v", outbound.messages)
	}
	if !strings.Contains(outbound.messages[0], "Commands: /help") {
		t.Fatalf("unexpected /help response: %q", outbound.messages[0])
	}
}

func TestTelegramListener_UnknownSlashFallsThroughToAgent(t *testing.T) {
	path := writeAllowedUsersFile(t, `{
  "users": [
    {"id":"111","channel":"telegram","username":"alice","name":"Alice","added_at":"2026-02-19T14:30:00Z"}
  ]
}
`)

	listener := NewTelegram("token", path)
	if err := listener.loadAllowedUsers(); err != nil {
		t.Fatalf("load users: %v", err)
	}

	next := &telegramTestHandler{done: make(chan *runtime.Message, 2)}
	router := commands.Router{
		Commands: commands.New(nil, nil, nil, 0, 0),
		Next:     next,
	}
	dispatcher, stop := startTestDispatcher(t, router)
	defer stop()

	outbound := &outboundMessages{}
	configureTelegramSendCapture(listener, outbound)
	listener.handleInboundMessage(
		context.Background(),
		dispatcher,
		&models.Message{
			From: &models.User{ID: 111, Username: "alice"},
			Chat: models.Chat{ID: 10},
			Text: "/doesnotexist",
		},
	)

	select {
	case msg := <-next.done:
		if msg.Text != "/doesnotexist" {
			t.Fatalf("unexpected dispatched text: %q", msg.Text)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected unknown slash command to be dispatched to agent")
	}
}

func TestTelegramListener_EnqueueIsNonBlocking(t *testing.T) {
	path := writeAllowedUsersFile(t, `{
  "users": [
    {"id":"111","channel":"telegram","username":"alice","name":"Alice","added_at":"2026-02-19T14:30:00Z"}
  ]
}
`)

	listener := NewTelegram("token", path)
	if err := listener.loadAllowedUsers(); err != nil {
		t.Fatalf("load users: %v", err)
	}

	block := make(chan struct{})
	handler := &telegramBlockingHandler{block: block}
	dispatcher, stop := startTestDispatcher(t, handler)
	defer stop()

	done := make(chan struct{})
	start := time.Now()
	go func() {
		configureTelegramSendCapture(listener, &outboundMessages{})
		listener.handleInboundMessage(
			context.Background(),
			dispatcher,
			&models.Message{
				From: &models.User{ID: 111, Username: "alice"},
				Chat: models.Chat{ID: 10},
				Text: "hello",
			},
		)
		close(done)
	}()

	select {
	case <-done:
		if time.Since(start) > 100*time.Millisecond {
			t.Fatalf("enqueue unexpectedly slow: %s", time.Since(start))
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected enqueue path to return quickly")
	}
	close(block)
}

func TestTelegramApprovalHandler_SendsTypingForNonSlash(t *testing.T) {
	listener := NewTelegram("token", "")
	actionCalls := make(chan *bot.SendChatActionParams, 1)
	listener.sendChatAction = func(_ context.Context, params *bot.SendChatActionParams) (bool, error) {
		select {
		case actionCalls <- params:
		default:
		}
		return true, nil
	}

	block := make(chan struct{})
	handler := &telegramApprovalHandler{
		listener: listener,
		handler:  &telegramBlockingHandler{block: block},
	}
	writer := &telegramWriter{
		listener: listener,
		chatID:   42,
		userID:   "111",
		username: "alice",
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- handler.HandleMessage(ctx, writer, &runtime.Message{Text: "hello"})
	}()

	select {
	case params := <-actionCalls:
		if got := chatIDFromAny(params.ChatID); got != 42 {
			t.Fatalf("unexpected typing chat id: %d", got)
		}
		if params.Action != models.ChatActionTyping {
			t.Fatalf("unexpected chat action: %q", params.Action)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected typing action for non-slash message")
	}

	close(block)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("handler failed: %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("handler did not complete")
	}
}

func TestTelegramApprovalHandler_DoesNotSendTypingForSlash(t *testing.T) {
	listener := NewTelegram("token", "")
	actionCalls := make(chan *bot.SendChatActionParams, 1)
	listener.sendChatAction = func(_ context.Context, params *bot.SendChatActionParams) (bool, error) {
		select {
		case actionCalls <- params:
		default:
		}
		return true, nil
	}

	block := make(chan struct{})
	handler := &telegramApprovalHandler{
		listener: listener,
		handler:  &telegramBlockingHandler{block: block},
	}
	writer := &telegramWriter{
		listener: listener,
		chatID:   42,
		userID:   "111",
		username: "alice",
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- handler.HandleMessage(ctx, writer, &runtime.Message{Text: "/help"})
	}()

	select {
	case <-actionCalls:
		t.Fatal("did not expect typing action for slash command")
	case <-time.After(120 * time.Millisecond):
	}

	close(block)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("handler failed: %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("handler did not complete")
	}
}

func TestTelegramListenerCurrentChannelID(t *testing.T) {
	listener := NewTelegram("token", "")
	if got := listener.CurrentChannelID(); got != "" {
		t.Fatalf("expected empty channel id without active target, got %q", got)
	}

	listener.setActiveApprovalTarget("111", "alice", 42)
	if got := listener.CurrentChannelID(); got != "telegram-42" {
		t.Fatalf("expected telegram-42, got %q", got)
	}
}

func TestMessagePreview_TruncatesToLimit(t *testing.T) {
	full := strings.Repeat("x", 120)
	got := messagePreview(full, 100)
	if len(got) != 100 {
		t.Fatalf("expected 100-char preview, got %d", len(got))
	}
}

func TestTelegramListenerRequestApproval_Approve(t *testing.T) {
	listener := NewTelegram("token", "")
	listener.setActiveApprovalTarget("111", "alice", 42)

	api := newMockTelegramAPI()
	listener.sendMessage = api.sendMessage
	listener.answerCallbackQuery = api.answerCallback
	listener.editMessageReplyMarkup = api.editReplyMarkup

	done := make(chan struct{})
	var decision approval.ApprovalDecision
	var err error
	go func() {
		decision, err = listener.RequestApproval(context.Background(), approval.ApprovalRequest{
			Tool:        "run_command",
			Description: "Run: ls -la",
		})
		close(done)
	}()

	sendParams := api.waitForSend(t)
	approveData, _ := callbackDataFromReplyMarkup(t, sendParams)

	listener.onApprovalApproveCallback(context.Background(), nil, &models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "callback-1",
			From: models.User{ID: 111, Username: "alice"},
			Data: approveData,
			Message: models.MaybeInaccessibleMessage{
				Message: &models.Message{
					ID:   500,
					Chat: models.Chat{ID: 42},
				},
			},
		},
	})

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("request approval did not complete")
	}
	if err != nil {
		t.Fatalf("request approval failed: %v", err)
	}
	if decision != approval.Approved {
		t.Fatalf("expected Approved, got %v", decision)
	}

	if len(api.answerCalls) != 1 {
		t.Fatalf("expected one answer callback call, got %d", len(api.answerCalls))
	}
	if api.answerCalls[0].CallbackQueryID != "callback-1" {
		t.Fatalf("unexpected callback id: %q", api.answerCalls[0].CallbackQueryID)
	}
	if len(api.editCalls) != 1 {
		t.Fatalf("expected one edit reply markup call, got %d", len(api.editCalls))
	}
	if api.editCalls[0].MessageID != 500 {
		t.Fatalf("unexpected message id: %d", api.editCalls[0].MessageID)
	}
}

func TestTelegramListenerRequestApproval_Deny(t *testing.T) {
	listener := NewTelegram("token", "")
	listener.setActiveApprovalTarget("111", "alice", 42)

	api := newMockTelegramAPI()
	listener.sendMessage = api.sendMessage
	listener.answerCallbackQuery = api.answerCallback
	listener.editMessageReplyMarkup = api.editReplyMarkup

	done := make(chan struct{})
	var decision approval.ApprovalDecision
	var err error
	go func() {
		decision, err = listener.RequestApproval(context.Background(), approval.ApprovalRequest{
			Tool:        "write_file",
			Description: "Write config.toml",
		})
		close(done)
	}()

	sendParams := api.waitForSend(t)
	_, denyData := callbackDataFromReplyMarkup(t, sendParams)

	listener.onApprovalDenyCallback(context.Background(), nil, &models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "callback-2",
			From: models.User{ID: 111, Username: "alice"},
			Data: denyData,
			Message: models.MaybeInaccessibleMessage{
				Message: &models.Message{
					ID:   700,
					Chat: models.Chat{ID: 42},
				},
			},
		},
	})

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("request approval did not complete")
	}
	if err != nil {
		t.Fatalf("request approval failed: %v", err)
	}
	if decision != approval.Denied {
		t.Fatalf("expected Denied, got %v", decision)
	}

	if len(api.answerCalls) != 1 {
		t.Fatalf("expected one answer callback call, got %d", len(api.answerCalls))
	}
	if api.answerCalls[0].CallbackQueryID != "callback-2" {
		t.Fatalf("unexpected callback id: %q", api.answerCalls[0].CallbackQueryID)
	}
	if len(api.editCalls) != 1 {
		t.Fatalf("expected one edit reply markup call, got %d", len(api.editCalls))
	}
	if api.editCalls[0].MessageID != 700 {
		t.Fatalf("unexpected message id: %d", api.editCalls[0].MessageID)
	}
}

func TestTelegramListenerRequestApproval_ContextCanceledReturnsDenied(t *testing.T) {
	listener := NewTelegram("token", "")
	listener.setActiveApprovalTarget("111", "alice", 42)

	api := newMockTelegramAPI()
	listener.sendMessage = api.sendMessage
	listener.answerCallbackQuery = api.answerCallback
	listener.editMessageReplyMarkup = api.editReplyMarkup

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	var decision approval.ApprovalDecision
	var err error
	go func() {
		decision, err = listener.RequestApproval(ctx, approval.ApprovalRequest{
			Tool:        "run_command",
			Description: "Run: pwd",
		})
		close(done)
	}()

	api.waitForSend(t)
	cancel()

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("request approval did not return after cancellation")
	}
	if err != nil {
		t.Fatalf("expected nil error on cancellation, got %v", err)
	}
	if decision != approval.Denied {
		t.Fatalf("expected Denied on cancellation, got %v", decision)
	}
	if len(api.editCalls) != 1 {
		t.Fatalf("expected keyboard to be cleared once, got %d", len(api.editCalls))
	}
}

type telegramTestHandler struct {
	done chan *runtime.Message
}

func (h *telegramTestHandler) HandleMessage(ctx context.Context, w runtime.ResponseWriter, msg *runtime.Message) error {
	select {
	case h.done <- msg:
	default:
	}
	return w.WriteMessage(ctx, "ok")
}

type telegramBlockingHandler struct {
	block <-chan struct{}
}

func (h *telegramBlockingHandler) HandleMessage(context.Context, runtime.ResponseWriter, *runtime.Message) error {
	<-h.block
	return nil
}

type outboundMessages struct {
	messages []string
}

func (o *outboundMessages) append(text string) {
	o.messages = append(o.messages, text)
}

func startTestDispatcher(t *testing.T, handler runtime.Handler) (*runtime.Dispatcher, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	dispatcher := runtime.NewDispatcher(handler, defaultDispatchQueue)
	if err := dispatcher.Start(ctx); err != nil {
		cancel()
		t.Fatalf("start dispatcher: %v", err)
	}
	return dispatcher, func() {
		cancel()
		dispatcher.Wait()
	}
}

func writeAllowedUsersFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "allowed_users.json")
	if err := store.WriteFile(path, []byte(content)); err != nil {
		t.Fatalf("write allowed users file: %v", err)
	}
	return path
}

type mockTelegramAPI struct {
	mu          sync.Mutex
	sendCalls   []*bot.SendMessageParams
	answerCalls []*bot.AnswerCallbackQueryParams
	editCalls   []*bot.EditMessageReplyMarkupParams
	sendSignal  chan struct{}
}

func newMockTelegramAPI() *mockTelegramAPI {
	return &mockTelegramAPI{
		sendSignal: make(chan struct{}, 10),
	}
}

func (m *mockTelegramAPI) sendMessage(_ context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	m.mu.Lock()
	m.sendCalls = append(m.sendCalls, params)
	m.mu.Unlock()
	select {
	case m.sendSignal <- struct{}{}:
	default:
	}
	return &models.Message{
		ID:   1,
		Chat: models.Chat{ID: chatIDFromAny(params.ChatID)},
	}, nil
}

func (m *mockTelegramAPI) answerCallback(_ context.Context, params *bot.AnswerCallbackQueryParams) (bool, error) {
	m.mu.Lock()
	m.answerCalls = append(m.answerCalls, params)
	m.mu.Unlock()
	return true, nil
}

func (m *mockTelegramAPI) editReplyMarkup(_ context.Context, params *bot.EditMessageReplyMarkupParams) (*models.Message, error) {
	m.mu.Lock()
	m.editCalls = append(m.editCalls, params)
	m.mu.Unlock()
	return &models.Message{
		ID:   params.MessageID,
		Chat: models.Chat{ID: chatIDFromAny(params.ChatID)},
	}, nil
}

func (m *mockTelegramAPI) waitForSend(t *testing.T) *bot.SendMessageParams {
	t.Helper()
	select {
	case <-m.sendSignal:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("timed out waiting for send message call")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sendCalls) == 0 {
		t.Fatal("expected at least one send message call")
	}
	return m.sendCalls[len(m.sendCalls)-1]
}

func callbackDataFromReplyMarkup(t *testing.T, params *bot.SendMessageParams) (string, string) {
	t.Helper()
	markup, ok := params.ReplyMarkup.(*models.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf("expected inline keyboard markup, got %T", params.ReplyMarkup)
	}
	if len(markup.InlineKeyboard) == 0 || len(markup.InlineKeyboard[0]) < 2 {
		t.Fatalf("expected two inline keyboard buttons, got %#v", markup.InlineKeyboard)
	}
	return markup.InlineKeyboard[0][0].CallbackData, markup.InlineKeyboard[0][1].CallbackData
}

func chatIDFromAny(chatID any) int64 {
	switch v := chatID.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return 0
	}
}

func configureTelegramSendCapture(listener *TelegramListener, outbound *outboundMessages) {
	listener.sendMessage = func(_ context.Context, params *bot.SendMessageParams) (*models.Message, error) {
		if outbound != nil {
			outbound.append(params.Text)
		}
		return &models.Message{
			ID:   1,
			Chat: models.Chat{ID: chatIDFromAny(params.ChatID)},
		}, nil
	}
	listener.answerCallbackQuery = func(context.Context, *bot.AnswerCallbackQueryParams) (bool, error) {
		return true, nil
	}
	listener.editMessageReplyMarkup = func(_ context.Context, params *bot.EditMessageReplyMarkupParams) (*models.Message, error) {
		return &models.Message{
			ID:   params.MessageID,
			Chat: models.Chat{ID: chatIDFromAny(params.ChatID)},
		}, nil
	}
	listener.sendChatAction = func(context.Context, *bot.SendChatActionParams) (bool, error) {
		return true, nil
	}
}
