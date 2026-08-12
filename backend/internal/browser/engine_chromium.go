package browser

import (
	"context"
	"sync"
	"time"
)

type chromiumEngine struct {
	config     BrowserConfig
	resolver   BrowserExecutableResolver
	state      *runtimeState
	generation atomicCounter
	process    *browserProcess
	mu         sync.Mutex
}

func NewChromiumEngine(config BrowserConfig, resolver BrowserExecutableResolver) BrowserEngine {
	return &chromiumEngine{
		config:   config,
		resolver: resolver,
		state:    newRuntimeState(),
	}
}

func (e *chromiumEngine) Start(ctx context.Context) (*BrowserRuntimeInfo, error) {
	e.mu.Lock()
	currentState, _ := e.state.current()
	if currentState == BrowserRuntimeReady {
		info := e.snapshotLocked()
		e.mu.Unlock()
		return &info, nil
	}
	if !e.state.setStarting() {
		e.mu.Unlock()
		state, _ := e.state.current()
		if state == BrowserRuntimeReady {
			e.mu.Lock()
			info := e.snapshotLocked()
			e.mu.Unlock()
			return &info, nil
		}
		return nil, &BrowserError{
			Code:    ErrCodeBrowserStarting,
			Message: "browser runtime is already starting",
		}
	}
	e.mu.Unlock()

	execInfo, err := e.resolver.Resolve(ctx)
	if err != nil {
		e.state.setFailed()
		if browserErr, ok := err.(*BrowserError); ok {
			return nil, browserErr
		}
		return nil, &BrowserError{
			Code:    ErrCodeBrowserStartFailed,
			Message: err.Error(),
			Cause:   err,
		}
	}

	proc, err := e.launchProcess(ctx, execInfo)
	if err != nil {
		e.state.setFailed()
		if browserErr, ok := err.(*BrowserError); ok {
			return nil, browserErr
		}
		return nil, &BrowserError{
			Code:    ErrCodeBrowserStartFailed,
			Message: "failed to launch browser process: " + safeErrorMessage(err),
			Cause:   err,
		}
	}

	if err := e.connectCDP(ctx, proc); err != nil {
		proc.kill()
		e.cleanupProfile()
		e.state.setFailed()
		return nil, &BrowserError{
			Code:    ErrCodeBrowserConnFailed,
			Message: "failed to connect to browser CDP endpoint",
			Cause:   err,
		}
	}

	generation := e.generation.next()

	e.mu.Lock()
	e.process = proc
	e.state.state = BrowserRuntimeReady
	e.state.generation = generation
	now := time.Now()
	info := e.snapshotLockedWithTime(now)
	e.mu.Unlock()

	return &info, nil
}

func (e *chromiumEngine) Stop(ctx context.Context) error {
	e.mu.Lock()
	currentState, _ := e.state.current()
	if currentState == BrowserRuntimeStopped {
		e.mu.Unlock()
		return nil
	}
	if !e.state.setStopping() {
		state, _ := e.state.current()
		if state == BrowserRuntimeStopped {
			e.mu.Unlock()
			return nil
		}
		e.mu.Unlock()
		return &BrowserError{
			Code:    ErrCodeBrowserStopFailed,
			Message: "browser runtime is not in a stoppable state",
		}
	}
	proc := e.process
	e.mu.Unlock()

	stopCtx, cancel := context.WithTimeout(context.Background(), e.config.ShutdownTimeout)
	defer cancel()

	if proc != nil {
		if err := proc.gracefulClose(stopCtx); err != nil {
			proc.kill()
		}
	}

	e.cleanupProfile()

	e.mu.Lock()
	e.process = nil
	e.state.setStopped()
	e.mu.Unlock()

	return nil
}

func (e *chromiumEngine) Status(_ context.Context) BrowserRuntimeInfo {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.snapshotLocked()
}

func (e *chromiumEngine) Health(_ context.Context) BrowserRuntimeHealth {
	e.mu.Lock()
	state, _ := e.state.current()
	proc := e.process
	e.mu.Unlock()

	switch state {
	case BrowserRuntimeStopped:
		return BrowserHealthUnavailable
	case BrowserRuntimeStarting:
		return BrowserHealthStarting
	case BrowserRuntimeStopping:
		return BrowserHealthUnhealthy
	case BrowserRuntimeFailed:
		return BrowserHealthUnhealthy
	case BrowserRuntimeReady:
		if proc == nil {
			return BrowserHealthUnhealthy
		}
		if !proc.isAlive() {
			return BrowserHealthUnhealthy
		}
		if !proc.cdpConnected() {
			return BrowserHealthUnhealthy
		}
		if !proc.ping() {
			return BrowserHealthUnhealthy
		}
		return BrowserHealthHealthy
	default:
		return BrowserHealthUnknown
	}
}

func (e *chromiumEngine) launchProcess(ctx context.Context, execInfo BrowserExecutable) (*browserProcess, error) {
	profileDir := e.profileDirForGeneration(e.generation.get() + 1)
	proc := newBrowserProcess(execInfo, e.config, profileDir)

	if err := proc.start(ctx); err != nil {
		return nil, err
	}

	return proc, nil
}

func (e *chromiumEngine) connectCDP(ctx context.Context, proc *browserProcess) error {
	return proc.connectCDP(ctx)
}

func (e *chromiumEngine) cleanupProfile() {
	e.mu.Lock()
	proc := e.process
	e.mu.Unlock()
	if proc != nil {
		proc.cleanupProfile()
	}
}

func (e *chromiumEngine) snapshotLocked() BrowserRuntimeInfo {
	now := time.Now()
	return e.snapshotLockedWithTime(now)
}

func (e *chromiumEngine) snapshotLockedWithTime(_ time.Time) BrowserRuntimeInfo {
	state, generation := e.state.current()
	info := BrowserRuntimeInfo{
		State:     state,
		Generation: generation,
		Engine:    "chromium",
		Headless:  e.config.Headless,
	}

	if e.process != nil {
		info.BrowserName = e.process.browserName()
		info.BrowserVersion = e.process.browserVersion()
		info.ProcessAlive = e.process.isAlive()
		info.CDPConnected = e.process.cdpConnected()
	}

	if state == BrowserRuntimeReady && !info.ProcessAlive {
		info.LastErrorCode = string(ErrCodeBrowserProcessExited)
		info.State = BrowserRuntimeFailed
	} else if state == BrowserRuntimeReady && !info.CDPConnected {
		info.LastErrorCode = string(ErrCodeBrowserConnFailed)
	}

	return info
}

func (e *chromiumEngine) profileDirForGeneration(gen uint64) string {
	if e.config.UserDataRoot != "" {
		return profilePathFor(e.config.UserDataRoot, gen)
	}
	return ""
}

func safeErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) > 256 {
		return msg[:256] + "..."
	}
	return msg
}
