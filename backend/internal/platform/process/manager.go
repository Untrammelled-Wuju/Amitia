// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package process

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
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
		if scanErr := scanner.Err(); scanErr != nil && config.OnScannerError != nil {
			config.OnScannerError(scanErr)
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
		if scanErr := scanner.Err(); scanErr != nil && config.OnScannerError != nil {
			config.OnScannerError(scanErr)
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
		cancel()
	}()

	return mp, nil
}

func (m *DefaultProcessManager) Stop(pid int, handle ProcessTreeHandle, gracePeriod time.Duration) error {
	return m.StopContext(context.Background(), pid, handle, gracePeriod)
}

func (m *DefaultProcessManager) StopContext(ctx context.Context, pid int, handle ProcessTreeHandle, gracePeriod time.Duration) error {
	if pid <= 0 {
		return nil
	}
	if !m.IsAlive(pid) {
		return nil
	}

	gracefulErr := requestGracefulStop(pid)
	_ = gracefulErr

	done := make(chan struct{})
	go func() {
		deadline := time.Now().Add(gracePeriod)
		for {
			if !m.IsAlive(pid) {
				close(done)
				return
			}
			if time.Now().After(deadline) {
				break
			}
			select {
			case <-done:
				return
			case <-time.After(50 * time.Millisecond):
			}
		}
		forceStopProcessTree(pid, handle)
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		forceStopProcessTree(pid, handle)
		return ctx.Err()
	}
}

func (m *DefaultProcessManager) IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return isProcessAlive(pid)
}

func (m *DefaultProcessManager) IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return isProcessAlive(pid)
}

func sendTermination(proc *os.Process) error {
	return procSignalTerm(proc)
}

func requestGracefulStop(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return procSignalTerm(proc)
}
