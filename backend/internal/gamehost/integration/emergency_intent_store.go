package integration

import (
	"context"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type InMemoryEmergencyIntentStore struct {
	mu       sync.RWMutex
	latched  map[domain.RuntimeInstanceID]string
}

func NewInMemoryEmergencyIntentStore() *InMemoryEmergencyIntentStore {
	return &InMemoryEmergencyIntentStore{
		latched: make(map[domain.RuntimeInstanceID]string),
	}
}

func (s *InMemoryEmergencyIntentStore) CommitEmergencyIntent(ctx context.Context, runtimeID domain.RuntimeInstanceID, operationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latched[runtimeID] = operationID
	return nil
}

func (s *InMemoryEmergencyIntentStore) IsEmergencyLatched(ctx context.Context, runtimeID domain.RuntimeInstanceID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.latched[runtimeID]
	return ok
}

func (s *InMemoryEmergencyIntentStore) GetEmergencyOperationID(ctx context.Context, runtimeID domain.RuntimeInstanceID) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	opID, ok := s.latched[runtimeID]
	return opID, ok
}

func (s *InMemoryEmergencyIntentStore) ClearEmergencyLatch(ctx context.Context, runtimeID domain.RuntimeInstanceID, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.latched, runtimeID)
	return nil
}

var _ control.EmergencyIntentStore = (*InMemoryEmergencyIntentStore)(nil)
