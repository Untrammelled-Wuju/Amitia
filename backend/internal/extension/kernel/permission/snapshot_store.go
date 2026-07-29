package permission

import (
	"context"
	"fmt"
	"sync"
	"time"
)

var ErrPermissionSnapshotNotFound = fmt.Errorf("permission: snapshot not found")

type PermissionSnapshotStore interface {
	SaveSnapshot(ctx context.Context, snapshot PermissionSnapshot) error
	GetSnapshot(ctx context.Context, snapshotID string) (PermissionSnapshot, error)
	DeleteSnapshot(ctx context.Context, snapshotID string) error
	RevokeSnapshot(ctx context.Context, snapshotID string) error
	DeleteBySession(ctx context.Context, sessionID string) error
}

type MemoryPermissionSnapshotStore struct {
	mu        sync.RWMutex
	snapshots map[string]PermissionSnapshot
}

func NewMemoryPermissionSnapshotStore() *MemoryPermissionSnapshotStore {
	return &MemoryPermissionSnapshotStore{
		snapshots: make(map[string]PermissionSnapshot),
	}
}

func (s *MemoryPermissionSnapshotStore) SaveSnapshot(_ context.Context, snapshot PermissionSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[snapshot.SnapshotID] = snapshot
	return nil
}

func (s *MemoryPermissionSnapshotStore) GetSnapshot(_ context.Context, snapshotID string) (PermissionSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.snapshots[snapshotID]
	if !ok {
		return PermissionSnapshot{}, ErrPermissionSnapshotNotFound
	}
	return snap, nil
}

func (s *MemoryPermissionSnapshotStore) DeleteSnapshot(_ context.Context, snapshotID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.snapshots, snapshotID)
	return nil
}

func (s *MemoryPermissionSnapshotStore) RevokeSnapshot(_ context.Context, snapshotID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, ok := s.snapshots[snapshotID]
	if !ok {
		return ErrPermissionSnapshotNotFound
	}
	now := time.Now().UTC()
	snap.RevokedAt = &now
	s.snapshots[snapshotID] = snap
	return nil
}

func (s *MemoryPermissionSnapshotStore) DeleteBySession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, snap := range s.snapshots {
		if snap.SessionID == sessionID {
			delete(s.snapshots, id)
		}
	}
	return nil
}

var _ PermissionSnapshotStore = (*MemoryPermissionSnapshotStore)(nil)
