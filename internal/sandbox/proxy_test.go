package sandbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/machinae/betterclaw/internal/approval"
)

func TestDomainProxy_AllowsApprovedDomain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	policyPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	writeProxyPolicy(t, policyPath, map[string]any{
		"allow": []string{"127.0.0.1"},
		"deny":  []string{},
	})

	proxy, err := StartDomainProxy(approval.Checker{AllowedDomainsPath: policyPath})
	if err != nil {
		t.Fatalf("start proxy: %v", err)
	}
	defer proxy.Close()

	proxyURL, err := url.Parse(proxy.Addr())
	if err != nil {
		t.Fatalf("parse proxy url: %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
	}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("proxy request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestDomainProxy_DeniesBlockedDomain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	policyPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	writeProxyPolicy(t, policyPath, map[string]any{
		"allow": []string{},
		"deny":  []string{"*"},
	})

	proxy, err := StartDomainProxy(approval.Checker{AllowedDomainsPath: policyPath})
	if err != nil {
		t.Fatalf("start proxy: %v", err)
	}
	defer proxy.Close()

	proxyURL, err := url.Parse(proxy.Addr())
	if err != nil {
		t.Fatalf("parse proxy url: %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
	}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("proxy request error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestDomainProxy_PromptsAndPersistsUnknownDomain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	policyPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	writeProxyPolicy(t, policyPath, map[string]any{
		"allow": []string{},
		"deny":  []string{},
	})

	approver := &proxyApprover{decision: approval.Approved}
	proxy, err := StartDomainProxy(approval.Checker{
		AllowedDomainsPath: policyPath,
		Approver:           approver,
	})
	if err != nil {
		t.Fatalf("start proxy: %v", err)
	}
	defer proxy.Close()

	proxyURL, err := url.Parse(proxy.Addr())
	if err != nil {
		t.Fatalf("parse proxy url: %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
	}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("proxy request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if approver.calls == 0 {
		t.Fatal("expected proxy domain approval prompt")
	}
}

type proxyApprover struct {
	decision approval.ApprovalDecision
	calls    int
}

func (p *proxyApprover) RequestApproval(_ context.Context, _ approval.ApprovalRequest) (approval.ApprovalDecision, error) {
	p.calls++
	return p.decision, nil
}

func writeProxyPolicy(t *testing.T, path string, policy map[string]any) {
	t.Helper()
	raw, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		t.Fatalf("marshal policy: %v", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}
}
