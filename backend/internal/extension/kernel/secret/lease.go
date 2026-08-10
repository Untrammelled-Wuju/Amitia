package secret

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type LeaseID string

type Lease struct {
	ID                   LeaseID
	Ref                  SecretRef
	Purpose              string
	InvocationID         string
	RuntimeInstanceID    string
	UserID               string
	CharacterID          string
	ConversationID       string
	ExtensionID          string
	ModuleID             string
	Generation           int64
	PermissionSnapshotID string
	ScopeSnapshotID      string
	IssuedAt             time.Time
	ExpiresAt            time.Time
	MaxUses              int
	UsedCount            int
	Revoked              bool
}

func (l Lease) CanUse() bool {
	if l.Revoked {
		return false
	}
	if !l.ExpiresAt.IsZero() && time.Now().After(l.ExpiresAt) {
		return false
	}
	if l.MaxUses > 0 && l.UsedCount >= l.MaxUses {
		return false
	}
	return true
}

func (l Lease) Clone() Lease {
	return l
}

type leaseState struct {
	descriptor Lease
	value      []byte
}

func (s *leaseState) zeroize() {
	if s.value != nil {
		for i := range s.value {
			s.value[i] = 0
		}
		s.value = nil
	}
}

func (s *leaseState) hasValue() bool {
	return s.value != nil
}

type LeaseRequest struct {
	Ref                 SecretRef
	Purpose             string
	InvocationID        string
	RuntimeInstanceID   string
	UserID              string
	CharacterID         string
	ConversationID      string
	ExtensionID         string
	ModuleID            string
	Generation          int64
	PermissionSnapshotID string
	ScopeSnapshotID     string
	TTL                 time.Duration
	MaxUses             int
}

type LeaseUseContext struct {
	InvocationID      string
	RuntimeInstanceID string
	ExtensionID       string
	ModuleID          string
	Generation        int64
}

type LeaseStore struct {
	mu     sync.Mutex
	states map[LeaseID]*leaseState
	now    func() time.Time
}

func NewLeaseStore() *LeaseStore {
	return &LeaseStore{
		states: make(map[LeaseID]*leaseState),
		now:    time.Now,
	}
}

func (s *LeaseStore) Put(id LeaseID, state *leaseState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[id] = state
}

func (s *LeaseStore) Get(id LeaseID) (Lease, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[id]
	if !ok || st == nil {
		return Lease{}, false
	}
	return st.descriptor.Clone(), true
}

func (s *LeaseStore) GetState(id LeaseID) (*leaseState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[id]
	return st, ok
}

type consumeResult struct {
	value     []byte
	revoked   bool
	exhausted bool
	expired   bool
}

func (s *LeaseStore) Consume(id LeaseID) (*consumeResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[id]
	if !ok {
		return nil, false
	}
	if st.descriptor.Revoked {
		return &consumeResult{revoked: true}, true
	}
	now := time.Now()
	if !st.descriptor.ExpiresAt.IsZero() && now.After(st.descriptor.ExpiresAt) {
		return &consumeResult{expired: true}, true
	}
	if st.descriptor.MaxUses > 0 && st.descriptor.UsedCount >= st.descriptor.MaxUses {
		return &consumeResult{exhausted: true}, true
	}
	st.descriptor.UsedCount++
	if st.value != nil {
		valueCopy := make([]byte, len(st.value))
		copy(valueCopy, st.value)
		out := &consumeResult{value: valueCopy}
		if st.descriptor.MaxUses > 0 && st.descriptor.UsedCount >= st.descriptor.MaxUses {
			out.exhausted = true
		}
		return out, true
	}
	return &consumeResult{}, true
}

func (s *LeaseStore) MarkRevoked(id LeaseID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[id]
	if !ok || st.descriptor.Revoked {
		return false
	}
	st.descriptor.Revoked = true
	st.zeroize()
	return true
}

func (s *LeaseStore) Delete(id LeaseID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.states[id]; ok {
		st.zeroize()
		delete(s.states, id)
	}
}

func (s *LeaseStore) List() []Lease {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Lease, 0, len(s.states))
	for _, st := range s.states {
		out = append(out, st.descriptor.Clone())
	}
	return out
}

func (s *LeaseStore) ListByInvocation(invocationID string) []Lease {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Lease, 0)
	for _, st := range s.states {
		if st.descriptor.InvocationID == invocationID {
			out = append(out, st.descriptor.Clone())
		}
	}
	return out
}

func (s *LeaseStore) ListByRuntimeInstance(instanceID string) []Lease {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Lease, 0)
	for _, st := range s.states {
		if st.descriptor.RuntimeInstanceID == instanceID {
			out = append(out, st.descriptor.Clone())
		}
	}
	return out
}

func (s *LeaseStore) ListByExtensionGeneration(extensionID string, generation int64) []Lease {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Lease, 0)
	for _, st := range s.states {
		if st.descriptor.ExtensionID == extensionID && st.descriptor.Generation == generation {
			out = append(out, st.descriptor.Clone())
		}
	}
	return out
}

func (s *LeaseStore) DeleteByInvocation(invocationID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for id, st := range s.states {
		if st.descriptor.InvocationID == invocationID {
			st.zeroize()
			delete(s.states, id)
			count++
		}
	}
	return count
}

func (s *LeaseStore) DeleteByRuntimeInstance(instanceID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for id, st := range s.states {
		if st.descriptor.RuntimeInstanceID == instanceID {
			st.zeroize()
			delete(s.states, id)
			count++
		}
	}
	return count
}

func (s *LeaseStore) DeleteByExtensionGeneration(extensionID string, generation int64) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for id, st := range s.states {
		if st.descriptor.ExtensionID == extensionID && st.descriptor.Generation == generation {
			st.zeroize()
			delete(s.states, id)
			count++
		}
	}
	return count
}

func (s *LeaseStore) CleanupExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	count := 0
	for id, st := range s.states {
		if st.descriptor.Revoked {
			delete(s.states, id)
			count++
			continue
		}
		if !st.descriptor.ExpiresAt.IsZero() && now.After(st.descriptor.ExpiresAt) {
			st.zeroize()
			delete(s.states, id)
			count++
		}
	}
	return count
}

func generateLeaseID() (LeaseID, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return LeaseID("lease:" + hex.EncodeToString(b)), nil
}
