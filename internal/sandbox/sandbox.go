package sandbox

// RestrictProcess applies process-level filesystem sandboxing to the current process.
func RestrictProcess(mode, dataDir string) error {
	return restrictProcessImpl(mode, dataDir)
}
