package integration

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type HostAPIInvocationTracker struct {
	mu        sync.RWMutex
	inflight  map[string]hostAPIInvocation
	blocked   map[domain.RuntimeInstanceID]bool
	completed uint64
	failed    uint64
}

type hostAPIInvocation struct {
	runtimeID domain.RuntimeInstanceID
	startedAt time.Time
	cancel    context.CancelFunc
}

func NewHostAPIInvocationTracker() *HostAPIInvocationTracker {
	return &HostAPIInvocationTracker{
		inflight: make(map[string]hostAPIInvocation),
		blocked:  make(map[domain.RuntimeInstanceID]bool),
	}
}

func (t *HostAPIInvocationTracker) Begin(runtimeID domain.RuntimeInstanceID, callID string, cancel context.CancelFunc) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.blocked[runtimeID] {
		return false
	}
	t.inflight[callID] = hostAPIInvocation{runtimeID: runtimeID, startedAt: time.Now().UTC(), cancel: cancel}
	return true
}

func (t *HostAPIInvocationTracker) End(callID string, failed bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.inflight, callID)
	if failed {
		t.failed++
	} else {
		t.completed++
	}
}

func (t *HostAPIInvocationTracker) CancelRuntimeHostAPIWork(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error) {
	t.mu.Lock()
	t.blocked[runtimeID] = true
	cancellations := make([]context.CancelFunc, 0)
	for _, invocation := range t.inflight {
		if invocation.runtimeID == runtimeID && invocation.cancel != nil {
			cancellations = append(cancellations, invocation.cancel)
		}
	}
	t.mu.Unlock()
	for _, cancel := range cancellations {
		cancel()
	}
	return len(cancellations), ctx.Err()
}

func (t *HostAPIInvocationTracker) CountRuntimeHostAPIWork(runtimeID domain.RuntimeInstanceID) int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	count := 0
	for _, invocation := range t.inflight {
		if invocation.runtimeID == runtimeID {
			count++
		}
	}
	return count
}

func (t *HostAPIInvocationTracker) RearmRuntimeHostAPIWork(runtimeID domain.RuntimeInstanceID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.blocked, runtimeID)
}

func (t *HostAPIInvocationTracker) InflightCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.inflight)
}

func (t *HostAPIInvocationTracker) Stats() (inflight int, completed uint64, failed uint64) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.inflight), t.completed, t.failed
}
