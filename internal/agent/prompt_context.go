package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/machinae/betterclaw/internal/memory"
)

const autoRememberInstruction = "When you learn something important about the user (preferences, facts, relationships, ongoing constraints), write it to memory using memory_append without asking permission."
const maxSoulChars = 4000

// BuildSystemPrompt assembles the runtime system prompt from base instructions,
// SOUL.md, and long-term memory.
func BuildSystemPrompt(agentDir string, store *memory.Store) (string, error) {
	if strings.TrimSpace(agentDir) == "" {
		return "", errors.New("agent directory is required")
	}
	if store == nil {
		return "", errors.New("memory store is required")
	}

	prompt := DefaultSystemPrompt + "\n\n" + autoRememberInstruction

	soulPath := filepath.Join(agentDir, "SOUL.md")
	soulText, err := readOptionalFile(soulPath)
	if err != nil {
		return "", err
	}
	memoryText, err := store.LoadContext()
	if err != nil {
		return "", err
	}

	if soulText == "" && memoryText == "" {
		return prompt, nil
	}

	var b strings.Builder
	b.WriteString(prompt)
	b.WriteString("\n\nContext:\n")
	if soulText != "" {
		truncatedSoul, truncated := truncateStringByChars(soulText, maxSoulChars)
		b.WriteString("\n[SOUL.md]\n")
		b.WriteString(truncatedSoul)
		if !strings.HasSuffix(truncatedSoul, "\n") {
			b.WriteByte('\n')
		}
		if truncated {
			b.WriteString(fmt.Sprintf("[SOUL.md truncated to %d chars]\n", maxSoulChars))
		}
	}
	if memoryText != "" {
		b.WriteString("\n[Long-term memory]\n")
		b.WriteString(memoryText)
		if !strings.HasSuffix(memoryText, "\n") {
			b.WriteByte('\n')
		}
	}

	return b.String(), nil
}

func readOptionalFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err == nil {
		return string(raw), nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return "", fmt.Errorf("read %q: %w", path, err)
}

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
