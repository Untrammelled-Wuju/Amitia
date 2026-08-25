//go:build windows

package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	jobObjectCPURateControlEnable  = 0x1
	jobObjectCPURateControlHardCap = 0x4
)

type jobObjectCPURateControlInformation struct {
	ControlFlags uint32
	CPURate      uint32
}

func configureProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW | windows.CREATE_SUSPENDED,
		HideWindow:    true,
	}
}

func attachProcessTree(command *exec.Cmd) (ProcessTreeHandle, error) {
	return attachProcessTreeWithLimits(command, ResourceLimits{})
}

func attachProcessTreeWithLimits(command *exec.Cmd, limits ResourceLimits) (ProcessTreeHandle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}

	// Do not set BREAKAWAY_OK. A plugin child that can break away from the job can
	// evade tree termination and process-count/memory limits.
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE |
		windows.JOB_OBJECT_LIMIT_DIE_ON_UNHANDLED_EXCEPTION

	if limits.MaxMemoryBytes > 0 {
		if limits.MaxMemoryBytes > uint64(^uintptr(0)) {
			windows.CloseHandle(job)
			return 0, fmt.Errorf("process: memory limit overflows uintptr")
		}
		info.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_JOB_MEMORY
		info.JobMemoryLimit = uintptr(limits.MaxMemoryBytes)
	}
	if limits.MaxProcesses > 0 {
		info.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS
		info.BasicLimitInformation.ActiveProcessLimit = limits.MaxProcesses
	}
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}

	if limits.MaxCPUPercent > 0 {
		pct := limits.MaxCPUPercent
		if pct > 100 {
			pct = 100
		}
		cpu := jobObjectCPURateControlInformation{
			ControlFlags: jobObjectCPURateControlEnable | jobObjectCPURateControlHardCap,
			CPURate:      pct * 100, // Windows uses 1/100 of one percent (1..10000).
		}
		if cpu.CPURate == 0 || cpu.CPURate > 10000 {
			windows.CloseHandle(job)
			return 0, fmt.Errorf("process: invalid CPU rate %d", cpu.CPURate)
		}
		if _, err = windows.SetInformationJobObject(job, windows.JobObjectCpuRateControlInformation, uintptr(unsafe.Pointer(&cpu)), uint32(unsafe.Sizeof(cpu))); err != nil {
			windows.CloseHandle(job)
			return 0, fmt.Errorf("process: apply CPU job limit: %w", err)
		}
	}

	if command.Process == nil {
		windows.CloseHandle(job)
		return 0, errors.New("process: cannot attach job before process start")
	}
	processHandle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(command.Process.Pid))
	if err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	defer windows.CloseHandle(processHandle)
	if err := windows.AssignProcessToJobObject(job, processHandle); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}

	// The child is created suspended so it cannot execute code or create a child
	// before the Job Object boundary and requested limits are active. Resume only
	// after assignment succeeds; startup failure leaves the process suspended and
	// the supervisor's abort path terminates it.
	if err := resumeSuspendedProcess(uint32(command.Process.Pid)); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	return ProcessTreeHandle(job), nil
}

func resumeSuspendedProcess(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("process: snapshot threads for resume: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	if err := windows.Thread32First(snapshot, &entry); err != nil {
		return fmt.Errorf("process: enumerate suspended process threads: %w", err)
	}

	resumed := 0
	for {
		if entry.OwnerProcessID == pid {
			thread, openErr := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
			if openErr != nil {
				return fmt.Errorf("process: open suspended thread %d: %w", entry.ThreadID, openErr)
			}
			_, resumeErr := windows.ResumeThread(thread)
			windows.CloseHandle(thread)
			if resumeErr != nil {
				return fmt.Errorf("process: resume thread %d: %w", entry.ThreadID, resumeErr)
			}
			resumed++
		}

		err = windows.Thread32Next(snapshot, &entry)
		if err == nil {
			continue
		}
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			break
		}
		return fmt.Errorf("process: enumerate suspended process threads: %w", err)
	}
	if resumed == 0 {
		return fmt.Errorf("process: no suspended thread found for pid %d", pid)
	}
	return nil
}

func resourceLimitSupport() ResourceLimitSupport {
	return ResourceLimitSupport{Memory: true, CPU: true, Processes: true}
}

func terminateProcessTree(pid int, handle ProcessTreeHandle) error {
	if handle != 0 {
		return windows.TerminateJobObject(windows.Handle(handle), 1)
	}
	proc, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(proc)
	return windows.TerminateProcess(proc, 1)
}

func closeProcessTree(handle ProcessTreeHandle) {
	if handle != 0 {
		windows.CloseHandle(windows.Handle(handle))
	}
}

func requestGracefulStopSupported() bool { return false }

func processTreeSupported() bool { return true }

func procSignalTerm(proc *os.Process) error {
	if proc == nil {
		return errors.New("process: nil process")
	}
	// Try to send Ctrl+Break to the process group.
	err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(proc.Pid))
	if err != nil {
		return fmt.Errorf("process: %w", err)
	}
	return nil
}

const stillActive = 259 // STATUS_PENDING

func forceStopProcessTree(pid int, handle ProcessTreeHandle) error {
	return terminateProcessTree(pid, handle)
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	var code uint32
	if err := windows.GetExitCodeProcess(handle, &code); err != nil {
		return false
	}
	return code == stillActive
}

func detectIsolation() PlatformIsolationReport {
	return PlatformIsolationReport{
		Platform:             "windows",
		ProcessTreeIsolation: true,
		MemoryLimit:          true,
		CPULimit:             true,
		FilesystemIsolation:  false,
		NetworkIsolation:     false,
		UserNamespace:        false,
		Seccomp:              false,
		AppContainer:         false,
		SandboxProfile:       false,
		Limitations: []string{
			"filesystem isolation requires AppContainer or an equivalent Windows sandbox backend",
			"restricted/loopback network isolation requires AppContainer or WFP integration",
			"file-descriptor and disk quotas are not enforced by Job Objects",
		},
	}
}
