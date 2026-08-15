//go:build !windows

package browser

import (
	"os"
	"syscall"

	proc "github.com/u-ai/backend/internal/platform/process"
)

func isProcessAliveByPID(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func killProcessTreeHandle(_ proc.ProcessTreeHandle) {
}

func removeProfileDir(path string) {
	os.RemoveAll(path)
}

func makeProfileDir(path string) error {
	return os.MkdirAll(path, 0700)
}
