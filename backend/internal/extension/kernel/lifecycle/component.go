package lifecycle

import (
	"context"
	"sync"
)

type LifecycleComponent interface {
	ID() string
	Dependencies() []string
	Start(ctx context.Context) error
	Ready(ctx context.Context) error
	Stop(ctx context.Context, reason ShutdownReason) error
	Health(ctx context.Context) ComponentHealth
}

type RecoverableLifecycleComponent interface {
	LifecycleComponent
	Recover(ctx context.Context, state RecoveryContext) error
}

type DrainingComponent interface {
	LifecycleComponent
	Drain(ctx context.Context) error
}

type ComponentRegistry struct {
	mu         sync.RWMutex
	components map[string]LifecycleComponent
	metadata   map[string]BootstrapComponent
}

func NewComponentRegistry() *ComponentRegistry {
	return &ComponentRegistry{
		components: make(map[string]LifecycleComponent),
		metadata:   make(map[string]BootstrapComponent),
	}
}

func (r *ComponentRegistry) Register(component LifecycleComponent, meta BootstrapComponent) error {
	if component == nil {
		return ErrComponentNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	id := component.ID()
	if id == "" {
		return ErrComponentNotFound
	}
	if _, exists := r.components[id]; exists {
		return ErrDuplicateComponent
	}
	r.components[id] = component
	r.metadata[id] = meta
	return nil
}

func (r *ComponentRegistry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.components, id)
	delete(r.metadata, id)
}

func (r *ComponentRegistry) Get(id string) (LifecycleComponent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.components[id]
	return c, ok
}

func (r *ComponentRegistry) Metadata(id string) (BootstrapComponent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.metadata[id]
	return m, ok
}

func (r *ComponentRegistry) All() []LifecycleComponent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]LifecycleComponent, 0, len(r.components))
	for _, c := range r.components {
		out = append(out, c)
	}
	return out
}

func (r *ComponentRegistry) AllMetadata() []BootstrapComponent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]BootstrapComponent, 0, len(r.metadata))
	for _, m := range r.metadata {
		out = append(out, m)
	}
	return out
}

func (r *ComponentRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.components)
}

func (r *ComponentRegistry) AsRecoverable(id string) (RecoverableLifecycleComponent, bool) {
	c, ok := r.Get(id)
	if !ok {
		return nil, false
	}
	rc, ok := c.(RecoverableLifecycleComponent)
	return rc, ok
}

func (r *ComponentRegistry) AsDraining(id string) (DrainingComponent, bool) {
	c, ok := r.Get(id)
	if !ok {
		return nil, false
	}
	dc, ok := c.(DrainingComponent)
	return dc, ok
}
