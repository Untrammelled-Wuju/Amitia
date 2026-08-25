// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build darwin && !ios

package process

import (
	"os"
	"os/exec"
	"syscall"
)

func configureProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func attachProcessTree(cmd *exec.Cmd) (ProcessTreeHandle, error) {
	return 0, nil
}

func attachProcessTreeWithLimits(cmd *exec.Cmd, _ ResourceLimits) (ProcessTreeHandle, error) {
	return attachProcessTree(cmd)
}

func resourceLimitSupport() ResourceLimitSupport {
	return ResourceLimitSupport{}
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
	sandboxAvailable := executableFile("/usr/bin/sandbox-exec")
	limitations := []string{
		"hard CPU/memory/process-count limits are not available in the Darwin process backend",
		"Hardened Runtime check is not implemented",
		"code signature verification is enforced separately from the process backend",
	}
	if !sandboxAvailable {
		limitations = append(limitations, "sandbox-exec is unavailable; enforced service sandbox launches fail closed")
	}
	return PlatformIsolationReport{
		Platform:             "darwin",
		ProcessTreeIsolation: true,
		MemoryLimit:          false,
		CPULimit:             false,
		FilesystemIsolation:  sandboxAvailable,
		NetworkIsolation:     sandboxAvailable,
		UserNamespace:        false,
		Seccomp:              false,
		AppContainer:         false,
		SandboxProfile:       sandboxAvailable,
		Limitations:          limitations,
	}
}

func executableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0
}
