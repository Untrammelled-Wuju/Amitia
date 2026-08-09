package rpc

import (
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type IdempotencyIndex struct {
	mu      sync.Mutex
	entries map[RequestIdFingerprintKey]*IdempotencyEntry
}

type RequestIdFingerprintKey struct {
	RequestKey RequestKey
	Fingerprint RequestFingerprint
}

type IdempotencyEntry struct {
	State     RequestState
	CreatedAt time.Time
}

func NewIdempotencyIndex() *IdempotencyIndex {
	return &IdempotencyIndex{
		entries: make(map[RequestIdFingerprintKey]*IdempotencyEntry),
	}
}

func (idx *IdempotencyIndex) CheckOrAdd(key RequestKey, fp RequestFingerprint, state RequestState) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	combined := RequestIdFingerprintKey{key, fp}
	if _, exists := idx.entries[combined]; exists {
		return NewRPCErrorWithCause(
			"duplicate_request",
			domain.ErrInvalidArgument,
			"duplicate request with same fingerprint",
			nil,
		)
	}

	idx.entries[combined] = &IdempotencyEntry{
		State:     state,
		CreatedAt: time.Now().UTC(),
	}
	return nil
}

func (idx *IdempotencyIndex) Exists(key RequestKey, fp RequestFingerprint) RequestState {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if entry, ok := idx.entries[RequestIdFingerprintKey{key, fp}]; ok {
		_ = entry
		return entry.State
	}
	return ""
}

func (idx *IdempotencyIndex) UpdateState(key RequestKey, fp RequestFingerprint, state RequestState) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if entry, ok := idx.entries[RequestIdFingerprintKey{key, fp}]; ok {
		entry.State = state
	}
}

func (idx *IdempotencyIndex) Remove(key RequestKey, fp RequestFingerprint) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	delete(idx.entries, RequestIdFingerprintKey{key, fp})
}

func (idx *IdempotencyIndex) CheckIDReuse(key RequestKey, fp RequestFingerprint) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	combined := RequestIdFingerprintKey{key, fp}
	if _, exists := idx.entries[combined]; exists {
		return NewRPCErrorWithCause(
			"request_id_reused_with_different_payload",
			domain.ErrInvalidArgument,
			"request ID reused with different fingerprint",
			nil,
		)
	}
	return nil
}
