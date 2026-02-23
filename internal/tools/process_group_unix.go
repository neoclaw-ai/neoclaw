//go:build unix

package tools

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

func configureCommandForCancellation(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return killCommandProcessGroup(cmd)
	}
	// Avoid waiting indefinitely if descendants keep stdio pipes open.
	if cmd.WaitDelay == 0 {
		cmd.WaitDelay = 2 * time.Second
	}
}

// killCommandProcessGroup force-kills the command process group and ignores ESRCH.
func killCommandProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}
