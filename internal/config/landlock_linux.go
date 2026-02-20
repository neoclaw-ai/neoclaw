//go:build linux

package config

import (
	"errors"
	"os"
	"strings"

	"github.com/machinae/betterclaw/internal/store"
	"golang.org/x/sys/unix"
)

func isLandlockAvailable() bool {
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

	// Fallback for environments where the syscall probe is blocked or filtered.
	lsmRaw, err := store.ReadFile("/sys/kernel/security/lsm")
	if err != nil {
		return false
	}
	for _, item := range strings.Split(strings.TrimSpace(lsmRaw), ",") {
		if strings.TrimSpace(item) == "landlock" {
			return true
		}
	}
	return false
}
