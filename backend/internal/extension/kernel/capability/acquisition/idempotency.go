package acquisition

import (
	"errors"
	"sync"
	"sync/atomic"
)

// IdempotencyStore tracks acquisition keys to ensure that duplicate acquisition
// requests for the same key do not result in duplicate installations. Each key is
// reserved before execution begins and marked resolved (or released) when the
// acquisition completes.
type IdempotencyStore struct {
	mu      sync.RWMutex
	keys    map[string]*IdempotencyEntry
	pending atomic.Int64
}

// IdempotencyEntry represents the lifecycle of a single acquisition key.
type IdempotencyEntry struct {
	Key           string           `json:"key"`
	State         AcquisitionState `json:"state"`
	CandidateID   string           `json:"candidateId,omitempty"`
	Error         string           `json:"error,omitempty"`
	Attempted     bool             `json:"attempted"`
	Resolved      bool             `json:"resolved"`
	TransactionID string           `json:"transactionId,omitempty"`
}

// NewIdempotencyStore returns an empty IdempotencyStore ready for use.
func NewIdempotencyStore() *IdempotencyStore {
	return &IdempotencyStore{
		keys: make(map[string]*IdempotencyEntry),
	}
}

// Reserve attempts to reserve the entry for the given key.
//
//   - If an entry already exists and is Resolved, the request is considered a
//     duplicate. Reserve returns false together with the existing entry.
//   - If an entry already exists but is not Resolved (still in-progress), the
//     request must wait. Reserve returns false together with the in-progress entry.
//   - If no entry exists, a new one is created and Reserve returns true together
//     with the new entry.
func (s *IdempotencyStore) Reserve(key string) (bool, *IdempotencyEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.keys[key]
	if exists {
		if entry.Resolved {
			return false, entry
		}
		// In-progress; caller should wait.
		return false, entry
	}

	entry = &IdempotencyEntry{
		Key:       key,
		State:     StatePlanned,
		Attempted: false,
		Resolved:  false,
	}
	s.keys[key] = entry
	s.pending.Add(1)
	return true, entry
}

// Release marks the entry as completed with final state information. If err is
// non-nil the entry is recorded as failed. The pending counter is decremented
// and the entry is marked Resolved.
func (s *IdempotencyStore) Release(key string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.keys[key]
	if !exists {
		return
	}

	entry.Resolved = true
	entry.Attempted = true

	if err != nil {
		entry.Error = err.Error()
		entry.State = StateFailed
	}

	s.pending.Add(-1)
}

// MarkResolved marks the entry as resolved with the given transaction ID, state,
// and optional error message. Unlike Release it does not decrement the pending
// counter, allowing the caller to control the lifecycle independently.
func (s *IdempotencyStore) MarkResolved(key string, txnID string, state AcquisitionState, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.keys[key]
	if !exists {
		return
	}

	entry.TransactionID = txnID
	entry.State = state
	entry.Error = errMsg
	entry.Attempted = true
	entry.Resolved = true
}

// GetPending returns the number of keys currently in-flight (reserved but not
// yet resolved).
func (s *IdempotencyStore) GetPending() int64 {
	return s.pending.Load()
}

// ErrDuplicateAcquisition is returned when an acquisition request arrives for a
// key that has already been resolved.
var ErrDuplicateAcquisition = errors.New("duplicate acquisition: key already resolved")
