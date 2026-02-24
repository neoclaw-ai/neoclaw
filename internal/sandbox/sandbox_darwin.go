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

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve executable symlinks: %w", err)
	}

	profile := darwinProfile(mode, absDataDir)
	if strings.TrimSpace(profile) == "" {
		return fmt.Errorf("unsupported security mode %s", mode)
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

// darwinProfile builds the SBPL profile for the given security mode.
func darwinProfile(mode, dataDir string) string {
	switch strings.TrimSpace(mode) {
	case config.SecurityModeStrict:
		return strictDarwinProfile(dataDir)
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

// strictDarwinProfile returns the SBPL profile for strict mode.
// Uses deny-default with an explicit read allowlist, protecting user home files
// (such as ~/.ssh and ~/.aws) while allowing system paths and the betterclaw home directory.
// Writes are restricted to dataDir plus essential device handles.
// /Users and /tmp (including /private/tmp) are intentionally absent from the read allowlist.
// Real paths (/private/etc, /private/var) are used instead of symlinks (/etc, /var, /tmp)
// so that /private/tmp cannot be accessed via either path.
func strictDarwinProfile(dataDir string) string {
	homeDir := filepath.Dir(dataDir)
	var b strings.Builder

	b.WriteString("(version 1)\n")
	b.WriteString("(deny default)\n\n")

	// Read allowlist: system paths + betterclaw home (contains config.toml).
	// /Users is intentionally absent — protects ~/.ssh, ~/.aws, etc.
	// /private/etc and /private/var are used instead of their symlinks /etc and /var
	// so that /private/tmp remains blocked via both the symlink and real path.
	b.WriteString("(allow file-read* file-test-existence\n")
	b.WriteString("  (literal \"/\")\n")
	b.WriteString(fmt.Sprintf("  (subpath %q)\n", homeDir))
	b.WriteString("  (subpath \"/System\")\n")
	b.WriteString("  (subpath \"/Library\")\n")
	b.WriteString("  (subpath \"/usr\")\n")
	b.WriteString("  (subpath \"/bin\")\n")
	b.WriteString("  (subpath \"/sbin\")\n")
	b.WriteString("  (subpath \"/private/etc\")\n")
	b.WriteString("  (subpath \"/private/var\")\n")
	b.WriteString("  (subpath \"/dev\")\n")
	b.WriteString("  (subpath \"/Applications\"))\n\n")

	// Metadata traversal everywhere (stat/lstat, not content reads).
	// Required for path resolution and module loading.
	b.WriteString("(allow file-read-metadata)\n\n")

	// Process lifecycle.
	b.WriteString("(allow process-exec)\n")
	b.WriteString("(allow process-fork)\n")
	b.WriteString("(allow signal (target self))\n\n")

	// System queries needed at process startup.
	b.WriteString("(allow sysctl-read\n")
	b.WriteString("  (sysctl-name-prefix \"hw.\")\n")
	b.WriteString("  (sysctl-name-prefix \"kern.\"))\n\n")

	b.WriteString("(allow mach-lookup)\n\n")

	// Full network access: the claw process runs an HTTP proxy that subprocesses
	// route their traffic through.
	b.WriteString("(allow network*)\n\n")

	// TTY and device access.
	b.WriteString("(allow pseudo-tty)\n")
	b.WriteString("(allow file-read* file-write* file-ioctl (literal \"/dev/ptmx\"))\n")
	b.WriteString("(allow file-read* file-write* (regex #\"^/dev/ttys[0-9]+$\"))\n")
	b.WriteString("(allow file-ioctl (regex #\"^/dev/tty.*\"))\n")
	b.WriteString("(allow file-read* file-write* (literal \"/dev/null\"))\n")
	b.WriteString("(allow file-read* file-write* (literal \"/dev/stdout\"))\n")
	b.WriteString("(allow file-read* file-write* (literal \"/dev/stderr\"))\n\n")

	// Write zone: data dir only.
	b.WriteString(fmt.Sprintf("(allow file-write* (subpath %q))\n", dataDir))

	return b.String()
}
