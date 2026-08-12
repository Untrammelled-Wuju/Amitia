package browser

import (
	"sync"
	"time"
)

type sessionRecord struct {
	info              BrowserSessionInfo
	contextID         BrowserContextID
	runtimeGeneration uint64
	createdAt         time.Time
	updatedAt         time.Time
}

type sessionStore struct {
	mu       sync.RWMutex
	sessions map[BrowserSessionID]*sessionRecord
}

func newSessionStore() *sessionStore {
	return &sessionStore{
		sessions: make(map[BrowserSessionID]*sessionRecord),
	}
}

func (s *sessionStore) put(record *sessionRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	record.updatedAt = now
	if record.createdAt.IsZero() {
		record.createdAt = now
	}
	s.sessions[record.info.SessionID] = record
}

func (s *sessionStore) get(sessionID BrowserSessionID) (*sessionRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.sessions[sessionID]
	return record, ok
}

func (s *sessionStore) listActive(generation uint64) []*sessionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*sessionRecord, 0, len(s.sessions))
	for _, record := range s.sessions {
		if record.runtimeGeneration != generation {
			continue
		}
		if record.info.State == SessionStateClosed {
			continue
		}
		result = append(result, record)
	}
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].createdAt.After(result[j].createdAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

func (s *sessionStore) countActiveCreating(generation uint64) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, record := range s.sessions {
		if record.runtimeGeneration != generation {
			continue
		}
		if record.info.State == SessionStateReady || record.info.State == SessionStateCreated {
			count++
		}
	}
	return count
}

func (s *sessionStore) transition(sessionID BrowserSessionID, from BrowserSessionState, to BrowserSessionState) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.sessions[sessionID]
	if !ok {
		return false
	}
	if record.info.State != from {
		return false
	}
	record.info.State = to
	record.updatedAt = time.Now()
	return true
}

func (s *sessionStore) remove(sessionID BrowserSessionID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

func (s *sessionStore) clearGeneration(generation uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, record := range s.sessions {
		if record.runtimeGeneration == generation {
			delete(s.sessions, id)
		}
	}
}

func (s *sessionStore) count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

func (s *sessionStore) listActiveGenerationLocked(generation uint64) []*sessionRecord {
	result := make([]*sessionRecord, 0)
	for _, record := range s.sessions {
		if record.runtimeGeneration == generation {
			result = append(result, record)
		}
	}
	return result
}
