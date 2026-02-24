package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/neoclaw-ai/neoclaw/internal/config"
	"github.com/neoclaw-ai/neoclaw/internal/memory"
	"github.com/neoclaw-ai/neoclaw/internal/store"
)

const defaultDailyLogLookback = 24 * time.Hour

// BuildSystemPrompt assembles the runtime system prompt from base instructions,
// SOUL.md, long-term memory, and recent daily log entries.
func BuildSystemPrompt(agentDir string, store *memory.Store, contextCfg config.ContextConfig) (string, error) {
	return buildSystemPromptAt(agentDir, store, time.Now(), contextCfg)
}

func buildSystemPromptAt(agentDir string, store *memory.Store, now time.Time, contextCfg config.ContextConfig) (string, error) {
	if strings.TrimSpace(agentDir) == "" {
		return "", errors.New("agent directory is required")
	}
	if store == nil {
		return "", errors.New("memory store is required")
	}
	dailyLogLookback := contextCfg.DailyLogLookback
	if dailyLogLookback <= 0 {
		dailyLogLookback = defaultDailyLogLookback
	}

	prompt := DefaultSystemPrompt + "\n\n" + autoRememberInstruction

	soulPath := filepath.Join(agentDir, config.SoulFilePath)
	soulText, err := readOptionalFile(soulPath)
	if err != nil {
		return "", err
	}
	memoryText, err := store.LoadContext()
	if err != nil {
		return "", err
	}
	recentLogs, err := store.GetDailyLogs(now.Add(-dailyLogLookback), now)
	if err != nil {
		return "", err
	}

	if soulText == "" && memoryText == "" && len(recentLogs) == 0 {
		return prompt, nil
	}

	var b strings.Builder
	b.WriteString(prompt)
	b.WriteString("\n\nContext:\n")
	if soulText != "" {
		b.WriteString("\n[SOUL.md]\n")
		b.WriteString(soulText)
		if !strings.HasSuffix(soulText, "\n") {
			b.WriteByte('\n')
		}
	}
	if memoryText != "" {
		b.WriteString("\n[Long-term memory]\n")
		b.WriteString(memoryText)
		if !strings.HasSuffix(memoryText, "\n") {
			b.WriteByte('\n')
		}
	}
	if len(recentLogs) > 0 {
		b.WriteString("\n[Recent daily log]\n")
		for _, entry := range recentLogs {
			b.WriteString("- ")
			b.WriteString(entry.Timestamp.Format(time.RFC3339))
			b.WriteString(": ")
			b.WriteString(entry.Entry)
			b.WriteByte('\n')
		}
	}

	return b.String(), nil
}

// truncateStringByChars truncates s to at most maxChars Unicode code points,
// returning the truncated string and whether truncation occurred.
func truncateStringByChars(s string, maxChars int) (string, bool) {
	if maxChars <= 0 {
		return "", len(s) > 0
	}
	if utf8.RuneCountInString(s) <= maxChars {
		return s, false
	}
	var b strings.Builder
	b.Grow(len(s))
	charCount := 0
	for _, r := range s {
		if charCount >= maxChars {
			break
		}
		b.WriteRune(r)
		charCount++
	}
	return b.String(), true
}

func readOptionalFile(path string) (string, error) {
	content, err := store.ReadFile(path)
	if err == nil {
		return content, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return "", fmt.Errorf("read %s: %w", path, err)
}
