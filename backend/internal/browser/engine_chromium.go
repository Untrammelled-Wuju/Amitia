package browser

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
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
		State:      state,
		Generation: generation,
		Engine:     "chromium",
		Headless:   e.config.Headless,
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

func (e *chromiumEngine) Contexts() BrowserContextController {
	return &chromiumContextController{engine: e}
}

type chromiumContextController struct {
	engine *chromiumEngine
}

func (c *chromiumContextController) CreateBrowserContext(ctx context.Context) (BrowserContextID, error) {
	c.engine.mu.Lock()
	proc := c.engine.process
	state, _ := c.engine.state.current()
	c.engine.mu.Unlock()

	if state != BrowserRuntimeReady || proc == nil {
		return "", &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "browser runtime is not ready",
		}
	}

	id := uuid.New().String()
	return BrowserContextID(id), nil
}

func (c *chromiumContextController) DisposeBrowserContext(ctx context.Context, id BrowserContextID) error {
	if id == "" {
		return &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "browser context ID is required",
		}
	}
	return nil
}

func (e *chromiumEngine) Targets() BrowserTargetController {
	return &chromiumTargetController{engine: e}
}

type chromiumTargetController struct {
	engine *chromiumEngine
}

func (c *chromiumTargetController) CreateTarget(ctx context.Context, browserContextID BrowserContextID, initialURL string) (TargetID, error) {
	c.engine.mu.Lock()
	proc := c.engine.process
	state, _ := c.engine.state.current()
	c.engine.mu.Unlock()

	if state != BrowserRuntimeReady || proc == nil {
		return "", &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "browser runtime is not ready",
		}
	}

	if initialURL == "" {
		initialURL = "about:blank"
	}

	id := uuid.New().String()
	return TargetID(id), nil
}

func (c *chromiumTargetController) CloseTarget(ctx context.Context, targetID TargetID) error {
	if targetID == "" {
		return &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "target ID is required",
		}
	}
	return nil
}

func (c *chromiumTargetController) ActivateTarget(ctx context.Context, targetID TargetID) error {
	if targetID == "" {
		return &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "target ID is required",
		}
	}
	return nil
}

func (c *chromiumTargetController) TargetInfo(ctx context.Context, targetID TargetID) (TargetInfo, error) {
	if targetID == "" {
		return TargetInfo{}, &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "target ID is required",
		}
	}
	return TargetInfo{
		TargetID: targetID,
		Type:     "page",
	}, nil
}

func (e *chromiumEngine) Pages() BrowserPageController {
	return &chromiumPageController{engine: e}
}

type chromiumPageController struct {
	engine *chromiumEngine
}

func (c *chromiumPageController) Navigate(ctx context.Context, targetID TargetID, url string, waitUntil string, timeout time.Duration) (*pageNavigateResult, error) {
	c.engine.mu.Lock()
	proc := c.engine.process
	state, _ := c.engine.state.current()
	c.engine.mu.Unlock()

	if state != BrowserRuntimeReady || proc == nil {
		return nil, &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "browser runtime is not ready",
		}
	}

	return &pageNavigateResult{
		FinalURL:   url,
		Title:      "",
		Redirected: false,
		Loaded:     true,
		TimedOut:   false,
		DurationMS: 0,
	}, nil
}

func (c *chromiumPageController) Reload(ctx context.Context, targetID TargetID, ignoreCache bool, timeout time.Duration) (*pageNavigateResult, error) {
	c.engine.mu.Lock()
	proc := c.engine.process
	state, _ := c.engine.state.current()
	c.engine.mu.Unlock()

	if state != BrowserRuntimeReady || proc == nil {
		return nil, &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "browser runtime is not ready",
		}
	}

	return &pageNavigateResult{
		Title:      "",
		Redirected: false,
		Loaded:     true,
		TimedOut:   false,
		DurationMS: 0,
	}, nil
}

func (c *chromiumPageController) GoBack(ctx context.Context, targetID TargetID) (*pageNavigateResult, error) {
	c.engine.mu.Lock()
	proc := c.engine.process
	state, _ := c.engine.state.current()
	c.engine.mu.Unlock()

	if state != BrowserRuntimeReady || proc == nil {
		return nil, &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "browser runtime is not ready",
		}
	}

	return &pageNavigateResult{
		Title:      "",
		Redirected: false,
		Loaded:     true,
		TimedOut:   false,
		DurationMS: 0,
	}, nil
}

func (c *chromiumPageController) GoForward(ctx context.Context, targetID TargetID) (*pageNavigateResult, error) {
	c.engine.mu.Lock()
	proc := c.engine.process
	state, _ := c.engine.state.current()
	c.engine.mu.Unlock()

	if state != BrowserRuntimeReady || proc == nil {
		return nil, &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "browser runtime is not ready",
		}
	}

	return &pageNavigateResult{
		Title:      "",
		Redirected: false,
		Loaded:     true,
		TimedOut:   false,
		DurationMS: 0,
	}, nil
}

func (c *chromiumPageController) Stop(ctx context.Context, targetID TargetID) error {
	c.engine.mu.Lock()
	proc := c.engine.process
	state, _ := c.engine.state.current()
	c.engine.mu.Unlock()

	if state != BrowserRuntimeReady || proc == nil {
		return &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "browser runtime is not ready",
		}
	}
	return nil
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
