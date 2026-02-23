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

	profile := darwinProfile(mode, absDataDir)
	if strings.TrimSpace(profile) == "" {
		return fmt.Errorf("unsupported security mode %q", mode)
	}
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve executable symlinks: %w", err)
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
func darwinProfile(mode, dataDir string) string {
	writeRule := fmt.Sprintf("(allow file-write* (subpath %q))", dataDir)
	switch strings.TrimSpace(mode) {
	case config.SecurityModeStrict:
		readRoots := []string{
			dataDir,
			"/usr",
			"/bin",
			"/sbin",
			"/private/etc",
			"/private/tmp",
			"/private/var",
			"/dev/null",
		}
		var profile strings.Builder
		profile.WriteString("(version 1)\n")
		profile.WriteString("(deny default)\n")
		profile.WriteString("(allow process*)\n")
		profile.WriteString("(allow sysctl-read)\n")
		profile.WriteString("(allow network-outbound*)\n")
		for _, root := range readRoots {
			root = strings.TrimSpace(root)
			if root == "" {
				continue
			}
			profile.WriteString(fmt.Sprintf("(allow file-read* (subpath %q))\n", root))
		}
		profile.WriteString(writeRule)
		return profile.String()
	case config.SecurityModeStandard:
		return strings.Join([]string{
			"(version 1)",
			"(allow default)",
			fmt.Sprintf("(allow file-write* (subpath %q))", dataDir),
			"(deny file-write*)",
		}, "\n")
	default:
		return ""
	}
}
