package browser

import (
	"context"
	"encoding/json"
	"sync"
)

type productionProvider struct {
	runtime   BrowserRuntime
	sessions  BrowserSessionManager
	tabs      BrowserTabManager
	navigator BrowserNavigator
	observer  BrowserObserver
	interact  BrowserInteractor
	resources BrowserResourceTransfer
	devtools  BrowserDevTools
	caps      BrowserCapabilities
	mu        sync.RWMutex
}

func NewProductionProvider(config BrowserConfig, engineFactory BrowserEngineFactory) (BrowserProvider, error) {
	if !config.Enabled {
		return NewDisabledProvider(), nil
	}

	engine, err := engineFactory.Create(config)
	if err != nil {
		return nil, err
	}

	runtime := NewRuntimeController(engine)

	sessionBackend := NewChromiumSessionBackend(engine.Contexts())
	sessionManager := NewProductionSessionManager(
		runtime,
		sessionBackend,
		config.MaxSessions,
	)

	tabBackend := NewChromiumTabBackend(engine.Targets())
	tabManager := NewProductionTabManager(
		sessionManager.(SessionResolver),
		tabBackend,
		config.MaxTabsPerSession,
		config.MaxTabsTotal,
	)

	if sm, ok := sessionManager.(*productionSessionManager); ok {
		sm.SetTabCleaner(tabManager.(SessionTabCleaner))
	}

	navigationPolicy := NewNavigationPolicy(config)
	pageController := engine.Pages()
	navigator := NewProductionNavigator(
		tabManager.(TabResolver),
		pageController,
		navigationPolicy,
	)

	if pn, ok := navigator.(*productionNavigator); ok {
		if tm, ok := tabManager.(*productionTabManager); ok {
			pn.SetTabManager(tm)
		}
	}

	interactionPolicy := NewInteractionPolicy()
	elementStore := newElementStore(interactionPolicy.MaxElementRefsPerTab)
	domBackend := NewChromiumDOMBackend(engine)
	inputBackend := NewChromiumInputBackend(engine)

	observer := NewProductionObserver(
		tabManager.(TabResolver),
		domBackend,
		elementStore,
		interactionPolicy,
		tabManager.(*productionTabManager),
	)

	interact := NewProductionInteractor(
		tabManager.(TabResolver),
		domBackend,
		inputBackend,
		elementStore,
		interactionPolicy,
		tabManager.(*productionTabManager),
	)

	resources := NewProductionResourceTransfer(
		tabManager.(TabResolver),
		domBackend,
		elementStore,
		interactionPolicy,
		tabManager.(*productionTabManager),
	)
	devtools := NewChromiumDevTools(engine, tabManager.(TabResolver))

	return &productionProvider{
		runtime:   runtime,
		sessions:  sessionManager,
		tabs:      tabManager,
		navigator: navigator,
		observer:  observer,
		interact:  interact,
		resources: resources,
		devtools:  devtools,
		caps: BrowserCapabilities{
			SupportsNavigation:  true,
			SupportsDOM:         true,
			SupportsInteraction: true,
			SupportsDownload:    true,
			SupportsUpload:      true,
			SupportsScreenshot:  true,
			SupportsDevTools:    true,
			RiskLevels:          []string{"browser_runtime", "browser_navigation", "browser_dom", "browser_interaction", "browser_resource"},
		},
	}, nil
}

func NewDisabledProvider() BrowserProvider {
	return &productionProvider{
		runtime:   &disabledRuntime{},
		sessions:  &unsupportedSessionManager{},
		tabs:      &unsupportedTabManager{},
		navigator: &unsupportedNavigator{},
		observer:  &unsupportedObserver{},
		interact:  &unsupportedInteractor{},
		resources: &unsupportedResourceTransfer{},
		devtools:  &unsupportedDevTools{},
		caps: BrowserCapabilities{
			SupportsNavigation:  false,
			SupportsDOM:         false,
			SupportsInteraction: false,
			SupportsDownload:    false,
			SupportsUpload:      false,
			SupportsScreenshot:  false,
			SupportsDevTools:    false,
		},
	}
}

func (p *productionProvider) BrowserCapabilities() BrowserCapabilities {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.caps
}

func (p *productionProvider) Runtime() BrowserRuntime {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.runtime
}

func (p *productionProvider) Sessions() BrowserSessionManager {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sessions
}

func (p *productionProvider) Tabs() BrowserTabManager {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tabs
}

func (p *productionProvider) Navigate() BrowserNavigator {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.navigator
}

func (p *productionProvider) Observe() BrowserObserver {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.observer
}

func (p *productionProvider) Interact() BrowserInteractor {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.interact
}

func (p *productionProvider) Resources() BrowserResourceTransfer {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.resources
}

func (p *productionProvider) DevTools() BrowserDevTools {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.devtools
}

type runtimeController struct {
	engine BrowserEngine
}

func NewRuntimeController(engine BrowserEngine) BrowserRuntime {
	return &runtimeController{engine: engine}
}

func (c *runtimeController) Engine() BrowserEngine {
	return c.engine
}

func (c *runtimeController) Start(ctx context.Context) (*BrowserRuntimeInfo, *BrowserError) {
	info, err := c.engine.Start(ctx)
	if err != nil {
		return nil, engineErrorToBrowserError(err)
	}
	return info, nil
}

func (c *runtimeController) Stop(ctx context.Context) *BrowserError {
	err := c.engine.Stop(ctx)
	if err != nil {
		return engineErrorToBrowserError(err)
	}
	return nil
}

func (c *runtimeController) Status(ctx context.Context) BrowserRuntimeInfo {
	return c.engine.Status(ctx)
}

func (c *runtimeController) Health(ctx context.Context) BrowserRuntimeHealth {
	return c.engine.Health(ctx)
}

func engineErrorToBrowserError(err error) *BrowserError {
	if err == nil {
		return nil
	}
	if be, ok := err.(*BrowserError); ok {
		return be
	}
	return &BrowserError{
		Code:    ErrCodeBrowserStartFailed,
		Message: err.Error(),
		Cause:   err,
	}
}

type disabledRuntime struct{}

func (r *disabledRuntime) Start(_ context.Context) (*BrowserRuntimeInfo, *BrowserError) {
	return nil, &BrowserError{
		Code:    ErrCodeBrowserDisabled,
		Message: "browser runtime is disabled",
	}
}

func (r *disabledRuntime) Stop(_ context.Context) *BrowserError {
	return &BrowserError{
		Code:    ErrCodeBrowserDisabled,
		Message: "browser runtime is disabled",
	}
}

func (r *disabledRuntime) Status(_ context.Context) BrowserRuntimeInfo {
	return BrowserRuntimeInfo{
		State:         BrowserRuntimeStopped,
		Engine:        "disabled",
		LastErrorCode: string(ErrCodeBrowserDisabled),
	}
}

func (r *disabledRuntime) Health(_ context.Context) BrowserRuntimeHealth {
	return BrowserHealthUnavailable
}

func (r *disabledRuntime) Engine() BrowserEngine { return nil }

type unsupportedSessionManager struct{}

func (m *unsupportedSessionManager) CreateSession(_ context.Context) (BrowserSessionInfo, *BrowserError) {
	return BrowserSessionInfo{}, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser sessions are not supported in this build"}
}

func (m *unsupportedSessionManager) CloseSession(_ context.Context, _ BrowserSessionID) *BrowserError {
	return &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser sessions are not supported in this build"}
}

func (m *unsupportedSessionManager) GetSession(_ context.Context, _ BrowserSessionID) (BrowserSessionInfo, *BrowserError) {
	return BrowserSessionInfo{}, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser sessions are not supported in this build"}
}

func (m *unsupportedSessionManager) ListSessions(_ context.Context) ([]BrowserSessionInfo, *BrowserError) {
	return nil, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser sessions are not supported in this build"}
}

type unsupportedTabManager struct{}

func (m *unsupportedTabManager) CreateTab(_ context.Context, _ BrowserSessionID) (BrowserTabInfo, *BrowserError) {
	return BrowserTabInfo{}, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser tabs are not supported in this build"}
}

func (m *unsupportedTabManager) CloseTab(_ context.Context, _ BrowserSessionID, _ BrowserTabID) *BrowserError {
	return &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser tabs are not supported in this build"}
}

func (m *unsupportedTabManager) GetTab(_ context.Context, _ BrowserSessionID, _ BrowserTabID) (BrowserTabInfo, *BrowserError) {
	return BrowserTabInfo{}, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser tabs are not supported in this build"}
}

func (m *unsupportedTabManager) ListTabs(_ context.Context, _ BrowserSessionID) ([]BrowserTabInfo, *BrowserError) {
	return nil, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser tabs are not supported in this build"}
}

func (m *unsupportedTabManager) ActivateTab(_ context.Context, _ BrowserSessionID, _ BrowserTabID) *BrowserError {
	return &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser tabs are not supported in this build"}
}

type unsupportedNavigator struct{}

func (n *unsupportedNavigator) Navigate(_ context.Context, _ BrowserSessionID, _ BrowserTabID, _ NavigateRequest) (NavigationResult, *BrowserError) {
	return NavigationResult{}, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser navigation is not supported in this build"}
}

func (n *unsupportedNavigator) Reload(_ context.Context, _ BrowserSessionID, _ BrowserTabID, _ NavigateRequest) (NavigationResult, *BrowserError) {
	return NavigationResult{}, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser navigation is not supported in this build"}
}

func (n *unsupportedNavigator) GoBack(_ context.Context, _ BrowserSessionID, _ BrowserTabID) (NavigationResult, *BrowserError) {
	return NavigationResult{}, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser navigation is not supported in this build"}
}

func (n *unsupportedNavigator) GoForward(_ context.Context, _ BrowserSessionID, _ BrowserTabID) (NavigationResult, *BrowserError) {
	return NavigationResult{}, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser navigation is not supported in this build"}
}

func (n *unsupportedNavigator) Stop(_ context.Context, _ BrowserSessionID, _ BrowserTabID) *BrowserError {
	return &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser navigation is not supported in this build"}
}

type unsupportedObserver struct{}

func (o *unsupportedObserver) GetDOMSnapshot(_ context.Context, _ BrowserSessionID, _ BrowserTabID, _ int) (*BrowserDOMSnapshot, *BrowserError) {
	return nil, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser DOM observation is not supported in this build"}
}

func (o *unsupportedObserver) FindElement(_ context.Context, _ BrowserSessionID, _ BrowserTabID, _ string) (*BrowserElementRef, *BrowserError) {
	return nil, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser DOM observation is not supported in this build"}
}

func (o *unsupportedObserver) ScrollToElement(_ context.Context, _ BrowserSessionID, _ BrowserTabID, _ BrowserElementRef) *BrowserError {
	return &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser DOM observation is not supported in this build"}
}

type unsupportedInteractor struct{}

func (i *unsupportedInteractor) Click(_ context.Context, _ BrowserSessionID, _ BrowserTabID, _ BrowserElementRef) (*BrowserInteractionResult, *BrowserError) {
	return nil, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser interaction is not supported in this build"}
}

func (i *unsupportedInteractor) Input(_ context.Context, _ BrowserSessionID, _ BrowserTabID, _ BrowserElementRef, _ string) (*BrowserInteractionResult, *BrowserError) {
	return nil, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser interaction is not supported in this build"}
}

func (i *unsupportedInteractor) Select(_ context.Context, _ BrowserSessionID, _ BrowserTabID, _ BrowserElementRef, _ string) (*BrowserInteractionResult, *BrowserError) {
	return nil, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser interaction is not supported in this build"}
}

func (i *unsupportedInteractor) Hover(_ context.Context, _ BrowserSessionID, _ BrowserTabID, _ BrowserElementRef) (*BrowserInteractionResult, *BrowserError) {
	return nil, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser interaction is not supported in this build"}
}

func (i *unsupportedInteractor) Scroll(_ context.Context, _ BrowserSessionID, _ BrowserTabID, _ string) (*BrowserInteractionResult, *BrowserError) {
	return nil, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser interaction is not supported in this build"}
}

type unsupportedResourceTransfer struct{}

func (r *unsupportedResourceTransfer) Download(_ context.Context, _ BrowserDownloadRequest) (*BrowserDownloadResult, *BrowserError) {
	return nil, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser download is not supported in this build"}
}

func (r *unsupportedResourceTransfer) Upload(_ context.Context, _ BrowserUploadRequest) (*BrowserUploadResult, *BrowserError) {
	return nil, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser upload is not supported in this build"}
}

func (r *unsupportedResourceTransfer) Screenshot(_ context.Context, _ BrowserScreenshotRequest) (*BrowserScreenshotResult, *BrowserError) {
	return nil, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser screenshot is not supported in this build"}
}

type unsupportedDevTools struct{}

func (d *unsupportedDevTools) Execute(_ context.Context, _ string, _ BrowserSessionID, _ BrowserTabID, _ json.RawMessage) (json.RawMessage, *BrowserError) {
	return nil, &BrowserError{Code: ErrCodeUnsupportedAction, Message: "browser DevTools are not supported in this build"}
}
