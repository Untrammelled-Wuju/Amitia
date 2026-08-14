package browser

import (
	"net/url"
	"strings"
	"sync"
	"time"
)

type RecoveryStore struct {
	mu       sync.RWMutex
	sessions map[BrowserSessionID]*sessionRecoveryDescriptor
	tabs     map[BrowserTabID]*tabRecoveryState
	maxTabs  int
}

func NewRecoveryStore(maxTabs int) *RecoveryStore {
	return &RecoveryStore{
		sessions: make(map[BrowserSessionID]*sessionRecoveryDescriptor),
		tabs:     make(map[BrowserTabID]*tabRecoveryState),
		maxTabs:  maxTabs,
	}
}

func (s *RecoveryStore) SaveSession(desc *sessionRecoveryDescriptor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[desc.sessionID] = desc
}

func (s *RecoveryStore) RemoveSession(sessionID BrowserSessionID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

func (s *RecoveryStore) GetRecoverableSessions() []*sessionRecoveryDescriptor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*sessionRecoveryDescriptor
	for _, desc := range s.sessions {
		if desc.recoverable {
			result = append(result, desc)
		}
	}
	return result
}

func (s *RecoveryStore) SaveTab(state *tabRecoveryState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tabs[state.tabID] = state
}

func (s *RecoveryStore) RemoveTab(tabID BrowserTabID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tabs, tabID)
}

func (s *RecoveryStore) GetRecoverableTabs() []*tabRecoveryState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*tabRecoveryState
	for _, state := range s.tabs {
		if state.recoverable {
			result = append(result, state)
		}
	}
	return result
}

func (s *RecoveryStore) GetTabState(tabID BrowserTabID) (*tabRecoveryState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.tabs[tabID]
	return state, ok
}

func (s *RecoveryStore) GetSession(sessionID BrowserSessionID) (*sessionRecoveryDescriptor, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	desc, ok := s.sessions[sessionID]
	return desc, ok
}

func (s *RecoveryStore) MarkSessionRecoverable(sessionID BrowserSessionID, recoverable bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if desc, ok := s.sessions[sessionID]; ok {
		desc.recoverable = recoverable
	}
}

func (s *RecoveryStore) MarkTabRecoverable(tabID BrowserTabID, recoverable bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state, ok := s.tabs[tabID]; ok {
		state.recoverable = recoverable
	}
}

func (s *RecoveryStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions = make(map[BrowserSessionID]*sessionRecoveryDescriptor)
	s.tabs = make(map[BrowserTabID]*tabRecoveryState)
}

func (s *RecoveryStore) InvalidateAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, desc := range s.sessions {
		desc.recoverable = false
	}
	for _, state := range s.tabs {
		state.recoverable = false
	}
}

type CrashDetector struct {
	mu         sync.RWMutex
	tabCrashes map[BrowserTabID]time.Time
	runtimeOK  bool
}

func NewCrashDetector() *CrashDetector {
	return &CrashDetector{
		tabCrashes: make(map[BrowserTabID]time.Time),
		runtimeOK:  true,
	}
}

func (d *CrashDetector) MarkTabCrashed(tabID BrowserTabID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tabCrashes[tabID] = time.Now()
}

func (d *CrashDetector) IsTabCrashed(tabID BrowserTabID) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, ok := d.tabCrashes[tabID]
	return ok
}

func (d *CrashDetector) MarkRuntimeFailure() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.runtimeOK = false
}

func (d *CrashDetector) IsRuntimeOK() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.runtimeOK
}

func (d *CrashDetector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tabCrashes = make(map[BrowserTabID]time.Time)
	d.runtimeOK = true
}

func IsSafeRecoverableURL(rawURL string, allowedSchemes []string) bool {
	if rawURL == "" {
		return false
	}

	if rawURL == "about:blank" {
		return true
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	scheme := strings.ToLower(parsed.Scheme)

	hasSafeParam := false
	if parsed.RawQuery != "" {
		q := parsed.Query()
		for key := range q {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "token") ||
				strings.Contains(lower, "code") ||
				strings.Contains(lower, "secret") ||
				strings.Contains(lower, "password") ||
				strings.Contains(lower, "key") {
				hasSafeParam = true
				break
			}
		}
	}

	if hasSafeParam && (scheme == "http" || scheme == "https") {
		return false
	}

	if parsed.Fragment != "" && (scheme == "http" || scheme == "https") {
		return false
	}

	for _, allowed := range allowedSchemes {
		if scheme == strings.ToLower(allowed) {
			return true
		}
	}

	return false
}
