package integration

import (
	"sync"
	"time"
)

type HostAPIInvocationTracker struct {
	mu         sync.RWMutex
	inflight   map[string]time.Time
	completed  uint64
	failed     uint64
}

func NewHostAPIInvocationTracker() *HostAPIInvocationTracker {
	return &HostAPIInvocationTracker{
		inflight: make(map[string]time.Time),
	}
}

func (t *HostAPIInvocationTracker) Begin(callID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.inflight[callID] = time.Now().UTC()
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
