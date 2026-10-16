package gitworktree

import (
	"os/exec"
	"syscall"
)

func configureCheckpointTestProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateCheckpointTestProcess(command *exec.Cmd) {
	if command.Process != nil {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
}
