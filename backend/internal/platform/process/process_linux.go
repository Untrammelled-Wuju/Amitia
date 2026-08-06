// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build linux && !android

package process

import (
	"os"
	"os/exec"
	"syscall"
)

func configureProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGTERM,
	}
}

func attachProcessTree(cmd *exec.Cmd) (ProcessTreeHandle, error) {
	return 0, nil
}

func forceStopProcessTree(pid int, handle ProcessTreeHandle) error {
	return terminateProcessTree(pid, handle)
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

func requestGracefulStopSupported() bool { return true }

func processTreeSupported() bool { return true }

func procSignalTerm(proc *os.Process) error {
	return proc.Signal(syscall.SIGTERM)
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func detectIsolation() PlatformIsolationReport {
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
}
