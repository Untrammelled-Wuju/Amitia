package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const resourceContentBinExt = ".bin"

type ResourceContentStore struct {
	root string
	mu   sync.Mutex
}

func NewResourceContentStore(root string) *ResourceContentStore {
	return &ResourceContentStore{root: root}
}

func (s *ResourceContentStore) StoreContent(absPath string) (storageRef, contentHash string, size int64, err error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", "", 0, fmt.Errorf("kernel: read resource file: %w", err)
	}
	return s.StoreBytes(data)
}

func (s *ResourceContentStore) StoreBytes(data []byte) (storageRef, contentHash string, size int64, err error) {
	if len(data) == 0 {
		return "", "", 0, fmt.Errorf("kernel: store empty resource content")
	}
	sum := sha256.Sum256(data)
	contentHash = "sha256:" + hex.EncodeToString(sum[:])
	hexDigest := strings.TrimPrefix(contentHash, "sha256:")
	size = int64(len(data))

	finalPath := s.canonicalPath(hexDigest)
	relPath, relErr := filepath.Rel(s.root, finalPath)
	if relErr != nil {
		relPath = filepath.ToSlash(finalPath)
	}
	storageRef = filepath.ToSlash(relPath)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, statErr := os.Stat(finalPath); statErr == nil {
		return storageRef, contentHash, size, nil
	}

	if mkErr := os.MkdirAll(filepath.Dir(finalPath), 0o700); mkErr != nil {
		return "", "", 0, fmt.Errorf("kernel: create content directory: %w", mkErr)
	}

	tmp, tmpErr := os.CreateTemp(filepath.Dir(finalPath), ".content-*.tmp")
	if tmpErr != nil {
		return "", "", 0, fmt.Errorf("kernel: create temp content file: %w", tmpErr)
	}
	tmpPath := tmp.Name()

	_, writeErr := tmp.Write(data)
	if writeErr != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", "", 0, fmt.Errorf("kernel: write content file: %w", writeErr)
	}

	if syncErr := tmp.Sync(); syncErr != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", "", 0, fmt.Errorf("kernel: sync content file: %w", syncErr)
	}

	if closeErr := tmp.Close(); closeErr != nil {
		os.Remove(tmpPath)
		return "", "", 0, fmt.Errorf("kernel: close content file: %w", closeErr)
	}

	if renameErr := os.Rename(tmpPath, finalPath); renameErr != nil {
		os.Remove(tmpPath)
		return "", "", 0, fmt.Errorf("kernel: rename content file: %w", renameErr)
	}

	if dirSyncErr := syncDir(filepath.Dir(finalPath)); dirSyncErr != nil {
		return "", "", 0, fmt.Errorf("kernel: sync content directory: %w", dirSyncErr)
	}

	return storageRef, contentHash, size, nil
}

func (s *ResourceContentStore) ReadContent(storageRef string) ([]byte, error) {
	if storageRef == "" {
		return nil, fmt.Errorf("kernel: empty content storage reference")
	}
	path := s.resolvePath(storageRef)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("kernel: read content file: %w", err)
	}
	return data, nil
}

func (s *ResourceContentStore) VerifyContent(storageRef, expectedHash string) error {
	if expectedHash == "" {
		return nil
	}
	data, err := s.ReadContent(storageRef)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	actual := "sha256:" + hex.EncodeToString(sum[:])
	if actual != expectedHash {
		return fmt.Errorf("kernel: content hash mismatch for %s: expected %s got %s", storageRef, expectedHash, actual)
	}
	return nil
}

func (s *ResourceContentStore) ValidateContentRef(contentStorageRef string) error {
	if contentStorageRef == "" {
		return fmt.Errorf("kernel: content storage reference empty")
	}
	if filepath.IsAbs(contentStorageRef) {
		return fmt.Errorf("kernel: content storage reference must be relative: %s", contentStorageRef)
	}
	cleanRef := filepath.Clean(contentStorageRef)
	if strings.HasPrefix(cleanRef, "..") {
		return fmt.Errorf("kernel: content storage reference escapes root: %s", contentStorageRef)
	}
	resolvedPath := filepath.Join(s.root, cleanRef)
	absResolved, err := filepath.Abs(resolvedPath)
	if err != nil {
		return fmt.Errorf("kernel: resolve content ref path: %w", err)
	}
	absRoot, err := filepath.Abs(s.root)
	if err != nil {
		return fmt.Errorf("kernel: resolve content root: %w", err)
	}
	if !strings.HasPrefix(absResolved, absRoot+string(filepath.Separator)) {
		return fmt.Errorf("kernel: content storage reference %s escapes content store root", contentStorageRef)
	}
	return nil
}

func (s *ResourceContentStore) VerifyContentRef(contentStorageRef, expectedHash string, expectedSize int64) error {
	if err := s.ValidateContentRef(contentStorageRef); err != nil {
		return err
	}
	if err := s.VerifyContent(contentStorageRef, expectedHash); err != nil {
		return err
	}
	data, err := s.ReadContent(contentStorageRef)
	if err != nil {
		return err
	}
	if int64(len(data)) != expectedSize {
		return fmt.Errorf("kernel: content size mismatch for %s: expected %d got %d", contentStorageRef, expectedSize, len(data))
	}
	return nil
}

func (s *ResourceContentStore) canonicalPath(hexDigest string) string {
	return filepath.Join(s.root, "content", "sha256", hexDigest[:2], hexDigest[2:4], hexDigest+resourceContentBinExt)
}

func (s *ResourceContentStore) resolvePath(storageRef string) string {
	if filepath.IsAbs(storageRef) {
		return storageRef
	}
	return filepath.Join(s.root, storageRef)
}

func (s *ResourceContentStore) RestoreResourceFile(logicalPath, storageRef, expectedHash string) error {
	if logicalPath == "" {
		return fmt.Errorf("kernel: empty logical path for restore")
	}
	if storageRef == "" {
		return fmt.Errorf("kernel: empty storage reference for restore")
	}
	data, err := s.ReadContent(storageRef)
	if err != nil {
		return err
	}
	if expectedHash != "" {
		sum := sha256.Sum256(data)
		actual := "sha256:" + hex.EncodeToString(sum[:])
		if actual != expectedHash {
			return fmt.Errorf("kernel: content hash mismatch on restore for %s: expected %s got %s", logicalPath, expectedHash, actual)
		}
	}
	absLogical := s.resolvePath(logicalPath)
	if mkErr := os.MkdirAll(filepath.Dir(absLogical), 0o700); mkErr != nil {
		return fmt.Errorf("kernel: create directory for restored resource: %w", mkErr)
	}
	tmp, tmpErr := os.CreateTemp(filepath.Dir(absLogical), ".restore-*.tmp")
	if tmpErr != nil {
		return fmt.Errorf("kernel: create temp file for restored resource: %w", tmpErr)
	}
	tmpPath := tmp.Name()
	if _, writeErr := tmp.Write(data); writeErr != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("kernel: write restored resource: %w", writeErr)
	}
	if syncErr := tmp.Sync(); syncErr != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("kernel: sync restored resource: %w", syncErr)
	}
	if closeErr := tmp.Close(); closeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("kernel: close restored resource: %w", closeErr)
	}
	if renameErr := os.Rename(tmpPath, absLogical); renameErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("kernel: rename restored resource: %w", renameErr)
	}
	return syncDir(filepath.Dir(absLogical))
}

func (s *ResourceContentStore) RestoreQuarantineEntry(entry ResourceQuarantineEntry, expectedHash string) error {
	if entry.QuarantinePath == "" {
		return fmt.Errorf("kernel: empty quarantine reference for restore")
	}
	if entry.StorageRef == "" {
		return fmt.Errorf("kernel: empty storage reference for restore")
	}
	data, err := s.ReadContent(entry.QuarantinePath)
	if err != nil {
		return err
	}
	if expectedHash != "" {
		sum := sha256.Sum256(data)
		actual := "sha256:" + hex.EncodeToString(sum[:])
		if actual != expectedHash {
			return fmt.Errorf("kernel: content hash mismatch on restore for %s: expected %s got %s", entry.ResourceID, expectedHash, actual)
		}
	}
	absLogical := s.resolvePath(entry.StorageRef)
	if mkErr := os.MkdirAll(filepath.Dir(absLogical), 0o700); mkErr != nil {
		return fmt.Errorf("kernel: create directory for restored resource: %w", mkErr)
	}
	tmp, tmpErr := os.CreateTemp(filepath.Dir(absLogical), ".restore-*.tmp")
	if tmpErr != nil {
		return fmt.Errorf("kernel: create temp file for restored resource: %w", tmpErr)
	}
	tmpPath := tmp.Name()
	if _, writeErr := tmp.Write(data); writeErr != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("kernel: write restored resource: %w", writeErr)
	}
	if syncErr := tmp.Sync(); syncErr != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("kernel: sync restored resource: %w", syncErr)
	}
	if closeErr := tmp.Close(); closeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("kernel: close restored resource: %w", closeErr)
	}
	if renameErr := os.Rename(tmpPath, absLogical); renameErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("kernel: rename restored resource: %w", renameErr)
	}
	return syncDir(filepath.Dir(absLogical))
}

func syncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
