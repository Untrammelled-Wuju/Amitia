//go:build windows

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const supervisorFlag = "--supervise"

func hasSupervisorFlag(args []string) bool {
	for _, arg := range args {
		if arg == supervisorFlag {
			return true
		}
	}
	return false
}

func supervisorChildArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == supervisorFlag {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func runSupervisor(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(job)

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		return err
	}

	cmdLine := windows.ComposeCommandLine(append([]string{exe}, supervisorChildArgs(args)...))
	cmdLinePtr, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		return err
	}
	startup := windows.StartupInfo{
		Cb:        uint32(unsafe.Sizeof(windows.StartupInfo{})),
		Flags:     windows.STARTF_USESTDHANDLES,
		StdInput:  windows.Handle(os.Stdin.Fd()),
		StdOutput: windows.Handle(os.Stdout.Fd()),
		StdErr:    windows.Handle(os.Stderr.Fd()),
	}
	processInfo := windows.ProcessInformation{}
	if err := windows.CreateProcess(nil, cmdLinePtr, nil, nil, true, windows.CREATE_SUSPENDED, nil, nil, &startup, &processInfo); err != nil {
		return err
	}
	defer windows.CloseHandle(processInfo.Thread)
	defer windows.CloseHandle(processInfo.Process)

	if err := windows.AssignProcessToJobObject(job, processInfo.Process); err != nil {
		_ = windows.TerminateProcess(processInfo.Process, 1)
		return err
	}
	if _, err := windows.ResumeThread(processInfo.Thread); err != nil {
		_ = windows.TerminateJobObject(job, 1)
		return err
	}

	fmt.Printf("AMITIA_CORE_PID=%d\n", processInfo.ProcessId)
	_ = os.Stdout.Sync()

	terminate := make(chan struct{}, 1)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == "shutdown" {
				terminate <- struct{}{}
				return
			}
		}
		terminate <- struct{}{}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		<-signals
		terminate <- struct{}{}
	}()

	exited := make(chan struct{}, 1)
	go func() {
		_, _ = windows.WaitForSingleObject(processInfo.Process, windows.INFINITE)
		exited <- struct{}{}
	}()

	select {
	case <-terminate:
		_ = windows.TerminateJobObject(job, 1)
		<-exited
	case <-exited:
	}

	var exitCode uint32
	_ = windows.GetExitCodeProcess(processInfo.Process, &exitCode)
	if exitCode != 0 {
		return fmt.Errorf("core exited with code %d", exitCode)
	}
	return nil
}
