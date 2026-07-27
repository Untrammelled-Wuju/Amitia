//go:build !windows

package process

import (
	"os/exec"
	"runtime"
	"syscall"
)

func configureProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func attachProcessTree(*exec.Cmd) (ProcessTreeHandle, error) {
	return 0, nil
}

func terminateProcessTree(pid int, _ ProcessTreeHandle) error {
	if pid <= 0 {
		return nil
	}
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return syscall.Kill(pid, syscall.SIGKILL)
	}
	return syscall.Kill(-pgid, syscall.SIGKILL)
}

func closeProcessTree(ProcessTreeHandle) {}

func detectIsolation() PlatformIsolationReport {
	switch runtime.GOOS {
	case "linux":
		return PlatformIsolationReport{
			Platform:             "linux",
			ProcessTreeIsolation: true,
			MemoryLimit:          false,
			CPULimit:             false,
			FilesystemIsolation:  false,
			NetworkIsolation:     false,
			UserNamespace:        false,
			Seccomp:              false,
			AppContainer:         false,
			SandboxProfile:       false,
			Limitations: []string{
				"cgroup v2 integration not implemented in first version",
				"seccomp filter not applied",
				"namespace/bubblewrap isolation not implemented",
				"network namespace not applied",
			},
		}
	case "darwin":
		return PlatformIsolationReport{
			Platform:             "darwin",
			ProcessTreeIsolation: true,
			MemoryLimit:          false,
			CPULimit:             false,
			FilesystemIsolation:  false,
			NetworkIsolation:     false,
			UserNamespace:        false,
			Seccomp:              false,
			AppContainer:         false,
			SandboxProfile:       false,
			Limitations: []string{
				"sandbox-exec profile not applied in first version",
				"Hardened Runtime check not implemented",
				"code signature verification not integrated",
			},
		}
	default:
		return PlatformIsolationReport{
			Platform:             runtime.GOOS,
			ProcessTreeIsolation: true,
			MemoryLimit:          false,
			CPULimit:             false,
			FilesystemIsolation:  false,
			NetworkIsolation:     false,
			UserNamespace:        false,
			Seccomp:              false,
			AppContainer:         false,
			SandboxProfile:       false,
			Limitations: []string{
				"unknown platform: full isolation capabilities not determined",
			},
		}
	}
}
