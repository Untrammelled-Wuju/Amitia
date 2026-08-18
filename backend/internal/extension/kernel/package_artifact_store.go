package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ArtifactMetadata struct {
	ExtensionID  string
	Version      string
	SourceURI    string
	ExpectedHash string
}

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

func (s *PackageArtifactStore) PutArchiveFromURI(ctx context.Context, uri string, metadata ArtifactMetadata) (PackageArtifact, error) {
	if err := ctx.Err(); err != nil {
		return PackageArtifact{}, err
	}
	archivePath, err := s.downloadToCanonical(ctx, uri, metadata.ExpectedHash)
	if err != nil {
		return PackageArtifact{}, err
	}
	hash, err := s.hashFile(archivePath)
	if err != nil {
		return PackageArtifact{}, err
	}
	if metadata.ExpectedHash != "" && hash != metadata.ExpectedHash {
		return PackageArtifact{}, fmt.Errorf("downloaded archive hash mismatch: expected %s, got %s", metadata.ExpectedHash, hash)
	}
	return PackageArtifact{
		ArtifactID:  s.ArtifactIDFromHash(hash),
		ArchiveHash: hash,
		ArchivePath: archivePath,
	}, nil
}

func (s *PackageArtifactStore) downloadToCanonical(ctx context.Context, uri string, expectedHash string) (string, error) {
	tempRoot := filepath.Join(s.root, "artifacts", "temp")
	if err := os.MkdirAll(tempRoot, 0o700); err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	temp, err := os.CreateTemp(tempRoot, ".remote-*.amitiax")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := s.downloadFile(ctx, uri, temp); err != nil {
		temp.Close()
		return "", err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return "", fmt.Errorf("sync temp file: %w", err)
	}
	temp.Close()

	hash, err := s.hashFile(tempPath)
	if err != nil {
		return "", err
	}
	finalPath := s.canonicalArchivePath(hash)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o700); err != nil {
		return "", fmt.Errorf("create canonical dir: %w", err)
	}
	if _, err := os.Stat(finalPath); err == nil {
		return finalPath, nil
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return "", fmt.Errorf("move to canonical: %w", err)
	}
	return finalPath, nil
}

func (s *PackageArtifactStore) downloadFile(ctx context.Context, uri string, dest *os.File) error {
	return downloadFileTo(ctx, uri, dest)
}

func downloadFileTo(ctx context.Context, uri string, dest io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}
	if _, err := io.Copy(dest, resp.Body); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func (s *PackageArtifactStore) hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file for hashing: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *PackageArtifactStore) ArtifactIDFromHash(hash string) string {
	hexDigest := strings.TrimPrefix(hash, "sha256:")
	return artifactIDFromDigest(hexDigest)
}

func (s *PackageArtifactStore) HasArtifactAtHash(expectedHash string) (string, error) {
	if expectedHash != "" {
		if hexDigest, err := archiveHexDigest(expectedHash); err == nil {
			candidate := s.canonicalArchivePath(hexDigest)
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
				return candidate, nil
			}
		}
	}
	return "", os.ErrNotExist
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
