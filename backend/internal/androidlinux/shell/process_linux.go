//go:build linux && !android

package shell

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

type ProcessConfig struct {
	Argv        []string
	WorkingDir  string
	Env         []string
	Stdin       []byte
	TimeoutMs   int64
	Setpgid     bool
}

type ProcessResult struct {
	ExitCode    int
	Stdout      []byte
	Stderr      []byte
	DurationMs  int64
	TimedOut    bool
	Signal      string
}

type ProcessManager struct {
	gracePeriod time.Duration
}

func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		gracePeriod: 2 * time.Second,
	}
}

func (pm *ProcessManager) Start(ctx context.Context, cfg ProcessConfig) (*exec.Cmd, *processState, error) {
	timeout := cfg.TimeoutMs
	if timeout <= 0 {
		timeout = int64(DefaultShellPolicy().DefaultTimeout / time.Millisecond)
	}

	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)

	cmd := exec.CommandContext(execCtx, cfg.Argv[0], cfg.Argv[1:]...)
	cmd.Dir = cfg.WorkingDir
	cmd.Env = cfg.Env

	if cfg.Setpgid {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
			Pgid:    0,
		}
	}

	if len(cfg.Stdin) > 0 {
		cmd.Stdin = bytes.NewReader(cfg.Stdin)
	}

	if _, err := cmd.StdoutPipe(); err != nil {
		cancel()
		return nil, nil, ErrProcessStartFailed(err.Error())
	}

	if _, err := cmd.StderrPipe(); err != nil {
		cancel()
		return nil, nil, ErrProcessStartFailed(err.Error())
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, nil, ErrProcessStartFailed(err.Error())
	}

	state := &processState{
		cmd:      cmd,
		cancel:   cancel,
		doneCh:   make(chan struct{}),
		pgid:     getProcessGroup(cmd),
		hasPgid:  cfg.Setpgid,
	}

	go func() {
		cmd.Wait()
		close(state.doneCh)
	}()

	return cmd, state, nil
}

func (pm *ProcessManager) KillProcessTree(state *processState) error {
	if state == nil {
		return nil
	}

	if !state.hasPgid || state.pgid == 0 {
		return state.cmd.Process.Kill()
	}

	if err := syscall.Kill(-state.pgid, syscall.SIGTERM); err != nil {
		return state.cmd.Process.Kill()
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-state.doneCh:
			close(done)
		case <-time.After(pm.gracePeriod):
			close(done)
		}
	}()

	<-done

	if !state.cmd.ProcessState.Exited() {
		syscall.Kill(-state.pgid, syscall.SIGKILL)
	}

	return nil
}

type processState struct {
	cmd      *exec.Cmd
	cancel   context.CancelFunc
	doneCh   chan struct{}
	pgid     int
	hasPgid  bool
}

func getProcessGroup(cmd *exec.Cmd) int {
	if cmd.Process == nil {
		return 0
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return 0
	}
	return pgid
}

func calculateExitCode(cmd *exec.Cmd, err error) (int, string) {
	if cmd.ProcessState == nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			status := exitErr.Sys().(syscall.WaitStatus)
			if status.Signaled() {
				return 128 + int(status.Signal()), signalName(status.Signal())
			}
			return exitErr.ExitCode(), ""
		}
		return 1, ""
	}

	status := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if status.Signaled() {
		return 128 + int(status.Signal()), signalName(status.Signal())
	}
	return status.ExitStatus(), ""
}

func signalName(sig syscall.Signal) string {
	switch sig {
	case syscall.SIGTERM:
		return "SIGTERM"
	case syscall.SIGKILL:
		return "SIGKILL"
	case syscall.SIGINT:
		return "SIGINT"
	case syscall.SIGQUIT:
		return "SIGQUIT"
	case syscall.SIGSEGV:
		return "SIGSEGV"
	case syscall.SIGABRT:
		return "SIGABRT"
	default:
		return fmt.Sprintf("SIG_%d", int(sig))
	}
}
