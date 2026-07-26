//go:build !windows

package lifecycle

import (
	"os"
	"syscall"
)

func isWindowsProcessAlive(pid int) bool {
	return false
}

func isUnixProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}
