//go:build darwin

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/machinae/betterclaw/internal/config"
)

// IsSandboxSupported reports whether sandbox-exec is available on Darwin.
func IsSandboxSupported() bool {
	_, err := exec.LookPath("sandbox-exec")
	return err == nil
}

func restrictProcessImpl(mode, dataDir string) error {
	if IsAlreadySandboxed() {
		return nil
	}
	// Go test binaries are named "*.test"; avoid replacing the test process.
	if strings.HasSuffix(filepath.Base(os.Args[0]), ".test") {
		return nil
	}
	if !IsSandboxSupported() {
		return fmt.Errorf("sandbox-exec is unavailable on this host")
	}

	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return fmt.Errorf("data dir is required")
	}
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return fmt.Errorf("resolve data dir: %w", err)
	}
	absDataDir, err = filepath.EvalSymlinks(absDataDir)
	if err != nil {
		return fmt.Errorf("resolve data dir symlinks: %w", err)
	}

	// Resolve home dir for strict mode HOME deny. Best-effort: if unresolvable,
	// strict mode degrades to standard mode behavior for reads.
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		if resolved, err := filepath.EvalSymlinks(homeDir); err == nil {
			homeDir = resolved
		}
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve executable symlinks: %w", err)
	}

	profile := darwinProfile(mode, absDataDir, homeDir)
	if strings.TrimSpace(profile) == "" {
		return fmt.Errorf("unsupported security mode %q", mode)
	}

	args := append([]string{
		"sandbox-exec",
		"-p",
		profile,
		execPath,
	}, os.Args[1:]...)
	env := append(os.Environ(), sandboxedEnvVar+"=1")
	if err := syscall.Exec("/usr/bin/sandbox-exec", args, env); err != nil {
		return fmt.Errorf("exec sandbox-exec: %w", err)
	}
	return nil
}

// darwinProfile builds the SBPL profile for process-level filesystem restrictions.
//
// Both modes start from (allow default) to avoid breaking dyld and system services.
// Strict mode additionally denies reads from the user home directory, except for
// dataDir which is explicitly allowed before the home deny fires.
// If homeDir is empty, the home read deny is skipped and strict behaves like standard.
func darwinProfile(mode, dataDir, homeDir string) string {
	switch strings.TrimSpace(mode) {
	case config.SecurityModeStrict:
		var profile strings.Builder
		profile.WriteString("(version 1)\n")
		profile.WriteString("(allow default)\n")
		// Punch a hole for dataDir reads before denying the rest of HOME.
		// Order matters: SBPL uses first-match for explicit rules.
		if homeDir != "" {
			profile.WriteString(fmt.Sprintf("(allow file-read* (subpath %q))\n", filepath.Dir(dataDir)))
			profile.WriteString(fmt.Sprintf("(deny file-read* (subpath %q))\n", homeDir))
		}
		profile.WriteString(fmt.Sprintf("(allow file-write* (subpath %q))\n", dataDir))
		profile.WriteString("(allow file-write* (subpath \"/dev\"))\n")
		profile.WriteString("(deny file-write*)")
		return profile.String()
	case config.SecurityModeStandard:
		return strings.Join([]string{
			"(version 1)",
			"(allow default)",
			fmt.Sprintf("(allow file-write* (subpath %q))", dataDir),
			"(allow file-write* (subpath \"/dev\"))",
			"(deny file-write*)",
		}, "\n")
	default:
		return ""
	}
}
