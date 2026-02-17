package agent

import "github.com/machinae/betterclaw/internal/llm"

func appendUserMessage(history []llm.ChatMessage, text string) []llm.ChatMessage {
	next := append([]llm.ChatMessage{}, history...)
	next = append(next, llm.ChatMessage{
		Role:    llm.RoleUser,
		Content: text,
	})
	return next
}
