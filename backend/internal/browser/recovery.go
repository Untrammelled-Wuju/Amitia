package browser

import (
	"context"
	"sync"
	"time"
)

type productionRecovery struct {
	runtime   BrowserRuntime
	sessions  *productionSessionManager
	tabs      *productionTabManager
	elements  *elementStore
	resources *productionResourceTransfer
	policy    BrowserRecoveryPolicy
	mu        sync.RWMutex
	attempts  int
	lastResult *BrowserRecoveryResult
}

func NewProductionRecovery(runtime BrowserRuntime, sessions *productionSessionManager, tabs *productionTabManager, elements *elementStore, resources *productionResourceTransfer, policy BrowserRecoveryPolicy) *productionRecovery {
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

	if r.policy.AutoRestartRuntime && status.State != BrowserRuntimeReady {
		restartErr := r.runtime.Stop(ctx)
		if restartErr != nil {
			result.Warnings = append(result.Warnings, "failed to stop runtime before restart: " + restartErr.Message)
		}
	}

	result.RuntimeGeneration = status.Generation
	r.lastResult = result

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
}

func (r *productionRecovery) CanRecover() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.policy.Enabled && r.attempts < r.policy.MaxAttempts
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
