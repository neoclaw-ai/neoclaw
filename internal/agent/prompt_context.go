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

	var promptBuilder strings.Builder
	promptBuilder.WriteString(DefaultSystemPrompt)
	promptBuilder.WriteString("\n\n")
	promptBuilder.WriteString(toolGuidance)
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

	activeFacts := store.ActiveFacts(now)
	dates := lookbackDates(now, contextCfg.DailyLogLookbackDays)
	dailyLogsByDate := make(map[string][]memory.LogEntry, len(dates))
	hasDailyLogs := false
	for _, date := range dates {
		key := date.In(time.Local).Format("2006-01-02")
		entries := store.DailyLogsByDate([]time.Time{date})
		dailyLogsByDate[key] = entries
		if len(entries) > 0 {
			hasDailyLogs = true
		}
	}

	includedFiles := map[string]int{}
	if soulText != "" {
		includedFiles[config.SoulFilePath] = estimateTokens(soulText, nil)
	}
	if userText != "" {
		includedFiles[config.UserFilePath] = estimateTokens(userText, nil)
	}
	if soulText == "" && userText == "" && len(activeFacts) == 0 && !hasDailyLogs {
		logging.Logger().Debug(
			"built system prompt",
			"included_files", includedFiles,
			"total_tokens", estimateTokens(prompt, nil),
		)
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
		b.WriteString("\n[User profile]\n")
		b.WriteString(userText)
		if !strings.HasSuffix(userText, "\n") {
			b.WriteByte('\n')
		}
	}
	if len(activeFacts) > 0 {
		var factsBlock strings.Builder
		factsBlock.WriteString("\n[Persistent facts]\n")
		factsBlock.WriteString("age\ttags\ttext\tkv\n")
		for _, entry := range activeFacts {
			factsBlock.WriteString(formatAge(now, entry.Timestamp))
			factsBlock.WriteByte('\t')
			factsBlock.WriteString(entry.FormatLLM())
			factsBlock.WriteByte('\n')
		}
		block := factsBlock.String()
		b.WriteString(block)
		includedFiles[config.MemoryFilePath] = estimateTokens(block, nil)
	}
	for _, date := range dates {
		dayKey := date.In(time.Local).Format("2006-01-02")
		entries := dailyLogsByDate[dayKey]
		if len(entries) == 0 {
			continue
		}
		var dayBlock strings.Builder
		dayBlock.WriteString("\n[Daily log — ")
		dayBlock.WriteString(dayKey)
		dayBlock.WriteString("]\n")
		dayBlock.WriteString("time\ttags\ttext\tkv\n")
		for _, entry := range entries {
			dayBlock.WriteString(entry.Timestamp.In(time.Local).Format("15:04"))
			dayBlock.WriteByte('\t')
			dayBlock.WriteString(entry.FormatLLM())
			dayBlock.WriteByte('\n')
		}
		block := dayBlock.String()
		b.WriteString(block)
		includedFiles[dayKey+".tsv"] = estimateTokens(block, nil)
	}
	systemPrompt := b.String()
	logging.Logger().Debug(
		"built system prompt",
		"included_files", includedFiles,
		"total_tokens", estimateTokens(systemPrompt, nil),
	)
	return systemPrompt, nil
}

// lookbackDates returns local calendar dates from most recent to oldest.
func lookbackDates(now time.Time, days int) []time.Time {
	if days <= 0 {
		return nil
	}
	dates := make([]time.Time, 0, days)
	base := now.In(time.Local)
	for i := 0; i < days; i++ {
		dates = append(dates, base.AddDate(0, 0, -i))
	}
	return dates
}

// formatAge formats the elapsed time using the largest supported unit.
func formatAge(now, then time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	age := now.Sub(then)
	if age < 0 {
		age = 0
	}
	switch {
	case age < time.Hour:
		return fmt.Sprintf("%dm", int(age/time.Minute))
	case age < 24*time.Hour:
		return fmt.Sprintf("%dh", int(age/time.Hour))
	case age < 30*24*time.Hour:
		return fmt.Sprintf("%dd", int(age/(24*time.Hour)))
	case age < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(age/(30*24*time.Hour)))
	default:
		return fmt.Sprintf("%dy", int(age/(365*24*time.Hour)))
	}
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
