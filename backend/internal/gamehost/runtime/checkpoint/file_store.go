package checkpoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/runtime"
	"github.com/u-ai/backend/internal/gamehost/storage"
)

const (
	metadataFileName   = "metadata.json"
	checkpointFileName = "checkpoint.json"
	tmpFileExt         = ".tmp"
)

type FileStore struct {
	dir    storage.DirectoryManager
	mu     sync.Mutex
	locks  map[string]*sync.Mutex
}

func NewFileStore(dir storage.DirectoryManager) (*FileStore, error) {
	if dir == nil {
		return nil, errors.New("directory manager must not be nil")
	}
	return &FileStore{
		dir:   dir,
		locks: make(map[string]*sync.Mutex),
	}, nil
}

func (s *FileStore) runtimeLock(runtimeID domain.RuntimeInstanceID) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := string(runtimeID)
	lock, exists := s.locks[id]
	if !exists {
		lock = &sync.Mutex{}
		s.locks[id] = lock
	}
	return lock
}

func (s *FileStore) SaveMetadata(ctx context.Context, metadata RuntimeMetadata) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateMetadataFields(&metadata); err != nil {
		return err
	}

	lock := s.runtimeLock(metadata.RuntimeID)
	lock.Lock()
	defer lock.Unlock()

	paths, err := s.dir.ResolveRuntimePaths(metadata.RuntimeID)
	if err != nil {
		return newError("save_metadata", ErrCorruptMetadata, string(metadata.RuntimeID), err)
	}

	metadata.SchemaVersion = MetadataSchemaVersion
	return atomicJSONFileWrite(paths.Root, metadataFileName, &metadata)
}

func (s *FileStore) LoadMetadata(ctx context.Context, runtimeID domain.RuntimeInstanceID) (RuntimeMetadata, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeMetadata{}, err
	}
	if runtimeID == "" {
		return RuntimeMetadata{}, errCorruptMetadata("load_metadata", "", nil)
	}

	paths, err := s.dir.ResolveRuntimePaths(runtimeID)
	if err != nil {
		return RuntimeMetadata{}, newError("load_metadata", ErrCorruptMetadata, string(runtimeID), err)
	}

	metadata, err := readMetadataFile(paths.Root)
	if err != nil {
		return RuntimeMetadata{}, err
	}

	if err := s.validateLoadedMetadata(metadata, runtimeID); err != nil {
		return RuntimeMetadata{}, err
	}

	return metadata, nil
}

func (s *FileStore) HasMetadata(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	paths, err := s.dir.ResolveRuntimePaths(runtimeID)
	if err != nil {
		return false, nil
	}
	info, err := os.Stat(filepath.Join(paths.Root, metadataFileName))
	if err != nil {
		return false, nil
	}
	return !info.IsDir(), nil
}

func (s *FileStore) SaveCheckpoint(ctx context.Context, checkpoint RuntimeCheckpoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sortCheckpointServices(&checkpoint)
	if err := validateCheckpointFields(&checkpoint); err != nil {
		return err
	}

	lock := s.runtimeLock(checkpoint.RuntimeID)
	lock.Lock()
	defer lock.Unlock()

	paths, err := s.dir.ResolveRuntimePaths(checkpoint.RuntimeID)
	if err != nil {
		return newError("save_checkpoint", ErrCorrupt, string(checkpoint.RuntimeID), err)
	}

	checkpoint.SchemaVersion = MetadataSchemaVersion
	return atomicJSONFileWrite(paths.Root, checkpointFileName, &checkpoint)
}

func (s *FileStore) LoadCheckpoint(ctx context.Context, runtimeID domain.RuntimeInstanceID) (RuntimeCheckpoint, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeCheckpoint{}, err
	}
	if runtimeID == "" {
		return RuntimeCheckpoint{}, errCorrupt("load_checkpoint", "", nil)
	}

	paths, err := s.dir.ResolveRuntimePaths(runtimeID)
	if err != nil {
		return RuntimeCheckpoint{}, newError("load_checkpoint", ErrCorrupt, string(runtimeID), err)
	}

	checkpoint, err := readCheckpointFile(paths.Root)
	if err != nil {
		return RuntimeCheckpoint{}, err
	}

	if err := s.validateLoadedCheckpoint(checkpoint, runtimeID); err != nil {
		return RuntimeCheckpoint{}, err
	}

	return checkpoint.Clone(), nil
}

func (s *FileStore) DeleteMetadata(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	paths, err := s.dir.ResolveRuntimePaths(runtimeID)
	if err != nil {
		return err
	}
	return safeRemove(filepath.Join(paths.Root, metadataFileName))
}

func (s *FileStore) DeleteCheckpoint(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	paths, err := s.dir.ResolveRuntimePaths(runtimeID)
	if err != nil {
		return err
	}
	return safeRemove(filepath.Join(paths.Root, checkpointFileName))
}

func (s *FileStore) validateLoadedMetadata(metadata RuntimeMetadata, expectedID domain.RuntimeInstanceID) error {
	if metadata.SchemaVersion != MetadataSchemaVersion {
		return errUnsupportedSchema("validate_metadata", string(expectedID))
	}
	if metadata.RuntimeID != expectedID {
		return errRuntimeIDMismatch("validate_metadata", string(expectedID),
			fmt.Errorf("expected %s, got %s", expectedID, metadata.RuntimeID))
	}
	if metadata.PluginID == "" {
		return errCorruptMetadata("validate_metadata", string(expectedID), errors.New("missing plugin id"))
	}
	return nil
}

func (s *FileStore) validateLoadedCheckpoint(checkpoint RuntimeCheckpoint, expectedID domain.RuntimeInstanceID) error {
	if checkpoint.SchemaVersion != MetadataSchemaVersion {
		return errUnsupportedSchema("validate_checkpoint", string(expectedID))
	}
	if checkpoint.RuntimeID != expectedID {
		return errRuntimeIDMismatch("validate_checkpoint", string(expectedID),
			fmt.Errorf("expected %s, got %s", expectedID, checkpoint.RuntimeID))
	}
	if checkpoint.PluginID == "" {
		return errCorrupt("validate_checkpoint", string(expectedID), errors.New("missing plugin id"))
	}
	if !domain.IsValidRuntimeState(checkpoint.RuntimeState) {
		return errInvalidState("validate_checkpoint", string(expectedID),
			fmt.Errorf("invalid runtime state: %s", checkpoint.RuntimeState))
	}
	for i, svc := range checkpoint.Services {
		if !runtime.IsValidServiceRuntimeState(svc.State) {
			return errInvalidState("validate_checkpoint", string(expectedID),
				fmt.Errorf("service[%d] invalid state: %s", i, svc.State))
		}
	}
	return nil
}

func validateMetadataFields(m *RuntimeMetadata) error {
	if m.RuntimeID == "" {
		return errCorruptMetadata("validate_fields", "", errors.New("missing runtime id"))
	}
	if m.PluginID == "" {
		return errCorruptMetadata("validate_fields", string(m.RuntimeID), errors.New("missing plugin id"))
	}
	return nil
}

func validateCheckpointFields(c *RuntimeCheckpoint) error {
	if c.RuntimeID == "" {
		return errCorrupt("validate_fields", "", errors.New("missing runtime id"))
	}
	if c.PluginID == "" {
		return errCorrupt("validate_fields", string(c.RuntimeID), errors.New("missing plugin id"))
	}
	if !domain.IsValidRuntimeState(c.RuntimeState) {
		return errInvalidState("validate_fields", string(c.RuntimeID),
			fmt.Errorf("invalid runtime state: %s", c.RuntimeState))
	}
	for _, svc := range c.Services {
		if !runtime.IsValidServiceRuntimeState(svc.State) {
			return errInvalidState("validate_fields", string(c.RuntimeID),
				fmt.Errorf("invalid service state: %s", svc.State))
		}
	}
	if len(c.Services) > MaxServicesPerCheckpoint {
		return newError("validate_fields", ErrTooLarge, string(c.RuntimeID),
			fmt.Errorf("services count %d exceeds max %d", len(c.Services), MaxServicesPerCheckpoint))
	}
	if len(c.Reason) > MaxReasonLength {
		c.Reason = c.Reason[:MaxReasonLength]
	}
	return nil
}

func atomicJSONFileWrite(dir, name string, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return newError("atomic_write", ErrCorrupt, name, err)
	}

	if len(data) > MaxCheckpointSize {
		return errTooLarge("atomic_write", name)
	}

	tmpPath := filepath.Join(dir, name+tmpFileExt)
	finalPath := filepath.Join(dir, name)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return newError("atomic_write", ErrCorrupt, name, err)
	}

	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return newError("atomic_write", ErrCorrupt, name, err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return newError("atomic_write", ErrCorrupt, name, err)
	}

	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return newError("atomic_write", ErrCorrupt, name, err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return newError("atomic_write", ErrCorrupt, name, err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return newError("atomic_write", ErrCorrupt, name, err)
	}

	return nil
}

func readMetadataFile(dir string) (RuntimeMetadata, error) {
	path := filepath.Join(dir, metadataFileName)
	data, err := readFileLimited(path, MaxCheckpointSize)
	if err != nil {
		if os.IsNotExist(err) {
			return RuntimeMetadata{}, errNotFound("read_metadata", dir)
		}
		return RuntimeMetadata{}, errCorruptMetadata("read_metadata", dir, err)
	}

	var metadata RuntimeMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return RuntimeMetadata{}, errCorruptMetadata("read_metadata", dir, err)
	}

	return metadata, nil
}

func readCheckpointFile(dir string) (RuntimeCheckpoint, error) {
	path := filepath.Join(dir, checkpointFileName)
	data, err := readFileLimited(path, MaxCheckpointSize)
	if err != nil {
		if os.IsNotExist(err) {
			return RuntimeCheckpoint{}, errNotFound("read_checkpoint", dir)
		}
		return RuntimeCheckpoint{}, errCorrupt("read_checkpoint", dir, err)
	}

	var checkpoint RuntimeCheckpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return RuntimeCheckpoint{}, errCorrupt("read_checkpoint", dir, err)
	}

	if len(data) > MaxCheckpointSize {
		return RuntimeCheckpoint{}, errTooLarge("read_checkpoint", dir)
	}

	return checkpoint, nil
}

func readFileLimited(path string, limit int64) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("file size %d exceeds limit %d", info.Size(), limit)
	}
	if info.IsDir() {
		return nil, errors.New("path is a directory")
	}
	return os.ReadFile(path)
}

func safeRemove(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return newError("safe_remove", ErrCorrupt, path, err)
	}
	return nil
}

func sortCheckpointServices(cp *RuntimeCheckpoint) {
	sort.SliceStable(cp.Services, func(i, j int) bool {
		return cp.Services[i].ServiceID < cp.Services[j].ServiceID
	})
}

func isErrKind(err error, kind string) bool {
	if err == nil {
		return false
	}
	var cpErr *CheckpointError
	if errors.As(err, &cpErr) {
		return cpErr.Kind == kind
	}
	return false
}

var _ = io.EOF
