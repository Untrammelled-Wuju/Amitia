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
)

type PackageArtifactStore struct {
	root string
	mu   sync.Mutex
}

func NewPackageArtifactStore(root string) *PackageArtifactStore {
	return &PackageArtifactStore{root: root}
}

func (s *PackageArtifactStore) PutArchive(ctx context.Context, reader io.Reader, limit int64) (PackageArtifact, error) {
	tempRoot := filepath.Join(s.root, "artifacts", "temp")
	if err := os.MkdirAll(tempRoot, 0o700); err != nil {
		return PackageArtifact{}, err
	}
	temp, err := os.CreateTemp(tempRoot, ".upload-*.amitiax")
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
	hexDigest := hex.EncodeToString(hash.Sum(nil))
	digest := "sha256:" + hexDigest
	finalPath := s.canonicalArchivePath(hexDigest)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o700); err != nil {
		return PackageArtifact{}, err
	}
	if _, err := os.Stat(finalPath); err == nil {
		if err := os.Remove(tempPath); err != nil {
			return PackageArtifact{}, err
		}
	} else if !os.IsNotExist(err) {
		return PackageArtifact{}, err
	} else {
		if err := os.Rename(tempPath, finalPath); err != nil {
			return PackageArtifact{}, err
		}
	}
	keep = true
	return PackageArtifact{ArtifactID: artifactIDFromDigest(hexDigest), ArchiveHash: digest, ArchivePath: finalPath, SizeBytes: written}, nil
}

func (s *PackageArtifactStore) PlaceArchive(a PackageArtifact) (PackageArtifact, error) {
	path, err := s.ResolveArchivePath(a)
	if err != nil {
		return a, err
	}
	a.ArchivePath = path
	return a, nil
}

func (s *PackageArtifactStore) VerifyArchive(a PackageArtifact) error {
	path, err := s.ResolveArchivePath(a)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
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

func (s *PackageArtifactStore) ResolveArchivePath(a PackageArtifact) (string, error) {
	hexDigest, err := archiveHexDigest(a.ArchiveHash)
	if err != nil {
		return "", err
	}
	candidates := []string{s.canonicalArchivePath(hexDigest), a.ArchivePath}
	if a.ExtensionID != "" && a.Version != "" {
		candidates = append(candidates, filepath.Join(s.root, "artifacts", safeDirectoryName(a.ExtensionID), a.Version, hexDigest+".amitiax"))
	}
	candidates = append(candidates, filepath.Join(s.root, "artifacts", "uploads", hexDigest+".amitiax"))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		cleaned := filepath.Clean(candidate)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		info, statErr := os.Stat(cleaned)
		if statErr == nil && !info.IsDir() {
			return cleaned, nil
		}
		if statErr != nil && !os.IsNotExist(statErr) {
			return "", statErr
		}
	}
	return "", os.ErrNotExist
}

func (s *PackageArtifactStore) RemoveArchive(a PackageArtifact) error {
	path, err := s.ResolveArchivePath(a)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	root, err := filepath.Abs(filepath.Join(s.root, "artifacts"))
	if err != nil {
		return err
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if target == root || !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		return fmt.Errorf("artifact path outside artifact store")
	}
	return os.Remove(target)
}

func (s *PackageArtifactStore) canonicalArchivePath(hexDigest string) string {
	return filepath.Join(s.root, "artifacts", "sha256", hexDigest[:2], hexDigest[2:4], hexDigest+".amitiax")
}

func archiveHexDigest(value string) (string, error) {
	digest := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(value, "sha256:")))
	if len(digest) != sha256.Size*2 {
		return "", fmt.Errorf("invalid artifact archive hash")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return "", fmt.Errorf("invalid artifact archive hash: %w", err)
	}
	return digest, nil
}

func artifactIDFromDigest(hexDigest string) string {
	return "artifact-sha256-" + hexDigest
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
