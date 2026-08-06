// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build android || ios

package process

import (
	"errors"
	"os"
	"os/exec"
)

func configureProcess(cmd *exec.Cmd) {}

func attachProcessTree(cmd *exec.Cmd) (ProcessTreeHandle, error) {
	return 0, nil
}

func forceStopProcessTree(pid int, handle ProcessTreeHandle) error {
	return ErrProcessUnsupported
}

func terminateProcessTree(pid int, _ ProcessTreeHandle) error {
	return ErrProcessUnsupported
}

func closeProcessTree(ProcessTreeHandle) {}

func requestGracefulStopSupported() bool { return false }

func processTreeSupported() bool { return false }

func procSignalTerm(proc *os.Process) error {
	return ErrProcessUnsupported
}

func isProcessAlive(pid int) bool {
	return false
}

var ErrProcessUnsupported = errors.New("process: execution unsupported in restricted build")

func detectIsolation() PlatformIsolationReport {
	return PlatformIsolationReport{
		Platform:             "restricted",
		ProcessTreeIsolation: false,
		MemoryLimit:          false,
		CPULimit:             false,
		FilesystemIsolation:  false,
		NetworkIsolation:     false,
		UserNamespace:        false,
		Seccomp:              false,
		AppContainer:         false,
		SandboxProfile:       false,
		Limitations:          []string{"process execution disabled in restricted build"},
	}
}
