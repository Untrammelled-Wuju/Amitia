package workbenchterminal

import (
	"context"
	amitialog "log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func terminateTerminalProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	if root := os.Getenv("SystemRoot"); root != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, filepath.Join(root, "System32", "taskkill.exe"), "/T", "/F", "/PID", strconv.Itoa(pid))
		command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		if err := command.Run(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
}
