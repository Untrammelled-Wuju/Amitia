package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/storage"
)

type FileStore struct {
	dir       storage.DirectoryManager
	mu        sync.Mutex
	pathLocks map[string]*sync.Mutex
}

const (
	MaxConfigSize         = 512 * 1024
	schemaFilename        = "schema.json"
	pluginConfigFilename  = "plugin.json"
	runtimeConfigFilename = "runtime.json"
	serviceConfigFilename = "service.json"
)

func NewFileStore(dirMgr storage.DirectoryManager) *FileStore {
	return &FileStore{
		dir:       dirMgr,
		pathLocks: make(map[string]*sync.Mutex),
	}
}

func (s *FileStore) loadLock(path string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if l, ok := s.pathLocks[path]; ok {
		return l
	}
	l := &sync.Mutex{}
	s.pathLocks[path] = l
	return l
}

func (s *FileStore) atomicJSONFileWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmpPath := filepath.Join(dir, ".tmp_"+filepath.Base(path))
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func (s *FileStore) LoadSchema(ctx context.Context) (*ConfigSchema, error) {
	paths, err := s.dir.ResolvePluginPaths("__schema__")
	if err != nil {
		return nil, newConfigError("load_schema", ErrPathViolation, "", err)
	}
	fullPath := filepath.Join(paths.Root, schemaFilename)
	return s.readSchema(fullPath)
}

func (s *FileStore) readSchema(fullPath string) (*ConfigSchema, error) {
	l := s.loadLock(fullPath)
	l.Lock()
	defer l.Unlock()

	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, newConfigError("load_schema", ErrSchemaNotFound, "", err)
		}
		return nil, newConfigError("load_schema", ErrCorruptConfig, "", err)
	}
	if len(data) > MaxConfigSize {
		return nil, newConfigError("load_schema", ErrCorruptConfig, "", errors.New("schema too large"))
	}

	var schema ConfigSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, newConfigError("load_schema", ErrCorruptConfig, "", err)
	}
	schema.fieldIndex = buildFieldIndex(schema.Fields)
	return &schema, nil
}

func buildFieldIndex(fields []ConfigField) map[string]int {
	idx := make(map[string]int, len(fields))
	for i, f := range fields {
		idx[f.Key] = i
	}
	return idx
}

func (s *FileStore) SaveSchema(ctx context.Context, schema *ConfigSchema) error {
	if schema == nil {
		return newConfigError("save_schema", ErrInvalidJSON, "", errors.New("nil schema"))
	}
	paths, err := s.dir.ResolvePluginPaths("__schema__")
	if err != nil {
		return newConfigError("save_schema", ErrPathViolation, "", err)
	}
	fullPath := filepath.Join(paths.Root, schemaFilename)
	return s.writeSchema(fullPath, schema)
}

func (s *FileStore) writeSchema(fullPath string, schema *ConfigSchema) error {
	l := s.loadLock(fullPath)
	l.Lock()
	defer l.Unlock()

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return newConfigError("write_schema", ErrInvalidJSON, "", err)
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
		return newConfigError("write_schema", ErrPathViolation, "", err)
	}
	return s.atomicJSONFileWrite(fullPath, data)
}

func (s *FileStore) LoadPluginConfig(ctx context.Context, pluginID string) (*ConfigBlob, error) {
	if err := validateIDString(pluginID); err != nil {
		return nil, newConfigError("load_plugin", ErrInvalidScope, pluginID, err)
	}
	paths, err := s.dir.ResolvePluginPaths(domain.PluginID(pluginID))
	if err != nil {
		return nil, newConfigError("load_plugin", ErrPathViolation, pluginID, err)
	}
	fullPath := filepath.Join(paths.Root, pluginConfigFilename)
	return s.readConfigBlob(fullPath, pluginID)
}

func (s *FileStore) SavePluginConfig(ctx context.Context, pluginID string, entries []ConfigEntry) error {
	if err := validateIDString(pluginID); err != nil {
		return newConfigError("save_plugin", ErrInvalidScope, pluginID, err)
	}
	paths, err := s.dir.ResolvePluginPaths(domain.PluginID(pluginID))
	if err != nil {
		return newConfigError("save_plugin", ErrPathViolation, pluginID, err)
	}
	fullPath := filepath.Join(paths.Root, pluginConfigFilename)
	return s.writeConfigBlob(fullPath, ConfigScopePlugin, entries)
}

func (s *FileStore) LoadRuntimeConfig(ctx context.Context, runtimeID string) (*ConfigBlob, error) {
	if err := validateIDString(runtimeID); err != nil {
		return nil, newConfigError("load_runtime", ErrInvalidScope, runtimeID, err)
	}
	paths, err := s.dir.ResolveRuntimePaths(domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return nil, newConfigError("load_runtime", ErrPathViolation, runtimeID, err)
	}
	fullPath := filepath.Join(paths.Root, runtimeConfigFilename)
	return s.readConfigBlob(fullPath, runtimeID)
}

func (s *FileStore) SaveRuntimeConfig(ctx context.Context, runtimeID string, entries []ConfigEntry) error {
	if err := validateIDString(runtimeID); err != nil {
		return newConfigError("save_runtime", ErrInvalidScope, runtimeID, err)
	}
	paths, err := s.dir.ResolveRuntimePaths(domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return newConfigError("save_runtime", ErrPathViolation, runtimeID, err)
	}
	fullPath := filepath.Join(paths.Root, runtimeConfigFilename)
	return s.writeConfigBlob(fullPath, ConfigScopeRuntime, entries)
}

func (s *FileStore) LoadServiceConfig(ctx context.Context, runtimeID, serviceID string) (*ConfigBlob, error) {
	if err := validateIDString(runtimeID); err != nil {
		return nil, newConfigError("load_service", ErrInvalidScope, serviceID, err)
	}
	if err := validateIDString(serviceID); err != nil {
		return nil, newConfigError("load_service", ErrInvalidScope, serviceID, err)
	}
	paths, err := s.dir.ResolveServicePaths(domain.RuntimeInstanceID(runtimeID), domain.ServiceID(serviceID))
	if err != nil {
		return nil, newConfigError("load_service", ErrPathViolation, serviceID, err)
	}
	fullPath := filepath.Join(paths.Root, serviceConfigFilename)
	return s.readConfigBlob(fullPath, serviceID)
}

func (s *FileStore) SaveServiceConfig(ctx context.Context, runtimeID, serviceID string, entries []ConfigEntry) error {
	if err := validateIDString(runtimeID); err != nil {
		return newConfigError("save_service", ErrInvalidScope, serviceID, err)
	}
	if err := validateIDString(serviceID); err != nil {
		return newConfigError("save_service", ErrInvalidScope, serviceID, err)
	}
	paths, err := s.dir.ResolveServicePaths(domain.RuntimeInstanceID(runtimeID), domain.ServiceID(serviceID))
	if err != nil {
		return newConfigError("save_service", ErrPathViolation, serviceID, err)
	}
	fullPath := filepath.Join(paths.Root, serviceConfigFilename)
	return s.writeConfigBlob(fullPath, ConfigScopeService, entries)
}

func (s *FileStore) readConfigBlob(fullPath, id string) (*ConfigBlob, error) {
	l := s.loadLock(fullPath)
	l.Lock()
	defer l.Unlock()

	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, newConfigError("load_config", ErrConfigNotFound, id, err)
		}
		return nil, newConfigError("load_config", ErrCorruptConfig, id, err)
	}
	if len(data) > MaxConfigSize {
		return nil, newConfigError("load_config", ErrCorruptConfig, id, errors.New("config too large"))
	}
	var blob ConfigBlob
	if err := json.Unmarshal(data, &blob); err != nil {
		return nil, newConfigError("load_config", ErrCorruptConfig, id, err)
	}
	return &blob, nil
}

func (s *FileStore) writeConfigBlob(fullPath string, scope ConfigScope, entries []ConfigEntry) error {
	l := s.loadLock(fullPath)
	l.Lock()
	defer l.Unlock()

	blob := ConfigBlob{
		Scope:   scope,
		Entries: entries,
	}
	data, err := json.Marshal(blob)
	if err != nil {
		return newConfigError("write_config", ErrInvalidJSON, "", err)
	}
	if len(data) > MaxConfigSize {
		return newConfigError("write_config", ErrCorruptConfig, "", errors.New("config too large"))
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
		return newConfigError("write_config", ErrPathViolation, "", err)
	}
	return s.atomicJSONFileWrite(fullPath, data)
}

func validateIDString(id string) error {
	if id == "" {
		return errors.New("empty id")
	}
	for _, r := range id {
		if r == 0 || (r >= 0x01 && r <= 0x1f) {
			return errors.New("id contains control characters")
		}
	}
	return nil
}
