//go:build linux

package sandbox

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/landlock-lsm/go-landlock/landlock"
	"github.com/machinae/betterclaw/internal/config"
	"golang.org/x/sys/unix"
)

// IsSandboxSupported reports whether Landlock is available on Linux.
func IsSandboxSupported() bool {
	abi, _, errno := unix.Syscall(
		unix.SYS_LANDLOCK_CREATE_RULESET,
		0,
		0,
		uintptr(unix.LANDLOCK_CREATE_RULESET_VERSION),
	)
	if errno == 0 && abi >= 1 {
		return true
	}
	if errors.Is(errno, unix.ENOSYS) || errors.Is(errno, unix.EOPNOTSUPP) {
		return false
	}
	return false
}

func restrictProcessImpl(mode, dataDir string) error {
	trimmedMode := strings.TrimSpace(mode)
	if trimmedMode == config.SecurityModeStrict && !IsSandboxSupported() {
		return errors.New("landlock is unavailable on this host")
	}

	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return errors.New("data dir is required")
	}
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return fmt.Errorf("resolve data dir: %w", err)
	}
	absDataDir, err = filepath.EvalSymlinks(absDataDir)
	if err != nil {
		return fmt.Errorf("resolve data dir symlinks: %w", err)
	}

	rules := []landlock.Rule{
		landlock.RWDirs(absDataDir),
		landlock.RWDirs("/dev"),
	}
	if trimmedMode == config.SecurityModeStrict {
		rules = append(rules, strictLinuxReadRules(absDataDir)...)
	} else {
		rules = append(rules, landlock.RODirs("/"))
	}

	if err := landlock.V6.BestEffort().RestrictPaths(rules...); err != nil {
		return fmt.Errorf("restrict process with landlock: %w", err)
	}
	return nil
}

func strictLinuxReadRules(dataDir string) []landlock.Rule {
	readRoots := []string{
		filepath.Dir(dataDir),
		"/bin",
		"/sbin",
		"/usr",
		"/usr/lib",
		"/usr/lib64",
		"/usr/libexec",
		"/lib",
		"/lib64",
		"/etc",
		"/dev",
		"/proc",
		"/sys",
		"/run",
		"/tmp",
	}
	rules := make([]landlock.Rule, 0, len(readRoots))
	for _, root := range readRoots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		rules = append(rules, landlock.RODirs(root))
	}
	return rules
}
