package agent

import "github.com/neoclaw-ai/neoclaw/internal/provider"

func appendUserMessage(history []provider.ChatMessage, text string) []provider.ChatMessage {
	next := append([]provider.ChatMessage{}, history...)
	next = append(next, provider.ChatMessage{
		Role:    provider.RoleUser,
		Content: text,
	})
	return next
}
