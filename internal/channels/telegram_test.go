package channels

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/machinae/betterclaw/internal/commands"
	"github.com/machinae/betterclaw/internal/runtime"
	"github.com/machinae/betterclaw/internal/store"
)

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

	listener := NewTelegram("token", path, nil)
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
	listener.handleInboundMessage(
		context.Background(),
		dispatcher,
		&models.Message{
			From: &models.User{ID: 111, Username: "alice"},
			Chat: models.Chat{ID: 10},
			Text: "hello",
		},
		outbound.send,
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

	listener := NewTelegram("token", path, nil)
	if err := listener.loadAllowedUsers(); err != nil {
		t.Fatalf("load users: %v", err)
	}

	handler := &telegramTestHandler{done: make(chan *runtime.Message, 2)}
	dispatcher, stop := startTestDispatcher(t, handler)
	defer stop()

	outbound := &outboundMessages{}
	listener.handleInboundMessage(
		context.Background(),
		dispatcher,
		&models.Message{
			From: &models.User{ID: 111, Username: "alice"},
			Chat: models.Chat{ID: 10},
			Text: "hello",
		},
		outbound.send,
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

	listener := NewTelegram("token", path, commands.New(nil, nil, nil, 0, 0))
	if err := listener.loadAllowedUsers(); err != nil {
		t.Fatalf("load users: %v", err)
	}

	handler := &telegramTestHandler{done: make(chan *runtime.Message, 2)}
	dispatcher, stop := startTestDispatcher(t, handler)
	defer stop()

	outbound := &outboundMessages{}
	listener.handleInboundMessage(
		context.Background(),
		dispatcher,
		&models.Message{
			From: &models.User{ID: 111, Username: "alice"},
			Chat: models.Chat{ID: 10},
			Text: "/help",
		},
		outbound.send,
	)

	select {
	case <-handler.done:
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

	listener := NewTelegram("token", path, commands.New(nil, nil, nil, 0, 0))
	if err := listener.loadAllowedUsers(); err != nil {
		t.Fatalf("load users: %v", err)
	}

	handler := &telegramTestHandler{done: make(chan *runtime.Message, 2)}
	dispatcher, stop := startTestDispatcher(t, handler)
	defer stop()

	outbound := &outboundMessages{}
	listener.handleInboundMessage(
		context.Background(),
		dispatcher,
		&models.Message{
			From: &models.User{ID: 111, Username: "alice"},
			Chat: models.Chat{ID: 10},
			Text: "/doesnotexist",
		},
		outbound.send,
	)

	select {
	case msg := <-handler.done:
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

	listener := NewTelegram("token", path, nil)
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
		listener.handleInboundMessage(
			context.Background(),
			dispatcher,
			&models.Message{
				From: &models.User{ID: 111, Username: "alice"},
				Chat: models.Chat{ID: 10},
				Text: "hello",
			},
			func(context.Context, string) error { return nil },
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

func TestMessagePreview_TruncatesToLimit(t *testing.T) {
	full := strings.Repeat("x", 120)
	got := messagePreview(full, 100)
	if len(got) != 100 {
		t.Fatalf("expected 100-char preview, got %d", len(got))
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

func (o *outboundMessages) send(_ context.Context, text string) error {
	o.messages = append(o.messages, text)
	return nil
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
	path := t.TempDir() + "/allowed_users.json"
	if err := store.WriteFile(path, []byte(content)); err != nil {
		t.Fatalf("write allowed users file: %v", err)
	}
	return path
}
