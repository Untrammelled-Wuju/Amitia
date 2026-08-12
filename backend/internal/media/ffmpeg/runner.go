package ffmpeg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/runtimehost"
)

type ProcessResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	Duration time.Duration
}

type Runner struct {
	host     runtimehost.RuntimeHost
	config   Config
	mu       sync.Mutex
	running  map[runtimehost.ProcessID]context.CancelFunc
	sem      chan struct{}
}

func NewRunner(host runtimehost.RuntimeHost, config Config) *Runner {
	maxConcurrent := config.EffectiveMaxConcurrent()
	return &Runner{
		host:    host,
		config:  config,
		running: make(map[runtimehost.ProcessID]context.CancelFunc),
		sem:     make(chan struct{}, maxConcurrent),
	}
}

func (r *Runner) RunProcess(ctx context.Context, executable string, args []string) (*ProcessResult, error) {
	return r.RunProcessWithOptions(ctx, executable, args, RunOptions{})
}

type RunOptions struct {
	StdinInput   []byte
	OnStdoutLine func(line string)
	OnStderrLine func(line string)
}

func (r *Runner) RunProcessWithOptions(ctx context.Context, executable string, args []string, opts RunOptions) (*ProcessResult, error) {
	if !r.host.Capabilities().Supports(runtimehost.CapProcessSpawn) {
		return nil, NewError(FFMPEG_PROCESS_SPAWN_UNSUPPORTED, "process spawn capability not available")
	}

	select {
	case r.sem <- struct{}{}:
		defer func() { <-r.sem }()
	case <-ctx.Done():
		return nil, NewError(FFMPEG_PROCESS_CANCELLED, "cancelled while waiting for execution slot")
	}

	processID := generateProcessID("media.ffmpeg")
	runCtx, cancel := context.WithTimeout(ctx, r.config.MaxProcessDuration)
	defer cancel()

	r.trackProcess(processID, cancel)
	defer r.untrackProcess(processID)

	var stdoutBuf, stderrBuf []byte
	var stdoutLen, stderrLen int64
	maxStdout := r.config.EffectiveMaxStdout()
	maxStderr := r.config.EffectiveMaxStderr()

	spec := runtimehost.ProcessSpec{
		ID:         processID,
		Executable: executable,
		Args:       args,
		WorkingDir: r.host.Paths().TempDir,
		Environment: runtimehost.EnvironmentSpec{
			Policy: runtimehost.EnvPolicyMinimal,
		},
		RestartPolicy: runtimehost.RestartPolicy{
			Mode: runtimehost.RestartNever,
		},
		OnStdout: func(line string) {
			if int64(len(line))+stdoutLen <= maxStdout {
				stdoutBuf = append(stdoutBuf, line...)
				stdoutLen += int64(len(line)) + 1
				stdoutBuf = append(stdoutBuf, '\n')
			}
			if opts.OnStdoutLine != nil {
				opts.OnStdoutLine(line)
			}
		},
		OnStderr: func(line string) {
			if int64(len(line))+stderrLen <= maxStderr {
				stderrBuf = append(stderrBuf, line...)
				stderrLen += int64(len(line)) + 1
				stderrBuf = append(stderrBuf, '\n')
			}
			if opts.OnStderrLine != nil {
				opts.OnStderrLine(line)
			}
		},
	}

	supervisor := r.host.Processes()
	if err := supervisor.Register(spec); err != nil {
		return nil, NewError(FFMPEG_PROCESS_FAILED, fmt.Sprintf("failed to register process: %v", err))
	}
	defer supervisor.Unregister(processID)

	startTime := time.Now()
	if err := supervisor.Start(runCtx, processID); err != nil {
		return nil, NewError(FFMPEG_PROCESS_FAILED, fmt.Sprintf("failed to start process: %v", err))
	}

	<-runCtx.Done()
	duration := time.Since(startTime)

	if runCtx.Err() == context.DeadlineExceeded {
		_ = supervisor.Stop(context.Background(), processID)
		return &ProcessResult{
			ExitCode: -1,
			Stdout:   stdoutBuf,
			Stderr:   stderrBuf,
			Duration: duration,
		}, NewError(FFMPEG_PROCESS_TIMEOUT, "process exceeded maximum duration")
	}

	if runCtx.Err() == context.Canceled {
		return &ProcessResult{
			ExitCode: -1,
			Stdout:   stdoutBuf,
			Stderr:   stderrBuf,
			Duration: duration,
		}, NewError(FFMPEG_PROCESS_CANCELLED, "process was cancelled")
	}

	snapshot, _ := supervisor.Snapshot(processID)
	return &ProcessResult{
		ExitCode: snapshot.LastExitCode,
		Stdout:   stdoutBuf,
		Stderr:   stderrBuf,
		Duration: duration,
	}, nil
}

func (r *Runner) trackProcess(id runtimehost.ProcessID, cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running[id] = cancel
}

func (r *Runner) untrackProcess(id runtimehost.ProcessID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.running, id)
}

func (r *Runner) CancelAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, cancel := range r.running {
		cancel()
	}
}

func generateProcessID(prefix string) runtimehost.ProcessID {
	b := make([]byte, 4)
	rand.Read(b)
	id := fmt.Sprintf("%s.%s.%s", prefix, hex.EncodeToString(b), shortTimestamp())
	return runtimehost.ProcessID(id)
}

func shortTimestamp() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
