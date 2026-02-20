package approval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/machinae/betterclaw/internal/logging"
	"github.com/machinae/betterclaw/internal/store"
)

// Checker validates outbound domains against an allowlist and can request user
// approval for unknown domains.
type Checker struct {
	AllowedDomainsPath string
	Approver           Approver
}

// Allow checks whether host is permitted. Unknown domains are approved via the
// configured approver, and approved domains are persisted.
func (c Checker) Allow(ctx context.Context, host string) error {
	target, err := normalizeDomain(host)
	if err != nil {
		return err
	}

	if isAllowedDomain(c.AllowedDomainsPath, target) {
		return nil
	}

	if c.Approver == nil {
		return fmt.Errorf("domain %q is not allowlisted and no approver is configured", target)
	}

	decision, err := c.Approver.RequestApproval(ctx, ApprovalRequest{
		Tool:        "network_domain",
		Description: fmt.Sprintf("allow access to %s?", target),
		Args: map[string]any{
			"domain": target,
		},
	})
	if err != nil {
		return err
	}
	if decision == Denied {
		return fmt.Errorf(
			"user denied domain %q. User denied this action. Try a different approach or ask the user for guidance",
			target,
		)
	}
	if decision == Approved {
		if err := addAllowedDomain(c.AllowedDomainsPath, target); err != nil {
			logging.Logger().Warn(
				"failed to persist approved domain",
				"domain", target,
				"err", err,
			)
		}
	}

	return nil
}

// RoundTripper wraps an HTTP transport and enforces domain approval checks
// before forwarding requests.
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

func isAllowedDomain(allowedDomainsPath, host string) bool {
	if strings.TrimSpace(allowedDomainsPath) == "" {
		return false
	}

	content, err := store.ReadFile(allowedDomainsPath)
	if err != nil {
		return false
	}

	var allowed []string
	if err := json.Unmarshal([]byte(content), &allowed); err != nil {
		return false
	}

	for _, candidate := range allowed {
		normalized, err := normalizeDomain(candidate)
		if err != nil {
			continue
		}
		if domainMatches(normalized, host) {
			return true
		}
	}
	return false
}

func addAllowedDomain(allowedDomainsPath, host string) error {
	if strings.TrimSpace(allowedDomainsPath) == "" {
		return fmt.Errorf("allowed domains path is required")
	}

	target, err := normalizeDomain(host)
	if err != nil {
		return err
	}

	allowed := make([]string, 0)
	content, err := store.ReadFile(allowedDomainsPath)
	switch {
	case err == nil:
		if len(strings.TrimSpace(content)) > 0 {
			if err := json.Unmarshal([]byte(content), &allowed); err != nil {
				return fmt.Errorf("decode allowlist %q: %w", allowedDomainsPath, err)
			}
		}
	case errors.Is(err, os.ErrNotExist):
		// Missing file is treated as empty allowlist.
	default:
		return fmt.Errorf("read allowlist %q: %w", allowedDomainsPath, err)
	}

	for _, candidate := range allowed {
		normalized, err := normalizeDomain(candidate)
		if err != nil {
			continue
		}
		if normalized == target {
			return nil
		}
	}

	allowed = append(allowed, target)
	encoded, err := json.MarshalIndent(allowed, "", "  ")
	if err != nil {
		return fmt.Errorf("encode allowlist: %w", err)
	}
	encoded = append(encoded, '\n')

	if err := store.WriteFile(allowedDomainsPath, encoded); err != nil {
		return fmt.Errorf("replace allowlist: %w", err)
	}

	return nil
}

func normalizeDomain(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errors.New("domain is required")
	}

	if !strings.Contains(value, "://") {
		value = "https://" + value
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("parse domain %q: %w", raw, err)
	}

	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(parsed.Hostname())), ".")
	if host == "" {
		return "", fmt.Errorf("invalid domain %q", raw)
	}
	return host, nil
}

func domainMatches(allowed, host string) bool {
	return host == allowed || strings.HasSuffix(host, "."+allowed)
}
