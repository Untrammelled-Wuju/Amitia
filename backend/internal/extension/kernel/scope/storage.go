package scope

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

type ScopeStore interface {
	SaveBinding(ctx context.Context, binding ScopeBinding) error
	GetBinding(ctx context.Context, bindingID string) (ScopeBinding, error)
	DeleteBinding(ctx context.Context, bindingID string) error
	ListBindings(ctx context.Context, filter ScopeBindingFilter) ([]ScopeBinding, error)
	SaveSnapshot(ctx context.Context, snapshot ScopeSnapshot) error
	GetSnapshot(ctx context.Context, snapshotID string) (ScopeSnapshot, error)
	DeleteSnapshot(ctx context.Context, snapshotID string) error
	DeleteSnapshotsBySession(ctx context.Context, sessionID string) error
}

type MemoryScopeStore struct {
	snapshotMu sync.RWMutex
	bindings   map[string]ScopeBinding
	snapshots  map[string]ScopeSnapshot
}

func NewMemoryScopeStore() *MemoryScopeStore {
	return &MemoryScopeStore{
		bindings:  make(map[string]ScopeBinding),
		snapshots: make(map[string]ScopeSnapshot),
	}
}

func (s *MemoryScopeStore) SaveBinding(ctx context.Context, binding ScopeBinding) error {
	s.bindings[binding.BindingID] = binding
	return nil
}

func (s *MemoryScopeStore) GetBinding(ctx context.Context, bindingID string) (ScopeBinding, error) {
	if b, ok := s.bindings[bindingID]; ok {
		return b, nil
	}
	return ScopeBinding{}, ErrBindingNotFound
}

func (s *MemoryScopeStore) DeleteBinding(ctx context.Context, bindingID string) error {
	delete(s.bindings, bindingID)
	return nil
}

func (s *MemoryScopeStore) ListBindings(ctx context.Context, filter ScopeBindingFilter) ([]ScopeBinding, error) {
	result := make([]ScopeBinding, 0)
	for _, b := range s.bindings {
		if filter.SubjectType != "" && b.SubjectType != filter.SubjectType {
			continue
		}
		if filter.SubjectID != "" && b.SubjectID != filter.SubjectID {
			continue
		}
		if filter.ScopeType != "" && b.Scope.Type != filter.ScopeType {
			continue
		}
		if filter.State != "" && b.State != filter.State {
			continue
		}
		result = append(result, b)
	}
	return result, nil
}

func (s *MemoryScopeStore) SaveSnapshot(ctx context.Context, snapshot ScopeSnapshot) error {
	if len(snapshot.OwnedExecutionScope) > 0 && (!json.Valid(snapshot.OwnedExecutionScope) || len(snapshot.OwnedExecutionScope) > 64<<10) {
		return fmt.Errorf("设备执行授权快照无效或超过上限")
	}
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	if previous, exists := s.snapshots[snapshot.SnapshotID]; exists && len(previous.OwnedExecutionScope) > 0 {
		oldJSON, oldErr := json.Marshal(previous)
		newJSON, newErr := json.Marshal(snapshot)
		if oldErr != nil || newErr != nil || !bytes.Equal(oldJSON, newJSON) {
			return fmt.Errorf("已保存的设备执行授权快照不能修改或降级")
		}
	}
	s.snapshots[snapshot.SnapshotID] = cloneScopeSnapshot(snapshot)
	return nil
}

func (s *MemoryScopeStore) GetSnapshot(ctx context.Context, snapshotID string) (ScopeSnapshot, error) {
	s.snapshotMu.RLock()
	defer s.snapshotMu.RUnlock()
	if snap, ok := s.snapshots[snapshotID]; ok {
		return cloneScopeSnapshot(snap), nil
	}
	return ScopeSnapshot{}, ErrSnapshotNotFound
}

func (s *MemoryScopeStore) DeleteSnapshot(_ context.Context, snapshotID string) error {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	delete(s.snapshots, snapshotID)
	return nil
}

func cloneScopeSnapshot(snapshot ScopeSnapshot) ScopeSnapshot {
	snapshot.OwnedExecutionScope = append(json.RawMessage(nil), snapshot.OwnedExecutionScope...)
	snapshot.ResolvedScopes = append([]ScopeRef(nil), snapshot.ResolvedScopes...)
	if snapshot.ExpiresAt != nil {
		expires := *snapshot.ExpiresAt
		snapshot.ExpiresAt = &expires
	}
	return snapshot
}

func (s *MemoryScopeStore) DeleteSnapshotsBySession(_ context.Context, _ string) error {
	return nil
}
