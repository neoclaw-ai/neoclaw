package tools

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type webRoundTripFunc func(*http.Request) (*http.Response, error)

func (f webRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestWebSearchToolExecute(t *testing.T) {
	client := &http.Client{
		Transport: webRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.String() != "https://api.search.brave.com/res/v1/web/search?q=golang" {
				t.Fatalf("unexpected url: %s", req.URL.String())
			}
			if got := req.Header.Get("X-Subscription-Token"); got != "brave-key" {
				t.Fatalf("expected brave token header, got %q", got)
			}
			if got := req.Header.Get("User-Agent"); got != "BetterClaw" {
				t.Fatalf("expected BetterClaw user agent, got %q", got)
			}
			body := `{"web":{"results":[{"title":"Go","url":"https://go.dev","description":"The Go programming language"}]}}`
			return &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}

	tool := WebSearchTool{
		Client:   client,
		Provider: "brave",
		APIKey:   "brave-key",
	}
	result, err := tool.Execute(context.Background(), map[string]any{"query": "golang"})
	if err != nil {
		t.Fatalf("execute web_search: %v", err)
	}
	if !strings.Contains(result.Output, "1. Go") {
		t.Fatalf("expected title in output, got %q", result.Output)
	}
	if !strings.Contains(result.Output, "URL: https://go.dev") {
		t.Fatalf("expected url in output, got %q", result.Output)
	}
	if !strings.Contains(result.Output, "Snippet: The Go programming language") {
		t.Fatalf("expected snippet in output, got %q", result.Output)
	}
}

func TestWebSearchToolRequiresAPIKey(t *testing.T) {
	tool := WebSearchTool{Provider: "brave"}
	_, err := tool.Execute(context.Background(), map[string]any{"query": "golang"})
	if err == nil || !strings.Contains(err.Error(), "web.search.api_key is required") {
		t.Fatalf("expected missing api key error, got %v", err)
	}
}

func TestWebSearchToolRequiresClient(t *testing.T) {
	tool := WebSearchTool{
		Provider: "brave",
		APIKey:   "brave-key",
	}
	_, err := tool.Execute(context.Background(), map[string]any{"query": "golang"})
	if err == nil || !strings.Contains(err.Error(), "http client is required") {
		t.Fatalf("expected missing client error, got %v", err)
	}
}

func TestHTTPRequestToolExecute(t *testing.T) {
	client := &http.Client{
		Transport: webRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPatch {
				t.Fatalf("expected PATCH, got %s", req.Method)
			}
			if req.URL.String() != "https://example.com/path" {
				t.Fatalf("unexpected url: %s", req.URL.String())
			}
			if got := req.Header.Get("Accept"); got != "application/json, text/markdown, text/plain" {
				t.Fatalf("expected multi Accept header, got %q", got)
			}
			if got := req.Header.Get("User-Agent"); got != "BetterClaw" {
				t.Fatalf("expected BetterClaw user agent, got %q", got)
			}
			if got := req.Header.Get("X-Test"); got != "value" {
				t.Fatalf("expected custom header value, got %q", got)
			}
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			if string(bodyBytes) != "payload" {
				t.Fatalf("expected body payload, got %q", string(bodyBytes))
			}
			return &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("hello body")),
			}, nil
		}),
	}

	tool := HTTPRequestTool{Client: client}
	result, err := tool.Execute(context.Background(), map[string]any{
		"method": "patch",
		"url":    "https://example.com/path",
		"body":   "payload",
		"headers": map[string]any{
			"X-Test": "value",
		},
	})
	if err != nil {
		t.Fatalf("execute http_request: %v", err)
	}
	if !strings.Contains(result.Output, "URL: https://example.com/path") {
		t.Fatalf("expected URL in output, got %q", result.Output)
	}
	if !strings.Contains(result.Output, "Status: 200 OK") {
		t.Fatalf("expected status in output, got %q", result.Output)
	}
	if !strings.Contains(result.Output, "hello body") {
		t.Fatalf("expected body in output, got %q", result.Output)
	}
}

func TestHTTPRequestToolRejectsInvalidHeaderValueType(t *testing.T) {
	tool := HTTPRequestTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"method": "GET",
		"url":    "https://example.com",
		"headers": map[string]any{
			"X-Test": 1,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "header X-Test must be a string") {
		t.Fatalf("expected header type error, got %v", err)
	}
}

func TestHTTPRequestToolRequiresClient(t *testing.T) {
	tool := HTTPRequestTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"method": "GET",
		"url":    "https://example.com",
	})
	if err == nil || !strings.Contains(err.Error(), "http client is required") {
		t.Fatalf("expected missing client error, got %v", err)
	}
}
