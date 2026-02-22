//go:build !linux && !darwin

package sandbox

// IsSandboxSupported reports sandbox support on non-Linux/non-Darwin platforms.
func IsSandboxSupported() bool {
	return false
}
