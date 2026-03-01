package costs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEstimateAnthropicUSD(t *testing.T) {
	t.Parallel()

	cases := []string{"claude-haiku-4-5", "claude-sonnet-4-6", "claude-opus-4-1"}
	for _, model := range cases {
		model := model
		t.Run(model, func(t *testing.T) {
			t.Parallel()
			usd, ok := EstimateAnthropicUSD(model, 1_000_000, 1_000_000)
			if !ok {
				t.Fatalf("expected fallback pricing for model %q", model)
			}
			if usd <= 0 {
				t.Fatalf("expected positive cost for model %q, got %.8f", model, usd)
			}
		})
	}

	if _, ok := EstimateAnthropicUSD("unknown-model", 10, 10); ok {
		t.Fatalf("expected unknown model to have no fallback pricing")
	}
}

func TestTrackerAppendAndSpend(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "costs.tsv")
	// Seed with header row like bootstrap does.
	if err := os.WriteFile(path, []byte("ts\tprovider\tmodel\tinput_tokens\toutput_tokens\ttotal_tokens\tcost_usd\n"), 0o644); err != nil {
		t.Fatalf("seed header: %v", err)
	}

	tracker := New(path)
	now := time.Date(2026, 2, 19, 12, 0, 0, 0, time.Local)

	if err := tracker.Append(context.Background(), Record{
		Timestamp:    now.Add(-1 * time.Hour),
		Provider:     "anthropic",
		Model:        "claude-sonnet-4-6",
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		CostUSD:      1.25,
	}); err != nil {
		t.Fatalf("append first record: %v", err)
	}

	if err := tracker.Append(context.Background(), Record{
		Timestamp:    now.AddDate(0, 0, -1),
		Provider:     "anthropic",
		Model:        "claude-sonnet-4-6",
		InputTokens:  50,
		OutputTokens: 25,
		TotalTokens:  75,
		CostUSD:      0.75,
	}); err != nil {
		t.Fatalf("append second record: %v", err)
	}

	spend, err := tracker.Spend(context.Background(), now)
	if err != nil {
		t.Fatalf("compute spend: %v", err)
	}
	if spend.TodayUSD != 1.25 {
		t.Fatalf("expected today spend 1.25, got %.2f", spend.TodayUSD)
	}
	if spend.MonthUSD != 2.00 {
		t.Fatalf("expected month spend 2.00, got %.2f", spend.MonthUSD)
	}
}

func TestTrackerSpendSkipsMalformedLines(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "costs.tsv")
	content := strings.Join([]string{
		"ts\tprovider\tmodel\tinput_tokens\toutput_tokens\ttotal_tokens\tcost_usd",
		"not-a-timestamp\tanthropic\tclaude-sonnet-4-6\t1\t1\t2\t2.50",
		"2026-02-19T12:00:00Z\tanthropic\tclaude-sonnet-4-6\t1\t1\t2\t2.50",
		"",
	}, "\n")

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	tracker := New(path)
	spend, err := tracker.Spend(context.Background(), time.Date(2026, 2, 19, 13, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("compute spend: %v", err)
	}
	if spend.TodayUSD <= 0 || spend.MonthUSD <= 0 {
		t.Fatalf("expected positive spend from valid line, got today=%.2f month=%.2f", spend.TodayUSD, spend.MonthUSD)
	}
}
