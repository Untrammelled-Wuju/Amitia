package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

var knownKinds = map[WorkspaceKind]bool{
	WorkspaceKindLocal:    true,
	WorkspaceKindSAF:      true,
	WorkspaceKindRemote:   true,
	WorkspaceKindIsolated: true,
}

func RegisterKnownKind(kind WorkspaceKind) {
	knownKinds[kind] = true
}

func IsKnownKind(kind WorkspaceKind) bool {
	return knownKinds[kind]
}

type Registry struct {
	mu      sync.RWMutex
	mounts  map[WorkspaceID]WorkspaceMount
	backend map[WorkspaceKind]WorkspaceBackend
}

func NewRegistry() *Registry {
	return &Registry{
		mounts:  make(map[WorkspaceID]WorkspaceMount),
		backend: make(map[WorkspaceKind]WorkspaceBackend),
	}
}

func (r *Registry) RegisterBackend(kind WorkspaceKind, backend WorkspaceBackend) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.backend[kind]; exists {
		return fmt.Errorf("workspace backend %q already registered", kind)
	}
	r.backend[kind] = backend
	return nil
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

func (r *Registry) RegisterRemoteMount(ctx context.Context, name string, config RemoteMountConfig, credRef string, readOnly bool) (WorkspaceMount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	id := WorkspaceID(uuid.NewString())
	configJSON, err := json.Marshal(config)
	if err != nil {
		return WorkspaceMount{}, err
	}
	mount := WorkspaceMount{
		ID:            id,
		Name:          name,
		Kind:          WorkspaceKindRemote,
		ReadOnly:      readOnly,
		Available:     false,
		Status:        WorkspaceStatusUnavailable,
		RootURI:       MountURI(id),
		BackendConfig: string(configJSON),
		CredentialRef: credRef,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	r.mounts[id] = mount
	return mount, nil
}

func (r *Registry) UpdateRemoteMountConfig(id WorkspaceID, config RemoteMountConfig, credRef string, readOnly bool) (WorkspaceMount, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.mounts[id]
	if !ok {
		return WorkspaceMount{}, false
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return WorkspaceMount{}, false
	}
	m.BackendConfig = string(configJSON)
	if credRef != "" {
		m.CredentialRef = credRef
	}
	m.ReadOnly = readOnly
	m.Available = false
	m.Status = WorkspaceStatusUnavailable
	m.UpdatedAt = time.Now().UTC()
	r.mounts[id] = m
	return m, true
}

func (r *Registry) InvalidateRemoteClients(id WorkspaceID) {
	r.mu.RLock()
	defer r.mu.RUnlock()
}

func (r *Registry) RegisterIsolatedMount(ctx context.Context, name string, backendConfig string, readOnly bool) (WorkspaceMount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	id := WorkspaceID(uuid.NewString())
	status := WorkspaceStatusReady
	if readOnly {
		status = WorkspaceStatusReadOnly
	}
	mount := WorkspaceMount{
		ID:            id,
		Name:          name,
		Kind:          WorkspaceKindIsolated,
		ReadOnly:      readOnly,
		Available:     true,
		Status:        status,
		RootURI:       MountURI(id),
		BackendConfig: backendConfig,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	r.mounts[id] = mount
	return mount, nil
}

func (r *Registry) UpdateIsolatedMountConfig(id WorkspaceID, backendConfig string, readOnly bool) (WorkspaceMount, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.mounts[id]
	if !ok {
		return WorkspaceMount{}, false
	}
	if backendConfig != "" {
		m.BackendConfig = backendConfig
	}
	m.ReadOnly = readOnly
	m.UpdatedAt = time.Now().UTC()
	r.mounts[id] = m
	return m, true
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
		ID:          id,
		Name:        name,
		Kind:        WorkspaceKindSAF,
		ReadOnly:    readOnly,
		Available:   true,
		Status:      status,
		RootURI:     MountURI(id),
		NativeGrant: grantID,
		CreatedAt:   now,
		UpdatedAt:   now,
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

func (r *Registry) RestoreMount(mount WorkspaceMount) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mounts[mount.ID] = mount
	return nil
}

func (r *Registry) ReplaceSAFGrant(id WorkspaceID, grantID string, readOnly bool) (WorkspaceMount, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.mounts[id]
	if !ok {
		return WorkspaceMount{}, false
	}
	m.NativeGrant = grantID
	m.ReadOnly = readOnly
	m.UpdatedAt = time.Now().UTC()
	r.mounts[id] = m
	return m, true
}

func (r *Registry) GetMount(id WorkspaceID) (WorkspaceMount, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.mounts[id]
	return m, ok
}

func (r *Registry) GetMountOrEmpty(id WorkspaceID) (WorkspaceMount, bool) {
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
