//go:build windows

package process

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func configureProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW,
		HideWindow:    true,
	}
}

func attachProcessTree(command *exec.Cmd) (ProcessTreeHandle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE |
		windows.JOB_OBJECT_LIMIT_DIE_ON_UNHANDLED_EXCEPTION |
		windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	if command.Process == nil {
		windows.CloseHandle(job)
		return 0, nil
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(command.Process.Pid))
	if err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	return ProcessTreeHandle(job), nil
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

func detectIsolation() PlatformIsolationReport {
	return PlatformIsolationReport{
		Platform:             "windows",
		ProcessTreeIsolation: true,
		MemoryLimit:          true,
		CPULimit:             false,
		FilesystemIsolation:  false,
		NetworkIsolation:     false,
		UserNamespace:        false,
		Seccomp:              false,
		AppContainer:         false,
		SandboxProfile:       false,
		Limitations: []string{
			"CPU rate limit requires Job Object CPU rate control (not implemented in first version)",
			"Filesystem isolation requires Windows AppContainer or Sandbox (not implemented)",
			"Network isolation requires Windows Firewall integration (not implemented)",
		},
	}
}
