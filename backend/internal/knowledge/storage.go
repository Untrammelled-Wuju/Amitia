package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type StoredObject struct {
	Key       string
	Path      string
	SizeBytes int64
	SHA256    string
}

type Storage struct {
	root        string
	maxBytes    int64
	bufferBytes int
}

func NewStorage(root string) *Storage {
	return NewStorageWithRuntime(root, loadRuntimeSettings())
}

func NewStorageWithRuntime(root string, settings runtimeSettings) *Storage {
	return newStorage(root, settings.MaxSourceBytes, settings.UploadBufferBytes)
}

func NewStorageWithLimit(root string, maxBytes int64) *Storage {
	return newStorage(root, maxBytes, defaultRuntimeSettings().UploadBufferBytes)
}

func newStorage(root string, maxBytes int64, bufferBytes int) *Storage {
	root = strings.TrimSpace(root)
	if root == "" {
		root = filepath.Join(os.TempDir(), "amitia", "knowledge")
	}
	defaults := defaultRuntimeSettings()
	if maxBytes <= 0 {
		maxBytes = defaults.MaxSourceBytes
	}
	if bufferBytes <= 0 {
		bufferBytes = defaults.UploadBufferBytes
	}
	return &Storage{root: root, maxBytes: maxBytes, bufferBytes: bufferBytes}
}

func (s *Storage) Root() string { return s.root }

func (s *Storage) Save(ctx context.Context, tenantID, sourceID string, revision int64, filename string, src io.Reader) (StoredObject, error) {
	if strings.TrimSpace(sourceID) == "" {
		return StoredObject{}, fmt.Errorf("source id required")
	}
	if revision <= 0 {
		revision = 1
	}
	safeName := filepath.Base(strings.TrimSpace(filename))
	if safeName == "." || safeName == "" {
		safeName = "source.bin"
	}
	tenantHash := sha256.Sum256([]byte(strings.TrimSpace(tenantID)))
	tenantPart := hex.EncodeToString(tenantHash[:8])
	rel := filepath.Join("sources", tenantPart, sourceID, fmt.Sprintf("r%d", revision), safeName)
	target := filepath.Join(s.root, rel)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return StoredObject{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".upload-*")
	if err != nil {
		return StoredObject{}, err
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()

	h := sha256.New()
	var written int64
	bufferBytes := s.bufferBytes
	if bufferBytes <= 0 {
		bufferBytes = defaultRuntimeSettings().UploadBufferBytes
	}
	buf := make([]byte, bufferBytes)
	for {
		select {
		case <-ctx.Done():
			return StoredObject{}, ctx.Err()
		default:
		}
		n, readErr := src.Read(buf)
		if n > 0 {
			written += int64(n)
			if s.maxBytes > 0 && written > s.maxBytes {
				return StoredObject{}, fmt.Errorf("knowledge source exceeds max size %d bytes", s.maxBytes)
			}
			if _, err := h.Write(buf[:n]); err != nil {
				return StoredObject{}, err
			}
			if _, err := tmp.Write(buf[:n]); err != nil {
				return StoredObject{}, err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return StoredObject{}, readErr
		}
	}
	if err := tmp.Sync(); err != nil {
		return StoredObject{}, err
	}
	if err := tmp.Close(); err != nil {
		return StoredObject{}, err
	}
	if _, statErr := os.Stat(target); statErr == nil {
		if err := os.Remove(target); err != nil {
			return StoredObject{}, err
		}
	}
	if err := os.Rename(tmpName, target); err != nil {
		return StoredObject{}, err
	}
	committed = true
	return StoredObject{Key: filepath.ToSlash(rel), Path: target, SizeBytes: written, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

func (s *Storage) Resolve(key string) (string, error) {
	key = filepath.Clean(filepath.FromSlash(strings.TrimSpace(key)))
	if key == "." || filepath.IsAbs(key) || strings.HasPrefix(key, ".."+string(filepath.Separator)) || key == ".." {
		return "", fmt.Errorf("invalid storage key")
	}
	rootAbs, err := filepath.Abs(s.root)
	if err != nil {
		return "", err
	}
	p, err := filepath.Abs(filepath.Join(rootAbs, key))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("storage key escapes root")
	}
	return p, nil
}

func (s *Storage) Delete(key string) error {
	p, err := s.Resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
