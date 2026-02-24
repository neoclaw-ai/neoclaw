package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/machinae/betterclaw/internal/config"
)

type anthropicProvider struct {
	client    anthropic.Client
	model     anthropic.Model
	maxTokens int
}

func newAnthropicProvider(cfg config.LLMProviderConfig) (Provider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("anthropic api key is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("anthropic model is required")
	}

	client := anthropic.NewClient(option.WithAPIKey(cfg.APIKey))
	return &anthropicProvider{
		client:    client,
		model:     anthropic.Model(cfg.Model),
		maxTokens: cfg.MaxTokens,
	}, nil
}

func newAnthropicProviderForTest(apiKey, model string, maxTokens int, baseURL string, httpClient *http.Client) (Provider, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("anthropic api key is required")
	}
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("anthropic model is required")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
		option.WithHTTPClient(httpClient),
	}

	client := anthropic.NewClient(opts...)
	return &anthropicProvider{
		client:    client,
		model:     anthropic.Model(model),
		maxTokens: maxTokens,
	}, nil
}

// Chat sends a provider-agnostic chat request to Anthropic and normalizes the response.
func (p *anthropicProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	msgs, err := toAnthropicMessages(req.Messages)
	if err != nil {
		return nil, err
	}

	body := anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: int64(resolveMaxTokens(req.MaxTokens, p.maxTokens)),
		Messages:  msgs,
	}

	if req.SystemPrompt != "" {
		body.System = []anthropic.TextBlockParam{{
			Text:         req.SystemPrompt,
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}}
	}
	if len(req.Tools) > 0 {
		body.Tools = toAnthropicTools(req.Tools)
	}

	msg, err := p.client.Messages.New(ctx, body)
	if err != nil {
		return nil, err
	}

	var contentParts []string
	var calls []ToolCall
	for _, block := range msg.Content {
		switch v := block.AsAny().(type) {
		case anthropic.TextBlock:
			if v.Text != "" {
				contentParts = append(contentParts, v.Text)
			}
		case anthropic.ToolUseBlock:
			calls = append(calls, ToolCall{
				ID:        v.ID,
				Name:      v.Name,
				Arguments: string(v.Input),
			})
		}
	}

	usage := TokenUsage{
		InputTokens:  int(msg.Usage.InputTokens),
		OutputTokens: int(msg.Usage.OutputTokens),
	}
	usage.TotalTokens = usage.InputTokens + usage.OutputTokens

	return &ChatResponse{
		Content:   strings.Join(contentParts, "\n"),
		ToolCalls: calls,
		Usage:     usage,
	}, nil
}

func toAnthropicMessages(messages []ChatMessage) ([]anthropic.MessageParam, error) {
	out := make([]anthropic.MessageParam, 0, len(messages))
	for i := 0; i < len(messages); {
		msg := messages[i]
		switch msg.Role {
		case RoleUser:
			out = append(out, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
			i++
		case RoleAssistant:
			blocks := make([]anthropic.ContentBlockParamUnion, 0, len(msg.ToolCalls)+1)
			if msg.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
			}
			for _, tc := range msg.ToolCalls {
				input := map[string]any{}
				if tc.Arguments != "" {
					if err := json.Unmarshal([]byte(tc.Arguments), &input); err != nil {
						return nil, fmt.Errorf("parse assistant tool call args for %s: %w", tc.Name, err)
					}
				}
				blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, input, tc.Name))
			}
			if len(blocks) == 0 {
				blocks = append(blocks, anthropic.NewTextBlock(""))
			}
			out = append(out, anthropic.NewAssistantMessage(blocks...))
			i++
		case RoleTool:
			// Anthropic requires all tool results from one assistant turn in a
			// single user message. Collect consecutive RoleTool entries.
			var blocks []anthropic.ContentBlockParamUnion
			for i < len(messages) && messages[i].Role == RoleTool {
				if messages[i].ToolCallID == "" {
					return nil, fmt.Errorf("tool message requires tool_call_id")
				}
				blocks = append(blocks, anthropic.NewToolResultBlock(messages[i].ToolCallID, messages[i].Content, false))
				i++
			}
			out = append(out, anthropic.NewUserMessage(blocks...))
		default:
			return nil, fmt.Errorf("unsupported message role %s", msg.Role)
		}
	}
	applyHistoryCacheBreakpoint(out)
	return out, nil
}

// applyHistoryCacheBreakpoint marks the second-to-last message block as a cache
// breakpoint so the latest message remains uncached while the full prior prefix
// can be reused.
func applyHistoryCacheBreakpoint(messages []anthropic.MessageParam) {
	if len(messages) < 2 {
		return
	}
	addCacheControlToLastBlock(&messages[len(messages)-2])
}

func addCacheControlToLastBlock(message *anthropic.MessageParam) {
	if message == nil || len(message.Content) == 0 {
		return
	}
	block := &message.Content[len(message.Content)-1]
	cacheControl := anthropic.NewCacheControlEphemeralParam()

	switch {
	case block.OfText != nil:
		block.OfText.CacheControl = cacheControl
	case block.OfImage != nil:
		block.OfImage.CacheControl = cacheControl
	case block.OfDocument != nil:
		block.OfDocument.CacheControl = cacheControl
	case block.OfSearchResult != nil:
		block.OfSearchResult.CacheControl = cacheControl
	case block.OfToolUse != nil:
		block.OfToolUse.CacheControl = cacheControl
	case block.OfToolResult != nil:
		block.OfToolResult.CacheControl = cacheControl
	case block.OfServerToolUse != nil:
		block.OfServerToolUse.CacheControl = cacheControl
	case block.OfWebSearchToolResult != nil:
		block.OfWebSearchToolResult.CacheControl = cacheControl
	}
}

func toAnthropicTools(tools []ToolDefinition) []anthropic.ToolUnionParam {
	out := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		toolParam := anthropic.ToolParam{
			Name:        tool.Name,
			Description: anthropic.String(tool.Description),
			InputSchema: toAnthropicInputSchema(tool.Parameters),
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: &toolParam})
	}
	return out
}

func toAnthropicInputSchema(schema map[string]any) anthropic.ToolInputSchemaParam {
	if len(schema) == 0 {
		return anthropic.ToolInputSchemaParam{}
	}

	var required []string
	if rawRequired, ok := schema["required"]; ok {
		switch v := rawRequired.(type) {
		case []string:
			required = v
		case []any:
			required = make([]string, 0, len(v))
			for _, item := range v {
				if s, ok := item.(string); ok {
					required = append(required, s)
				}
			}
		}
	}

	inputSchema := anthropic.ToolInputSchemaParam{
		Required: required,
	}
	if props, ok := schema["properties"]; ok {
		inputSchema.Properties = props
	}

	extras := make(map[string]any)
	for k, v := range schema {
		if k == "properties" || k == "required" || k == "type" {
			continue
		}
		extras[k] = v
	}
	if len(extras) > 0 {
		inputSchema.ExtraFields = extras
	}

	return inputSchema
}
