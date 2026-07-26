package scope

import (
	"context"
)

type ScopeStore interface {
	SaveBinding(ctx context.Context, binding ScopeBinding) error
	GetBinding(ctx context.Context, bindingID string) (ScopeBinding, error)
	DeleteBinding(ctx context.Context, bindingID string) error
	ListBindings(ctx context.Context, filter ScopeBindingFilter) ([]ScopeBinding, error)
	SaveSnapshot(ctx context.Context, snapshot ScopeSnapshot) error
	GetSnapshot(ctx context.Context, snapshotID string) (ScopeSnapshot, error)
}

type MemoryScopeStore struct {
	bindings  map[string]ScopeBinding
	snapshots map[string]ScopeSnapshot
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
	s.snapshots[snapshot.SnapshotID] = snapshot
	return nil
}

func (s *MemoryScopeStore) GetSnapshot(ctx context.Context, snapshotID string) (ScopeSnapshot, error) {
	if snap, ok := s.snapshots[snapshotID]; ok {
		return snap, nil
	}
	return ScopeSnapshot{}, ErrSnapshotNotFound
}
