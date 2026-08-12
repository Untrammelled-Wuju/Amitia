package browser

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	cmd := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(root.Pid))
	cmd.Run()
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
