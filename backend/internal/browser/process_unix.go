//go:build !windows

package browser

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func isProcessAlive(proc *os.Process) bool {
	if proc == nil {
		return false
	}
	err := proc.Signal(syscall.Signal(0))
	return err == nil
}

func killProcessTree(root *os.Process) {
	if root == nil {
		return
	}
	pgid, err := syscall.Getpgid(root.Pid)
	if err == nil {
		syscall.Kill(-pgid, syscall.SIGKILL)
	}
	root.Kill()
}

func isAmitiaOwnedPath(path, expectedRoot string) bool {
	expectedRoot = strings.TrimSpace(expectedRoot)
	if expectedRoot == "" {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(expectedRoot)
	if err != nil {
		return false
	}
	absRoot = filepath.Clean(absRoot)
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}
