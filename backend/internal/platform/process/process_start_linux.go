//go:build linux && !android

package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func startProcessTreeWithLimits(cmd *exec.Cmd, limits ResourceLimits) (ProcessTreeHandle, error) {
	if cmd == nil {
		return 0, fmt.Errorf("process: nil command")
	}
	if strings.TrimSpace(os.Getenv("AMITIA_RUNTIME_MODE")) == "android-proot" ||
		(limits.MaxMemoryBytes == 0 && limits.MaxCPUPercent == 0 && limits.MaxProcesses == 0) {
		if err := cmd.Start(); err != nil {
			return 0, err
		}
		return attachProcessTree(cmd)
	}
	support := resourceLimitSupport()
	if limits.MaxMemoryBytes > 0 && !support.Memory {
		return 0, fmt.Errorf("process: atomic cgroup launch requires delegated memory controller")
	}
	if limits.MaxCPUPercent > 0 && !support.CPU {
		return 0, fmt.Errorf("process: atomic cgroup launch requires delegated CPU controller")
	}
	if limits.MaxProcesses > 0 && !support.Processes {
		return 0, fmt.Errorf("process: atomic cgroup launch requires delegated pids controller")
	}
	parent, err := currentCgroupV2Dir()
	if err != nil {
		return 0, err
	}
	seq := linuxCgroupSequence.Add(1)
	group := filepath.Join(parent, fmt.Sprintf("amitia-task-%d-%d", os.Getpid(), seq))
	if err := os.Mkdir(group, 0o755); err != nil {
		return 0, fmt.Errorf("process: create pre-exec cgroup: %w", err)
	}
	cleanup := func() { _ = os.Remove(group) }
	if limits.MaxMemoryBytes > 0 {
		if err := writeCgroupValue(group, "memory.max", strconv.FormatUint(limits.MaxMemoryBytes, 10)); err != nil {
			cleanup()
			return 0, err
		}
	}
	if limits.MaxCPUPercent > 0 {
		pct := limits.MaxCPUPercent
		if pct > 100 {
			pct = 100
		}
		quota := uint64(100000) * uint64(pct) / 100
		if quota == 0 {
			quota = 1
		}
		if err := writeCgroupValue(group, "cpu.max", fmt.Sprintf("%d 100000", quota)); err != nil {
			cleanup()
			return 0, err
		}
	}
	if limits.MaxProcesses > 0 {
		if err := writeCgroupValue(group, "pids.max", strconv.FormatUint(uint64(limits.MaxProcesses), 10)); err != nil {
			cleanup()
			return 0, err
		}
	}
	fd, err := os.Open(group)
	if err != nil {
		cleanup()
		return 0, fmt.Errorf("process: open pre-exec cgroup: %w", err)
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	cmd.SysProcAttr.UseCgroupFD = true
	cmd.SysProcAttr.CgroupFD = int(fd.Fd())
	err = cmd.Start()
	_ = fd.Close()
	if err != nil {
		cleanup()
		return 0, fmt.Errorf("process: atomic cgroup start (clone3): %w", err)
	}
	handle := ProcessTreeHandle(linuxCgroupSequence.Add(1))
	if handle == 0 {
		handle = ProcessTreeHandle(linuxCgroupSequence.Add(1))
	}
	linuxCgroupMu.Lock()
	linuxCgroupHandles[handle] = group
	linuxCgroupMu.Unlock()
	return handle, nil
}
