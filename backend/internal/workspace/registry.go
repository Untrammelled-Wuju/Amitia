package workspace

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Registry struct {
	mu     sync.RWMutex
	mounts  map[WorkspaceID]WorkspaceMount
	backend map[WorkspaceKind]WorkspaceBackend
}

func NewRegistry() *Registry {
	return &Registry{
		mounts:  make(map[WorkspaceID]WorkspaceMount),
		backend: make(map[WorkspaceKind]WorkspaceBackend),
	}
}

func (r *Registry) RegisterBackend(kind WorkspaceKind, backend WorkspaceBackend) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.backend[kind] = backend
}

func (r *Registry) GetBackend(kind WorkspaceKind) (WorkspaceBackend, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.backend[kind]
	return b, ok
}

func (r *Registry) RegisterLocalMount(ctx context.Context, name string, localRoot string, readOnly bool) (WorkspaceMount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	id := WorkspaceID(uuid.NewString())
	mount := WorkspaceMount{
		ID:        id,
		Name:      name,
		Kind:      WorkspaceKindLocal,
		ReadOnly:  readOnly,
		Available: true,
		Status:    WorkspaceStatusReady,
		RootURI:   MountURI(id),
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.mounts[id] = mount
	return mount, nil
}

func (r *Registry) RegisterSAFMount(ctx context.Context, name string, grantID string, readOnly bool) (WorkspaceMount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	id := WorkspaceID(uuid.NewString())
	status := WorkspaceStatusReady
	if readOnly {
		status = WorkspaceStatusReadOnly
	}
	mount := WorkspaceMount{
		ID:        id,
		Name:      name,
		Kind:      WorkspaceKindSAF,
		ReadOnly:  readOnly,
		Available: true,
		Status:    status,
		RootURI:   MountURI(id),
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.mounts[id] = mount
	return mount, nil
}

func (r *Registry) RemoveMount(ctx context.Context, id WorkspaceID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.mounts, id)
	return nil
}

func (r *Registry) GetMount(id WorkspaceID) (WorkspaceMount, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.mounts[id]
	return m, ok
}

func (r *Registry) ListMounts() []WorkspaceMount {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]WorkspaceMount, 0, len(r.mounts))
	for _, m := range r.mounts {
		result = append(result, m)
	}
	return result
}

func (r *Registry) UpdateStatus(id WorkspaceID, status WorkspaceStatus, available bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.mounts[id]
	if !ok {
		return false
	}
	m.Status = status
	m.Available = available
	m.UpdatedAt = time.Now().UTC()
	r.mounts[id] = m
	return true
}

func MountURI(id WorkspaceID) string {
	return "amitia://workspace/@" + string(id) + "/"
}
