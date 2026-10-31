package gitworktree

import (
	amitialog "log"
	"os/exec"
	"syscall"
)

func configureCheckpointTestProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateCheckpointTestProcess(command *exec.Cmd) {
	if command.Process != nil {
		if err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
}
