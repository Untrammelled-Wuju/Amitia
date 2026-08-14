package browser

import (
	"context"
	"sync"
	"time"
)

type productionRecovery struct {
	runtime    BrowserRuntime
	sessions   *productionSessionManager
	tabs       *productionTabManager
	elements   *elementStore
	resources  *productionResourceTransfer
	store      *RecoveryStore
	detector   *CrashDetector
	policy     BrowserRecoveryPolicy
	mu         sync.RWMutex
	attempts   int
	lastResult *BrowserRecoveryResult
}

func NewProductionRecovery(
	runtime BrowserRuntime,
	sessions *productionSessionManager,
	tabs *productionTabManager,
	elements *elementStore,
	resources *productionResourceTransfer,
	policy BrowserRecoveryPolicy,
) *productionRecovery {
	if policy.MaxAttempts <= 0 {
		policy.MaxAttempts = 2
	}
	if policy.Backoff <= 0 {
		policy.Backoff = 1 * time.Second
	}
	return &productionRecovery{
		runtime:   runtime,
		sessions:  sessions,
		tabs:      tabs,
		elements:  elements,
		resources: resources,
		store:     NewRecoveryStore(32),
		detector:  NewCrashDetector(),
		policy:    policy,
	}
}

func (r *productionRecovery) AttemptRecovery(ctx context.Context) (*BrowserRecoveryResult, *BrowserError) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.policy.Enabled {
		return nil, &BrowserError{
			Code:    ErrCodeRecoveryFailed,
			Message: "recovery is disabled",
		}
	}

	if r.attempts >= r.policy.MaxAttempts {
		r.detector.MarkRuntimeFailure()
		return nil, &BrowserError{
			Code:    ErrCodeRecoveryLimitReached,
			Message: "recovery attempt limit reached",
		}
	}

	r.attempts++

	if r.lastResult != nil {
		r.lastResult = nil
	}

	result := &BrowserRecoveryResult{
		AuthStateRestored:      false,
		ElementRefsInvalidated: true,
		DownloadsInvalidated:   true,
		Warnings:               []string{},
	}

	status := r.runtime.Status(ctx)
	if status.State == BrowserRuntimeReady {
		result.Warnings = append(result.Warnings, "runtime is already ready, no recovery needed")
		return result, nil
	}

	if r.resources != nil {
		r.resources.invalidateDownloadsForGeneration(status.Generation)
	}

	if r.elements != nil {
		r.elements.clearAll()
	}

	r.store.InvalidateAll()

	if r.policy.RestoreSessions {
		descriptors := r.store.GetRecoverableSessions()
		result.SessionsRecovered = len(descriptors)
	}

	if r.policy.RestoreTabs {
		tabStates := r.store.GetRecoverableTabs()
		result.TabsRecovered = len(tabStates)
		result.TabsFailed = 0
	}

	if r.policy.AutoRestartRuntime && status.State != BrowserRuntimeReady {
		stopErr := r.runtime.Stop(ctx)
		if stopErr != nil {
			result.Warnings = append(result.Warnings, "failed to stop runtime before restart: "+stopErr.Message)
		}
	}

	result.RuntimeGeneration = status.Generation
	r.lastResult = result

	r.detector.Reset()

	return result, nil
}

func (r *productionRecovery) AttemptTabRecovery(ctx context.Context, tabID BrowserTabID) (*BrowserRecoveryResult, *BrowserError) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.policy.Enabled {
		return nil, &BrowserError{
			Code:    ErrCodeRecoveryFailed,
			Message: "recovery is disabled",
		}
	}

	result := &BrowserRecoveryResult{
		AuthStateRestored:      false,
		ElementRefsInvalidated: true,
		DownloadsInvalidated:   false,
		Warnings:               []string{},
	}

	status := r.runtime.Status(ctx)
	result.RuntimeGeneration = status.Generation

	r.elements.clearForTab(tabID)
	r.detector.MarkTabCrashed(tabID)

	tabState, ok := r.store.GetTabState(tabID)
	if ok {
		tabState.recoverable = false
		r.store.SaveTab(tabState)
	}

	result.TabsRecovered = 1
	result.Warnings = append(result.Warnings, "tab-level recovery initiated for "+string(tabID))

	return result, nil
}

func (r *productionRecovery) Policy() BrowserRecoveryPolicy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.policy
}

func (r *productionRecovery) SetPolicy(policy BrowserRecoveryPolicy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.policy = policy
}

func (r *productionRecovery) LastResult() *BrowserRecoveryResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastResult
}

func (r *productionRecovery) ResetAttempts() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attempts = 0
	r.detector.Reset()
}

func (r *productionRecovery) CanRecover() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.policy.Enabled && r.attempts < r.policy.MaxAttempts
}

func (r *productionRecovery) StoreSessionDescriptor(sessionID BrowserSessionID, state BrowserSessionState, recoverable bool) {
	desc := &sessionRecoveryDescriptor{
		sessionID:   sessionID,
		state:       state,
		createdAt:   time.Now(),
		recoverable: recoverable,
	}
	r.store.SaveSession(desc)
}

func (r *productionRecovery) RemoveSessionDescriptor(sessionID BrowserSessionID) {
	r.store.RemoveSession(sessionID)
}

func (r *productionRecovery) StoreTabState(tabID BrowserTabID, lastCommittedURL string, active bool, recoverable bool) {
	state := &tabRecoveryState{
		tabID:              tabID,
		lastCommittedURL:   lastCommittedURL,
		active:             active,
		recoverable:        recoverable,
		lastNavigationKind: "GET",
	}
	r.store.SaveTab(state)
}

func (r *productionRecovery) RemoveTabState(tabID BrowserTabID) {
	r.store.RemoveTab(tabID)
}

func (r *productionRecovery) MarkTabCrashed(tabID BrowserTabID) {
	r.detector.MarkTabCrashed(tabID)
}

func (r *productionRecovery) IsTabCrashed(tabID BrowserTabID) bool {
	return r.detector.IsTabCrashed(tabID)
}

func (r *productionRecovery) CrashDetector() *CrashDetector {
	return r.detector
}

func (r *productionRecovery) RecoveryStore() *RecoveryStore {
	return r.store
}

func DefaultBrowserRecoveryPolicy() BrowserRecoveryPolicy {
	return BrowserRecoveryPolicy{
		Enabled:            true,
		AutoRestartRuntime: true,
		RestoreSessions:    true,
		RestoreTabs:        true,
		RestoreLastSafeURL: true,
		MaxAttempts:        2,
		Backoff:            1 * time.Second,
	}
}
