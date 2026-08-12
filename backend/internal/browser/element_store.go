package browser

import (
	"container/list"
	"sync"
	"time"

	"github.com/google/uuid"
)

type elementRecord struct {
	stableID          string
	sessionID         BrowserSessionID
	tabID             BrowserTabID
	runtimeGeneration  uint64
	documentGeneration uint64
	targetID          TargetID
	cdpSessionID      CDPSessionID
	frameID           FrameID
	backendNodeID     BackendNodeID
	selector          string
	createdAt         time.Time
	lastResolvedAt    time.Time
}

type elementStore struct {
	mu         sync.RWMutex
	refs       map[string]*elementRecord
	lru        *list.List
	lruMap     map[string]*list.Element
	maxPerTab  int
}

func newElementStore(maxPerTab int) *elementStore {
	if maxPerTab <= 0 {
		maxPerTab = 1024
	}
	return &elementStore{
		refs:      make(map[string]*elementRecord),
		lru:       list.New(),
		lruMap:    make(map[string]*list.Element),
		maxPerTab: maxPerTab,
	}
}

func (s *elementStore) generateStableID() string {
	return "be_" + uuid.New().String()
}

func (s *elementStore) put(record *elementRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.refs[record.stableID]; ok {
		existing.lastResolvedAt = time.Now()
		if el, ok := s.lruMap[record.stableID]; ok {
			s.lru.MoveToFront(el)
		}
		_ = existing
		return
	}

	tabCount := s.countLocked(record.tabID)
	if tabCount >= s.maxPerTab {
		s.evictOldestForTabLocked(record.tabID)
	}

	now := time.Now()
	record.createdAt = now
	record.lastResolvedAt = now
	s.refs[record.stableID] = record
	el := s.lru.PushFront(record.stableID)
	s.lruMap[record.stableID] = el
}

func (s *elementStore) get(stableID string) (*elementRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.refs[stableID]
	return record, ok
}

func (s *elementStore) touch(stableID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if el, ok := s.lruMap[stableID]; ok {
		s.lru.MoveToFront(el)
	}
	if record, ok := s.refs[stableID]; ok {
		record.lastResolvedAt = time.Now()
	}
}

func (s *elementStore) remove(stableID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if el, ok := s.lruMap[stableID]; ok {
		s.lru.Remove(el)
		delete(s.lruMap, stableID)
	}
	delete(s.refs, stableID)
}

func (s *elementStore) clearForTab(tabID BrowserTabID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, record := range s.refs {
		if record.tabID == tabID {
			if el, ok := s.lruMap[id]; ok {
				s.lru.Remove(el)
				delete(s.lruMap, id)
			}
			delete(s.refs, id)
		}
	}
}

func (s *elementStore) clearForSession(sessionID BrowserSessionID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, record := range s.refs {
		if record.sessionID == sessionID {
			if el, ok := s.lruMap[id]; ok {
				s.lru.Remove(el)
				delete(s.lruMap, id)
			}
			delete(s.refs, id)
		}
	}
}

func (s *elementStore) clearForDocumentGeneration(tabID BrowserTabID, documentGeneration uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, record := range s.refs {
		if record.tabID == tabID && record.documentGeneration != documentGeneration {
			if el, ok := s.lruMap[id]; ok {
				s.lru.Remove(el)
				delete(s.lruMap, id)
			}
			delete(s.refs, id)
		}
	}
}

func (s *elementStore) clearAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refs = make(map[string]*elementRecord)
	s.lru = list.New()
	s.lruMap = make(map[string]*list.Element)
}

func (s *elementStore) count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.refs)
}

func (s *elementStore) countForTab(tabID BrowserTabID) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.countLocked(tabID)
}

func (s *elementStore) countLocked(tabID BrowserTabID) int {
	count := 0
	for _, record := range s.refs {
		if record.tabID == tabID {
			count++
		}
	}
	return count
}

func (s *elementStore) evictOldestForTabLocked(tabID BrowserTabID) {
	for el := s.lru.Back(); el != nil; el = el.Prev() {
		id := el.Value.(string)
		if record, ok := s.refs[id]; ok && record.tabID == tabID {
			s.lru.Remove(el)
			delete(s.lruMap, id)
			delete(s.refs, id)
			return
		}
	}
}
