package browser

import (
	"context"
	"sync"
	"time"

	proc "github.com/u-ai/backend/internal/platform/process"
	"github.com/u-ai/backend/internal/runtimehost"
)

type chromiumEngine struct {
	config      BrowserConfig
	resolver    BrowserExecutableResolver
	state       *runtimeState
	generation  atomicCounter
	process     *browserProcess
	processID   runtimehost.ProcessID
	supervisor  runtimehost.ProcessSupervisor
	mu          sync.Mutex
	cancelWatch context.CancelFunc
}

func NewChromiumEngine(config BrowserConfig, resolver BrowserExecutableResolver) BrowserEngine {
	engine := &chromiumEngine{
		config:   config,
		resolver: resolver,
		state:    newRuntimeState(),
	}
	if s := GetGlobalRuntimeSupervisor(); s != nil {
		engine.supervisor = s
	}
	return engine
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
			Message: "failed to connect to browser CDP",
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

	e.startWatchdog()

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
	processID := e.processID
	supervisor := e.supervisor
	e.mu.Unlock()

	e.stopWatchdog()

	stopCtx, cancel := context.WithTimeout(context.Background(), e.config.ShutdownTimeout)
	defer cancel()

	if supervisor != nil && processID != "" {
		_ = supervisor.Stop(stopCtx, processID)
		_ = supervisor.Unregister(processID)
	} else if proc != nil {
		if err := proc.gracefulClose(stopCtx); err != nil {
			proc.kill()
		}
	}

	e.cleanupProfile()

	e.mu.Lock()
	e.process = nil
	e.processID = ""
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

	if e.supervisor != nil {
		processID := runtimehost.ProcessID("browser-" + execInfo.Kind)
		spec := runtimehost.ProcessSpec{
			ID:                processID,
			ExecutableProcess: proc,
			Environment:       runtimehost.EnvironmentSpec{Policy: runtimehost.EnvPolicyMinimal},
			StartupTimeout:    e.config.StartupTimeout,
			StopGracePeriod:   10 * time.Second,
			RestartPolicy:     runtimehost.RestartPolicy{Mode: runtimehost.RestartNever},
		}
		if err := e.supervisor.Register(spec); err != nil {
			return nil, &BrowserError{
				Code:    ErrCodeBrowserStartFailed,
				Message: "failed to register browser with runtime host",
				Cause:   err,
			}
		}
		e.processID = processID
		if err := e.supervisor.Start(ctx, processID); err != nil {
			e.supervisor.Unregister(processID)
			return nil, &BrowserError{
				Code:    ErrCodeBrowserStartFailed,
				Message: "failed to start browser process via runtime host",
				Cause:   err,
			}
		}
	} else {
		if _, _, startErr := proc.Start(); startErr != nil {
			return nil, startErr
		}
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
		info.PID = e.process.pid
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

func (e *chromiumEngine) cdpClient() *cdpClient {
	e.mu.Lock()
	proc := e.process
	e.mu.Unlock()
	if proc == nil {
		return nil
	}
	return proc.cdp()
}

func (e *chromiumEngine) runtimeReady() bool {
	e.mu.Lock()
	state, _ := e.state.current()
	proc := e.process
	e.mu.Unlock()
	return state == BrowserRuntimeReady && proc != nil
}

func (e *chromiumEngine) processInfo() (pid int, handle proc.ProcessTreeHandle) {
	e.mu.Lock()
	proc := e.process
	e.mu.Unlock()
	if proc == nil {
		return 0, 0
	}
	return proc.pid, proc.procHandle
}

func (e *chromiumEngine) startWatchdog() {
	e.mu.Lock()
	if e.cancelWatch != nil {
		e.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.cancelWatch = cancel
	e.mu.Unlock()

	go e.watchdogLoop(ctx)
}

func (e *chromiumEngine) stopWatchdog() {
	e.mu.Lock()
	cancel := e.cancelWatch
	e.cancelWatch = nil
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (e *chromiumEngine) watchdogLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.mu.Lock()
			state, _ := e.state.current()
			proc := e.process
			e.mu.Unlock()

			if state != BrowserRuntimeReady {
				return
			}
			if proc == nil || !proc.isAlive() || !proc.cdpConnected() {
				e.handleDisconnect()
				return
			}
		}
	}
}

func (e *chromiumEngine) handleDisconnect() {
	e.mu.Lock()
	if e.state.state != BrowserRuntimeReady {
		e.mu.Unlock()
		return
	}
	e.state.setFailed()
	proc := e.process
	e.process = nil
	if e.supervisor != nil && e.processID != "" {
		_ = e.supervisor.Unregister(e.processID)
	}
	e.processID = ""
	e.mu.Unlock()

	if proc != nil {
		proc.kill()
	}

	e.generation.next()

	restartCtx, cancel := context.WithTimeout(context.Background(), e.config.StartupTimeout)
	defer cancel()

	if _, err := e.Start(restartCtx); err != nil {
		e.stopWatchdog()
		e.mu.Lock()
		e.state.setFailed()
		e.mu.Unlock()
	}
}

func (e *chromiumEngine) Contexts() BrowserContextController {
	return &chromiumContextController{engine: e}
}

type chromiumContextController struct {
	engine *chromiumEngine
}

func (c *chromiumContextController) CreateBrowserContext(ctx context.Context) (BrowserContextID, error) {
	if !c.engine.runtimeReady() {
		return "", &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "browser runtime is not ready",
		}
	}

	client := c.engine.cdpClient()
	if client == nil {
		return "", &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "CDP client not available",
		}
	}

	var result struct {
		BrowserContextID string `json:"browserContextId"`
	}
	if err := client.Call(ctx, "Target.createBrowserContext", "", nil, &result); err != nil {
		return "", &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "failed to create browser context",
			Cause:   err,
		}
	}

	return BrowserContextID(result.BrowserContextID), nil
}

func (c *chromiumContextController) DisposeBrowserContext(ctx context.Context, id BrowserContextID) error {
	if id == "" {
		return &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "browser context ID is required",
		}
	}

	client := c.engine.cdpClient()
	if client == nil {
		return &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "CDP client not available",
		}
	}

	params := map[string]string{"browserContextId": string(id)}
	if err := client.Call(ctx, "Target.disposeBrowserContext", "", params, nil); err != nil {
		return &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "failed to dispose browser context",
			Cause:   err,
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
	if !c.engine.runtimeReady() {
		return "", &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "browser runtime is not ready",
		}
	}

	client := c.engine.cdpClient()
	if client == nil {
		return "", &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "CDP client not available",
		}
	}

	if initialURL == "" {
		initialURL = "about:blank"
	}

	params := map[string]string{"url": initialURL}
	if browserContextID != "" {
		params["browserContextId"] = string(browserContextID)
	}

	var result struct {
		TargetID string `json:"targetId"`
	}
	if err := client.Call(ctx, "Target.createTarget", "", params, &result); err != nil {
		return "", &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "failed to create target",
			Cause:   err,
		}
	}

	return TargetID(result.TargetID), nil
}

func (c *chromiumTargetController) CloseTarget(ctx context.Context, targetID TargetID) error {
	if targetID == "" {
		return &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "target ID is required",
		}
	}

	client := c.engine.cdpClient()
	if client == nil {
		return &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "CDP client not available",
		}
	}

	params := map[string]string{"targetId": string(targetID)}
	if err := client.Call(ctx, "Target.closeTarget", "", params, nil); err != nil {
		return &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "failed to close target",
			Cause:   err,
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

	client := c.engine.cdpClient()
	if client == nil {
		return &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "CDP client not available",
		}
	}

	params := map[string]string{"targetId": string(targetID)}
	if err := client.Call(ctx, "Target.activateTarget", "", params, nil); err != nil {
		return &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "failed to activate target",
			Cause:   err,
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

	client := c.engine.cdpClient()
	if client == nil {
		return TargetInfo{}, &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "CDP client not available",
		}
	}

	var targetsResult struct {
		TargetInfos []struct {
			TargetID         string `json:"targetId"`
			Type             string `json:"type"`
			URL              string `json:"url"`
			Title            string `json:"title"`
			BrowserContextID string `json:"browserContextId"`
			Attached         bool   `json:"attached"`
		} `json:"targetInfos"`
	}
	if err := client.Call(ctx, "Target.getTargets", "", nil, &targetsResult); err != nil {
		return TargetInfo{}, &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "failed to get targets",
			Cause:   err,
		}
	}
	for _, info := range targetsResult.TargetInfos {
		if info.TargetID == string(targetID) {
			return TargetInfo{
				TargetID:         TargetID(info.TargetID),
				Type:             info.Type,
				URL:              info.URL,
				Title:            info.Title,
				BrowserContextID: BrowserContextID(info.BrowserContextID),
				Attached:         info.Attached,
			}, nil
		}
	}
	return TargetInfo{}, &BrowserError{
		Code:    ErrCodeBrowserRuntimeNotReady,
		Message: "target not found",
	}
}

func (e *chromiumEngine) Pages() BrowserPageController {
	return &chromiumPageController{engine: e}
}

type chromiumPageController struct {
	engine *chromiumEngine
}

type pageSession struct {
	mu        sync.Mutex
	targetID  TargetID
	sessionID string
}

var pageSessions sync.Map

func (c *chromiumPageController) getSession(targetID TargetID) string {
	if v, ok := pageSessions.Load(targetID); ok {
		ps := v.(*pageSession)
		ps.mu.Lock()
		defer ps.mu.Unlock()
		return ps.sessionID
	}
	return ""
}

func (c *chromiumPageController) ensureSession(ctx context.Context, client *cdpClient, targetID TargetID) string {
	if sid := c.getSession(targetID); sid != "" {
		return sid
	}

	var result struct {
		SessionID string `json:"sessionId"`
	}
	params := map[string]string{"targetId": string(targetID), "flatten": "true"}
	if err := client.Call(ctx, "Target.attachToTarget", "", params, &result); err != nil {
		return ""
	}

	if result.SessionID != "" {
		if err := client.Call(ctx, "Page.enable", result.SessionID, nil, nil); err != nil {
			return ""
		}
		if err := client.Call(ctx, "Runtime.enable", result.SessionID, nil, nil); err != nil {
			return ""
		}
		ps := &pageSession{targetID: targetID, sessionID: result.SessionID}
		pageSessions.Store(targetID, ps)
	}

	return result.SessionID
}

func (c *chromiumPageController) Navigate(ctx context.Context, targetID TargetID, url string, waitUntil string, timeout time.Duration) (*pageNavigateResult, error) {
	if !c.engine.runtimeReady() {
		return nil, &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "browser runtime is not ready",
		}
	}

	client := c.engine.cdpClient()
	if client == nil {
		return nil, &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "CDP client not available",
		}
	}

	sessionID := c.ensureSession(ctx, client, targetID)
	if sessionID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "failed to attach to target",
		}
	}

	params := map[string]string{"url": url}
	var result struct {
		FrameID   string `json:"frameId"`
		LoaderID  string `json:"loaderId"`
		ErrorText string `json:"errorText"`
	}
	if err := client.Call(ctx, "Page.navigate", sessionID, params, &result); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeNavigationFailed,
			Message: "navigation failed",
			Cause:   err,
		}
	}

	if waitUntil != "" && waitUntil != "none" {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			time.Sleep(200 * time.Millisecond)
			if time.Now().After(deadline) {
				return &pageNavigateResult{
					FrameID:   result.FrameID,
					LoaderID:  result.LoaderID,
					ErrorText: result.ErrorText,
					FinalURL:  url,
					TimedOut:  true,
				}, nil
			}
		}
	}

	finalURL := url
	var evalResult struct {
		Result struct {
			Value string `json:"value"`
		} `json:"result"`
	}
	if err := client.Call(ctx, "Runtime.evaluate", sessionID, map[string]string{"expression": "window.location.href"}, &evalResult); err == nil && evalResult.Result.Value != "" {
		finalURL = evalResult.Result.Value
	}

	pageTitle := ""
	if err := client.Call(ctx, "Runtime.evaluate", sessionID, map[string]string{"expression": "document.title"}, &evalResult); err == nil && evalResult.Result.Value != "" {
		pageTitle = evalResult.Result.Value
	}

	httpStatus := 0
	redirected := finalURL != url
	loaded := true

	return &pageNavigateResult{
		FrameID:    result.FrameID,
		LoaderID:   result.LoaderID,
		ErrorText:  result.ErrorText,
		FinalURL:   finalURL,
		Title:      pageTitle,
		HTTPStatus: &httpStatus,
		Redirected: redirected,
		Loaded:     loaded,
		TimedOut:   false,
		DurationMS: 0,
	}, nil
}

func (c *chromiumPageController) Reload(ctx context.Context, targetID TargetID, ignoreCache bool, timeout time.Duration) (*pageNavigateResult, error) {
	if !c.engine.runtimeReady() {
		return nil, &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "browser runtime is not ready",
		}
	}

	client := c.engine.cdpClient()
	if client == nil {
		return nil, &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "CDP client not available",
		}
	}

	sessionID := c.ensureSession(ctx, client, targetID)
	if sessionID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "failed to attach to target",
		}
	}

	params := map[string]interface{}{}
	if ignoreCache {
		params["ignoreCache"] = true
	}
	var reloadResult struct {
		FrameID   string `json:"frameId"`
		LoaderID  string `json:"loaderId"`
		ErrorText string `json:"errorText"`
	}
	if err := client.Call(ctx, "Page.reload", sessionID, params, &reloadResult); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeNavigationFailed,
			Message: "reload failed",
			Cause:   err,
		}
	}

	finalURL := ""
	var evalResult struct {
		Result struct {
			Value string `json:"value"`
		} `json:"result"`
	}
	if err := client.Call(ctx, "Runtime.evaluate", sessionID, map[string]string{"expression": "window.location.href"}, &evalResult); err == nil && evalResult.Result.Value != "" {
		finalURL = evalResult.Result.Value
	}

	pageTitle := ""
	if err := client.Call(ctx, "Runtime.evaluate", sessionID, map[string]string{"expression": "document.title"}, &evalResult); err == nil && evalResult.Result.Value != "" {
		pageTitle = evalResult.Result.Value
	}

	httpStatus := 0
	return &pageNavigateResult{
		FrameID:    reloadResult.FrameID,
		LoaderID:   reloadResult.LoaderID,
		ErrorText:  reloadResult.ErrorText,
		FinalURL:   finalURL,
		Title:      pageTitle,
		HTTPStatus: &httpStatus,
		Loaded:     true,
		TimedOut:   false,
		DurationMS: 0,
	}, nil
}

func (c *chromiumPageController) GoBack(ctx context.Context, targetID TargetID) (*pageNavigateResult, error) {
	return c.navigateHistory(ctx, targetID, -1)
}

func (c *chromiumPageController) GoForward(ctx context.Context, targetID TargetID) (*pageNavigateResult, error) {
	return c.navigateHistory(ctx, targetID, 1)
}

func (c *chromiumPageController) navigateHistory(ctx context.Context, targetID TargetID, offset int) (*pageNavigateResult, error) {
	if !c.engine.runtimeReady() {
		return nil, &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "browser runtime is not ready",
		}
	}

	client := c.engine.cdpClient()
	if client == nil {
		return nil, &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "CDP client not available",
		}
	}

	sessionID := c.ensureSession(ctx, client, targetID)
	if sessionID == "" {
		return nil, &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "failed to attach to target",
		}
	}

	var navResult struct {
		CurrentIndex int `json:"currentIndex"`
		Entries      []struct {
			ID    int    `json:"id"`
			URL   string `json:"url"`
			Title string `json:"title"`
		} `json:"entries"`
	}
	if err := client.Call(ctx, "Page.getNavigationHistory", sessionID, nil, &navResult); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeNavigationFailed,
			Message: "navigation history failed",
			Cause:   err,
		}
	}

	targetIndex := navResult.CurrentIndex + offset
	if targetIndex < 0 || targetIndex >= len(navResult.Entries) {
		return &pageNavigateResult{Loaded: true}, nil
	}

	entryID := navResult.Entries[targetIndex].ID
	params := map[string]interface{}{"entryId": entryID}
	if err := client.Call(ctx, "Page.navigateToHistoryEntry", sessionID, params, nil); err != nil {
		return nil, &BrowserError{
			Code:    ErrCodeNavigationFailed,
			Message: "navigate to history entry failed",
			Cause:   err,
		}
	}

	return &pageNavigateResult{Loaded: true}, nil
}

func (c *chromiumPageController) Stop(ctx context.Context, targetID TargetID) error {
	if !c.engine.runtimeReady() {
		return &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "browser runtime is not ready",
		}
	}

	client := c.engine.cdpClient()
	if client == nil {
		return &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "CDP client not available",
		}
	}

	sessionID := c.ensureSession(ctx, client, targetID)
	if sessionID == "" {
		return &BrowserError{
			Code:    ErrCodeBrowserRuntimeNotReady,
			Message: "failed to attach to target",
		}
	}

	if err := client.Call(ctx, "Page.stopLoading", sessionID, nil, nil); err != nil {
		return &BrowserError{
			Code:    ErrCodeNavigationFailed,
			Message: "stop loading failed",
			Cause:   err,
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
