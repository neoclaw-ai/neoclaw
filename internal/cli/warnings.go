package cli

import (
	"strings"

	"github.com/machinae/betterclaw/internal/config"
	"github.com/machinae/betterclaw/internal/logging"
	"github.com/machinae/betterclaw/internal/sandbox"
)

// Emit startup warnings derived from non-fatal config/runtime conditions.
func warnStartupConditions(cfg *config.Config) {
	if cfg == nil {
		return
	}

	if !sandbox.IsSandboxSupported() && cfg.Security.Mode != config.SecurityModeStrict {
		logging.Logger().Warn("sandbox is unavailable on this host. strict mode requires sandbox support")
	}
	if cfg.Security.Mode == config.SecurityModeDanger {
		logging.Logger().Warn("security.mode is danger; sandbox and approval checks are bypassed")
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Web.Search.Provider), "brave") &&
		strings.TrimSpace(cfg.Web.Search.APIKey) == "" {
		logging.Logger().Warn("web.search.api_key is empty while web.search.provider is brave. web_search tool will fail until this is set")
	}
}
