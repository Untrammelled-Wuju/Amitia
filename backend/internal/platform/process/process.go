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
	Executable  string
	Args        []string
	WorkingDir  string
	Env         []string
	OnStdout    func(line string)
	OnStderr    func(line string)
	StartTimeout time.Duration
}

type ProcessManager interface {
	Start(ctx context.Context, config ProcessConfig) (*ManagedProcess, error)
	Stop(pid int, handle ProcessTreeHandle, gracePeriod time.Duration) error
	IsAlive(pid int) bool
	NewEnvironment() *EnvironmentBuilder
	IsolationReport() PlatformIsolationReport
}

type ManagedProcess struct {
	mu          sync.Mutex
	PID         int
	Handle      ProcessTreeHandle
	cmd         *exec.Cmd
	done        chan struct{}
	exited      bool
	exitCode    int
	exitErr     error
	startedAt   time.Time
	cancel      context.CancelFunc
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

func (p *ManagedProcess) markExited(code int, err error) {
	p.mu.Lock()
	p.exited = true
	p.exitCode = code
	p.exitErr = err
	p.mu.Unlock()
	close(p.done)
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
