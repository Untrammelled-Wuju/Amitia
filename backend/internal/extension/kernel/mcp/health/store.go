package health

import "sync"

type InMemoryHealthStore struct {
	mu    sync.RWMutex
	snaps map[string]MCPHealthSnapshot
	gen   map[string]int64
}

func NewInMemoryHealthStore() *InMemoryHealthStore {
	return &InMemoryHealthStore{
		snaps: make(map[string]MCPHealthSnapshot),
		gen:   make(map[string]int64),
	}
}

func (s *InMemoryHealthStore) Load(serverID string) (MCPHealthSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.snaps[serverID]
	return snap, ok
}

func (s *InMemoryHealthStore) Save(snapshot MCPHealthSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snaps[snapshot.ServerID] = snapshot
}

func (s *InMemoryHealthStore) LoadGeneration(serverID string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.gen[serverID]
}

func (s *InMemoryHealthStore) IncrementGeneration(serverID string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gen[serverID]++
	return s.gen[serverID]
}
