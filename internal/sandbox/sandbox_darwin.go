//go:build darwin

package sandbox

import "os/exec"

// IsSandboxSupported reports whether sandbox-exec is available on Darwin.
func IsSandboxSupported() bool {
	_, err := exec.LookPath("sandbox-exec")
	return err == nil
}

func restrictProcessImpl(mode, dataDir string) error {
	return nil
}
