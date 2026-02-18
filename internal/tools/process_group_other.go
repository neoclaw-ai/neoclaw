//go:build !unix

package tools

import "os/exec"

func configureCommandForCancellation(_ *exec.Cmd) {}
