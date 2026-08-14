//go:build windows

package browser

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	proc "github.com/u-ai/backend/internal/platform/process"
	"golang.org/x/sys/windows"
)

const procThreadAttributeHandleList uintptr = 0x00020002

type cdpProcessResult struct {
	pid           int
	handle        proc.ProcessTreeHandle
	reader        syscall.Handle
	writer        syscall.Handle
	processHandle syscall.Handle
}

func launchChromiumWithPipes(executable string, args []string, workDir string, env []string) (*cdpProcessResult, error) {
	var sa syscall.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1

	var childReadFd3, parentWriteFd3 syscall.Handle
	if err := syscall.CreatePipe(&childReadFd3, &parentWriteFd3, &sa, 0); err != nil {
		return nil, fmt.Errorf("cdp pipe fd3: %w", err)
	}

	var parentReadFd4, childWriteFd4 syscall.Handle
	if err := syscall.CreatePipe(&parentReadFd4, &childWriteFd4, &sa, 0); err != nil {
		syscall.CloseHandle(childReadFd3)
		syscall.CloseHandle(parentWriteFd3)
		return nil, fmt.Errorf("cdp pipe fd4: %w", err)
	}

	attrList, si, err := buildStartupInfo(childReadFd3, childWriteFd4)
	if err != nil {
		closeHandles(childReadFd3, parentWriteFd3, parentReadFd4, childWriteFd4)
		return nil, fmt.Errorf("build startup info: %w", err)
	}
	defer attrList.Delete()

	cmdLine, err := syscall.UTF16PtrFromString(buildCmdLine(executable, args))
	if err != nil {
		closeHandles(childReadFd3, parentWriteFd3, parentReadFd4, childWriteFd4)
		return nil, fmt.Errorf("build cmdline: %w", err)
	}

	var workDirPtr *uint16
	if workDir != "" {
		workDirPtr, err = syscall.UTF16PtrFromString(workDir)
		if err != nil {
			closeHandles(childReadFd3, parentWriteFd3, parentReadFd4, childWriteFd4)
			return nil, fmt.Errorf("workdir: %w", err)
		}
	}

	envPtr, err := buildEnvBlock(env)
	if err != nil {
		closeHandles(childReadFd3, parentWriteFd3, parentReadFd4, childWriteFd4)
		return nil, fmt.Errorf("env block: %w", err)
	}

	var pi windows.ProcessInformation
	err = windows.CreateProcess(
		nil,
		cmdLine,
		nil,
		nil,
		true,
		windows.EXTENDED_STARTUPINFO_PRESENT,
		envPtr,
		workDirPtr,
		&si.StartupInfo,
		&pi,
	)

	syscall.CloseHandle(childReadFd3)
	syscall.CloseHandle(childWriteFd4)

	if err != nil {
		syscall.CloseHandle(parentWriteFd3)
		syscall.CloseHandle(parentReadFd4)
		return nil, fmt.Errorf("create process: %w", err)
	}

	syscall.CloseHandle(syscall.Handle(pi.Thread))

	return &cdpProcessResult{
		pid:           int(pi.ProcessId),
		handle:        proc.ProcessTreeHandle(pi.Process),
		reader:        parentReadFd4,
		writer:        parentWriteFd3,
		processHandle: syscall.Handle(pi.Process),
	}, nil
}

func buildStartupInfo(fd3, fd4 syscall.Handle) (*windows.ProcThreadAttributeListContainer, windows.StartupInfoEx, error) {
	attrList, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, windows.StartupInfoEx{}, err
	}
	handles := []syscall.Handle{fd3, fd4}
	if err := attrList.Update(procThreadAttributeHandleList, unsafe.Pointer(&handles[0]), unsafe.Sizeof(syscall.Handle(0))*2); err != nil {
		attrList.Delete()
		return nil, windows.StartupInfoEx{}, err
	}
	var si windows.StartupInfoEx
	si.StartupInfo.Cb = uint32(unsafe.Sizeof(si))
	si.ProcThreadAttributeList = attrList.List()
	return attrList, si, nil
}

func closeHandles(handles ...syscall.Handle) {
	for _, h := range handles {
		if h != 0 {
			syscall.CloseHandle(h)
		}
	}
}

func buildCmdLine(executable string, args []string) string {
	parts := []string{quoteArg(executable)}
	for _, a := range args {
		parts = append(parts, quoteArg(a))
	}
	return strings.Join(parts, " ")
}

func quoteArg(s string) string {
	if !strings.ContainsAny(s, " \t\n\v\"") {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); {
		c := s[i]
		if c == '\\' {
			backslashes := 0
			for i < len(s) && s[i] == '\\' {
				backslashes++
				i++
			}
			if i >= len(s) || s[i] == '"' {
				for j := 0; j < backslashes*2; j++ {
					b.WriteByte('\\')
				}
			} else {
				for j := 0; j < backslashes; j++ {
					b.WriteByte('\\')
				}
			}
			continue
		}
		if c == '"' {
			b.WriteByte('\\')
		}
		b.WriteByte(c)
		i++
	}
	b.WriteByte('"')
	return b.String()
}

func buildEnvBlock(env []string) (*uint16, error) {
	if len(env) == 0 {
		return nil, nil
	}
	var b strings.Builder
	for _, e := range env {
		b.WriteString(e)
		b.WriteByte(0)
	}
	b.WriteByte(0)
	return syscall.UTF16PtrFromString(b.String())
}

type handleReader struct {
	handle syscall.Handle
}

func (r *handleReader) Read(p []byte) (int, error) {
	var done uint32
	err := syscall.ReadFile(r.handle, p, &done, nil)
	if err != nil {
		if err == syscall.ERROR_BROKEN_PIPE {
			return 0, fmt.Errorf("pipe closed")
		}
		return 0, err
	}
	return int(done), nil
}

func (r *handleReader) Close() error {
	if r.handle != 0 {
		syscall.CloseHandle(r.handle)
		r.handle = 0
	}
	return nil
}

type handleWriter struct {
	handle syscall.Handle
}

func (w *handleWriter) Write(p []byte) (int, error) {
	var done uint32
	err := syscall.WriteFile(w.handle, p, &done, nil)
	if err != nil {
		return 0, err
	}
	return int(done), nil
}

func (w *handleWriter) Close() error {
	if w.handle != 0 {
		syscall.CloseHandle(w.handle)
		w.handle = 0
	}
	return nil
}

func handleToIOReader(h syscall.Handle) *handleReader {
	return &handleReader{handle: h}
}

func handleToIOWriter(h syscall.Handle) *handleWriter {
	return &handleWriter{handle: h}
}
