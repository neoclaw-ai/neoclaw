// Package costs tracks LLM usage and spend in a JSONL log.
package costs

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/machinae/betterclaw/internal/store"
)

// Record is one persisted usage entry.
type Record struct {
	Timestamp    time.Time `json:"timestamp"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	TotalTokens  int       `json:"total_tokens"`
	CostUSD      float64   `json:"cost_usd"`
}

// Spend holds aggregated spend totals in USD.
type Spend struct {
	TodayUSD float64
	MonthUSD float64
}

// Tracker appends usage records and computes period spend totals.
type Tracker struct {
	path string
	mu   sync.Mutex
}

// New returns a Tracker for the configured costs JSONL path.
func New(path string) *Tracker {
	return &Tracker{path: path}
}

// Append writes one usage record to the JSONL file.
func (t *Tracker) Append(ctx context.Context, rec Record) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	if t.path == "" {
		return errors.New("costs path is required")
	}
	if rec.Timestamp.IsZero() {
		rec.Timestamp = time.Now()
	}
	encoded, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal costs record: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := store.AppendFile(t.path, encoded); err != nil {
		return fmt.Errorf("append costs record: %w", err)
	}

	return nil
}

// Spend returns today's and this month's spend totals in USD.
func (t *Tracker) Spend(ctx context.Context, now time.Time) (Spend, error) {
	totals := Spend{}

	if err := ctx.Err(); err != nil {
		return Spend{}, err
	}
	if t.path == "" {
		return Spend{}, errors.New("costs path is required")
	}
	if now.IsZero() {
		now = time.Now()
	}

	content, err := store.ReadFile(t.path)
	if errors.Is(err, os.ErrNotExist) {
		return totals, nil
	}
	if err != nil {
		return Spend{}, fmt.Errorf("read costs file: %w", err)
	}

	nowLocal := now.In(time.Local)
	todayYear, todayMonth, todayDay := nowLocal.Date()

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return Spend{}, err
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		recLocal := rec.Timestamp.In(time.Local)
		y, m, d := recLocal.Date()
		if y == todayYear && m == todayMonth {
			totals.MonthUSD += rec.CostUSD
			if d == todayDay {
				totals.TodayUSD += rec.CostUSD
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Spend{}, fmt.Errorf("scan costs file: %w", err)
	}

	return totals, nil
}
