package browser

import (
	"context"
	"testing"
)

type fakeSessionResolver struct {
	sessionID  BrowserSessionID
	contextID  BrowserContextID
	generation uint64
	resolveErr *BrowserError
}

func (r *fakeSessionResolver) ResolveSession(_ context.Context, sessionID BrowserSessionID) (ResolvedBrowserSession, *BrowserError) {
	if r.resolveErr != nil {
		return ResolvedBrowserSession{}, r.resolveErr
	}
	return ResolvedBrowserSession{
		SessionID:         sessionID,
		BrowserContextID:  r.contextID,
		RuntimeGeneration: r.generation,
	}, nil
}

type fakeTabBackend struct {
	createErr   error
	closeErr    error
	activateErr error
	targetID    TargetID
}

func (b *fakeTabBackend) CreateTarget(_ context.Context, _ BrowserContextID, _ string) (TargetID, error) {
	return b.targetID, b.createErr
}

func (b *fakeTabBackend) CloseTarget(_ context.Context, _ TargetID) error {
	return b.closeErr
}

func (b *fakeTabBackend) ActivateTarget(_ context.Context, _ TargetID) error {
	return b.activateErr
}

func (b *fakeTabBackend) TargetInfo(_ context.Context, _ TargetID) (TargetInfo, error) {
	return TargetInfo{TargetID: b.targetID}, nil
}

func TestProductionTabManagerCreateTabSuccess(t *testing.T) {
	resolver := &fakeSessionResolver{
		contextID:  "ctx_1",
		generation: 1,
	}
	backend := &fakeTabBackend{targetID: TargetID("target_1")}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	tab, err := manager.CreateTab(context.Background(), "bs_s1")
	if err != nil {
		t.Fatalf("create tab failed: %v", err)
	}
	if tab.TabID == "" {
		t.Fatal("tab ID should not be empty")
	}
	if tab.SessionID != "bs_s1" {
		t.Fatalf("expected session ID bs_s1, got: %s", tab.SessionID)
	}
	if tab.State != TabStateReady {
		t.Fatalf("expected state ready, got: %s", tab.State)
	}
	if tab.URL != "about:blank" {
		t.Fatalf("expected URL about:blank, got: %s", tab.URL)
	}
}

func TestProductionTabManagerCreateTabEmptySessionID(t *testing.T) {
	resolver := &fakeSessionResolver{generation: 1}
	backend := &fakeTabBackend{}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	_, err := manager.CreateTab(context.Background(), "")
	if err == nil {
		t.Fatal("should fail with empty session ID")
	}
	if err.Code != ErrCodeInvalidRequest {
		t.Fatalf("expected invalid_request, got: %s", err.Code)
	}
}

func TestProductionTabManagerCreateTabSessionNotFound(t *testing.T) {
	resolver := &fakeSessionResolver{
		resolveErr: &BrowserError{Code: ErrCodeSessionNotFound, Message: "session not found"},
	}
	backend := &fakeTabBackend{}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	_, err := manager.CreateTab(context.Background(), "bs_nonexistent")
	if err == nil {
		t.Fatal("should fail when session not found")
	}
	if !IsSessionNotFound(err) {
		t.Fatalf("expected session_not_found, got: %v", err)
	}
}

func TestProductionTabManagerCreateTabQuotaPerSession(t *testing.T) {
	resolver := &fakeSessionResolver{
		contextID:  "ctx_1",
		generation: 1,
	}
	backend := &fakeTabBackend{targetID: TargetID("target_1")}
	manager := NewProductionTabManager(resolver, backend, 2, 32)

	_, err := manager.CreateTab(context.Background(), "bs_s1")
	if err != nil {
		t.Fatalf("first tab should succeed: %v", err)
	}
	_, err = manager.CreateTab(context.Background(), "bs_s1")
	if err != nil {
		t.Fatalf("second tab should succeed: %v", err)
	}
	_, err = manager.CreateTab(context.Background(), "bs_s1")
	if err == nil {
		t.Fatal("third tab should fail due to per-session quota")
	}
	if !IsTabQuotaReached(err) {
		t.Fatalf("expected tab_quota_reached, got: %v", err)
	}
}

func TestProductionTabManagerCreateTabQuotaTotal(t *testing.T) {
	resolver := &fakeSessionResolver{
		contextID:  "ctx_1",
		generation: 1,
	}
	backend := &fakeTabBackend{targetID: TargetID("target_1")}
	manager := NewProductionTabManager(resolver, backend, 8, 2)

	_, err := manager.CreateTab(context.Background(), "bs_s1")
	if err != nil {
		t.Fatalf("first tab should succeed: %v", err)
	}
	_, err = manager.CreateTab(context.Background(), "bs_s2")
	if err != nil {
		t.Fatalf("second tab should succeed: %v", err)
	}
	_, err = manager.CreateTab(context.Background(), "bs_s3")
	if err == nil {
		t.Fatal("third tab should fail due to total quota")
	}
	if !IsTabQuotaReached(err) {
		t.Fatalf("expected tab_quota_reached, got: %v", err)
	}
}

func TestProductionTabManagerCreateTabBackendFailure(t *testing.T) {
	resolver := &fakeSessionResolver{
		contextID:  "ctx_1",
		generation: 1,
	}
	backend := &fakeTabBackend{createErr: &BrowserError{Code: ErrCodeTabCreateFailed, Message: "backend error"}}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	_, err := manager.CreateTab(context.Background(), "bs_s1")
	if err == nil {
		t.Fatal("should fail when backend fails")
	}
	if err.Code != ErrCodeTabCreateFailed {
		t.Fatalf("expected tab_create_failed, got: %s", err.Code)
	}
}

func TestProductionTabManagerCreateTabContextCancelled(t *testing.T) {
	resolver := &fakeSessionResolver{generation: 1}
	backend := &fakeTabBackend{}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := manager.CreateTab(ctx, "bs_s1")
	if err == nil {
		t.Fatal("should fail with cancelled context")
	}
	if err.Code != ErrCodeInvalidRequest {
		t.Fatalf("expected invalid_request, got: %s", err.Code)
	}
}

func TestProductionTabManagerCloseTabSuccess(t *testing.T) {
	resolver := &fakeSessionResolver{
		contextID:  "ctx_1",
		generation: 1,
	}
	backend := &fakeTabBackend{targetID: TargetID("target_1")}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	tab, err := manager.CreateTab(context.Background(), "bs_s1")
	if err != nil {
		t.Fatalf("create tab failed: %v", err)
	}

	closeErr := manager.CloseTab(context.Background(), "bs_s1", tab.TabID)
	if closeErr != nil {
		t.Fatalf("close tab failed: %v", closeErr)
	}
}

func TestProductionTabManagerCloseTabNotFound(t *testing.T) {
	resolver := &fakeSessionResolver{generation: 1}
	backend := &fakeTabBackend{}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	err := manager.CloseTab(context.Background(), "bs_s1", "bt_nonexistent")
	if err == nil {
		t.Fatal("should fail when tab not found")
	}
	if !IsTabNotFound(err) {
		t.Fatalf("expected tab_not_found, got: %v", err)
	}
}

func TestProductionTabManagerCloseTabWrongSession(t *testing.T) {
	resolver := &fakeSessionResolver{
		contextID:  "ctx_1",
		generation: 1,
	}
	backend := &fakeTabBackend{targetID: TargetID("target_1")}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	tab, err := manager.CreateTab(context.Background(), "bs_s1")
	if err != nil {
		t.Fatalf("create tab failed: %v", err)
	}

	closeErr := manager.CloseTab(context.Background(), "bs_other", tab.TabID)
	if closeErr == nil {
		t.Fatal("should fail with wrong session")
	}
	if !IsTabNotFound(closeErr) {
		t.Fatalf("expected tab_not_found, got: %v", closeErr)
	}
}

func TestProductionTabManagerCloseTabIdempotent(t *testing.T) {
	resolver := &fakeSessionResolver{
		contextID:  "ctx_1",
		generation: 1,
	}
	backend := &fakeTabBackend{targetID: TargetID("target_1")}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	tab, err := manager.CreateTab(context.Background(), "bs_s1")
	if err != nil {
		t.Fatalf("create tab failed: %v", err)
	}

	if err := manager.CloseTab(context.Background(), "bs_s1", tab.TabID); err != nil {
		t.Fatalf("first close should succeed: %v", err)
	}

	err = manager.CloseTab(context.Background(), "bs_s1", tab.TabID)
	if err == nil {
		t.Fatal("second close should fail because tab is already removed from store")
	}
	if !IsTabNotFound(err) {
		t.Fatalf("expected tab_not_found after tab removed, got: %v", err)
	}
}

func TestProductionTabManagerGetTabSuccess(t *testing.T) {
	resolver := &fakeSessionResolver{
		contextID:  "ctx_1",
		generation: 1,
	}
	backend := &fakeTabBackend{targetID: TargetID("target_1")}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	tab, err := manager.CreateTab(context.Background(), "bs_s1")
	if err != nil {
		t.Fatalf("create tab failed: %v", err)
	}

	got, err := manager.GetTab(context.Background(), "bs_s1", tab.TabID)
	if err != nil {
		t.Fatalf("get tab failed: %v", err)
	}
	if got.TabID != tab.TabID {
		t.Fatalf("expected tab ID %s, got: %s", tab.TabID, got.TabID)
	}
}

func TestProductionTabManagerGetTabNotFound(t *testing.T) {
	resolver := &fakeSessionResolver{generation: 1}
	backend := &fakeTabBackend{}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	_, err := manager.GetTab(context.Background(), "bs_s1", "bt_nonexistent")
	if err == nil {
		t.Fatal("should fail when tab not found")
	}
	if !IsTabNotFound(err) {
		t.Fatalf("expected tab_not_found, got: %v", err)
	}
}

func TestProductionTabManagerListTabs(t *testing.T) {
	resolver := &fakeSessionResolver{
		contextID:  "ctx_1",
		generation: 1,
	}
	backend := &fakeTabBackend{targetID: TargetID("target_1")}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	_, err := manager.CreateTab(context.Background(), "bs_s1")
	if err != nil {
		t.Fatalf("create tab 1 failed: %v", err)
	}
	_, err = manager.CreateTab(context.Background(), "bs_s1")
	if err != nil {
		t.Fatalf("create tab 2 failed: %v", err)
	}
	_, err = manager.CreateTab(context.Background(), "bs_s2")
	if err != nil {
		t.Fatalf("create tab 3 failed: %v", err)
	}

	tabs, err := manager.ListTabs(context.Background(), "bs_s1")
	if err != nil {
		t.Fatalf("list tabs failed: %v", err)
	}
	if len(tabs) != 2 {
		t.Fatalf("expected 2 tabs for bs_s1, got: %d", len(tabs))
	}
}

func TestProductionTabManagerListTabsEmptySession(t *testing.T) {
	resolver := &fakeSessionResolver{generation: 1}
	backend := &fakeTabBackend{}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	_, err := manager.ListTabs(context.Background(), "")
	if err == nil {
		t.Fatal("should fail with empty session ID")
	}
	if err.Code != ErrCodeInvalidRequest {
		t.Fatalf("expected invalid_request, got: %s", err.Code)
	}
}

func TestProductionTabManagerActivateTabSuccess(t *testing.T) {
	resolver := &fakeSessionResolver{
		contextID:  "ctx_1",
		generation: 1,
	}
	backend := &fakeTabBackend{targetID: TargetID("target_1")}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	tab, err := manager.CreateTab(context.Background(), "bs_s1")
	if err != nil {
		t.Fatalf("create tab failed: %v", err)
	}

	if err := manager.ActivateTab(context.Background(), "bs_s1", tab.TabID); err != nil {
		t.Fatalf("activate tab failed: %v", err)
	}

	got, err := manager.GetTab(context.Background(), "bs_s1", tab.TabID)
	if err != nil {
		t.Fatalf("get tab failed: %v", err)
	}
	if !got.Active {
		t.Fatal("tab should be active after activation")
	}
}

func TestProductionTabManagerActivateTabNotFound(t *testing.T) {
	resolver := &fakeSessionResolver{generation: 1}
	backend := &fakeTabBackend{}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	err := manager.ActivateTab(context.Background(), "bs_s1", "bt_nonexistent")
	if err == nil {
		t.Fatal("should fail when tab not found")
	}
	if !IsTabNotFound(err) {
		t.Fatalf("expected tab_not_found, got: %v", err)
	}
}

func TestProductionTabManagerDefaults(t *testing.T) {
	resolver := &fakeSessionResolver{generation: 1}
	backend := &fakeTabBackend{}
	manager := NewProductionTabManager(resolver, backend, 0, 0)

	tm := manager.(*productionTabManager)
	if tm.maxTabsPerSession != DefaultMaxTabsPerSession {
		t.Fatalf("expected default max tabs per session %d, got: %d", DefaultMaxTabsPerSession, tm.maxTabsPerSession)
	}
	if tm.maxTabsTotal != DefaultMaxTabsTotal {
		t.Fatalf("expected default max tabs total %d, got: %d", DefaultMaxTabsTotal, tm.maxTabsTotal)
	}
}

func TestProductionTabManagerResolveTab(t *testing.T) {
	resolver := &fakeSessionResolver{
		contextID:  "ctx_1",
		generation: 1,
	}
	backend := &fakeTabBackend{targetID: TargetID("target_1")}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	tab, err := manager.CreateTab(context.Background(), "bs_s1")
	if err != nil {
		t.Fatalf("create tab failed: %v", err)
	}

	tm := manager.(*productionTabManager)
	resolved, err := tm.ResolveTab(context.Background(), "bs_s1", tab.TabID)
	if err != nil {
		t.Fatalf("resolve tab failed: %v", err)
	}
	if resolved.TabID != tab.TabID {
		t.Fatalf("expected tab ID %s, got: %s", tab.TabID, resolved.TabID)
	}
	if resolved.BrowserContextID != "ctx_1" {
		t.Fatalf("expected context ID ctx_1, got: %s", resolved.BrowserContextID)
	}
	if resolved.TargetID != TargetID("target_1") {
		t.Fatalf("expected target ID target_1, got: %s", resolved.TargetID)
	}
}

func TestProductionTabManagerCloseAllForSession(t *testing.T) {
	resolver := &fakeSessionResolver{
		contextID:  "ctx_1",
		generation: 1,
	}
	backend := &fakeTabBackend{targetID: TargetID("target_1")}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	_, err := manager.CreateTab(context.Background(), "bs_s1")
	if err != nil {
		t.Fatalf("create tab 1 failed: %v", err)
	}
	_, err = manager.CreateTab(context.Background(), "bs_s1")
	if err != nil {
		t.Fatalf("create tab 2 failed: %v", err)
	}
	_, err = manager.CreateTab(context.Background(), "bs_s2")
	if err != nil {
		t.Fatalf("create tab 3 failed: %v", err)
	}

	tm := manager.(*productionTabManager)
	if err := tm.CloseAllForSession(context.Background(), "bs_s1", 1); err != nil {
		t.Fatalf("close all for session failed: %v", err)
	}

	tabs, err := manager.ListTabs(context.Background(), "bs_s1")
	if err != nil {
		t.Fatalf("list tabs failed: %v", err)
	}
	if len(tabs) != 0 {
		t.Fatalf("expected 0 tabs for bs_s1 after close all, got: %d", len(tabs))
	}

	tabs, err = manager.ListTabs(context.Background(), "bs_s2")
	if err != nil {
		t.Fatalf("list tabs failed: %v", err)
	}
	if len(tabs) != 1 {
		t.Fatalf("expected 1 tab for bs_s2, got: %d", len(tabs))
	}
}

func TestProductionTabManagerStaleGeneration(t *testing.T) {
	resolver := &fakeSessionResolver{
		contextID:  "ctx_1",
		generation: 1,
	}
	backend := &fakeTabBackend{targetID: TargetID("target_1")}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	tab, err := manager.CreateTab(context.Background(), "bs_s1")
	if err != nil {
		t.Fatalf("create tab failed: %v", err)
	}

	resolver.generation = 2

	_, err = manager.GetTab(context.Background(), "bs_s1", tab.TabID)
	if err == nil {
		t.Fatal("should fail with stale generation")
	}
	if !IsTabStale(err) {
		t.Fatalf("expected tab_stale, got: %v", err)
	}
}

func TestProductionTabManagerContextCancelled(t *testing.T) {
	resolver := &fakeSessionResolver{generation: 1}
	backend := &fakeTabBackend{}
	manager := NewProductionTabManager(resolver, backend, 8, 32)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := manager.CloseTab(ctx, "bs_s1", "bt_1"); err == nil {
		t.Fatal("should fail with cancelled context")
	}
}
