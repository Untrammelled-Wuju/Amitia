//go:build !windows

package browser

import (
	"context"
	"os"
	"os/exec"
	"syscall"

	proc "github.com/u-ai/backend/internal/platform/process"
)

func launchPlatformProcess(p *browserProcess, args []string) error {
	safeEnv := p.sanitizeEnv(os.Environ())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, p.execInfo.Path, args...)
	cmd.Env = safeEnv
	cmd.Dir = p.profileDir

	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	cmd.ExtraFiles = []*os.File{pr, pw}

	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return err
	}

	p.pid = cmd.Process.Pid
	p.procHandle = proc.ProcessTreeHandle(0)
	p.reader = pw
	p.writer = pr

	go func() {
		cmd.Wait()
		p.mu.Lock()
		p.alive = false
		p.connected = false
		p.mu.Unlock()
	}()

	return nil
}

func isProcessAliveByPID(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func killProcessTreeHandle(_ proc.ProcessTreeHandle) {
}

func removeProfileDir(path string) {
	os.RemoveAll(path)
}

func makeProfileDir(path string) error {
	return os.MkdirAll(path, 0700)
}
