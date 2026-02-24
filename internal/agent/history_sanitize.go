package agent

import "github.com/neoclaw-ai/neoclaw/internal/provider"

// sanitizeToolTurns normalizes tool-use/tool-result sequencing for Anthropic.
func sanitizeToolTurns(messages []provider.ChatMessage) ([]provider.ChatMessage, bool) {
	if len(messages) == 0 {
		return []provider.ChatMessage{}, false
	}

	out := make([]provider.ChatMessage, 0, len(messages))
	changed := false

	for i := 0; i < len(messages); i++ {
		msg := messages[i]

		// Orphan tool results are invalid and must be dropped.
		if msg.Role == provider.RoleTool {
			changed = true
			continue
		}

		if msg.Role != provider.RoleAssistant || len(msg.ToolCalls) == 0 {
			out = append(out, msg)
			continue
		}

		j := i + 1
		for j < len(messages) && messages[j].Role == provider.RoleTool {
			j++
		}

		if j == i+1 {
			assistant := msg
			assistant.ToolCalls = nil
			out = append(out, assistant)
			changed = true
			continue
		}

		resultsByID := make(map[string][]provider.ChatMessage, len(msg.ToolCalls))
		resultOrder := make([]string, 0, len(msg.ToolCalls))
		for k := i + 1; k < j; k++ {
			id := messages[k].ToolCallID
			if id == "" {
				changed = true
				continue
			}
			if _, ok := resultsByID[id]; !ok {
				resultOrder = append(resultOrder, id)
			}
			resultsByID[id] = append(resultsByID[id], messages[k])
		}

		filteredCalls := make([]provider.ToolCall, 0, len(msg.ToolCalls))
		validCallIDs := make(map[string]struct{}, len(msg.ToolCalls))
		for _, call := range msg.ToolCalls {
			if len(resultsByID[call.ID]) == 0 {
				changed = true
				continue
			}
			filteredCalls = append(filteredCalls, call)
			validCallIDs[call.ID] = struct{}{}
		}

		assistant := msg
		assistant.ToolCalls = filteredCalls
		out = append(out, assistant)

		for _, id := range resultOrder {
			if _, ok := validCallIDs[id]; !ok {
				changed = true
				continue
			}
			out = append(out, resultsByID[id]...)
		}

		if len(filteredCalls) != len(msg.ToolCalls) {
			changed = true
		}
		i = j - 1
	}

	return out, changed
}

