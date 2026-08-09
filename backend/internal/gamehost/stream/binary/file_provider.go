package binary

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/storage"
)

const (
	binarySubDir = "binary"
	fileExtension = ".bin"
)

type FileProvider struct {
	mu       sync.RWMutex
	registry ObjectRegistry
	root     string
	tempRoot string
}

func NewFileProvider(root string) (*FileProvider, error) {
	cleaned, err := validateRoot(root)
	if err != nil {
		return nil, err
	}
	return &FileProvider{
		root:     cleaned,
		tempRoot: filepath.Join(cleaned, "writing"),
	}, nil
}

func NewFileProviderWithManager(mgr storage.DirectoryManager, runtimeID domain.RuntimeInstanceID) (*FileProvider, error) {
	paths, err := mgr.ResolveRuntimePaths(runtimeID)
	if err != nil {
		return nil, err
	}
	binaryRoot := filepath.Join(paths.Temp, binarySubDir)
	if err := os.MkdirAll(binaryRoot, 0o700); err != nil {
		return nil, fmt.Errorf("binary: failed to create binary root: %w", err)
	}
	return &FileProvider{
		root:     binaryRoot,
		tempRoot: filepath.Join(binaryRoot, "writing"),
	}, nil
}

func (p *FileProvider) Kind() BinaryStorageKind {
	return BinaryStorageFile
}

func (p *FileProvider) Create(
	ctx context.Context,
	owner BinaryOwner,
	request CreateRequest,
) (WritingHandle, error) {
	if err := owner.Validate(); err != nil {
		return WritingHandle{}, err
	}

	id := NewBinaryObjectID()

	p.mu.Lock()
	defer p.mu.Unlock()

	if err := os.MkdirAll(p.tempRoot, 0o700); err != nil {
		return WritingHandle{}, fmt.Errorf("binary: failed to ensure temp dir: %w", err)
	}

	tmpFile, err := os.CreateTemp(p.tempRoot, "tmp-*")
	if err != nil {
		return WritingHandle{}, fmt.Errorf("binary: failed to create temp file: %w", err)
	}

	handle := WritingHandle{
		ObjectID: id,
		Writer:   tmpFile,
		Seal: func(actualSize int64, checksum *Checksum) (BinaryReference, error) {
			return p.seal(id, owner, request, tmpFile, actualSize, checksum)
		},
	}
	return handle, nil
}

func (p *FileProvider) seal(
	id BinaryObjectID,
	owner BinaryOwner,
	request CreateRequest,
	tmpFile *os.File,
	actualSize int64,
	checksum *Checksum,
) (BinaryReference, error) {
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return BinaryReference{}, fmt.Errorf("binary: failed to sync file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return BinaryReference{}, fmt.Errorf("binary: failed to close file: %w", err)
	}

	finalName := strings.ReplaceAll(string(id), "-", "") + fileExtension
	finalPath := filepath.Join(p.root, finalName)

	if err := storage.EnsureWithinRoot(p.root, finalPath); err != nil {
		os.Remove(tmpFile.Name())
		return BinaryReference{}, ErrPathEscapesRoot
	}

	if err := os.Rename(tmpFile.Name(), finalPath); err != nil {
		os.Remove(tmpFile.Name())
		return BinaryReference{}, fmt.Errorf("binary: failed to publish file: %w", err)
	}

	var computedChecksum *Checksum
	if checksum == nil && actualSize >= 0 {
		computedChecksum = computeSHA256(finalPath)
	} else if checksum != nil {
		computedChecksum = checksum
	}

	return BinaryReference{
		ID:        id,
		Kind:      BinaryStorageFile,
		Size:      actualSize,
		MediaType: request.MediaType,
		Checksum:  computedChecksum,
		Lifetime:  BinaryLifetimeMessage,
		Metadata:  request.Metadata,
	}, nil
}

func (p *FileProvider) Resolve(
	ctx context.Context,
	owner BinaryOwner,
	ref BinaryReference,
) (ResolvedBinary, error) {
	if err := owner.Validate(); err != nil {
		return ResolvedBinary{}, err
	}
	if err := ValidateBinaryObjectID(ref.ID); err != nil {
		return ResolvedBinary{}, err
	}

	finalName := strings.ReplaceAll(string(ref.ID), "-", "") + fileExtension
	finalPath := filepath.Join(p.root, finalName)

	if err := storage.EnsureWithinRoot(p.root, finalPath); err != nil {
		return ResolvedBinary{}, ErrPathEscapesRoot
	}

	file, err := os.Open(finalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ResolvedBinary{}, ErrObjectNotFound
		}
		return ResolvedBinary{}, fmt.Errorf("binary: failed to open file: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return ResolvedBinary{}, fmt.Errorf("binary: failed to stat file: %w", err)
	}

	if info.Size() != ref.Size {
		file.Close()
		return ResolvedBinary{}, ErrSizeMismatch
	}

	return ResolvedBinary{
		Reference: ref,
		Reader:    file,
	}, nil
}

func (p *FileProvider) Release(
	ctx context.Context,
	owner BinaryOwner,
	id BinaryObjectID,
) error {
	if err := owner.Validate(); err != nil {
		return err
	}
	if err := ValidateBinaryObjectID(id); err != nil {
		return err
	}

	finalName := strings.ReplaceAll(string(id), "-", "") + fileExtension
	finalPath := filepath.Join(p.root, finalName)

	if err := storage.EnsureWithinRoot(p.root, finalPath); err != nil {
		return ErrPathEscapesRoot
	}

	if err := os.Remove(finalPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("binary: failed to remove file: %w", err)
	}
	return nil
}

func (p *FileProvider) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := os.RemoveAll(p.tempRoot); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("binary: failed to clean temp dir: %w", err)
	}
	return nil
}

func validateRoot(root string) (string, error) {
	if root == "" {
		return "", ErrPathEscapesRoot
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("binary: invalid root path: %w", err)
	}
	cleaned := filepath.Clean(abs)
	if !filepath.IsAbs(cleaned) {
		return "", ErrPathEscapesRoot
	}
	return cleaned, nil
}

func computeSHA256(path string) *Checksum {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return nil
	}

	return &Checksum{
		Algorithm: "sha256",
		Value:     hex.EncodeToString(hasher.Sum(nil)),
	}
}

