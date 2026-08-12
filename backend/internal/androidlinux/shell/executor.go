//go:build linux && !android

package shell

import (
	"context"
	"io"
	"os/exec"
	"time"
)

type ShellExecutor interface {
	Execute(ctx context.Context, request ShellExecuteRequest) ShellExecuteResult
}

type ShellExecutorImpl struct {
	policy          ShellPolicy
	envBuilder      *EnvironmentBuilder
	dirResolver     *WorkingDirResolver
	processManager  *ProcessManager
}

func NewShellExecutor(policy ShellPolicy, workspaceRoot, tempRoot string) *ShellExecutorImpl {
	return &ShellExecutorImpl{
		policy:         policy,
		envBuilder:     NewEnvironmentBuilder(policy),
		dirResolver:    NewWorkingDirResolver(workspaceRoot, tempRoot),
		processManager: NewProcessManager(),
	}
}

func (e *ShellExecutorImpl) Execute(ctx context.Context, request ShellExecuteRequest) ShellExecuteResult {
	startTime := time.Now()
	result := ShellExecuteResult{}

	if !e.policy.Enabled {
		result.ExitCode = 1
		result.Stderr = ErrShellNotAvailable("disabled").Error()
		result.WorkingDir = ""
		return result
	}

	if err := e.validateRequest(&request); err != nil {
		result.ExitCode = 1
		result.Stderr = err.Error()
		result.DurationMs = time.Since(startTime).Milliseconds()
		return result
	}

	workDir, err := e.dirResolver.Resolve(request.WorkingDir)
	if err != nil {
		result.ExitCode = 1
		result.Stderr = err.Error()
		result.DurationMs = time.Since(startTime).Milliseconds()
		return result
	}
	result.WorkingDir = workDir

	env, envSlice, err := e.envBuilder.Build(request.Environment)
	if err != nil {
		result.ExitCode = 1
		result.Stderr = err.Error()
		result.WorkingDir = workDir
		result.DurationMs = time.Since(startTime).Milliseconds()
		return result
	}
	_ = env

	timeoutMs := e.calculateTimeout(request.TimeoutMs)

	maxStdout := e.policy.MaxStdoutBytes
	maxStderr := e.policy.MaxStderrBytes
	maxCombined := maxStdout + maxStderr + 512*1024

	if request.MaxOutputBytes > 0 && request.MaxOutputBytes < maxCombined {
		ratio := float64(request.MaxOutputBytes) / float64(maxCombined)
		maxStdout = int64(float64(maxStdout) * ratio)
		maxStderr = int64(float64(maxStderr) * ratio)
		maxCombined = request.MaxOutputBytes
	}

	argv := e.buildArgv(request)

	outputBuf := NewOutputBuffer(maxStdout, maxStderr, maxCombined)

	procCfg := ProcessConfig{
		Argv:       argv,
		WorkingDir: workDir,
		Env:        envSlice,
		Stdin:      []byte(request.Stdin),
		TimeoutMs:  timeoutMs,
		Setpgid:    true,
	}

	execCtx, execCancel := context.WithCancel(ctx)
	defer execCancel()

	cmd, state, err := e.processManager.Start(execCtx, procCfg)
	if err != nil {
		result.ExitCode = 1
		result.Stderr = err.Error()
		result.WorkingDir = workDir
		result.DurationMs = time.Since(startTime).Milliseconds()
		return result
	}

	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})

	go e.readOutput(cmd, state, outputBuf, stdoutDone, stderrDone)

	waitDone := make(chan struct{})
	go func() {
		<-state.doneCh
		outputBuf.MarkComplete()
		close(waitDone)
	}()

	select {
	case <-ctx.Done():
		outputBuf.MarkComplete()
		e.processManager.KillProcessTree(state)
		result.ExitCode = 130
		result.Signal = "SIGINT"
		result.TimedOut = false
		result.Stderr = ErrCancelled().Error()
	case <-waitDone:
		result.ExitCode, result.Signal = calculateExitCode(cmd, nil)
		if result.ExitCode != 0 && result.ExitCode != 143 && result.Signal == "" {
		}
	case <-outputBuf.Done():
		result.TimedOut = true
		e.processManager.KillProcessTree(state)
		<-waitDone
		result.ExitCode = 137
		result.Signal = "SIGKILL"
		result.Stderr = ErrOutputLimitExceeded("combined", maxCombined).Error()
	}

	result.Stdout = outputBuf.StdoutString()
	result.Stderr = result.Stderr + outputBuf.StderrString()
	result.StdoutTruncated = outputBuf.IsStdoutTruncated()
	result.StderrTruncated = outputBuf.IsStderrTruncated()
	result.StdoutBytes = outputBuf.StdoutSize()
	result.StderrBytes = outputBuf.StderrSize()
	result.WorkingDir = workDir
	result.DurationMs = time.Since(startTime).Milliseconds()

	return result
}

func (e *ShellExecutorImpl) readOutput(cmd *exec.Cmd, state *processState, buf *OutputBuffer, stdoutDone, stderrDone chan struct{}) {
	if cmd.Stdout != nil {
		go func() {
			defer close(stdoutDone)
			tempBuf := make([]byte, 4096)
				for {
					n, err := cmd.Stdout.(io.Reader).Read(tempBuf)
					if n > 0 {
						if _, stop := buf.WriteStdout(tempBuf[:n]); stop {
							return
						}
					}
					if err != nil {
						return
				}
			}
		}()
	} else {
		close(stdoutDone)
	}

	if cmd.Stderr != nil {
		go func() {
			defer close(stderrDone)
			tempBuf := make([]byte, 4096)
			for {
				n, err := cmd.Stderr.(io.Reader).Read(tempBuf)
				if n > 0 {
					if _, stop := buf.WriteStderr(tempBuf[:n]); stop {
						return
					}
				}
				if err != nil {
					return
				}
			}
		}()
	} else {
		close(stderrDone)
	}
}

func (e *ShellExecutorImpl) validateRequest(request *ShellExecuteRequest) error {
	if request.Mode == "" {
		request.Mode = ShellModeArgv
	}

	switch request.Mode {
	case ShellModeArgv:
		if request.Executable == "" {
			return ErrExecutableRequired()
		}
	case ShellModeShell:
		if request.Command == "" {
			return ErrCommandRequired()
		}
	default:
		return ErrInvalidMode(string(request.Mode))
	}

	if int64(len(request.Stdin)) > e.policy.MaxStdinBytes {
		return ErrStdinTooLarge(int64(len(request.Stdin)), e.policy.MaxStdinBytes)
	}

	return nil
}

func (e *ShellExecutorImpl) calculateTimeout(requestTimeout int64) int64 {
	if requestTimeout <= 0 {
		return int64(e.policy.DefaultTimeout / time.Millisecond)
	}
	maxTimeout := int64(e.policy.MaxTimeout / time.Millisecond)
	if requestTimeout > maxTimeout {
		return maxTimeout
	}
	return requestTimeout
}

func (e *ShellExecutorImpl) buildArgv(request ShellExecuteRequest) []string {
	switch request.Mode {
	case ShellModeShell:
		return []string{"/bin/sh", "-lc", request.Command}
	case ShellModeArgv:
		fallthrough
	default:
		argv := []string{request.Executable}
		argv = append(argv, request.Args...)
		return argv
	}
}

var _ ShellExecutor = (*ShellExecutorImpl)(nil)
