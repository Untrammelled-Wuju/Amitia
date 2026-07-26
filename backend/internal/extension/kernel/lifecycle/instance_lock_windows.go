//go:build windows

package lifecycle

import (
	"os/exec"
	"strconv"
	"strings"
)

func isUnixProcessAlive(pid int) bool {
	return false
}

func isWindowsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	cmd := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/NH", "/FO", "CSV")
	out, err := cmd.Output()
	if err != nil {
		return true
	}
	text := strings.TrimSpace(string(out))
	if text == "" || strings.Contains(strings.ToLower(text), "no tasks") {
		return false
	}
	return true
}
