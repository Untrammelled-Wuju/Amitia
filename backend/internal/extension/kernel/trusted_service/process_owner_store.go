package trusted_service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ProcessOwnerMetadata struct {
	ProcessInstanceID string    `json:"process_instance_id"`
	PluginID          string    `json:"plugin_id"`
	RuntimeID         string    `json:"runtime_id"`
	LogicalServiceID  string    `json:"logical_service_id"`
	ExtensionID       string    `json:"extension_id"`
	ModuleID          string    `json:"module_id"`
	Generation        int64     `json:"generation"`
	HostInstanceID    string    `json:"host_instance_id"`
	HostSessionID     string    `json:"host_session_id"`
	PID               int       `json:"pid"`
	StartedAt         time.Time `json:"started_at"`
}

type ProcessOwnerStore struct {
	mu       sync.RWMutex
	metadata map[string]ProcessOwnerMetadata
	dataDir  string
	durable  bool
}

func NewProcessOwnerStore(dataDir string) *ProcessOwnerStore {
	return &ProcessOwnerStore{
		metadata: make(map[string]ProcessOwnerMetadata),
		dataDir:  dataDir,
		durable:  dataDir != "",
	}
}

func (s *ProcessOwnerStore) RecordOwnership(inst *ServiceInstance, hostInstanceID, hostSessionID string) ProcessOwnerMetadata {
	meta := ProcessOwnerMetadata{
		ProcessInstanceID: inst.ProcessInstanceID,
		PluginID:          inst.PluginID,
		RuntimeID:         inst.RuntimeID,
		LogicalServiceID:  inst.LogicalServiceID,
		ExtensionID:       inst.ExtensionID,
		ModuleID:          inst.ModuleID,
		Generation:        inst.Generation,
		HostInstanceID:    hostInstanceID,
		HostSessionID:     hostSessionID,
		PID:               inst.PID,
		StartedAt:         time.Now().UTC(),
	}
	s.mu.Lock()
	s.metadata[inst.ProcessInstanceID] = meta
	s.mu.Unlock()
	s.persist()
	return meta
}

func (s *ProcessOwnerStore) RemoveOwnership(processInstanceID string) {
	s.mu.Lock()
	delete(s.metadata, processInstanceID)
	s.mu.Unlock()
	s.persist()
}

func (s *ProcessOwnerStore) GetOwnedProcesses() []ProcessOwnerMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ProcessOwnerMetadata, 0, len(s.metadata))
	for _, meta := range s.metadata {
		result = append(result, meta)
	}
	return result
}

func (s *ProcessOwnerStore) LookupByProcessInstanceID(processInstanceID string) (ProcessOwnerMetadata, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	meta, ok := s.metadata[processInstanceID]
	return meta, ok
}

func (s *ProcessOwnerStore) LookupByRuntimeID(runtimeID string) []ProcessOwnerMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ProcessOwnerMetadata, 0)
	for _, meta := range s.metadata {
		if meta.RuntimeID == runtimeID {
			result = append(result, meta)
		}
	}
	return result
}

func (s *ProcessOwnerStore) LookupByPluginID(pluginID string) []ProcessOwnerMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ProcessOwnerMetadata, 0)
	for _, meta := range s.metadata {
		if meta.PluginID == pluginID {
			result = append(result, meta)
		}
	}
	return result
}

func (s *ProcessOwnerStore) persist() error {
	if !s.durable {
		return nil
	}
	s.mu.RLock()
	data, err := json.MarshalIndent(s.metadata, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("marshal process owner metadata: %w", err)
	}
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	path := filepath.Join(s.dataDir, "process_owners.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write process owner metadata tmp: %w", err)
	}
	return os.Rename(tmp, path)
}

func (s *ProcessOwnerStore) load() error {
	if !s.durable {
		return nil
	}
	path := filepath.Join(s.dataDir, "process_owners.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read process owner metadata: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Unmarshal(data, &s.metadata)
}
