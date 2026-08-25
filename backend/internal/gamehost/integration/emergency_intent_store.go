package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type InMemoryEmergencyIntentStore struct {
	mu      sync.RWMutex
	latched map[domain.RuntimeInstanceID]string
}

func NewInMemoryEmergencyIntentStore() *InMemoryEmergencyIntentStore {
	return &InMemoryEmergencyIntentStore{
		latched: make(map[domain.RuntimeInstanceID]string),
	}
}

func (s *InMemoryEmergencyIntentStore) CommitEmergencyIntent(ctx context.Context, runtimeID domain.RuntimeInstanceID, operationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latched[runtimeID] = operationID
	return nil
}

func (s *InMemoryEmergencyIntentStore) IsEmergencyLatched(ctx context.Context, runtimeID domain.RuntimeInstanceID) bool {
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.latched[runtimeID]
	return ok
}

func (s *InMemoryEmergencyIntentStore) GetEmergencyOperationID(ctx context.Context, runtimeID domain.RuntimeInstanceID) (string, bool) {
	if ctx != nil && ctx.Err() != nil {
		return "", false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	opID, ok := s.latched[runtimeID]
	return opID, ok
}

func (s *InMemoryEmergencyIntentStore) ClearEmergencyLatch(ctx context.Context, runtimeID domain.RuntimeInstanceID, actor string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.latched, runtimeID)
	return nil
}

var _ control.EmergencyIntentStore = (*InMemoryEmergencyIntentStore)(nil)

const persistentEmergencyIntentVersion = 1

type persistentEmergencyRecord struct {
	RuntimeID   domain.RuntimeInstanceID `json:"runtimeId"`
	PluginID    domain.PluginID          `json:"pluginId,omitempty"`
	OperationID string                   `json:"operationId"`
	UpdatedAt   time.Time                `json:"updatedAt"`
}

type persistentEmergencyDocument struct {
	Version int                         `json:"version"`
	Records []persistentEmergencyRecord `json:"records"`
}

// FileEmergencyIntentStore persists the emergency latch outside runtime
// checkpoints. Records carry both runtime and stable plugin identity so a host
// restart cannot silently clear an emergency stop merely because the runtime
// instance receives a new ID during reconciliation.
type FileEmergencyIntentStore struct {
	mu      sync.RWMutex
	path    string
	records map[domain.RuntimeInstanceID]persistentEmergencyRecord
}

func NewFileEmergencyIntentStore(path string) (*FileEmergencyIntentStore, error) {
	if filepath.Clean(path) == "." || path == "" {
		return nil, fmt.Errorf("emergency intent store: file path is required")
	}
	s := &FileEmergencyIntentStore{
		path:    filepath.Clean(path),
		records: make(map[domain.RuntimeInstanceID]persistentEmergencyRecord),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FileEmergencyIntentStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("emergency intent store: read: %w", err)
	}
	var doc persistentEmergencyDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("emergency intent store: corrupt state: %w", err)
	}
	if doc.Version != persistentEmergencyIntentVersion {
		return fmt.Errorf("emergency intent store: unsupported version %d", doc.Version)
	}
	for _, record := range doc.Records {
		if record.RuntimeID == "" || record.OperationID == "" {
			return fmt.Errorf("emergency intent store: corrupt record")
		}
		s.records[record.RuntimeID] = record
	}
	return nil
}

func (s *FileEmergencyIntentStore) CommitEmergencyIntent(ctx context.Context, runtimeID domain.RuntimeInstanceID, operationID string) error {
	return s.CommitEmergencyIntentForPlugin(ctx, runtimeID, "", operationID)
}

func (s *FileEmergencyIntentStore) CommitEmergencyIntentForPlugin(ctx context.Context, runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID, operationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if runtimeID == "" || operationID == "" {
		return fmt.Errorf("emergency intent store: runtimeId and operationId are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, existed := s.records[runtimeID]
	s.records[runtimeID] = persistentEmergencyRecord{
		RuntimeID: runtimeID, PluginID: pluginID, OperationID: operationID, UpdatedAt: time.Now().UTC(),
	}
	if err := s.persistLocked(); err != nil {
		if existed {
			s.records[runtimeID] = previous
		} else {
			delete(s.records, runtimeID)
		}
		return err
	}
	return nil
}

func (s *FileEmergencyIntentStore) IsEmergencyLatched(ctx context.Context, runtimeID domain.RuntimeInstanceID) bool {
	return s.IsEmergencyLatchedForPlugin(ctx, runtimeID, "")
}

func (s *FileEmergencyIntentStore) IsEmergencyLatchedForPlugin(ctx context.Context, runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID) bool {
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.records[runtimeID]; ok {
		return true
	}
	if pluginID == "" {
		return false
	}
	for _, record := range s.records {
		if record.PluginID == pluginID {
			return true
		}
	}
	return false
}

func (s *FileEmergencyIntentStore) GetEmergencyOperationID(ctx context.Context, runtimeID domain.RuntimeInstanceID) (string, bool) {
	return s.GetEmergencyOperationIDForPlugin(ctx, runtimeID, "")
}

func (s *FileEmergencyIntentStore) GetEmergencyOperationIDForPlugin(ctx context.Context, runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID) (string, bool) {
	if ctx != nil && ctx.Err() != nil {
		return "", false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if record, ok := s.records[runtimeID]; ok {
		return record.OperationID, true
	}
	if pluginID == "" {
		return "", false
	}
	var latest persistentEmergencyRecord
	found := false
	for _, record := range s.records {
		if record.PluginID != pluginID {
			continue
		}
		if !found || record.UpdatedAt.After(latest.UpdatedAt) || (record.UpdatedAt.Equal(latest.UpdatedAt) && record.RuntimeID > latest.RuntimeID) {
			latest = record
			found = true
		}
	}
	if !found {
		return "", false
	}
	return latest.OperationID, true
}

func (s *FileEmergencyIntentStore) ClearEmergencyLatch(ctx context.Context, runtimeID domain.RuntimeInstanceID, actor string) error {
	return s.ClearEmergencyLatchForPlugin(ctx, runtimeID, "", actor)
}

func (s *FileEmergencyIntentStore) ClearEmergencyLatchForPlugin(ctx context.Context, runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID, actor string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	before := make(map[domain.RuntimeInstanceID]persistentEmergencyRecord, len(s.records))
	for id, record := range s.records {
		before[id] = record
	}
	delete(s.records, runtimeID)
	if pluginID != "" {
		for id, record := range s.records {
			if record.PluginID == pluginID {
				delete(s.records, id)
			}
		}
	}
	if err := s.persistLocked(); err != nil {
		s.records = before
		return err
	}
	return nil
}

func (s *FileEmergencyIntentStore) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("emergency intent store: create directory: %w", err)
	}
	records := make([]persistentEmergencyRecord, 0, len(s.records))
	for _, record := range s.records {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].RuntimeID < records[j].RuntimeID })
	data, err := json.MarshalIndent(persistentEmergencyDocument{Version: persistentEmergencyIntentVersion, Records: records}, "", "  ")
	if err != nil {
		return fmt.Errorf("emergency intent store: encode: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".emergency-intents-*.tmp")
	if err != nil {
		return fmt.Errorf("emergency intent store: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		cleanup()
		return fmt.Errorf("emergency intent store: replace: %w", err)
	}
	if err := os.Chmod(s.path, 0o600); err != nil {
		return err
	}
	return nil
}

var _ control.EmergencyIntentStore = (*FileEmergencyIntentStore)(nil)
