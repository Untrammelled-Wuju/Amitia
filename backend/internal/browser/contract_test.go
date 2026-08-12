package browser

import (
	"context"
	"testing"
)

type fakeBrowserProvider struct {
	caps       BrowserCapabilities
	runtime    BrowserRuntime
	sessions   *fakeSessionManager
	tabs       *fakeTabManager
	navigator  *fakeNavigator
	observer   *fakeObserver
	interactor *fakeInteractor
	resources  *fakeResourceTransfer
}

func newFakeBrowserProvider() *fakeBrowserProvider {
	return &fakeBrowserProvider{
		caps: BrowserCapabilities{
			SupportsNavigation:  true,
			SupportsDOM:         true,
			SupportsInteraction: true,
			SupportsDownload:    true,
			SupportsUpload:      true,
			SupportsScreenshot:  true,
		},
		runtime:    &fakeRuntime{},
		sessions:   &fakeSessionManager{},
		tabs:       &fakeTabManager{},
		navigator:  &fakeNavigator{},
		observer:   &fakeObserver{},
		interactor: &fakeInteractor{},
		resources:  &fakeResourceTransfer{},
	}
}

func (p *fakeBrowserProvider) BrowserCapabilities() BrowserCapabilities { return p.caps }
func (p *fakeBrowserProvider) Runtime() BrowserRuntime                  { return p.runtime }
func (p *fakeBrowserProvider) Sessions() BrowserSessionManager          { return p.sessions }
func (p *fakeBrowserProvider) Tabs() BrowserTabManager                  { return p.tabs }
func (p *fakeBrowserProvider) Navigate() BrowserNavigator               { return p.navigator }
func (p *fakeBrowserProvider) Observe() BrowserObserver                 { return p.observer }
func (p *fakeBrowserProvider) Interact() BrowserInteractor              { return p.interactor }
func (p *fakeBrowserProvider) Resources() BrowserResourceTransfer       { return p.resources }

type fakeRuntime struct{}

func (r *fakeRuntime) Start(_ context.Context) (*BrowserRuntimeInfo, *BrowserError) {
	return &BrowserRuntimeInfo{State: BrowserRuntimeReady, Generation: 1}, nil
}

func (r *fakeRuntime) Stop(_ context.Context) *BrowserError {
	return nil
}

func (r *fakeRuntime) Status(_ context.Context) BrowserRuntimeInfo {
	return BrowserRuntimeInfo{State: BrowserRuntimeReady, Generation: 1}
}

func (r *fakeRuntime) Health(_ context.Context) BrowserRuntimeHealth {
	return BrowserHealthHealthy
}

type fakeSessionManager struct{}

func (m *fakeSessionManager) CreateSession(_ context.Context) (BrowserSessionInfo, *BrowserError) {
	return BrowserSessionInfo{
		SessionID: BrowserSessionID("test-session-1"),
		State:     SessionStateReady,
	}, nil
}
func (m *fakeSessionManager) CloseSession(_ context.Context, _ BrowserSessionID) *BrowserError {
	return nil
}
func (m *fakeSessionManager) GetSession(_ context.Context, id BrowserSessionID) (BrowserSessionInfo, *BrowserError) {
	if id == "" {
		return BrowserSessionInfo{}, &BrowserError{Code: ErrCodeSessionNotFound, Message: "session not found"}
	}
	return BrowserSessionInfo{SessionID: id, State: SessionStateReady}, nil
}
func (m *fakeSessionManager) ListSessions(_ context.Context) ([]BrowserSessionInfo, *BrowserError) {
	return []BrowserSessionInfo{{SessionID: BrowserSessionID("s1"), State: SessionStateReady}}, nil
}

type fakeTabManager struct{}

func (m *fakeTabManager) OpenTab(_ context.Context, sessionID BrowserSessionID, _ string) (BrowserTabInfo, *BrowserError) {
	if sessionID == "" {
		return BrowserTabInfo{}, &BrowserError{Code: ErrCodeSessionNotFound, Message: "session not found"}
	}
	return BrowserTabInfo{
		TabID:     BrowserTabID("tab-1"),
		SessionID: sessionID,
		State:     TabStateReady,
		Active:    true,
	}, nil
}
func (m *fakeTabManager) CloseTab(_ context.Context, _ BrowserSessionID, _ BrowserTabID) *BrowserError {
	return nil
}
func (m *fakeTabManager) ActivateTab(_ context.Context, _ BrowserSessionID, _ BrowserTabID) *BrowserError {
	return nil
}
func (m *fakeTabManager) ListTabs(_ context.Context, _ BrowserSessionID) ([]BrowserTabInfo, *BrowserError) {
	return []BrowserTabInfo{{TabID: BrowserTabID("t1"), Active: true}}, nil
}

type fakeNavigator struct{}

func (n *fakeNavigator) Navigate(_ context.Context, sessionID BrowserSessionID, tabID BrowserTabID, url string) (*BrowserNavigationResult, *BrowserError) {
	if sessionID == "" {
		return nil, &BrowserError{Code: ErrCodeSessionNotFound, Message: "session not found"}
	}
	if tabID == "" {
		return nil, &BrowserError{Code: ErrCodeTabNotFound, Message: "tab not found"}
	}
	if url == "" {
		return nil, &BrowserError{Code: ErrCodeInvalidRequest, Message: "url required"}
	}
	return &BrowserNavigationResult{
		SessionID: sessionID,
		TabID:     tabID,
		URL:       url,
		FinalURL:  url,
	}, nil
}
func (n *fakeNavigator) Reload(_ context.Context, _ BrowserSessionID, _ BrowserTabID) (*BrowserNavigationResult, *BrowserError) {
	return &BrowserNavigationResult{}, nil
}
func (n *fakeNavigator) GoBack(_ context.Context, _ BrowserSessionID, _ BrowserTabID) (*BrowserNavigationResult, *BrowserError) {
	return &BrowserNavigationResult{}, nil
}

type fakeObserver struct{}

func (o *fakeObserver) GetDOMSnapshot(_ context.Context, sessionID BrowserSessionID, tabID BrowserTabID, maxDepth int) (*BrowserDOMSnapshot, *BrowserError) {
	if sessionID == "" {
		return nil, &BrowserError{Code: ErrCodeSessionNotFound, Message: "session not found"}
	}
	return &BrowserDOMSnapshot{
		SessionID: sessionID,
		TabID:     tabID,
		MaxDepth:  maxDepth,
	}, nil
}
func (o *fakeObserver) FindElement(_ context.Context, _ BrowserSessionID, _ BrowserTabID, selector string) (*BrowserElementRef, *BrowserError) {
	if selector == "" {
		return nil, &BrowserError{Code: ErrCodeElementNotFound, Message: "element not found"}
	}
	return &BrowserElementRef{Selector: selector, StableID: "stable-" + selector}, nil
}
func (o *fakeObserver) ScrollToElement(_ context.Context, _ BrowserSessionID, _ BrowserTabID, _ BrowserElementRef) *BrowserError {
	return nil
}

type fakeInteractor struct{}

func (i *fakeInteractor) Click(_ context.Context, _ BrowserSessionID, _ BrowserTabID, element BrowserElementRef) (*BrowserInteractionResult, *BrowserError) {
	if element.StableID == "stale" {
		return &BrowserInteractionResult{Success: false, Stale: true}, &BrowserError{Code: ErrCodeStaleElement, Message: "element stale"}
	}
	return &BrowserInteractionResult{Success: true}, nil
}
func (i *fakeInteractor) Input(_ context.Context, _ BrowserSessionID, _ BrowserTabID, _ BrowserElementRef, text string) (*BrowserInteractionResult, *BrowserError) {
	return &BrowserInteractionResult{Success: true}, nil
}
func (i *fakeInteractor) Select(_ context.Context, _ BrowserSessionID, _ BrowserTabID, _ BrowserElementRef, _ string) (*BrowserInteractionResult, *BrowserError) {
	return &BrowserInteractionResult{Success: true}, nil
}
func (i *fakeInteractor) Hover(_ context.Context, _ BrowserSessionID, _ BrowserTabID, _ BrowserElementRef) (*BrowserInteractionResult, *BrowserError) {
	return &BrowserInteractionResult{Success: true}, nil
}
func (i *fakeInteractor) Scroll(_ context.Context, _ BrowserSessionID, _ BrowserTabID, _ string) (*BrowserInteractionResult, *BrowserError) {
	return &BrowserInteractionResult{Success: true}, nil
}

type fakeResourceTransfer struct{}

func (r *fakeResourceTransfer) Download(_ context.Context, req BrowserDownloadRequest) (*BrowserDownloadResult, *BrowserError) {
	if req.ResourceURI == "" {
		return nil, &BrowserError{Code: ErrCodeInvalidRequest, Message: "resourceURI required"}
	}
	if req.SessionID == "" {
		return nil, &BrowserError{Code: ErrCodeSessionNotFound, Message: "session not found"}
	}
	return &BrowserDownloadResult{
		ResourceURI: req.ResourceURI,
		Filename:    req.Filename,
	}, nil
}
func (r *fakeResourceTransfer) Upload(_ context.Context, req BrowserUploadRequest) (*BrowserUploadResult, *BrowserError) {
	if req.ResourceURI == "" {
		return nil, &BrowserError{Code: ErrCodeInvalidRequest, Message: "resourceURI required"}
	}
	return &BrowserUploadResult{
		ResourceURI: req.ResourceURI,
		Success:     true,
	}, nil
}
func (r *fakeResourceTransfer) Screenshot(_ context.Context, req BrowserScreenshotRequest) (*BrowserScreenshotResult, *BrowserError) {
	if req.SessionID == "" {
		return nil, &BrowserError{Code: ErrCodeSessionNotFound, Message: "session not found"}
	}
	return &BrowserScreenshotResult{
		ResourceURI: "amitia://temp/screenshots/snap.png",
		Width:       1920,
		Height:      1080,
	}, nil
}

func TestBrowserProviderContract(t *testing.T) {
	provider := newFakeBrowserProvider()
	ctx := context.Background()

	caps := provider.BrowserCapabilities()
	if !caps.SupportsNavigation {
		t.Fatal("fake provider should support navigation")
	}

	rt := provider.Runtime()
	if rt == nil {
		t.Fatal("provider Runtime() should not return nil")
	}

	info, err := rt.Start(ctx)
	if err != nil {
		t.Fatalf("runtime Start failed: %v", err)
	}
	if info.State != BrowserRuntimeReady {
		t.Fatalf("expected runtime state ready, got: %s", info.State)
	}

	status := rt.Status(ctx)
	if status.State != BrowserRuntimeReady {
		t.Fatalf("expected runtime status ready, got: %s", status.State)
	}

	health := rt.Health(ctx)
	if health != BrowserHealthHealthy {
		t.Fatalf("expected runtime healthy, got: %s", health)
	}

	if stopErr := rt.Stop(ctx); stopErr != nil {
		t.Fatalf("runtime Stop failed: %v", stopErr)
	}

	sess, err := provider.Sessions().CreateSession(ctx)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}
	if sess.SessionID == "" {
		t.Fatal("session ID should not be empty")
	}

	tab, err := provider.Tabs().OpenTab(ctx, sess.SessionID, "https://example.com")
	if err != nil {
		t.Fatalf("open tab failed: %v", err)
	}
	if tab.SessionID != sess.SessionID {
		t.Fatalf("tab should belong to session")
	}

	nav, err := provider.Navigate().Navigate(ctx, sess.SessionID, tab.TabID, "https://example.com")
	if err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if nav.FinalURL != "https://example.com" {
		t.Fatalf("unexpected final URL: %s", nav.FinalURL)
	}

	dom, err := provider.Observe().GetDOMSnapshot(ctx, sess.SessionID, tab.TabID, 3)
	if err != nil {
		t.Fatalf("get dom snapshot failed: %v", err)
	}
	if dom.MaxDepth != 3 {
		t.Fatalf("unexpected max depth: %d", dom.MaxDepth)
	}

	elem, err := provider.Observe().FindElement(ctx, sess.SessionID, tab.TabID, "button.submit")
	if err != nil {
		t.Fatalf("find element failed: %v", err)
	}
	if elem.StableID == "" {
		t.Fatal("element stable ID should not be empty")
	}

	interact, err := provider.Interact().Click(ctx, sess.SessionID, tab.TabID, *elem)
	if err != nil {
		t.Fatalf("click failed: %v", err)
	}
	if !interact.Success {
		t.Fatal("click should succeed")
	}

	download, err := provider.Resources().Download(ctx, BrowserDownloadRequest{
		SessionID:   sess.SessionID,
		TabID:       tab.TabID,
		ResourceURI: "amitia://workspace/downloads/test.pdf",
		Filename:    "test.pdf",
	})
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if download.ResourceURI == "" {
		t.Fatal("download result resource URI should not be empty")
	}

	upload, err := provider.Resources().Upload(ctx, BrowserUploadRequest{
		SessionID:   sess.SessionID,
		TabID:       tab.TabID,
		ResourceURI: "amitia://attachments/upload/file.png",
	})
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	if !upload.Success {
		t.Fatal("upload should succeed")
	}

	screenshot, err := provider.Resources().Screenshot(ctx, BrowserScreenshotRequest{
		SessionID: sess.SessionID,
		TabID:     tab.TabID,
	})
	if err != nil {
		t.Fatalf("screenshot failed: %v", err)
	}
	if screenshot.ResourceURI == "" {
		t.Fatal("screenshot resource URI should not be empty")
	}
}

func TestBrowserErrorClassification(t *testing.T) {
	tests := []struct {
		err      *BrowserError
		checkFn  func(error) bool
		expected bool
	}{
		{&BrowserError{Code: ErrCodeSessionNotFound, Message: "x"}, IsSessionNotFound, true},
		{&BrowserError{Code: ErrCodeTabNotFound, Message: "x"}, IsTabNotFound, true},
		{&BrowserError{Code: ErrCodeStaleElement, Message: "x"}, IsStaleElement, true},
		{&BrowserError{Code: ErrCodeSessionNotFound, Message: "x"}, IsTabNotFound, false},
	}

	for _, tc := range tests {
		if tc.checkFn(tc.err) != tc.expected {
			t.Fatalf("error classification mismatch for code %s", tc.err.Code)
		}
	}
}

func TestBrowserErrorUnwrap(t *testing.T) {
	inner := &BrowserError{Code: ErrCodeProviderUnavailable, Message: "provider down"}
	outer := &BrowserError{Code: ErrCodeNavigationFailed, Message: "nav failed", Cause: inner}

	if outer.Unwrap() != inner {
		t.Fatal("Unwrap should return cause")
	}
}

func TestBrowserProviderSessionNotFound(t *testing.T) {
	provider := newFakeBrowserProvider()
	ctx := context.Background()

	_, err := provider.Navigate().Navigate(ctx, "", "tab-1", "https://example.com")
	if err == nil {
		t.Fatal("should return error for empty session")
	}
	if !IsSessionNotFound(err) {
		t.Fatalf("expected session_not_found, got: %v", err)
	}
}

func TestBrowserProviderStaleElement(t *testing.T) {
	provider := newFakeBrowserProvider()
	ctx := context.Background()

	sess, _ := provider.Sessions().CreateSession(ctx)
	tabs, _ := provider.Tabs().ListTabs(ctx, sess.SessionID)
	if len(tabs) == 0 {
		t.Fatal("expected at least one tab")
	}

	staleElem := BrowserElementRef{
		SessionID: sess.SessionID,
		TabID:     tabs[0].TabID,
		Selector:  ".old",
		StableID:  "stale",
	}

	result, err := provider.Interact().Click(ctx, sess.SessionID, tabs[0].TabID, staleElem)
	if err == nil {
		t.Fatal("should return error for stale element")
	}
	if !IsStaleElement(err) {
		t.Fatalf("expected stale_element error, got: %v", err)
	}
	if !result.Stale {
		t.Fatal("interaction result should indicate stale")
	}
}

func TestBrowserProviderUploadRequiresResourceURI(t *testing.T) {
	provider := newFakeBrowserProvider()
	ctx := context.Background()

	sess, _ := provider.Sessions().CreateSession(ctx)
	tabs, _ := provider.Tabs().ListTabs(ctx, sess.SessionID)

	_, err := provider.Resources().Upload(ctx, BrowserUploadRequest{
		SessionID: sess.SessionID,
		TabID:     tabs[0].TabID,
	})
	if err == nil {
		t.Fatal("should fail without ResourceURI")
	}
	if err.Code != ErrCodeInvalidRequest {
		t.Fatalf("expected invalid_request, got: %v", err)
	}
}

func TestBrowserDownloadResultContainsResourceURI(t *testing.T) {
	provider := newFakeBrowserProvider()
	ctx := context.Background()

	sess, _ := provider.Sessions().CreateSession(ctx)
	tabs, _ := provider.Tabs().ListTabs(ctx, sess.SessionID)

	result, err := provider.Resources().Download(ctx, BrowserDownloadRequest{
		SessionID:   sess.SessionID,
		TabID:       tabs[0].TabID,
		ResourceURI: "amitia://workspace/downloads/report.xlsx",
		Filename:    "report.xlsx",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResourceURI != "amitia://workspace/downloads/report.xlsx" {
		t.Fatalf("download result ResourceURI mismatch: %s", result.ResourceURI)
	}
}
