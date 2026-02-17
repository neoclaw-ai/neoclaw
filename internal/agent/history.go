package agent

import providerapi "github.com/machinae/betterclaw/internal/provider"

func appendUserMessage(history []providerapi.ChatMessage, text string) []providerapi.ChatMessage {
	next := append([]providerapi.ChatMessage{}, history...)
	next = append(next, providerapi.ChatMessage{
		Role:    providerapi.RoleUser,
		Content: text,
	})
	return next
}
