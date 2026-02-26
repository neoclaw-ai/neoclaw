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
	"github.com/neoclaw-ai/neoclaw/internal/logging"
	"github.com/neoclaw-ai/neoclaw/internal/memory"
	"github.com/neoclaw-ai/neoclaw/internal/store"
)

const defaultDailyLogLookback = 12 * time.Hour

// BuildSystemPrompt assembles the runtime system prompt from base instructions,
// SOUL.md, USER.md, long-term memory, and recent daily log entries.
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

	var promptBuilder strings.Builder
	promptBuilder.WriteString(DefaultSystemPrompt)
	promptBuilder.WriteString("\n\n")
	promptBuilder.WriteString(autoRememberInstruction)
	if timeLine := currentTimeContextLine(now); timeLine != "" {
		promptBuilder.WriteString("\n\n")
		promptBuilder.WriteString(timeLine)
		promptBuilder.WriteString("\n")
		promptBuilder.WriteString(resolveRelativeTimeInstruction)
	}
	prompt := promptBuilder.String()

	soulPath := filepath.Join(agentDir, config.SoulFilePath)
	soulText, soulExists, err := readOptionalFile(soulPath)
	if err != nil {
		return "", err
	}
	if !soulExists {
		logging.Logger().Warn("missing SOUL.md; continuing without soul context", "path", soulPath)
	}
	userPath := filepath.Join(agentDir, config.UserFilePath)
	userText, userExists, err := readOptionalFile(userPath)
	if err != nil {
		return "", err
	}
	if !userExists {
		logging.Logger().Warn("missing USER.md; continuing without user context", "path", userPath)
	}

	memoryText, err := store.LoadContext()
	if err != nil {
		return "", err
	}
	recentLogs, err := store.GetDailyLogs(now.Add(-dailyLogLookback), now)
	if err != nil {
		return "", err
	}

	if soulText == "" && userText == "" && memoryText == "" && len(recentLogs) == 0 {
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
	if userText != "" {
		b.WriteString("\n[USER.md]\n")
		b.WriteString(userText)
		if !strings.HasSuffix(userText, "\n") {
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

// currentTimeContextLine returns a one-line current-time context string.
func currentTimeContextLine(now time.Time) string {
	if now.IsZero() {
		return ""
	}
	timestamp := now.Format(time.RFC3339)
	if strings.TrimSpace(timestamp) == "" {
		return ""
	}
	locationName := ""
	if loc := now.Location(); loc != nil {
		locationName = strings.TrimSpace(loc.String())
	}
	if locationName == "" {
		return fmt.Sprintf("Current time: %s", timestamp)
	}
	return fmt.Sprintf("Current time: %s (%s)", timestamp, locationName)
}

func readOptionalFile(path string) (text string, exists bool, err error) {
	content, err := store.ReadFile(path)
	if err == nil {
		return content, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	return "", false, fmt.Errorf("read %s: %w", path, err)
}
