//go:build !windows

package browser

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"time"

	proc "github.com/u-ai/backend/internal/platform/process"
	"github.com/u-ai/backend/internal/runtimehost"
)

func (p *browserProcess) Start() (pid int, handle proc.ProcessTreeHandle, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureProfileDir(); err != nil {
		return 0, 0, &BrowserError{
			Code:    ErrCodeBrowserStartFailed,
			Message: "failed to create browser profile directory",
			Cause:   err,
		}
	}

	args := p.buildArgs()
	safeEnv := p.sanitizeEnv(os.Environ())

	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, p.execInfo.Path, args...)
	cmd.Env = safeEnv
	cmd.Dir = p.profileDir

	pr, pw, err := os.Pipe()
	if err != nil {
		cancel()
		return 0, 0, err
	}
	cmd.ExtraFiles = []*os.File{pr, pw}

	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		cancel()
		return 0, 0, err
	}

	p.pid = cmd.Process.Pid
	p.procHandle = proc.ProcessTreeHandle(0)
	p.reader = pw
	p.writer = pr
	p.alive = true
	p.connected = false

	go func() {
		cmd.Wait()
		p.mu.Lock()
		p.alive = false
		p.connected = false
		p.mu.Unlock()
		cancel()
	}()

	return p.pid, p.procHandle, nil
}

func (p *browserProcess) Stop(handle proc.ProcessTreeHandle, pid int, gracePeriod time.Duration) error {
	p.mu.Lock()
	client := p.cdpClient
	reader := p.reader
	writer := p.writer
	alive := p.alive
	p.mu.Unlock()

	if !alive {
		return nil
	}

	if client != nil {
		client.Close()
	}
	if reader != nil {
		reader.Close()
	}
	if writer != nil {
		writer.Close()
	}

	if pid > 0 {
		_ = syscall.Kill(pid, syscall.SIGTERM)
		deadline := time.Now().Add(gracePeriod)
		for time.Now().Before(deadline) {
			if !isProcessAliveByPID(pid) {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if isProcessAliveByPID(pid) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}

	p.mu.Lock()
	p.alive = false
	p.connected = false
	p.mu.Unlock()

	return nil
}

var _ runtimehost.ProcessExec = (*browserProcess)(nil)
var _ runtimehost.ProcessStopper = (*browserProcess)(nil)
