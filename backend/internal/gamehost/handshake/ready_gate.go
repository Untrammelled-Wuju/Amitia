package handshake

import "sync"

type ReadyGate struct {
	mu      sync.RWMutex
	allowed []string
	ready   map[string]bool
}

func NewReadyGate(preReadyAllowed []string) *ReadyGate {
	allowed := make([]string, len(preReadyAllowed))
	copy(allowed, preReadyAllowed)
	return &ReadyGate{
		allowed: allowed,
		ready:   make(map[string]bool),
	}
}

func (g *ReadyGate) AllowPreReady(method string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.allowed = append(g.allowed, method)
}

func (g *ReadyGate) MarkReady(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.ready[key] = true
}

func (g *ReadyGate) MarkNotReady(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.ready, key)
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
	return g.ready[key]
}

func (g *ReadyGate) CanProcess(key, method string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.ready[key] {
		return true
	}
	for _, a := range g.allowed {
		if a == method {
			return true
		}
	}
	return false
}

func (g *ReadyGate) Remove(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.ready, key)
}

func (g *ReadyGate) ReadyCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.ready)
}
