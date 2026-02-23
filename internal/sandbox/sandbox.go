package sandbox

import (
	"os"
	"strings"
)

const sandboxedEnvVar = "CLAW_SANDBOXED"

// RestrictProcess applies process-level filesystem sandboxing to the current process.
func RestrictProcess(mode, dataDir string) error {
	return restrictProcessImpl(mode, dataDir)
}

// IsAlreadySandboxed reports whether the current process already re-execed under sandbox constraints.
func IsAlreadySandboxed() bool {
	return strings.TrimSpace(os.Getenv(sandboxedEnvVar)) == "1"
}
