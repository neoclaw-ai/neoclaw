package approval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/machinae/betterclaw/internal/store"
)

type domainMatchDecision int

const (
	domainNoMatch domainMatchDecision = iota
	domainAllowed
	domainDenied
)

type domainPolicy struct {
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
}

// Checker validates outbound domains against a policy and can request user approval for unknown domains.
type Checker struct {
	AllowedDomainsPath string
	Approver           Approver
}

// Allow checks whether host is permitted. Unknown domains are approved via the configured approver.
func (c Checker) Allow(ctx context.Context, host string) error {
	if isDangerMode() {
		return nil
	}

	target, err := normalizeDomain(host)
	if err != nil {
		return err
	}

	policy, err := loadCachedDomainPolicy(c.AllowedDomainsPath)
	if err != nil {
		return err
	}

	switch evaluateDomainPolicy(target, policy) {
	case domainAllowed:
		return nil
	case domainDenied:
		return toolDeniedError("network_domain")
	case domainNoMatch:
		// Continue to prompt path.
	}

	if c.Approver == nil {
		return fmt.Errorf("domain %s is not allowlisted and no approver is configured", target)
	}

	decision, err := c.Approver.RequestApproval(ctx, ApprovalRequest{
		Tool:        "network_domain",
		Description: fmt.Sprintf("Allow Domain: %s", target),
		Args: map[string]any{
			"domain": target,
		},
	})
	if err != nil {
		return err
	}

	switch decision {
	case Approved:
		policy.Allow = appendUnique(policy.Allow, target)
		return saveCachedDomainPolicy(c.AllowedDomainsPath, policy)
	case Denied:
		policy.Deny = appendUnique(policy.Deny, target)
		if err := saveCachedDomainPolicy(c.AllowedDomainsPath, policy); err != nil {
			return err
		}
		return toolDeniedError("network_domain")
	default:
		return toolDeniedError("network_domain")
	}
}

// RoundTripper wraps an HTTP transport and enforces domain approval checks before forwarding requests.
type RoundTripper struct {
	Checker Checker
	Base    http.RoundTripper
}

// RoundTrip checks the request domain via Checker and forwards to Base if allowed.
func (rt RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("request is required")
	}
	if req.URL == nil {
		return nil, errors.New("request URL is required")
	}

	if err := rt.Checker.Allow(req.Context(), req.URL.Host); err != nil {
		return nil, err
	}

	base := rt.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

// Load allow/deny domain policy from disk.
func loadDomainPolicy(path string) (domainPolicy, error) {
	if strings.TrimSpace(path) == "" {
		return domainPolicy{}, errors.New("allowed domains path is required")
	}

	raw, err := store.ReadFile(path)
	if err != nil {
		return domainPolicy{}, fmt.Errorf("read domain policy %s: %w", path, err)
	}
	if strings.TrimSpace(raw) == "" {
		return domainPolicy{}, nil
	}

	var policy domainPolicy
	if err := json.Unmarshal([]byte(raw), &policy); err != nil {
		return domainPolicy{}, fmt.Errorf("decode domain policy %s: %w", path, err)
	}
	return policy, nil
}

// Save allow/deny domain policy to disk.
func saveDomainPolicy(path string, policy domainPolicy) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("allowed domains path is required")
	}

	encoded, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return fmt.Errorf("encode domain policy: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := store.WriteFile(path, encoded); err != nil {
		return fmt.Errorf("write domain policy %s: %w", path, err)
	}
	return nil
}

// Evaluate deny first, then allow, then no match.
func evaluateDomainPolicy(host string, policy domainPolicy) domainMatchDecision {
	for _, candidate := range policy.Deny {
		normalized, err := normalizeDomain(candidate)
		if err != nil {
			continue
		}
		if domainMatches(normalized, host) {
			return domainDenied
		}
	}

	for _, candidate := range policy.Allow {
		normalized, err := normalizeDomain(candidate)
		if err != nil {
			continue
		}
		if domainMatches(normalized, host) {
			return domainAllowed
		}
	}

	return domainNoMatch
}

// Normalize host/domain inputs to lowercase host-only form.
func normalizeDomain(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errors.New("domain is required")
	}

	host := value
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return "", fmt.Errorf("parse domain %s: %w", raw, err)
		}
		host = parsed.Host
	}

	host = strings.TrimSpace(host)
	if host == "" {
		return "", fmt.Errorf("invalid domain %s", raw)
	}

	parsedHost, _, err := net.SplitHostPort(host)
	if err == nil {
		host = parsedHost
	}

	host = strings.Trim(host, "[]")
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if strings.HasPrefix(host, "*.") {
		host = strings.TrimPrefix(host, "*.")
	}
	if host == "" {
		return "", fmt.Errorf("invalid domain %s", raw)
	}
	return host, nil
}

func domainMatches(allowed, host string) bool {
	if allowed == "*" {
		return true
	}
	return host == allowed || strings.HasSuffix(host, "."+allowed)
}
