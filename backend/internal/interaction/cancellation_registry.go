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

func (r *CancellationRegistry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.entries)
}
