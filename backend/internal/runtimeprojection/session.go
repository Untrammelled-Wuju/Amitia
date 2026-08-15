package runtimeprojection

import (
	"sync"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type SessionPartition struct {
	mu         sync.RWMutex
	sessions   map[runtimeidentity.RuntimeSessionID]RuntimeProjection
	generation int64
}

func NewSessionPartition() *SessionPartition {
	return &SessionPartition{
		sessions: make(map[runtimeidentity.RuntimeSessionID]RuntimeProjection),
	}
}

func (p *SessionPartition) Upsert(proj RuntimeProjection) {
	p.mu.Lock()
	defer p.mu.Unlock()

	existing, exists := p.sessions[proj.SessionID]
	if exists && proj.ConnectionGeneration < existing.ConnectionGeneration {
		return
	}

	if !exists || proj.ConnectionGeneration > existing.ConnectionGeneration {
		p.generation++
	}

	p.sessions[proj.SessionID] = proj
}

func (p *SessionPartition) Remove(sessionID runtimeidentity.RuntimeSessionID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.sessions, sessionID)
}

func (p *SessionPartition) Get(sessionID runtimeidentity.RuntimeSessionID) (RuntimeProjection, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	proj, ok := p.sessions[sessionID]
	return proj, ok
}

func (p *SessionPartition) Active() []RuntimeProjection {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var result []RuntimeProjection
	for _, proj := range p.sessions {
		if proj.Online {
			result = append(result, proj)
		}
	}
	return result
}

func (p *SessionPartition) CurrentGeneration() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.generation
}
