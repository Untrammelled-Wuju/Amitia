package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/capabilities"
)

var ErrCapabilityExposureRevision = errors.New("设备能力开放状态已变化，请刷新后重试")

type CapabilityExposureItem struct {
	capabilities.Definition
	Enabled bool `json:"enabled"`
}

type CapabilityExposureState struct {
	Revision     int64                    `json:"revision"`
	UpdatedAt    time.Time                `json:"updatedAt"`
	Capabilities []CapabilityExposureItem `json:"capabilities"`
	EnabledIDs   []string                 `json:"enabledCapabilities"`
}

type storedCapabilityExposure struct {
	Revision  int64           `json:"revision"`
	UpdatedAt time.Time       `json:"updatedAt"`
	Enabled   map[string]bool `json:"enabled"`
}

type CapabilityExposureStore struct {
	mu          sync.Mutex
	filePath    string
	subscribers map[chan CapabilityExposureState]struct{}
}

func NewCapabilityExposureStore(dataDir string) *CapabilityExposureStore {
	return &CapabilityExposureStore{
		filePath:    filepath.Join(dataDir, "device-mesh", "capability-exposure.json"),
		subscribers: make(map[chan CapabilityExposureState]struct{}),
	}
}

func defaultCapabilityExposure() storedCapabilityExposure {
	enabled := make(map[string]bool, len(capabilities.IDs()))
	for _, id := range capabilities.IDs() {
		enabled[id] = true
	}
	return storedCapabilityExposure{Revision: 1, UpdatedAt: time.Now().UTC(), Enabled: enabled}
}

func (s *CapabilityExposureStore) loadLocked() (storedCapabilityExposure, error) {
	data, err := os.ReadFile(s.filePath)
	if errors.Is(err, os.ErrNotExist) {
		return defaultCapabilityExposure(), nil
	}
	if err != nil {
		return storedCapabilityExposure{}, err
	}
	var stored storedCapabilityExposure
	if err := json.Unmarshal(data, &stored); err != nil {
		return storedCapabilityExposure{}, err
	}
	if stored.Revision < 1 || stored.UpdatedAt.IsZero() || stored.Enabled == nil {
		return storedCapabilityExposure{}, errors.New("设备能力开放配置无效")
	}
	for id := range stored.Enabled {
		if !capabilities.Valid(id) {
			return storedCapabilityExposure{}, fmt.Errorf("设备能力开放配置包含未知能力: %s", id)
		}
	}
	return stored, nil
}

func (s *CapabilityExposureStore) Snapshot() (CapabilityExposureState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, err := s.loadLocked()
	if err != nil {
		return CapabilityExposureState{}, err
	}
	return exposureState(stored), nil
}

func (s *CapabilityExposureStore) Enabled() (int64, []string, error) {
	state, err := s.Snapshot()
	if err != nil {
		return 0, nil, err
	}
	return state.Revision, append([]string(nil), state.EnabledIDs...), nil
}

func (s *CapabilityExposureStore) IsEnabled(capability string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, err := s.loadLocked()
	if err != nil {
		return false, err
	}
	enabled, ok := stored.Enabled[capability]
	return ok && enabled, nil
}

func (s *CapabilityExposureStore) SetEnabled(capability string, expectedRevision int64, enabled bool) (CapabilityExposureState, error) {
	if !capabilities.Valid(capability) || expectedRevision < 1 {
		return CapabilityExposureState{}, fmt.Errorf("无效的设备能力开放请求: %s", capability)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, err := s.loadLocked()
	if err != nil {
		return CapabilityExposureState{}, err
	}
	if stored.Revision != expectedRevision {
		return CapabilityExposureState{}, ErrCapabilityExposureRevision
	}
	if stored.Enabled[capability] == enabled {
		return exposureState(stored), nil
	}
	stored.Enabled[capability] = enabled
	stored.Revision++
	stored.UpdatedAt = time.Now().UTC()
	if err := s.saveLocked(stored); err != nil {
		return CapabilityExposureState{}, err
	}
	state := exposureState(stored)
	for subscriber := range s.subscribers {
		select {
		case subscriber <- state:
		default:
		}
	}
	return state, nil
}

func (s *CapabilityExposureStore) Subscribe() (<-chan CapabilityExposureState, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := make(chan CapabilityExposureState, 4)
	s.subscribers[events] = struct{}{}
	return events, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.subscribers[events]; ok {
			delete(s.subscribers, events)
			close(events)
		}
	}
}

func (s *CapabilityExposureStore) saveLocked(stored storedCapabilityExposure) error {
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.filePath + ".tmp"
	file, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		if removeErr := os.Remove(tmp); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return errors.Join(err, removeErr)
		}
		return err
	}
	if err := file.Sync(); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.filePath); err != nil {
		if removeErr := os.Remove(tmp); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return errors.Join(err, removeErr)
		}
		return err
	}
	return nil
}

func exposureState(stored storedCapabilityExposure) CapabilityExposureState {
	enabledIDs := make([]string, 0, len(stored.Enabled))
	items := make([]CapabilityExposureItem, 0, len(capabilities.All()))
	for _, definition := range capabilities.All() {
		enabled := stored.Enabled[definition.ID]
		items = append(items, CapabilityExposureItem{Definition: definition, Enabled: enabled})
		if enabled {
			enabledIDs = append(enabledIDs, definition.ID)
		}
	}
	sort.Strings(enabledIDs)
	return CapabilityExposureState{Revision: stored.Revision, UpdatedAt: stored.UpdatedAt, Capabilities: items, EnabledIDs: enabledIDs}
}
