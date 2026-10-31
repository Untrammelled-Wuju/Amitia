// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build linux && !android

package process

import (
	"bufio"
	"fmt"
	amitialog "log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
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

var (
	linuxCgroupProbeOnce sync.Once
	linuxCgroupSupport   ResourceLimitSupport
	linuxCgroupMu        sync.Mutex
	linuxCgroupHandles   = make(map[ProcessTreeHandle]string)
	linuxCgroupSequence  atomic.Uint64
)

func resourceLimitSupport() ResourceLimitSupport {
	linuxCgroupProbeOnce.Do(func() {
		linuxCgroupSupport = probeCgroupV2Support()
	})
	return linuxCgroupSupport
}

func probeCgroupV2Support() ResourceLimitSupport {
	parent, err := currentCgroupV2Dir()
	if err != nil {
		return ResourceLimitSupport{}
	}
	probe := filepath.Join(parent, fmt.Sprintf(".amitia-cgroup-probe-%d-%d", os.Getpid(), time.Now().UnixNano()))
	if err := os.Mkdir(probe, 0o755); err != nil {
		return ResourceLimitSupport{}
	}
	defer os.Remove(probe)
	return ResourceLimitSupport{
		Memory:    regularWritableFile(filepath.Join(probe, "memory.max")),
		CPU:       regularWritableFile(filepath.Join(probe, "cpu.max")),
		Processes: regularWritableFile(filepath.Join(probe, "pids.max")),
	}
}

func currentCgroupV2Dir() (string, error) {
	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return "", fmt.Errorf("process: read current cgroup: %w", err)
	}
	defer f.Close()

	var relative string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "0::") {
			relative = strings.TrimPrefix(line, "0::")
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("process: scan current cgroup: %w", err)
	}
	if relative == "" {
		return "", fmt.Errorf("process: unified cgroup v2 membership is unavailable")
	}
	cleaned := filepath.Clean("/" + strings.TrimPrefix(relative, "/"))
	root := filepath.Clean("/sys/fs/cgroup")
	candidate := filepath.Join(root, strings.TrimPrefix(cleaned, "/"))
	if candidate != root && !strings.HasPrefix(candidate, root+string(filepath.Separator)) {
		return "", fmt.Errorf("process: invalid cgroup path")
	}
	if _, err := os.Stat(filepath.Join(root, "cgroup.controllers")); err != nil {
		return "", fmt.Errorf("process: cgroup v2 is unavailable: %w", err)
	}
	if info, err := os.Stat(candidate); err != nil || !info.IsDir() {
		return "", fmt.Errorf("process: current cgroup directory is unavailable")
	}
	return candidate, nil
}

func regularWritableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	if err := f.Close(); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	return true
}

func writeCgroupValue(group, name, value string) error {
	path := filepath.Join(group, name)
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		return fmt.Errorf("process: write cgroup %s: %w", name, err)
	}
	return nil
}

func cgroupPathForHandle(handle ProcessTreeHandle) string {
	if handle == 0 {
		return ""
	}
	linuxCgroupMu.Lock()
	defer linuxCgroupMu.Unlock()
	return linuxCgroupHandles[handle]
}

func forceStopProcessTree(pid int, handle ProcessTreeHandle) error {
	return terminateProcessTree(pid, handle)
}

func terminateProcessTree(pid int, handle ProcessTreeHandle) error {

	if group := cgroupPathForHandle(handle); group != "" {
		if _, err := os.Stat(filepath.Join(group, "cgroup.kill")); err == nil {
			if err := writeCgroupValue(group, "cgroup.kill", "1"); err == nil {
				return nil
			}
		}
	}
	if pid <= 0 {
		return nil
	}
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return syscall.Kill(pid, syscall.SIGKILL)
	}
	return syscall.Kill(-pgid, syscall.SIGKILL)
}

func closeProcessTree(handle ProcessTreeHandle) {
	if handle == 0 {
		return
	}
	linuxCgroupMu.Lock()
	group := linuxCgroupHandles[handle]
	delete(linuxCgroupHandles, handle)
	linuxCgroupMu.Unlock()
	if group == "" {
		return
	}
	// A service must not orphan daemonized children after its root process exits.
	if _, err := os.Stat(filepath.Join(group, "cgroup.kill")); err == nil {
		if err := writeCgroupValue(group, "cgroup.kill", "1"); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
	for i := 0; i < 5; i++ {
		if err := os.Remove(group); err == nil || os.IsNotExist(err) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func terminateRemainingProcessTree(pid int, handle ProcessTreeHandle) error {
	if group := cgroupPathForHandle(handle); group != "" {
		if err := writeCgroupValue(group, "cgroup.kill", "1"); err == nil {
			return nil
		}
	}
	if pid <= 0 {
		return nil
	}
	err := syscall.Kill(-pid, syscall.SIGKILL)
	if err == syscall.ESRCH {
		return nil
	}
	return err
}

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
	support := resourceLimitSupport()
	limitations := []string{
		"seccomp filter is not applied by the process backend",
		"filesystem/network isolation is enforced by TrustedService bubblewrap launch plans rather than the process manager",
	}
	if !support.Memory || !support.CPU || !support.Processes {
		limitations = append(limitations, "cgroup v2 resource limits require a writable/delegated current cgroup")
	}
	return PlatformIsolationReport{
		Platform:             "linux",
		ProcessTreeIsolation: true,
		MemoryLimit:          support.Memory,
		CPULimit:             support.CPU,
		FilesystemIsolation:  firstExecutable("/usr/bin/bwrap", "/bin/bwrap") != "",
		NetworkIsolation:     firstExecutable("/usr/bin/bwrap", "/bin/bwrap") != "",
		UserNamespace:        firstExecutable("/usr/bin/bwrap", "/bin/bwrap") != "",
		Seccomp:              false,
		AppContainer:         false,
		SandboxProfile:       firstExecutable("/usr/bin/bwrap", "/bin/bwrap") != "",
		Limitations:          limitations,
	}
}

func firstExecutable(paths ...string) string {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0 {
			return path
		}
	}
	return ""
}
