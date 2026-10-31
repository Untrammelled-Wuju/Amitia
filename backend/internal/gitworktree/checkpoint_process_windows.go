package gitworktree

import (
	amitialog "log"
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
			if err := exec.Command(path, "/T", "/F", "/PID", strconv.Itoa(command.Process.Pid)).Run(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
		}
	}
	if err := command.Process.Kill(); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
}
