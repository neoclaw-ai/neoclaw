//go:build linux

package sandbox

import (
	"errors"

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
