package kernel

import (
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

type memoryDefinitionProvider struct {
	mu   sync.RWMutex
	defs map[string]*trusted_service.ServiceRuntimeDefinition
}

func newMemoryDefinitionProvider() *memoryDefinitionProvider {
	return &memoryDefinitionProvider{
		defs: make(map[string]*trusted_service.ServiceRuntimeDefinition),
	}
}

func (p *memoryDefinitionProvider) Register(def *trusted_service.ServiceRuntimeDefinition) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.defs[def.ServiceID] = def
}

func (p *memoryDefinitionProvider) GetServiceDefinition(serviceID string) (*trusted_service.ServiceRuntimeDefinition, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	def, exists := p.defs[serviceID]
	if !exists {
		return nil, trusted_service.ErrServiceNotFound
	}
	return def, nil
}

func (p *memoryDefinitionProvider) List() []*trusted_service.ServiceRuntimeDefinition {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]*trusted_service.ServiceRuntimeDefinition, 0, len(p.defs))
	for _, def := range p.defs {
		out = append(out, def)
	}
	return out
}

func (p *memoryDefinitionProvider) Remove(serviceID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.defs, serviceID)
}
