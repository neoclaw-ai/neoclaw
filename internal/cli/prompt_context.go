package cli

import (
	"errors"
	"strings"
	"time"

	"github.com/machinae/betterclaw/internal/agent"
	"github.com/machinae/betterclaw/internal/memory"
)

const autoRememberInstruction = "When you learn something important about the user (preferences, facts, relationships, ongoing constraints), write it to memory using memory_append without asking permission."

func buildSystemPrompt(store *memory.Store, now time.Time) (string, error) {
	if store == nil {
		return "", errors.New("memory store is required")
	}

	prompt := agent.DefaultSystemPrompt + "\n\n" + autoRememberInstruction

	memoryText, dailyText, err := store.LoadContext(now)
	if err != nil {
		return "", err
	}

	if memoryText == "" && dailyText == "" {
		return prompt, nil
	}

	var b strings.Builder
	b.WriteString(prompt)
	b.WriteString("\n\nContext:\n")
	if memoryText != "" {
		b.WriteString("\n[Long-term memory]\n")
		b.WriteString(memoryText)
		if !strings.HasSuffix(memoryText, "\n") {
			b.WriteByte('\n')
		}
	}
	if dailyText != "" {
		b.WriteString("\n[Today's daily log]\n")
		b.WriteString(dailyText)
		if !strings.HasSuffix(dailyText, "\n") {
			b.WriteByte('\n')
		}
	}

	return b.String(), nil
}
