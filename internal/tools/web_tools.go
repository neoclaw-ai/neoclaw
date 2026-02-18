package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const braveSearchEndpoint = "https://api.search.brave.com/res/v1/web/search"
const defaultUserAgent = "BetterClaw"

// WebSearchTool searches the web and returns structured text results for the LLM.
type WebSearchTool struct {
	Client   *http.Client
	Provider string
	APIKey   string
}

// Name returns the tool name.
func (t WebSearchTool) Name() string {
	return "web_search"
}

// Description returns the tool description for the model.
func (t WebSearchTool) Description() string {
	return "Search the web and return titles, URLs, and snippets"
}

// Schema returns the JSON schema for web_search args.
func (t WebSearchTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query text",
			},
		},
		"required": []string{"query"},
	}
}

// Permission declares default permission behavior for this tool.
func (t WebSearchTool) Permission() Permission {
	return AutoApprove
}

// Execute performs a provider-backed web search and returns text results.
func (t WebSearchTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	query, err := stringArg(args, "query")
	if err != nil {
		return nil, err
	}

	provider := strings.ToLower(strings.TrimSpace(t.Provider))
	if provider == "" {
		provider = "brave"
	}
	if provider != "brave" {
		return nil, fmt.Errorf("unsupported web.search.provider %q", provider)
	}
	if strings.TrimSpace(t.APIKey) == "" {
		return nil, errors.New("web.search.api_key is required")
	}

	if t.Client == nil {
		return nil, errors.New("http client is required")
	}
	client := t.Client

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, braveSearchEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create search request: %w", err)
	}
	q := req.URL.Query()
	q.Set("q", query)
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", t.APIKey)
	req.Header.Set("User-Agent", defaultUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute search request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read search response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("search request failed: %s", resp.Status)
	}

	var payload struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}
	if len(payload.Web.Results) == 0 {
		return &ToolResult{Output: "no results"}, nil
	}

	var out strings.Builder
	for i, result := range payload.Web.Results {
		if i > 0 {
			out.WriteString("\n\n")
		}
		title := strings.TrimSpace(result.Title)
		if title == "" {
			title = "(untitled)"
		}
		out.WriteString(fmt.Sprintf("%d. %s\n", i+1, title))
		out.WriteString("URL: ")
		out.WriteString(strings.TrimSpace(result.URL))
		description := strings.TrimSpace(result.Description)
		if description != "" {
			out.WriteString("\nSnippet: ")
			out.WriteString(description)
		}
	}

	return TruncateOutput(out.String())
}

// HTTPRequestTool makes HTTP requests and returns URL, status, and body text.
type HTTPRequestTool struct {
	Client *http.Client
}

// Name returns the tool name.
func (t HTTPRequestTool) Name() string {
	return "http_request"
}

// Description returns the tool description for the model.
func (t HTTPRequestTool) Description() string {
	return "Make an HTTP request and return URL, status, and body"
}

// Schema returns the JSON schema for http_request args.
func (t HTTPRequestTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"method": map[string]any{
				"type":        "string",
				"description": "HTTP method: GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS, TRACE, CONNECT",
			},
			"url": map[string]any{
				"type":        "string",
				"description": "Absolute HTTP or HTTPS URL",
			},
			"body": map[string]any{
				"type":        "string",
				"description": "Optional request body",
			},
			"headers": map[string]any{
				"type":        "object",
				"description": "Optional headers as key/value pairs",
			},
		},
		"required": []string{"method", "url"},
	}
}

// Permission declares default permission behavior for this tool.
func (t HTTPRequestTool) Permission() Permission {
	return AutoApprove
}

// Execute performs an HTTP request and returns response text.
func (t HTTPRequestTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	method, err := stringArg(args, "method")
	if err != nil {
		return nil, err
	}
	method = strings.ToUpper(method)
	switch method {
	case http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions,
		http.MethodTrace,
		http.MethodConnect:
	default:
		return nil, fmt.Errorf("unsupported method %q", method)
	}

	rawURL, err := stringArg(args, "url")
	if err != nil {
		return nil, err
	}

	body := ""
	if raw, ok := args["body"]; ok {
		value, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("argument %q must be a string", "body")
		}
		body = value
	}
	headers, err := parseHeaderArgs(args)
	if err != nil {
		return nil, err
	}

	if t.Client == nil {
		return nil, errors.New("http client is required")
	}
	client := t.Client

	var requestBody io.Reader
	if body != "" {
		requestBody = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("Accept", "application/json, text/markdown, text/plain")
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", defaultUserAgent)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	output := fmt.Sprintf("URL: %s\nStatus: %s\n\n%s", req.URL.String(), resp.Status, string(respBody))
	return TruncateOutput(output)
}

func parseHeaderArgs(args map[string]any) (map[string]string, error) {
	rawHeaders, ok := args["headers"]
	if !ok {
		return map[string]string{}, nil
	}

	obj, ok := rawHeaders.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("argument %q must be an object", "headers")
	}
	headers := make(map[string]string, len(obj))
	for key, rawValue := range obj {
		value, ok := rawValue.(string)
		if !ok {
			return nil, fmt.Errorf("header %q must be a string", key)
		}
		headers[key] = value
	}
	return headers, nil
}
