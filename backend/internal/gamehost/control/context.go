package control

import (
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type TakeoverContext struct {
	RuntimeID     domain.RuntimeInstanceID
	TakenFromMode domain.ControlMode
	TakeoverEpoch uint64
	RecordedAt    time.Time
}

type TakeoverContextStore struct {
	mu       sync.RWMutex
	contexts map[domain.RuntimeInstanceID]*TakeoverContext
}

func NewTakeoverContextStore() *TakeoverContextStore {
	return &TakeoverContextStore{
		contexts: make(map[domain.RuntimeInstanceID]*TakeoverContext),
	}
}

func (s *TakeoverContextStore) Record(runtimeID domain.RuntimeInstanceID, fromMode domain.ControlMode, epoch uint64, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contexts[runtimeID] = &TakeoverContext{
		RuntimeID:     runtimeID,
		TakenFromMode: fromMode,
		TakeoverEpoch: epoch,
		RecordedAt:    now,
	}
}

func (s *TakeoverContextStore) Get(runtimeID domain.RuntimeInstanceID) (*TakeoverContext, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ctx, ok := s.contexts[runtimeID]
	if !ok {
		return nil, false
	}
	return &TakeoverContext{
		RuntimeID:     ctx.RuntimeID,
		TakenFromMode: ctx.TakenFromMode,
		TakeoverEpoch: ctx.TakeoverEpoch,
		RecordedAt:    ctx.RecordedAt,
	}, true
}

func (s *TakeoverContextStore) Remove(runtimeID domain.RuntimeInstanceID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.contexts, runtimeID)
}

func (s *TakeoverContextStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contexts = make(map[domain.RuntimeInstanceID]*TakeoverContext)
}
