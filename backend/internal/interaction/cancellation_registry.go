package interaction

import (
	"context"
	"sync"
	"time"
)

type CancellationRegistry struct {
	mu      sync.Mutex
	entries map[string]cancellationEntry
}

const cancellationRegistryMaxAge = 2 * time.Hour

type cancellationEntry struct {
	cancel    context.CancelFunc
	createdAt time.Time
}

func NewCancellationRegistry() *CancellationRegistry {
	return &CancellationRegistry{entries: make(map[string]cancellationEntry)}
}

func (r *CancellationRegistry) Register(interactionID string, cancel context.CancelFunc) {
	if interactionID == "" || cancel == nil {
		return
	}
	r.mu.Lock()
	r.cleanupStaleLocked(cancellationRegistryMaxAge, time.Now())
	r.entries[interactionID] = cancellationEntry{cancel: cancel, createdAt: time.Now()}
	r.mu.Unlock()
}

func (r *CancellationRegistry) Cancel(interactionID string) bool {
	r.mu.Lock()
	entry, ok := r.entries[interactionID]
	r.mu.Unlock()
	if !ok || entry.cancel == nil {
		return false
	}
	entry.cancel()
	return true
}

func (r *CancellationRegistry) Unregister(interactionID string) {
	r.mu.Lock()
	delete(r.entries, interactionID)
	r.mu.Unlock()
}

func (r *CancellationRegistry) CleanupStale(maxAge time.Duration) int {
	return r.CleanupStaleAt(maxAge, time.Now())
}

func (r *CancellationRegistry) CleanupStaleAt(maxAge time.Duration, now time.Time) int {
	if maxAge <= 0 {
		return 0
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	cleaned := 0
	cleaned = r.cleanupStaleLocked(maxAge, now)
	return cleaned
}

func (r *CancellationRegistry) cleanupStaleLocked(maxAge time.Duration, now time.Time) int {
	cleaned := 0
	for interactionID, entry := range r.entries {
		if now.Sub(entry.createdAt) > maxAge {
			delete(r.entries, interactionID)
			cleaned++
		}
	}
	return cleaned
}

func (r *CancellationRegistry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.entries)
}
