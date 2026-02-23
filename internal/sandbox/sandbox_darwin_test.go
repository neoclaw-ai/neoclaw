//go:build darwin

package sandbox

import (
	"strings"
	"testing"

	"github.com/machinae/betterclaw/internal/config"
)

func TestDarwinProfileStandard(t *testing.T) {
	dataDir := "/tmp/betterclaw/data"
	profile := darwinProfile(config.SecurityModeStandard, dataDir)

	required := []string{
		"(version 1)",
		"(allow default)",
		"(deny file-write*)",
		`(allow file-write* (subpath "/tmp/betterclaw/data"))`,
	}
	for _, token := range required {
		if !strings.Contains(profile, token) {
			t.Fatalf("standard profile missing %q:\n%s", token, profile)
		}
	}
	if strings.Contains(profile, "(deny default)") {
		t.Fatalf("standard profile should not deny by default:\n%s", profile)
	}
}

func TestDarwinProfileStrict(t *testing.T) {
	dataDir := "/tmp/betterclaw/data"
	profile := darwinProfile(config.SecurityModeStrict, dataDir)

	required := []string{
		"(version 1)",
		"(deny default)",
		"(allow process*)",
		"(allow sysctl-read)",
		"(allow network-outbound)",
		`(allow file-read* (subpath "/tmp/betterclaw/data"))`,
		`(allow file-read* (subpath "/usr"))`,
		`(allow file-read* (subpath "/bin"))`,
		`(allow file-read* (subpath "/sbin"))`,
		`(allow file-read* (subpath "/private/etc"))`,
		`(allow file-read* (subpath "/private/tmp"))`,
		`(allow file-read* (subpath "/private/var"))`,
		`(allow file-read* (subpath "/dev/null"))`,
		`(allow file-write* (subpath "/tmp/betterclaw/data"))`,
	}
	for _, token := range required {
		if !strings.Contains(profile, token) {
			t.Fatalf("strict profile missing %q:\n%s", token, profile)
		}
	}

	if strings.Contains(profile, "(allow file-read*)") {
		t.Fatalf("strict profile should not allow global file reads:\n%s", profile)
	}
}

func TestDarwinProfileUnknownModeReturnsEmpty(t *testing.T) {
	profile := darwinProfile("unknown", "/tmp/betterclaw/data")
	if profile != "" {
		t.Fatalf("expected empty profile for unknown mode, got:\n%s", profile)
	}
}
