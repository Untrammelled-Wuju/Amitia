package browser

import (
	"sync"
	"time"
)

type tabRecord struct {
	info               BrowserTabInfo
	sessionID          BrowserSessionID
	browserContextID   BrowserContextID
	targetID           TargetID
	runtimeGeneration  uint64
	documentGeneration uint64
	mainFrameID        string
	createdAt          time.Time
	updatedAt          time.Time
}

type tabStore struct {
	mu   sync.RWMutex
	tabs map[BrowserTabID]*tabRecord
}

func newTabStore() *tabStore {
	return &tabStore{
		tabs: make(map[BrowserTabID]*tabRecord),
	}
}

func (s *tabStore) put(record *tabRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	record.updatedAt = now
	if record.createdAt.IsZero() {
		record.createdAt = now
	}
	s.tabs[record.info.TabID] = record
}

func (s *tabStore) get(tabID BrowserTabID) (*tabRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.tabs[tabID]
	return record, ok
}

func (s *tabStore) getBySession(sessionID BrowserSessionID, generation uint64) []*tabRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*tabRecord, 0)
	for _, record := range s.tabs {
		if record.sessionID != sessionID {
			continue
		}
		if record.runtimeGeneration != generation {
			continue
		}
		if record.info.State == TabStateClosed {
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

func (s *tabStore) countBySession(sessionID BrowserSessionID, generation uint64) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, record := range s.tabs {
		if record.sessionID != sessionID {
			continue
		}
		if record.runtimeGeneration != generation {
			continue
		}
		if record.info.State == TabStateClosing || record.info.State == TabStateClosed {
			continue
		}
		count++
	}
	return count
}

func (s *tabStore) countActive(generation uint64) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, record := range s.tabs {
		if record.runtimeGeneration != generation {
			continue
		}
		if record.info.State == TabStateClosing || record.info.State == TabStateClosed {
			continue
		}
		count++
	}
	return count
}

func (s *tabStore) bumpDocumentGeneration(tabID BrowserTabID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.tabs[tabID]
	if !ok {
		return
	}
	record.documentGeneration++
	record.updatedAt = time.Now()
}

func (s *tabStore) getDocumentGeneration(tabID BrowserTabID) (uint64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.tabs[tabID]
	if !ok {
		return 0, false
	}
	return record.documentGeneration, true
}

func (s *tabStore) transition(tabID BrowserTabID, from BrowserTabState, to BrowserTabState) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.tabs[tabID]
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

func (s *tabStore) updateTabInfo(tabID BrowserTabID, url, title string, active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.tabs[tabID]
	if !ok {
		return
	}
	if url != "" {
		record.info.URL = url
	}
	if title != "" {
		record.info.Title = title
	}
	record.info.Active = active
	record.updatedAt = time.Now()
}

func (s *tabStore) clearActive(sessionID BrowserSessionID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, record := range s.tabs {
		if record.sessionID == sessionID {
			record.info.Active = false
		}
	}
}

func (s *tabStore) remove(tabID BrowserTabID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tabs, tabID)
}

func (s *tabStore) clearGeneration(generation uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, record := range s.tabs {
		if record.runtimeGeneration == generation {
			delete(s.tabs, id)
		}
	}
}

func (s *tabStore) closeAllForSession(sessionID BrowserSessionID, generation uint64) []*tabRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*tabRecord, 0)
	for id, record := range s.tabs {
		if record.sessionID != sessionID {
			continue
		}
		if record.runtimeGeneration != generation {
			continue
		}
		if record.info.State == TabStateClosed {
			continue
		}
		result = append(result, record)
		delete(s.tabs, id)
	}
	return result
}

func (s *tabStore) count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tabs)
}
