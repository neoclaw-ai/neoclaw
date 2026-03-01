// Package costs tracks LLM usage and spend in a TSV log.
package costs

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/neoclaw-ai/neoclaw/internal/store"
)

// Record is one persisted usage entry.
type Record struct {
	Timestamp    time.Time
	Provider     string
	Model        string
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	CostUSD      float64
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

// New returns a Tracker for the configured costs TSV path.
func New(path string) *Tracker {
	return &Tracker{path: path}
}

// Append writes one usage record as a TSV line.
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
	line := fmt.Sprintf("%s\t%s\t%s\t%d\t%d\t%d\t%.8f\n",
		rec.Timestamp.Format(time.RFC3339),
		rec.Provider,
		rec.Model,
		rec.InputTokens,
		rec.OutputTokens,
		rec.TotalTokens,
		rec.CostUSD,
	)
	if err := store.AppendFile(t.path, []byte(line)); err != nil {
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
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "ts\t") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}
		ts, err := time.Parse(time.RFC3339, fields[0])
		if err != nil {
			continue
		}
		costUSD, err := strconv.ParseFloat(fields[6], 64)
		if err != nil {
			continue
		}
		recLocal := ts.In(time.Local)
		y, m, d := recLocal.Date()
		if y == todayYear && m == todayMonth {
			totals.MonthUSD += costUSD
			if d == todayDay {
				totals.TodayUSD += costUSD
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Spend{}, fmt.Errorf("scan costs file: %w", err)
	}

	return totals, nil
}
