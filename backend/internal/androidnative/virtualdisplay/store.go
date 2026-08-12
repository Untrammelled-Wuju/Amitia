package virtualdisplay

import (
	"strings"
	"sync"
	"time"
)

type Store struct {
	mu      sync.RWMutex
	active  *VirtualDisplayRecord
	counter uint64
}

func (s *Store) Get() *VirtualDisplayRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return nil
	}
	cp := *s.active
	return &cp
}

func (s *Store) HasActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active != nil && s.active.State.IsActive()
}

func (s *Store) Insert(rec *VirtualDisplayRecord) *VirtualDisplayRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counter++
	if rec.Ref.IsEmpty() {
		rec.Ref = newVirtualDisplayRef(s.counter)
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now()
	}
	rec.Generation = s.counter
	cp := *rec
	s.active = &cp
	return s.active
}

func (s *Store) Update(ref VirtualDisplayRef, fn func(*VirtualDisplayRecord) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return NewError(ErrVirtualDisplayNotFound, "no active display")
	}
	if s.active.Ref != ref {
		return NewError(ErrVirtualDisplayIdMismatch, "reference mismatch")
	}
	return fn(s.active)
}

func (s *Store) Remove(ref VirtualDisplayRef) (*VirtualDisplayRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return nil, NewError(ErrVirtualDisplayNotFound, "no active display")
	}
	if s.active.Ref != ref {
		return nil, NewError(ErrVirtualDisplayIdMismatch, "reference mismatch")
	}
	removed := s.active
	s.active = nil
	return removed, nil
}

func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = nil
}

func (s *Store) BumpGeneration(ref VirtualDisplayRef) error {
	return s.Update(ref, func(rec *VirtualDisplayRecord) error {
		rec.Generation++
		return nil
	})
}

func newVirtualDisplayRef(counter uint64) VirtualDisplayRef {
	var b strings.Builder
	b.WriteString("vd_")
	b.WriteString(uintToOpaque(counter))
	return VirtualDisplayRef(b.String())
}

func uintToOpaque(n uint64) string {
	if n == 0 {
		return "0"
	}
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	var buf [13]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = alphabet[n%36]
		n /= 36
	}
	return string(buf[i:])
}
