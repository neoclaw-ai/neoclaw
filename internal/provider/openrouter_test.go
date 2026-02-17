package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenRouterProviderChat_RequestAndResponse(t *testing.T) {
	var gotAuth string
	var gotReq map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")

		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[
				{
					"message":{
						"role":"assistant",
						"content":"4",
						"tool_calls":[
							{
								"id":"call_1",
								"type":"function",
								"function":{
									"name":"calculator",
									"arguments":"{\"expr\":\"2+2\"}"
								}
							}
						]
					}
				}
			],
			"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}
		}`))
	}))
	defer srv.Close()

	p, err := newOpenRouterProviderForTest("test-key", "deepseek/deepseek-chat", 8192, srv.URL+"/api/v1/chat/completions", srv.Client())
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	resp, err := p.Chat(context.Background(), ChatRequest{
		SystemPrompt: "be concise",
		MaxTokens:    123,
		Messages: []ChatMessage{
			{Role: RoleUser, Content: "what is 2+2?"},
		},
		Tools: []ToolDefinition{
			{
				Name:        "calculator",
				Description: "Do math",
				Parameters: map[string]any{
					"type": "object",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}

	if gotAuth != "Bearer test-key" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	if gotReq["model"] != "deepseek/deepseek-chat" {
		t.Fatalf("unexpected model in request: %#v", gotReq["model"])
	}
	if int(gotReq["max_tokens"].(float64)) != 123 {
		t.Fatalf("unexpected max_tokens: %#v", gotReq["max_tokens"])
	}

	if resp.Content != "4" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "calculator" {
		t.Fatalf("unexpected tool call: %+v", resp.ToolCalls[0])
	}
	if resp.Usage.InputTokens != 11 || resp.Usage.OutputTokens != 7 || resp.Usage.TotalTokens != 18 {
		t.Fatalf("unexpected usage: %+v", resp.Usage)
	}
}
