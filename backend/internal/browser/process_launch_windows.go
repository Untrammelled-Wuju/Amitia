//go:build windows

package browser

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	proc "github.com/u-ai/backend/internal/platform/process"
	"golang.org/x/sys/windows"
)

func launchPlatformProcess(p *browserProcess, args []string) error {
	safeEnv := p.sanitizeEnv(os.Environ())

	result, err := launchChromiumWithPipes(p.execInfo.Path, args, p.profileDir, safeEnv)
	if err != nil {
		return err
	}

	p.pid = result.pid
	p.procHandle = result.handle
	p.reader = handleToIOReader(result.reader)
	p.writer = handleToIOWriter(result.writer)
	return nil
}

func isProcessAliveByPID(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(syscall.Handle(h))
	var code uint32
	if err := syscall.GetExitCodeProcess(syscall.Handle(h), &code); err != nil {
		return false
	}
	return code == 259
}

func killProcessTreeHandle(handle proc.ProcessTreeHandle) {
	if handle == 0 {
		return
	}
	_ = exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", int(handle))).Run()
}

func removeProfileDir(path string) {
	_ = os.RemoveAll(path)
}

func makeProfileDir(path string) error {
	return os.MkdirAll(path, 0700)
}
