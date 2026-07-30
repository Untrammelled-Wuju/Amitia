package kernel

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

	"github.com/google/uuid"
)

type PackageArtifactStore struct {
	root string
	mu   sync.Mutex
}

func NewPackageArtifactStore(root string) *PackageArtifactStore {
	return &PackageArtifactStore{root: root}
}

func (s *PackageArtifactStore) PutArchive(ctx context.Context, reader io.Reader, limit int64) (PackageArtifact, error) {
	uploadRoot := filepath.Join(s.root, "artifacts", "uploads")
	if err := os.MkdirAll(uploadRoot, 0o700); err != nil {
		return PackageArtifact{}, err
	}
	temp, err := os.CreateTemp(uploadRoot, ".upload-*.amitiax")
	if err != nil {
		return PackageArtifact{}, err
	}
	tempPath := temp.Name()
	keep := false
	defer func() {
		temp.Close()
		if !keep {
			os.Remove(tempPath)
		}
	}()
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(temp, hash), io.LimitReader(reader, limit+1))
	if err != nil {
		return PackageArtifact{}, err
	}
	if written > limit {
		return PackageArtifact{}, fmt.Errorf("package archive exceeds limit")
	}
	if err := ctx.Err(); err != nil {
		return PackageArtifact{}, err
	}
	if err := temp.Sync(); err != nil {
		return PackageArtifact{}, err
	}
	if err := temp.Close(); err != nil {
		return PackageArtifact{}, err
	}
	digest := "sha256:" + hex.EncodeToString(hash.Sum(nil))
	finalPath := filepath.Join(uploadRoot, strings.TrimPrefix(digest, "sha256:")+".amitiax")
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(finalPath); err == nil {
		if err := os.Remove(tempPath); err != nil {
			return PackageArtifact{}, err
		}
	} else if err := os.Rename(tempPath, finalPath); err != nil {
		return PackageArtifact{}, err
	}
	keep = true
	return PackageArtifact{ArtifactID: "artifact-" + uuid.NewString(), ArchiveHash: digest, ArchivePath: finalPath, SizeBytes: written}, nil
}

func (s *PackageArtifactStore) PlaceArchive(a PackageArtifact) (PackageArtifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a.ExtensionID == "" || a.Version == "" {
		return a, fmt.Errorf("package identity incomplete")
	}
	dir := filepath.Join(s.root, "artifacts", safeDirectoryName(a.ExtensionID), a.Version)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return a, err
	}
	target := filepath.Join(dir, strings.TrimPrefix(a.ArchiveHash, "sha256:")+".amitiax")
	if filepath.Clean(a.ArchivePath) != filepath.Clean(target) {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			if err := copyFile(a.ArchivePath, target); err != nil {
				return a, err
			}
		}
	}
	a.ArchivePath = target
	return a, nil
}

func (s *PackageArtifactStore) VerifyArchive(a PackageArtifact) error {
	file, err := os.Open(a.ArchivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := "sha256:" + hex.EncodeToString(hash.Sum(nil))
	if actual != a.ArchiveHash {
		return fmt.Errorf("artifact archive hash mismatch")
	}
	return nil
}

func (s *PackageArtifactStore) InstalledPath(extensionID, version, artifactHash string) string {
	return filepath.Join(s.root, "installed", safeDirectoryName(extensionID), version, strings.TrimPrefix(artifactHash, "sha256:"))
}

func (s *PackageArtifactStore) RemoveInstalled(path string) error {
	root, err := filepath.Abs(filepath.Join(s.root, "installed"))
	if err != nil {
		return err
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if target == root || !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		return fmt.Errorf("installed path outside artifact store")
	}
	return os.RemoveAll(target)
}
