package handshake

import (
	"context"
	"errors"
	"sync"
)

var ErrReadyGateRemoved = errors.New("handshake: ready gate entry removed")

type readyEntry struct {
	ready   bool
	removed bool
	changed chan struct{}
}

type ReadyGate struct {
	mu      sync.RWMutex
	allowed []string
	entries map[string]*readyEntry
}

func NewReadyGate(preReadyAllowed []string) *ReadyGate {
	allowed := make([]string, len(preReadyAllowed))
	copy(allowed, preReadyAllowed)
	return &ReadyGate{
		allowed: allowed,
		entries: make(map[string]*readyEntry),
	}
}

func (g *ReadyGate) AllowPreReady(method string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.allowed = append(g.allowed, method)
}

func (g *ReadyGate) ensureEntryLocked(key string) *readyEntry {
	entry := g.entries[key]
	if entry == nil {
		entry = &readyEntry{changed: make(chan struct{})}
		g.entries[key] = entry
	}
	return entry
}

func notifyReadyEntry(entry *readyEntry) {
	if entry == nil || entry.changed == nil {
		return
	}
	close(entry.changed)
	entry.changed = make(chan struct{})
}

func (g *ReadyGate) Register(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	entry := g.ensureEntryLocked(key)
	if entry.ready || entry.removed {
		entry.ready = false
		entry.removed = false
		notifyReadyEntry(entry)
	}
}

func (g *ReadyGate) MarkReady(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	entry := g.ensureEntryLocked(key)
	if entry.removed || entry.ready {
		return
	}
	entry.ready = true
	notifyReadyEntry(entry)
}

func (g *ReadyGate) MarkNotReady(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	entry := g.ensureEntryLocked(key)
	if entry.removed || !entry.ready {
		return
	}
	entry.ready = false
	notifyReadyEntry(entry)
}

func (g *ReadyGate) IsAllowedPreReady(method string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, a := range g.allowed {
		if a == method {
			return true
		}
	}
	return false
}

func (g *ReadyGate) IsReady(key string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	entry := g.entries[key]
	return entry != nil && !entry.removed && entry.ready
}

func (g *ReadyGate) CanProcess(key, method string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	entry := g.entries[key]
	if entry != nil && !entry.removed && entry.ready {
		return true
	}
	for _, a := range g.allowed {
		if a == method {
			return true
		}
	}
	return false
}

// WaitReady blocks until key has completed the protocol handshake. A removed
// connection fails immediately instead of being mistaken for a successful wake.
func (g *ReadyGate) WaitReady(ctx context.Context, key string) error {
	for {
		g.mu.Lock()
		entry := g.ensureEntryLocked(key)
		if entry.removed {
			g.mu.Unlock()
			return ErrReadyGateRemoved
		}
		if entry.ready {
			g.mu.Unlock()
			return nil
		}
		changed := entry.changed
		g.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}

func (g *ReadyGate) Remove(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	entry := g.ensureEntryLocked(key)
	entry.ready = false
	entry.removed = true
	notifyReadyEntry(entry)
}

func (g *ReadyGate) ReadyCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	count := 0
	for _, entry := range g.entries {
		if entry != nil && !entry.removed && entry.ready {
			count++
		}
	}
	return count
}
