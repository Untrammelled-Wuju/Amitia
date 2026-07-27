package process

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

func (m *DefaultProcessManager) Start(ctx context.Context, config ProcessConfig) (*ManagedProcess, error) {
	if config.Executable == "" {
		return nil, fmt.Errorf("%w: empty executable", ErrInvalidConfig)
	}

	procCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(procCtx, config.Executable, config.Args...)
	cmd.Dir = config.WorkingDir
	cmd.Env = config.Env
	configureProcess(cmd)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("process: create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("process: create stderr pipe: %w", err)
	}

	mp := &ManagedProcess{
		done:      make(chan struct{}),
		startedAt: time.Now(),
		cancel:    cancel,
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("process: start: %w", err)
	}

	mp.cmd = cmd
	mp.PID = cmd.Process.Pid

	handle, attachErr := attachProcessTree(cmd)
	if attachErr != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		cancel()
		return nil, fmt.Errorf("process: attach process tree: %w", attachErr)
	}
	mp.Handle = handle

	var stdoutWG sync.WaitGroup
	var stderrWG sync.WaitGroup

	stdoutWG.Add(1)
	go func() {
		defer stdoutWG.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if config.OnStdout != nil {
				config.OnStdout(line)
			}
		}
	}()

	stderrWG.Add(1)
	go func() {
		defer stderrWG.Done()
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if config.OnStderr != nil {
				config.OnStderr(line)
			}
		}
	}()

	go func() {
		err := cmd.Wait()
		stdoutWG.Wait()
		stderrWG.Wait()
		closeProcessTree(mp.Handle)
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}
		mp.markExited(exitCode, err)
	}()

	return mp, nil
}

func (m *DefaultProcessManager) Stop(pid int, handle ProcessTreeHandle, gracePeriod time.Duration) error {
	if pid <= 0 {
		return fmt.Errorf("process: invalid pid %d", pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}

	if err := sendTermination(proc); err != nil {
	}

	timer := time.NewTimer(gracePeriod)
	defer timer.Stop()
	deadline := time.Now().Add(gracePeriod)
	for {
		if !m.IsAlive(pid) {
			return nil
		}
		if time.Now().After(deadline) {
			break
		}
		select {
		case <-timer.C:
			if !m.IsAlive(pid) {
				return nil
			}
			if time.Now().After(deadline) {
				goto forceKill
			}
			timer.Reset(time.Until(deadline))
		case <-time.After(100 * time.Millisecond):
		}
	}

forceKill:
	if err := terminateProcessTree(pid, handle); err != nil {
		_ = proc.Kill()
	}
	return nil
}

func (m *DefaultProcessManager) IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func sendTermination(proc *os.Process) error {
	return proc.Signal(syscall.SIGTERM)
}
