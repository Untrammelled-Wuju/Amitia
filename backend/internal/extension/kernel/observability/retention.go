package observability

import (
	"context"
	"sync"
	"time"
)

type RetentionPolicy struct {
	Category         string
	MaxAge           time.Duration
	MaxCount         int
	CleanupBatchSize int
}

func DefaultRetentionPolicies() []RetentionPolicy {
	return []RetentionPolicy{
		{Category: "invocation", MaxAge: 30 * 24 * time.Hour, MaxCount: 100000, CleanupBatchSize: 1000},
		{Category: "runtime_event", MaxAge: 7 * 24 * time.Hour, MaxCount: 500000, CleanupBatchSize: 5000},
		{Category: "audit_high_risk", MaxAge: 365 * 24 * time.Hour, MaxCount: 0, CleanupBatchSize: 100},
		{Category: "audit_lifecycle", MaxAge: 365 * 24 * time.Hour, MaxCount: 0, CleanupBatchSize: 100},
		{Category: "error", MaxAge: 90 * 24 * time.Hour, MaxCount: 50000, CleanupBatchSize: 1000},
		{Category: "attempt", MaxAge: 30 * 24 * time.Hour, MaxCount: 100000, CleanupBatchSize: 1000},
	}
}

type RetentionManager struct {
	store    StorageBackend
	policies []RetentionPolicy
	mu       sync.Mutex
	stopCh   chan struct{}
}

func NewRetentionManager(store StorageBackend, policies []RetentionPolicy) *RetentionManager {
	return &RetentionManager{
		store:    store,
		policies: policies,
		stopCh:   make(chan struct{}),
	}
}

func (m *RetentionManager) Start(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.runCleanup(ctx)
			}
		}
	}()
}

func (m *RetentionManager) Stop() {
	close(m.stopCh)
}

func (m *RetentionManager) runCleanup(ctx context.Context) {
	for _, policy := range m.policies {
		cutoff := time.Now().Add(-policy.MaxAge)
		switch policy.Category {
		case "invocation":
			m.cleanupInvocations(ctx, cutoff, policy)
		case "runtime_event":
			m.cleanupRuntimeEvents(ctx, cutoff, policy)
		}
	}
}

func (m *RetentionManager) cleanupInvocations(ctx context.Context, cutoff time.Time, policy RetentionPolicy) {
	filter := InvocationFilter{
		Until:       &cutoff,
		ListOptions: ListOptions{Limit: policy.CleanupBatchSize},
	}

	invs, _, err := m.store.ListInvocations(ctx, filter)
	if err != nil {
		return
	}

	for _, inv := range invs {
		if !inv.Status.IsTerminal() {
			continue
		}
	}
}

func (m *RetentionManager) cleanupRuntimeEvents(ctx context.Context, cutoff time.Time, policy RetentionPolicy) {
	filter := EventFilter{
		Until:       &cutoff,
		ListOptions: ListOptions{Limit: policy.CleanupBatchSize},
	}

	_, _, err := m.store.ListRuntimeEvents(ctx, filter)
	if err != nil {
		return
	}
}
