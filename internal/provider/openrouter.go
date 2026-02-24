package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/neoclaw-ai/neoclaw/internal/config"
)

const defaultOpenRouterURL = "https://openrouter.ai/api/v1/chat/completions"

type openRouterProvider struct {
	apiKey     string
	model      string
	maxTokens  int
	endpoint   string
	httpClient *http.Client
}

func newOpenRouterProvider(cfg config.LLMProviderConfig) (Provider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("openrouter api key is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("openrouter model is required")
	}
	return &openRouterProvider{
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		maxTokens:  cfg.MaxTokens,
		endpoint:   defaultOpenRouterURL,
		httpClient: http.DefaultClient,
	}, nil
}

func newOpenRouterProviderForTest(apiKey, model string, maxTokens int, endpoint string, httpClient *http.Client) (Provider, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("openrouter api key is required")
	}
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("openrouter model is required")
	}
	if strings.TrimSpace(endpoint) == "" {
		return nil, fmt.Errorf("openrouter endpoint is required")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &openRouterProvider{
		apiKey:     apiKey,
		model:      model,
		maxTokens:  maxTokens,
		endpoint:   endpoint,
		httpClient: httpClient,
	}, nil
}

// Chat sends a provider-agnostic chat request to OpenRouter and normalizes the response.
func (p *openRouterProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	payload := openRouterRequest{
		Model:     p.model,
		Messages:  toOpenRouterMessages(req.Messages),
		MaxTokens: resolveMaxTokens(req.MaxTokens, p.maxTokens),
	}
	if req.SystemPrompt != "" {
		payload.Messages = append([]openRouterMessage{{
			Role:    "system",
			Content: req.SystemPrompt,
		}}, payload.Messages...)
	}
	if len(req.Tools) > 0 {
		payload.Tools = make([]openRouterTool, 0, len(req.Tools))
		for _, tool := range req.Tools {
			payload.Tools = append(payload.Tools, openRouterTool{
				Type: "function",
				Function: openRouterFunction{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			})
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal openrouter request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build openrouter request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openrouter request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read openrouter response: %w", err)
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("openrouter API returned %s: %s", httpResp.Status, strings.TrimSpace(string(respBody)))
	}

	var parsed openRouterResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode openrouter response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("openrouter response has no choices")
	}

	msg := parsed.Choices[0].Message
	toolCalls := make([]ToolCall, 0, len(msg.ToolCalls))
	for _, tc := range msg.ToolCalls {
		toolCalls = append(toolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	return &ChatResponse{
		Content:   msg.Content,
		ToolCalls: toolCalls,
		Usage: TokenUsage{
			InputTokens:  parsed.Usage.PromptTokens,
			OutputTokens: parsed.Usage.CompletionTokens,
			TotalTokens:  parsed.Usage.TotalTokens,
			CostUSD:      parseOptionalCost(parsed.Usage.Cost),
		},
	}, nil
}

type openRouterRequest struct {
	Model     string              `json:"model"`
	Messages  []openRouterMessage `json:"messages"`
	Tools     []openRouterTool    `json:"tools,omitempty"`
	MaxTokens int                 `json:"max_tokens,omitempty"`
}

type openRouterMessage struct {
	Role       string               `json:"role"`
	Content    string               `json:"content,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
	ToolCalls  []openRouterToolCall `json:"tool_calls,omitempty"`
}

type openRouterTool struct {
	Type     string             `json:"type"`
	Function openRouterFunction `json:"function"`
}

type openRouterFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Arguments   string         `json:"arguments,omitempty"`
}

type openRouterToolCall struct {
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type,omitempty"`
	Function openRouterFunction `json:"function"`
}

type openRouterResponse struct {
	Choices []struct {
		Message openRouterMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		Cost             any `json:"cost"`
	} `json:"usage"`
}

func parseOptionalCost(raw any) *float64 {
	switch v := raw.(type) {
	case float64:
		out := v
		return &out
	case string:
		value := strings.TrimSpace(v)
		if value == "" {
			return nil
		}
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil
		}
		return &parsed
	default:
		return nil
	}
}

func toOpenRouterMessages(messages []ChatMessage) []openRouterMessage {
	out := make([]openRouterMessage, 0, len(messages))
	for _, msg := range messages {
		m := openRouterMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
		if msg.Role == RoleTool {
			m.ToolCallID = msg.ToolCallID
		}
		if len(msg.ToolCalls) > 0 {
			m.ToolCalls = make([]openRouterToolCall, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				m.ToolCalls = append(m.ToolCalls, openRouterToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: openRouterFunction{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				})
			}
		}
		out = append(out, m)
	}
	return out
}
