package approval

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mockDomainApprover struct {
	decision ApprovalDecision
	err      error
	calls    int
	lastReq  ApprovalRequest
}

func (m *mockDomainApprover) RequestApproval(_ context.Context, req ApprovalRequest) (ApprovalDecision, error) {
	m.calls++
	m.lastReq = req
	if m.err != nil {
		return Denied, m.err
	}
	return m.decision, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCheckerAllow_AllowAndSubdomainMatch(t *testing.T) {
	allowedPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	writeDomainPolicy(t, allowedPath, domainPolicy{
		Allow: []string{"github.com"},
		Deny:  nil,
	})

	checker := Checker{AllowedDomainsPath: allowedPath}
	if err := checker.Allow(context.Background(), "github.com"); err != nil {
		t.Fatalf("expected github.com allowed, got err: %v", err)
	}
	if err := checker.Allow(context.Background(), "api.github.com"); err != nil {
		t.Fatalf("expected api.github.com allowed, got err: %v", err)
	}
}

func TestCheckerAllow_DenyTakesPrecedenceOverAllow(t *testing.T) {
	allowedPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	writeDomainPolicy(t, allowedPath, domainPolicy{
		Allow: []string{"example.com"},
		Deny:  []string{"example.com"},
	})

	approver := &mockDomainApprover{decision: Approved}
	checker := Checker{
		AllowedDomainsPath: allowedPath,
		Approver:           approver,
	}
	err := checker.Allow(context.Background(), "api.example.com")
	if err == nil {
		t.Fatal("expected denied domain")
	}
	if approver.calls != 0 {
		t.Fatalf("expected no prompt when deny rule matches, got %d", approver.calls)
	}
}

func TestCheckerAllow_WildcardDotRuleIsNormalizedAndMatches(t *testing.T) {
	allowedPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	writeDomainPolicy(t, allowedPath, domainPolicy{
		Allow: []string{"*.github.com"},
		Deny:  nil,
	})

	checker := Checker{AllowedDomainsPath: allowedPath}
	if err := checker.Allow(context.Background(), "api.github.com"); err != nil {
		t.Fatalf("expected wildcard-dot allow rule to match subdomain, got %v", err)
	}
}

func TestCheckerAllow_DenyWildcardDotRuleBlocks(t *testing.T) {
	allowedPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	writeDomainPolicy(t, allowedPath, domainPolicy{
		Allow: []string{"*"},
		Deny:  []string{"*.github.com"},
	})

	checker := Checker{AllowedDomainsPath: allowedPath}
	err := checker.Allow(context.Background(), "api.github.com")
	if err == nil {
		t.Fatal("expected deny rule to block wildcard-dot match")
	}
}

func TestCheckerAllow_AllowAllStarRule(t *testing.T) {
	allowedPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	writeDomainPolicy(t, allowedPath, domainPolicy{
		Allow: []string{"*"},
		Deny:  nil,
	})

	approver := &mockDomainApprover{decision: Denied}
	checker := Checker{AllowedDomainsPath: allowedPath, Approver: approver}
	if err := checker.Allow(context.Background(), "any.domain.example"); err != nil {
		t.Fatalf("expected allow-all rule to permit domain, got %v", err)
	}
	if approver.calls != 0 {
		t.Fatalf("expected no prompt when allow-all rule is present, got %d", approver.calls)
	}
}

func TestCheckerAllow_DenyAllStarRule(t *testing.T) {
	allowedPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	writeDomainPolicy(t, allowedPath, domainPolicy{
		Allow: []string{"api.github.com"},
		Deny:  []string{"*"},
	})

	approver := &mockDomainApprover{decision: Approved}
	checker := Checker{AllowedDomainsPath: allowedPath, Approver: approver}
	err := checker.Allow(context.Background(), "api.github.com")
	if err == nil {
		t.Fatal("expected deny-all rule to reject domain")
	}
	if approver.calls != 0 {
		t.Fatalf("expected no prompt when deny-all rule matches, got %d", approver.calls)
	}
}

func TestCheckerAllow_UnknownDomainPromptApprovePersistsAllow(t *testing.T) {
	allowedPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	writeDomainPolicy(t, allowedPath, domainPolicy{
		Allow: []string{"api.anthropic.com"},
		Deny:  nil,
	})

	approver := &mockDomainApprover{decision: Approved}
	checker := Checker{
		AllowedDomainsPath: allowedPath,
		Approver:           approver,
	}
	if err := checker.Allow(context.Background(), "api.stripe.com:443"); err != nil {
		t.Fatalf("allow unknown domain: %v", err)
	}
	if approver.calls != 1 {
		t.Fatalf("expected one prompt, got %d", approver.calls)
	}
	if approver.lastReq.Description != "Allow Domain: api.stripe.com" {
		t.Fatalf("unexpected prompt description %q", approver.lastReq.Description)
	}

	policy := readDomainPolicy(t, allowedPath)
	if !containsString(policy.Allow, "api.stripe.com") {
		t.Fatalf("expected approved domain in allow list, got %#v", policy.Allow)
	}
}

func TestCheckerAllow_UnknownDomainPromptDenyPersistsDeny(t *testing.T) {
	allowedPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	writeDomainPolicy(t, allowedPath, domainPolicy{
		Allow: nil,
		Deny:  nil,
	})

	approver := &mockDomainApprover{decision: Denied}
	checker := Checker{
		AllowedDomainsPath: allowedPath,
		Approver:           approver,
	}
	err := checker.Allow(context.Background(), "evil.example.com")
	if err == nil {
		t.Fatal("expected denied domain")
	}
	if approver.calls != 1 {
		t.Fatalf("expected one prompt, got %d", approver.calls)
	}

	policy := readDomainPolicy(t, allowedPath)
	if !containsString(policy.Deny, "evil.example.com") {
		t.Fatalf("expected denied domain in deny list, got %#v", policy.Deny)
	}
}

func TestCheckerAllow_SubdomainEntryDoesNotMatchSiblingSubdomain(t *testing.T) {
	allowedPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	writeDomainPolicy(t, allowedPath, domainPolicy{
		Allow: []string{"search.brave.com"},
		Deny:  nil,
	})

	approver := &mockDomainApprover{decision: Denied}
	checker := Checker{
		AllowedDomainsPath: allowedPath,
		Approver:           approver,
	}

	if err := checker.Allow(context.Background(), "api.search.brave.com"); err != nil {
		t.Fatalf("expected nested subdomain allowed, got err: %v", err)
	}

	err := checker.Allow(context.Background(), "www.brave.com")
	if err == nil {
		t.Fatal("expected www.brave.com to require approval when only search.brave.com is allowlisted")
	}
	if approver.calls == 0 {
		t.Fatal("expected approver to be called for www.brave.com")
	}
}

func TestRoundTripperBlocksBeforeBaseTransport(t *testing.T) {
	allowedPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	writeDomainPolicy(t, allowedPath, domainPolicy{
		Allow: nil,
		Deny:  []string{"example.com"},
	})

	approver := &mockDomainApprover{decision: Approved}
	called := false
	base := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     make(http.Header),
		}, nil
	})

	rt := RoundTripper{
		Checker: Checker{
			AllowedDomainsPath: allowedPath,
			Approver:           approver,
		},
		Base: base,
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	_, err = rt.RoundTrip(req)
	if err == nil {
		t.Fatal("expected denied domain error")
	}
	if called {
		t.Fatal("expected base transport not called when domain denied")
	}
}

func TestCheckerAllow_OldFlatArrayFormatReturnsError(t *testing.T) {
	allowedPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	if err := os.WriteFile(allowedPath, []byte("[\"github.com\"]\n"), 0o644); err != nil {
		t.Fatalf("write old format: %v", err)
	}

	checker := Checker{AllowedDomainsPath: allowedPath}
	err := checker.Allow(context.Background(), "github.com")
	if err == nil {
		t.Fatal("expected parse error for old flat-array format")
	}
	if !strings.Contains(err.Error(), "decode domain policy") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestNormalizeDomain_SplitsHostPort(t *testing.T) {
	host, err := normalizeDomain("api.github.com:443")
	if err != nil {
		t.Fatalf("normalize host: %v", err)
	}
	if host != "api.github.com" {
		t.Fatalf("expected api.github.com, got %q", host)
	}
}

func TestNormalizeDomain_StripsWildcardDotPrefix(t *testing.T) {
	host, err := normalizeDomain("*.GitHub.com")
	if err != nil {
		t.Fatalf("normalize host: %v", err)
	}
	if host != "github.com" {
		t.Fatalf("expected github.com, got %q", host)
	}
}

func writeDomainPolicy(t *testing.T, path string, policy domainPolicy) {
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

func readDomainPolicy(t *testing.T, path string) domainPolicy {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read policy: %v", err)
	}
	var policy domainPolicy
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatalf("unmarshal policy: %v", err)
	}
	return policy
}
