package dev_mode

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type WorkspaceID string
type ExtensionID string

type WorkspaceStatus string

const (
	WorkspaceStatusRegistered WorkspaceStatus = "registered"
	WorkspaceStatusBuilding   WorkspaceStatus = "building"
	WorkspaceStatusReady      WorkspaceStatus = "ready"
	WorkspaceStatusReloading  WorkspaceStatus = "reloading"
	WorkspaceStatusFailed     WorkspaceStatus = "failed"
	WorkspaceStatusDisabled   WorkspaceStatus = "disabled"
)

type DevelopmentWorkspace struct {
	WorkspaceID     WorkspaceID
	ExtensionID     ExtensionID
	PathReference   string
	ManifestPath    string
	CurrentRevision RevisionID
	Status          WorkspaceStatus
	WatchEnabled    bool
	AutoReload      bool
	DevTrust        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastReloadAt    time.Time
	FailureCount    int
	LastError       string
}

type WorkspaceRegistry struct {
	mu         sync.RWMutex
	workspaces map[WorkspaceID]*DevelopmentWorkspace
	byExtension map[ExtensionID]WorkspaceID
}

func NewWorkspaceRegistry() *WorkspaceRegistry {
	return &WorkspaceRegistry{
		workspaces:  make(map[WorkspaceID]*DevelopmentWorkspace),
		byExtension: make(map[ExtensionID]WorkspaceID),
	}
}

var (
	ErrWorkspaceNotFound      = errors.New("dev_mode: workspace not found")
	ErrWorkspaceExists        = errors.New("dev_mode: workspace exists")
	ErrWorkspaceConflict      = errors.New("dev_mode: extension already has a workspace")
	ErrWorkspaceNotReady      = errors.New("dev_mode: workspace not ready")
	ErrDevTrustNotGranted     = errors.New("dev_mode: developer trust not granted")
	ErrInvalidWorkspaceInput  = errors.New("dev_mode: invalid workspace input")
)

type RegisterWorkspaceInput struct {
	WorkspaceID  WorkspaceID
	ExtensionID  ExtensionID
	PathReference string
	ManifestPath string
	WatchEnabled bool
	AutoReload   bool
}

func (r *WorkspaceRegistry) Register(ctx context.Context, in RegisterWorkspaceInput) (*DevelopmentWorkspace, error) {
	if in.WorkspaceID == "" || in.ExtensionID == "" || in.PathReference == "" || in.ManifestPath == "" {
		return nil, ErrInvalidWorkspaceInput
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.workspaces[in.WorkspaceID]; exists {
		return nil, fmt.Errorf("%w: %s", ErrWorkspaceExists, in.WorkspaceID)
	}
	if _, conflict := r.byExtension[in.ExtensionID]; conflict {
		return nil, fmt.Errorf("%w: %s", ErrWorkspaceConflict, in.ExtensionID)
	}
	now := time.Now().UTC()
	ws := &DevelopmentWorkspace{
		WorkspaceID:   in.WorkspaceID,
		ExtensionID:   in.ExtensionID,
		PathReference: in.PathReference,
		ManifestPath:  in.ManifestPath,
		Status:        WorkspaceStatusRegistered,
		WatchEnabled:  in.WatchEnabled,
		AutoReload:    in.AutoReload,
		DevTrust:      false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	r.workspaces[in.WorkspaceID] = ws
	r.byExtension[in.ExtensionID] = in.WorkspaceID
	return ws, nil
}

func (r *WorkspaceRegistry) Get(id WorkspaceID) (*DevelopmentWorkspace, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ws, ok := r.workspaces[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrWorkspaceNotFound, id)
	}
	return ws, nil
}

func (r *WorkspaceRegistry) GetByExtension(ext ExtensionID) (*DevelopmentWorkspace, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byExtension[ext]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrWorkspaceNotFound, ext)
	}
	return r.workspaces[id], nil
}

func (r *WorkspaceRegistry) List() []*DevelopmentWorkspace {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*DevelopmentWorkspace, 0, len(r.workspaces))
	for _, ws := range r.workspaces {
		out = append(out, ws)
	}
	return out
}

func (r *WorkspaceRegistry) UpdateStatus(id WorkspaceID, status WorkspaceStatus, lastErr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws, ok := r.workspaces[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrWorkspaceNotFound, id)
	}
	ws.Status = status
	ws.UpdatedAt = time.Now().UTC()
	if lastErr != "" {
		ws.LastError = lastErr
		ws.FailureCount++
	}
	if status == WorkspaceStatusReady {
		ws.LastReloadAt = ws.UpdatedAt
	}
	return nil
}

func (r *WorkspaceRegistry) SetCurrentRevision(id WorkspaceID, rev RevisionID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws, ok := r.workspaces[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrWorkspaceNotFound, id)
	}
	ws.CurrentRevision = rev
	ws.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *WorkspaceRegistry) GrantDevTrust(id WorkspaceID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws, ok := r.workspaces[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrWorkspaceNotFound, id)
	}
	ws.DevTrust = true
	ws.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *WorkspaceRegistry) RevokeDevTrust(id WorkspaceID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws, ok := r.workspaces[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrWorkspaceNotFound, id)
	}
	ws.DevTrust = false
	ws.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *WorkspaceRegistry) Disable(id WorkspaceID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws, ok := r.workspaces[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrWorkspaceNotFound, id)
	}
	ws.Status = WorkspaceStatusDisabled
	ws.DevTrust = false
	ws.WatchEnabled = false
	ws.AutoReload = false
	ws.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *WorkspaceRegistry) Remove(id WorkspaceID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ws, ok := r.workspaces[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrWorkspaceNotFound, id)
	}
	delete(r.workspaces, id)
	delete(r.byExtension, ws.ExtensionID)
	return nil
}
