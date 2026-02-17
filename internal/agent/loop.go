package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/llm"
	"github.com/machinae/betterclaw/internal/logging"
	"github.com/machinae/betterclaw/internal/tools"
)

const defaultMaxIterations = 10

// Run executes the agent loop until the model returns a final text response.
func Run(
	ctx context.Context,
	provider llm.Provider,
	registry *tools.Registry,
	approver approval.Approver,
	systemPrompt string,
	messages []llm.ChatMessage,
	maxIterations int,
) (*llm.ChatResponse, []llm.ChatMessage, error) {
	if provider == nil {
		return nil, nil, fmt.Errorf("provider is required")
	}
	if registry == nil {
		return nil, nil, fmt.Errorf("tool registry is required")
	}
	if maxIterations <= 0 {
		maxIterations = defaultMaxIterations
	}

	history := append([]llm.ChatMessage(nil), messages...)
	toolDefs := registry.ToolDefinitions()
	availableTools := toolNames(toolDefs)
	totalUsage := llm.TokenUsage{}

	for i := 0; i < maxIterations; i++ {
		// Each iteration sends the full conversation state and available tools.
		// The model either returns final text or a set of tool calls.
		logging.Logger().Info(
			"llm request",
			"iteration", i+1,
			"message_count", len(history),
			"tool_count", len(toolDefs),
			"latest_user_message", summarizeTextForLog(latestUserMessage(history), 300),
		)

		resp, err := provider.Chat(ctx, llm.ChatRequest{
			SystemPrompt: systemPrompt,
			Messages:     history,
			Tools:        toolDefs,
		})
		if err != nil {
			return nil, history, err
		}
		logging.Logger().Info(
			"llm response",
			"iteration", i+1,
			"tool_call_count", len(resp.ToolCalls),
			"input_tokens", resp.Usage.InputTokens,
			"output_tokens", resp.Usage.OutputTokens,
			"total_tokens", resp.Usage.TotalTokens,
		)

		totalUsage.InputTokens += resp.Usage.InputTokens
		totalUsage.OutputTokens += resp.Usage.OutputTokens
		totalUsage.TotalTokens += resp.Usage.TotalTokens

		if len(resp.ToolCalls) == 0 {
			// No tool calls means we are done for this turn.
			if resp.Content != "" {
				history = append(history, llm.ChatMessage{
					Role:    llm.RoleAssistant,
					Content: resp.Content,
				})
			}
			resp.Usage = totalUsage
			return resp, history, nil
		}

		history = append(history, llm.ChatMessage{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, call := range resp.ToolCalls {
			startedAt := time.Now()
			tool, ok := registry.Lookup(call.Name)
			if !ok {
				// Unknown tools are surfaced to the model as tool-result errors so
				// the loop can continue and the model can retry with a valid tool.
				logging.Logger().Warn(
					"tool call rejected: unknown tool",
					"tool", call.Name,
					"tool_call_id", call.ID,
					"arguments", call.Arguments,
					"available_tools", availableTools,
				)
				history = append(history, llm.ChatMessage{
					Role:       llm.RoleTool,
					ToolCallID: call.ID,
					Content: fmt.Sprintf(
						`tool execution error: unknown tool %q. Available tools: %s. Use an available tool name exactly.`,
						call.Name,
						availableTools,
					),
				})
				continue
			}

			args := map[string]any{}
			if call.Arguments != "" {
				if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
					logging.Logger().Warn(
						"tool call rejected: invalid arguments",
						"tool", call.Name,
						"tool_call_id", call.ID,
						"arguments", call.Arguments,
						"err", err,
					)
					history = append(history, llm.ChatMessage{
						Role:       llm.RoleTool,
						ToolCallID: call.ID,
						Content:    fmt.Sprintf("tool execution error: invalid tool arguments for %q: %v", call.Name, err),
					})
					continue
				}
			}

			logging.Logger().Info(
				"tool call start",
				"tool", call.Name,
				"tool_call_id", call.ID,
				"args", summarizeToolArgs(args),
			)

			// Approval and execution are coupled here so both policy errors and
			// runtime execution errors are returned to the model uniformly.
			result, err := approval.ExecuteTool(ctx, approver, tool, args, fmt.Sprintf("%s %s", call.Name, call.Arguments))
			if err != nil {
				logging.Logger().Warn(
					"tool call failed",
					"tool", call.Name,
					"tool_call_id", call.ID,
					"duration_ms", time.Since(startedAt).Milliseconds(),
					"err", err,
				)
				history = append(history, llm.ChatMessage{
					Role:       llm.RoleTool,
					ToolCallID: call.ID,
					Content:    fmt.Sprintf("tool execution error: %v", err),
				})
				continue
			}

			logging.Logger().Info(
				"tool call complete",
				"tool", call.Name,
				"tool_call_id", call.ID,
				"duration_ms", time.Since(startedAt).Milliseconds(),
			)

			history = append(history, llm.ChatMessage{
				Role:       llm.RoleTool,
				ToolCallID: call.ID,
				Content:    result.Output,
			})
		}
	}

	return nil, history, fmt.Errorf("max iterations exceeded (%d)", maxIterations)
}

func toolNames(defs []llm.ToolDefinition) string {
	if len(defs) == 0 {
		return "<none>"
	}
	names := make([]string, 0, len(defs))
	for _, d := range defs {
		names = append(names, d.Name)
	}
	return strings.Join(names, ", ")
}

func summarizeToolArgs(args map[string]any) map[string]any {
	out := make(map[string]any, len(args))
	for key, value := range args {
		out[key] = summarizeToolArgValue(value)
	}
	return out
}

func summarizeToolArgValue(value any) any {
	const maxLoggedStringLen = 200

	switch v := value.(type) {
	case string:
		if len(v) <= maxLoggedStringLen {
			return v
		}
		return fmt.Sprintf("%s...[truncated %d chars]", v[:maxLoggedStringLen], len(v)-maxLoggedStringLen)
	case []byte:
		return fmt.Sprintf("<bytes len=%d>", len(v))
	default:
		return value
	}
}

func latestUserMessage(history []llm.ChatMessage) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == llm.RoleUser && strings.TrimSpace(history[i].Content) != "" {
			return history[i].Content
		}
	}
	return ""
}

func summarizeTextForLog(text string, maxLen int) string {
	if maxLen <= 0 || len(text) <= maxLen {
		return text
	}
	return fmt.Sprintf("%s...[truncated %d chars]", text[:maxLen], len(text)-maxLen)
}
