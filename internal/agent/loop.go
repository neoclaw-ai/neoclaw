package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/machinae/betterclaw/internal/approval"
	"github.com/machinae/betterclaw/internal/llm"
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
		resp, err := provider.Chat(ctx, llm.ChatRequest{
			SystemPrompt: systemPrompt,
			Messages:     history,
			Tools:        toolDefs,
		})
		if err != nil {
			return nil, history, err
		}

		totalUsage.InputTokens += resp.Usage.InputTokens
		totalUsage.OutputTokens += resp.Usage.OutputTokens
		totalUsage.TotalTokens += resp.Usage.TotalTokens

		if len(resp.ToolCalls) == 0 {
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
			tool, ok := registry.Lookup(call.Name)
			if !ok {
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
					return nil, history, fmt.Errorf("parse tool arguments for %q: %w", call.Name, err)
				}
			}

			result, err := approval.ExecuteTool(ctx, approver, tool, args, fmt.Sprintf("%s %s", call.Name, call.Arguments))
			if err != nil {
				history = append(history, llm.ChatMessage{
					Role:       llm.RoleTool,
					ToolCallID: call.ID,
					Content:    fmt.Sprintf("tool execution error: %v", err),
				})
				continue
			}

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
