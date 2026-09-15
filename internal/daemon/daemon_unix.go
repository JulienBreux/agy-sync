//go:build !windows

package daemon

import (
	"errors"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func terminateProcess(proc *os.Process) error {
	return proc.Signal(unix.SIGTERM)
}

func killProcess(proc *os.Process) error {
	return proc.Signal(unix.SIGKILL)
}

func isProcessNotExist(err error) bool {
	return errors.Is(err, unix.ESRCH)
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(unix.Signal(0))
	if err == nil {
		return true
	}
	if errors.Is(err, unix.EPERM) {
		return true
	}
	return false
}
