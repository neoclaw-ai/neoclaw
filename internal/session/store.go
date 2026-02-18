package session

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/machinae/betterclaw/internal/provider"
)

// Store persists conversation history in a JSONL file.
type Store struct {
	path string
	mu   sync.Mutex
}

type record struct {
	Kind       string              `json:"kind,omitempty"`
	Role       provider.Role       `json:"role"`
	Content    string              `json:"content,omitempty"`
	ToolCallID string              `json:"tool_call_id,omitempty"`
	ToolCalls  []provider.ToolCall `json:"tool_calls,omitempty"`
}

// New creates a session store for one channel session file.
func New(path string) *Store {
	return &Store{path: path}
}

// Load reads all valid JSONL records from disk into chat messages.
// Malformed lines are skipped.
func (s *Store) Load(ctx context.Context) ([]provider.ChatMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.path == "" {
		return nil, errors.New("session path is required")
	}

	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []provider.ChatMessage{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	messages := make([]provider.ChatMessage, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec record
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		messages = append(messages, provider.ChatMessage{
			Kind:       rec.Kind,
			Role:       rec.Role,
			Content:    rec.Content,
			ToolCallID: rec.ToolCallID,
			ToolCalls:  rec.ToolCalls,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan session file: %w", err)
	}
	return messages, nil
}

// Append appends messages as JSONL records.
func (s *Store) Append(ctx context.Context, messages []provider.ChatMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	if len(messages) == 0 {
		return nil
	}
	if s == nil || s.path == "" {
		return errors.New("session path is required")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create session directory: %w", err)
	}

	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	for _, msg := range messages {
		if err := ctx.Err(); err != nil {
			return err
		}
		rec := record{
			Kind:       msg.Kind,
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
			ToolCalls:  msg.ToolCalls,
		}
		encoded, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("marshal session record: %w", err)
		}
		if _, err := f.Write(append(encoded, '\n')); err != nil {
			return fmt.Errorf("append session record: %w", err)
		}
	}
	return nil
}

// Rewrite replaces the session file with the provided message list.
func (s *Store) Rewrite(ctx context.Context, messages []provider.ChatMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.path == "" {
		return errors.New("session path is required")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create session directory: %w", err)
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	for _, msg := range messages {
		if err := ctx.Err(); err != nil {
			return err
		}
		rec := record{
			Kind:       msg.Kind,
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
			ToolCalls:  msg.ToolCalls,
		}
		encoded, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("marshal session record: %w", err)
		}
		if _, err := f.Write(append(encoded, '\n')); err != nil {
			return fmt.Errorf("rewrite session record: %w", err)
		}
	}
	return nil
}

// Reset clears all persisted session history.
func (s *Store) Reset(ctx context.Context) error {
	return s.Rewrite(ctx, nil)
}
