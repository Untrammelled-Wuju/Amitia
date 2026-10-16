package gitworktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

func configureCheckpointTestProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func terminateCheckpointTestProcess(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	if root := os.Getenv("SystemRoot"); root != "" {
		path := filepath.Join(root, "System32", "taskkill.exe")
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			_ = exec.Command(path, "/T", "/F", "/PID", strconv.Itoa(command.Process.Pid)).Run()
		}
	}
	_ = command.Process.Kill()
}
