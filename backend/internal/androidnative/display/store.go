package display

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/androidnative/virtualdisplay"
)

type DisplayStore struct {
	mu               sync.RWMutex
	globalGeneration uint64
	displays         map[int]*DisplayRecord
	classifier       *DisplayClassifier
}

func NewDisplayStore(classifier *DisplayClassifier) *DisplayStore {
	return &DisplayStore{
		displays:   make(map[int]*DisplayRecord),
		classifier: classifier,
	}
}

func (s *DisplayStore) GlobalGeneration() uint64 {
	return atomic.LoadUint64(&s.globalGeneration)
}

func (s *DisplayStore) bumpGlobal() {
	atomic.AddUint64(&s.globalGeneration, 1)
}

func (s *DisplayStore) Snapshot() DisplaySnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	displays := make([]DisplayInfo, 0, len(s.displays))
	var defaultID int
	for _, rec := range s.displays {
		displays = append(displays, rec.Info)
		if rec.Info.IsDefault {
			defaultID = rec.Info.DisplayID
		}
	}

	return DisplaySnapshot{
		Generation:       s.globalGeneration,
		DefaultDisplayID: defaultID,
		Displays:         displays,
		CapturedAt:       time.Now().UnixMilli(),
	}
}

func (s *DisplayStore) Get(displayID int) (*DisplayRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.displays[displayID]
	return rec, ok
}

func (s *DisplayStore) GetAll() map[int]DisplayInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[int]DisplayInfo, len(s.displays))
	for id, rec := range s.displays {
		result[id] = rec.Info
	}
	return result
}

func (s *DisplayStore) Put(info DisplayInfo) (isNew bool, prevGen uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	rec, exists := s.displays[info.DisplayID]
	if exists {
		prevGen = rec.Info.Generation
		identityChanged := rec.Info.Generation != info.Generation
		info.Type = string(s.classifier.Classify(info))
		rec.Info = info
		rec.LastSeenAt = now
		if identityChanged {
			rec.IdentityGeneration++
		}
		s.displays[info.DisplayID] = rec
		s.bumpGlobal()
		return false, prevGen
	}

	info.Type = string(s.classifier.Classify(info))
	s.displays[info.DisplayID] = &DisplayRecord{
		Info:               info,
		IdentityGeneration: info.Generation,
		FirstSeenAt:        now,
		LastSeenAt:         now,
	}
	s.bumpGlobal()
	return true, 0
}

func (s *DisplayStore) Remove(displayID int) (*DisplayRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.displays[displayID]
	if !ok {
		return nil, false
	}
	delete(s.displays, displayID)
	s.bumpGlobal()
	return rec, true
}

func (s *DisplayStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.displays)
}

func (s *DisplayStore) SetManagedVirtual(displayID int, ref *virtualdisplay.VirtualDisplayRef) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.classifier.SetManaged(displayID, ref)
	if rec, ok := s.displays[displayID]; ok {
		rec.ManagedVirtualRef = ref
		rec.Info.ManagedByAmitia = true
		rec.Info.VirtualRef = ref
		rec.Info.Type = string(DisplayTypeVirtualAmitia)
	}
	s.bumpGlobal()
}

func (s *DisplayStore) RemoveManagedVirtual(displayID int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.classifier.RemoveManaged(displayID)
	if rec, ok := s.displays[displayID]; ok {
		rec.ManagedVirtualRef = nil
		rec.Info.ManagedByAmitia = false
		rec.Info.VirtualRef = nil
		rec.Info.Type = string(s.classifier.Classify(rec.Info))
	}
	s.bumpGlobal()
}

func (s *DisplayStore) InvalidateAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, rec := range s.displays {
		rec.Info.IsValid = false
	}
	s.bumpGlobal()
}
