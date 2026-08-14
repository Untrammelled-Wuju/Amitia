// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package process

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"time"
)

type ProcessTreeHandle uintptr

type ProcessConfig struct {
	Executable   string
	Args         []string
	WorkingDir   string
	Env          []string
	OnStdout     func(line string)
	OnStderr     func(line string)
	OnScannerError func(error)
	StartTimeout time.Duration
}

type ProcessManager interface {
	Start(ctx context.Context, config ProcessConfig) (*ManagedProcess, error)
	Stop(pid int, handle ProcessTreeHandle, gracePeriod time.Duration) error
	StopContext(ctx context.Context, pid int, handle ProcessTreeHandle, gracePeriod time.Duration) error
	IsAlive(pid int) bool
	IsProcessAlive(pid int) bool
	NewEnvironment() *EnvironmentBuilder
	IsolationReport() PlatformIsolationReport
	GracefulStopSupported() bool
	ProcessTreeSupported() bool
}

type ManagedProcess struct {
	mu            sync.Mutex
	PID           int
	Handle        ProcessTreeHandle
	cmd           *exec.Cmd
	done          chan struct{}
	exited        bool
	exitCode      int
	exitErr       error
	startedAt     time.Time
	cancel        context.CancelFunc
}

func (p *ManagedProcess) Wait() (int, error) {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exitCode, p.exitErr
}

func (p *ManagedProcess) Done() <-chan struct{} {
	return p.done
}

func (p *ManagedProcess) Exited() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exited
}

func (p *ManagedProcess) ExitCode() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exitCode
}

func (p *ManagedProcess) StartedAt() time.Time {
	return p.startedAt
}

func (p *ManagedProcess) markExited(code int, err error) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.exited {
		return false
	}
	p.exited = true
	p.exitCode = code
	p.exitErr = err
	close(p.done)
	return true
}

func NewExternalManagedProcess(pid int, handle ProcessTreeHandle) *ManagedProcess {
	p := &ManagedProcess{
		PID:       pid,
		Handle:    handle,
		done:      make(chan struct{}),
		startedAt: time.Now(),
	}
	go func() {
		for {
			if !isProcessAlive(pid) {
				p.markExited(0, nil)
				return
			}
			select {
			case <-p.done:
				return
			case <-time.After(500 * time.Millisecond):
			}
		}
	}()
	return p
}

var (
	ErrProcessAlreadyExited = errors.New("process: already exited")
	ErrStartTimeout         = errors.New("process: start timeout")
	ErrInvalidConfig        = errors.New("process: invalid config")
)

type DefaultProcessManager struct {
	isolation PlatformIsolationReport
}

func NewDefaultProcessManager() *DefaultProcessManager {
	return &DefaultProcessManager{
		isolation: detectIsolation(),
	}
}

func (m *DefaultProcessManager) IsolationReport() PlatformIsolationReport {
	return m.isolation
}

func (m *DefaultProcessManager) NewEnvironment() *EnvironmentBuilder {
	return NewEnvironmentBuilder()
}

func (m *DefaultProcessManager) GracefulStopSupported() bool {
	return requestGracefulStopSupported()
}

func (m *DefaultProcessManager) ProcessTreeSupported() bool {
	return processTreeSupported()
}

var _ ProcessManager = (*DefaultProcessManager)(nil)
