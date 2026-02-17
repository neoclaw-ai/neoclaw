package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnthropicProviderChat_RequestAndResponse(t *testing.T) {
	var gotAPIKey string
	var gotReq map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotAPIKey = r.Header.Get("X-Api-Key")
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"msg_1",
			"type":"message",
			"role":"assistant",
			"model":"claude-sonnet-4-5",
			"content":[
				{"type":"text","text":"I can call a tool."},
				{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{"city":"SF"}}
			],
			"stop_reason":"tool_use",
			"stop_sequence":"",
			"usage":{
				"cache_creation":{"ephemeral_1h_input_tokens":0,"ephemeral_5m_input_tokens":0},
				"cache_creation_input_tokens":0,
				"cache_read_input_tokens":0,
				"inference_geo":"us",
				"input_tokens":21,
				"output_tokens":9,
				"server_tool_use":{"web_search_requests":0},
				"service_tier":"standard"
			}
		}`))
	}))
	defer srv.Close()

	p, err := newAnthropicProviderForTest("test-key", "claude-sonnet-4-5", srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	resp, err := p.Chat(context.Background(), ChatRequest{
		SystemPrompt: "be concise",
		MaxTokens:    256,
		Messages: []ChatMessage{
			{Role: RoleUser, Content: "weather in SF?"},
		},
		Tools: []ToolDefinition{
			{
				Name:        "get_weather",
				Description: "Look up weather",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"city": map[string]any{"type": "string"},
					},
					"required": []any{"city"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}

	if gotAPIKey != "test-key" {
		t.Fatalf("unexpected api key header: %q", gotAPIKey)
	}
	if gotReq["model"] != "claude-sonnet-4-5" {
		t.Fatalf("unexpected model in request: %#v", gotReq["model"])
	}
	if int(gotReq["max_tokens"].(float64)) != 256 {
		t.Fatalf("unexpected max_tokens: %#v", gotReq["max_tokens"])
	}

	if resp.Content != "I can call a tool." {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ID != "toolu_1" || resp.ToolCalls[0].Name != "get_weather" {
		t.Fatalf("unexpected tool call: %+v", resp.ToolCalls[0])
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(resp.ToolCalls[0].Arguments), &args); err != nil {
		t.Fatalf("tool args should be valid JSON, got %q", resp.ToolCalls[0].Arguments)
	}
	if args["city"] != "SF" {
		t.Fatalf("unexpected tool args: %#v", args)
	}
	if resp.Usage.InputTokens != 21 || resp.Usage.OutputTokens != 9 || resp.Usage.TotalTokens != 30 {
		t.Fatalf("unexpected usage: %+v", resp.Usage)
	}
}
