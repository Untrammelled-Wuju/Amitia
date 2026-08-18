package process

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// ProcessIdentity is a restart-stable fingerprint used to prevent PID-reuse kills.
// StartIdentity must identify the OS process creation instance, not the supervisor session.
type ProcessIdentity struct {
	Executable    string
	StartIdentity string
}

func ReadProcessIdentity(pid int) (ProcessIdentity, error) {
	if pid <= 0 {
		return ProcessIdentity{}, fmt.Errorf("invalid pid %d", pid)
	}
	switch runtime.GOOS {
	case "linux":
		exePath, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
		if err != nil {
			return ProcessIdentity{}, fmt.Errorf("read executable: %w", err)
		}
		f, err := os.Open(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
		if err != nil {
			return ProcessIdentity{}, fmt.Errorf("read stat: %w", err)
		}
		defer f.Close()
		line, err := bufio.NewReader(f).ReadString('\n')
		if err != nil && len(line) == 0 {
			return ProcessIdentity{}, fmt.Errorf("read stat: %w", err)
		}
		closeParen := strings.LastIndex(line, ")")
		if closeParen < 0 {
			return ProcessIdentity{}, fmt.Errorf("malformed /proc stat")
		}
		fields := strings.Fields(line[closeParen+1:])
		// /proc/<pid>/stat field 22 is starttime. fields[0] is original field 3.
		if len(fields) <= 19 {
			return ProcessIdentity{}, fmt.Errorf("malformed /proc stat fields")
		}
		return ProcessIdentity{Executable: canonicalExecutable(exePath), StartIdentity: fields[19]}, nil
	case "darwin":
		out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "lstart=", "-o", "comm=").Output()
		if err != nil {
			return ProcessIdentity{}, fmt.Errorf("ps process identity: %w", err)
		}
		line := strings.TrimSpace(string(out))
		parts := strings.Fields(line)
		if len(parts) < 6 {
			return ProcessIdentity{}, fmt.Errorf("malformed ps identity")
		}
		exePath := parts[len(parts)-1]
		start := strings.Join(parts[:len(parts)-1], " ")
		return ProcessIdentity{Executable: canonicalExecutable(exePath), StartIdentity: start}, nil
	case "windows":
		script := fmt.Sprintf(`$p=Get-CimInstance Win32_Process -Filter "ProcessId=%d"; if($null -eq $p){exit 3}; Write-Output ($p.ExecutablePath); Write-Output ($p.CreationDate)`, pid)
		out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
		if err != nil {
			return ProcessIdentity{}, fmt.Errorf("powershell process identity: %w", err)
		}
		lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
		var nonempty []string
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				nonempty = append(nonempty, strings.TrimSpace(line))
			}
		}
		if len(nonempty) < 2 {
			return ProcessIdentity{}, fmt.Errorf("malformed windows process identity")
		}
		return ProcessIdentity{Executable: canonicalExecutable(nonempty[0]), StartIdentity: nonempty[1]}, nil
	default:
		return ProcessIdentity{}, fmt.Errorf("process identity unsupported on %s", runtime.GOOS)
	}
}

func SameProcessIdentity(expectedExecutable, expectedStart string, actual ProcessIdentity) bool {
	if strings.TrimSpace(expectedExecutable) == "" || strings.TrimSpace(expectedStart) == "" || strings.TrimSpace(actual.StartIdentity) == "" {
		return false
	}
	return canonicalExecutable(expectedExecutable) == canonicalExecutable(actual.Executable) && expectedStart == actual.StartIdentity
}

func canonicalExecutable(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if runtime.GOOS == "windows" {
		cleaned = strings.ToLower(cleaned)
	}
	return cleaned
}
