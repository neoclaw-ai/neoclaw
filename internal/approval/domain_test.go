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

func TestCheckerAllow_KnownDomainAndSubdomainMatch(t *testing.T) {
	allowedPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	if err := os.WriteFile(allowedPath, []byte("[\"http://brave.com\",\"search.brave.com\"]\n"), 0o644); err != nil {
		t.Fatalf("write allowlist: %v", err)
	}

	checker := Checker{AllowedDomainsPath: allowedPath}

	if err := checker.Allow(context.Background(), "api.brave.com"); err != nil {
		t.Fatalf("expected api.brave.com allowed via brave.com, got err: %v", err)
	}
	if err := checker.Allow(context.Background(), "api.search.brave.com"); err != nil {
		t.Fatalf("expected api.search.brave.com allowed via search.brave.com, got err: %v", err)
	}
	if err := checker.Allow(context.Background(), "www.brave.com"); err != nil {
		t.Fatalf("expected www.brave.com allowed via brave.com, got err: %v", err)
	}
}

func TestCheckerAllow_DenyUnknownDomainIncludesRecoveryMessage(t *testing.T) {
	allowedPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	if err := os.WriteFile(allowedPath, []byte("[\"brave.com\"]\n"), 0o644); err != nil {
		t.Fatalf("write allowlist: %v", err)
	}

	approver := &mockDomainApprover{decision: Denied}
	checker := Checker{
		AllowedDomainsPath: allowedPath,
		Approver:           approver,
	}

	err := checker.Allow(context.Background(), "https://example.com/path")
	if err == nil {
		t.Fatal("expected deny error")
	}
	if !strings.Contains(err.Error(), "User denied this action. Try a different approach or ask the user for guidance") {
		t.Fatalf("expected recovery guidance in error, got: %v", err)
	}
	if approver.calls != 1 {
		t.Fatalf("expected one approval request, got %d", approver.calls)
	}
}

func TestCheckerAllow_ApprovedPersistsDomain(t *testing.T) {
	allowedPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	if err := os.WriteFile(allowedPath, []byte("[\"brave.com\"]\n"), 0o644); err != nil {
		t.Fatalf("write allowlist: %v", err)
	}

	approver := &mockDomainApprover{decision: Approved}
	checker := Checker{
		AllowedDomainsPath: allowedPath,
		Approver:           approver,
	}

	if err := checker.Allow(context.Background(), "https://search.example.com/path?q=1"); err != nil {
		t.Fatalf("allow with approval: %v", err)
	}

	raw, err := os.ReadFile(allowedPath)
	if err != nil {
		t.Fatalf("read allowlist: %v", err)
	}
	var domains []string
	if err := json.Unmarshal(raw, &domains); err != nil {
		t.Fatalf("decode allowlist: %v", err)
	}

	found := false
	for _, d := range domains {
		if d == "search.example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected search.example.com persisted, got %#v", domains)
	}
}

func TestRoundTripperBlocksBeforeBaseTransport(t *testing.T) {
	allowedPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	if err := os.WriteFile(allowedPath, []byte("[\"brave.com\"]\n"), 0o644); err != nil {
		t.Fatalf("write allowlist: %v", err)
	}

	approver := &mockDomainApprover{decision: Denied}
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

func TestCheckerAllow_SubdomainEntryDoesNotMatchSiblingSubdomain(t *testing.T) {
	allowedPath := filepath.Join(t.TempDir(), "allowed_domains.json")
	if err := os.WriteFile(allowedPath, []byte("[\"search.brave.com\"]\n"), 0o644); err != nil {
		t.Fatalf("write allowlist: %v", err)
	}

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
