//go:build !windows

package transport

import (
	"os/exec"
	"syscall"
)

func configureProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func attachProcessTree(*exec.Cmd) (uintptr, error) { return 0, nil }

func terminateProcessTree(command *exec.Cmd, _ uintptr) error {
	return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
}

func closeProcessTree(uintptr) {}
