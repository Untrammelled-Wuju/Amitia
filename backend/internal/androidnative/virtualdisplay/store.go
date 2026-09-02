package virtualdisplay

import (
	"strings"
	"sync"
	"time"
)

// Store tracks multiple live VirtualDisplay instances. Get() remains as a
// backwards-compatible view of the most recently created active display.
type Store struct {
	mu      sync.RWMutex
	records map[VirtualDisplayRef]*VirtualDisplayRecord
	counter uint64
}

func (s *Store) ensureLocked() {
	if s.records == nil {
		s.records = make(map[VirtualDisplayRef]*VirtualDisplayRecord)
	}
}

func (s *Store) Get() *VirtualDisplayRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return latestRecord(s.records)
}

func (s *Store) GetByRef(ref VirtualDisplayRef) *VirtualDisplayRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if ref.IsEmpty() {
		return latestRecord(s.records)
	}
	rec := s.records[ref]
	if rec == nil {
		return nil
	}
	cp := *rec
	return &cp
}

func (s *Store) List() []VirtualDisplayRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]VirtualDisplayRecord, 0, len(s.records))
	for _, rec := range s.records {
		if rec == nil || !rec.State.IsActive() {
			continue
		}
		out = append(out, *rec)
	}
	return out
}

func (s *Store) HasActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, rec := range s.records {
		if rec != nil && rec.State.IsActive() {
			return true
		}
	}
	return false
}

func (s *Store) Insert(rec *VirtualDisplayRecord) *VirtualDisplayRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureLocked()
	s.counter++
	if rec.Ref.IsEmpty() {
		rec.Ref = newVirtualDisplayRef(s.counter)
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now()
	}
	if rec.Generation == 0 {
		rec.Generation = s.counter
	}
	cp := *rec
	s.records[rec.Ref] = &cp
	return cloneRecord(&cp)
}

func (s *Store) Update(ref VirtualDisplayRef, fn func(*VirtualDisplayRecord) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	resolved := ref
	if resolved.IsEmpty() {
		latest := latestRecordRef(s.records)
		resolved = latest
	}
	rec := s.records[resolved]
	if rec == nil {
		return NewError(ErrVirtualDisplayNotFound, "virtual display not found")
	}
	return fn(rec)
}

func (s *Store) Remove(ref VirtualDisplayRef) (*VirtualDisplayRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resolved := ref
	if resolved.IsEmpty() {
		resolved = latestRecordRef(s.records)
	}
	rec := s.records[resolved]
	if rec == nil {
		return nil, NewError(ErrVirtualDisplayNotFound, "virtual display not found")
	}
	delete(s.records, resolved)
	return cloneRecord(rec), nil
}

func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = make(map[VirtualDisplayRef]*VirtualDisplayRecord)
}

func (s *Store) BumpGeneration(ref VirtualDisplayRef) error {
	return s.Update(ref, func(rec *VirtualDisplayRecord) error {
		rec.Generation++
		return nil
	})
}

func latestRecord(records map[VirtualDisplayRef]*VirtualDisplayRecord) *VirtualDisplayRecord {
	ref := latestRecordRef(records)
	if ref.IsEmpty() {
		return nil
	}
	return cloneRecord(records[ref])
}

func latestRecordRef(records map[VirtualDisplayRef]*VirtualDisplayRecord) VirtualDisplayRef {
	var chosen VirtualDisplayRef
	var generation uint64
	for ref, rec := range records {
		if rec == nil || !rec.State.IsActive() {
			continue
		}
		if chosen.IsEmpty() || rec.Generation > generation {
			chosen = ref
			generation = rec.Generation
		}
	}
	return chosen
}

func cloneRecord(rec *VirtualDisplayRecord) *VirtualDisplayRecord {
	if rec == nil {
		return nil
	}
	cp := *rec
	return &cp
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
