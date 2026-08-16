package artifact

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

type FilesystemBlobStore struct {
	root string
}

func NewFilesystemBlobStore(root string) *FilesystemBlobStore {
	return &FilesystemBlobStore{root: root}
}

func (s *FilesystemBlobStore) blobPath(digest BlobDigest) string {
	d := string(digest)
	if len(d) < 8 {
		return filepath.Join(s.root, d)
	}
	return filepath.Join(s.root, d[5:7], d[7:9], d)
}

func (s *FilesystemBlobStore) Put(ctx context.Context, reader io.Reader, limit int64) (BlobInfo, error) {
	tmp, err := os.CreateTemp(s.root, "blob-*.tmp")
	if err != nil {
		return BlobInfo{}, fmt.Errorf("artifact: create temp failed: %w", err)
	}
	tmpName := tmp.Name()
	h := sha256.New()
	lr := io.LimitReader(reader, limit+1)
	written, err := io.Copy(io.MultiWriter(tmp, h), lr)
	tmp.Sync()
	tmp.Close()
	if err != nil {
		os.Remove(tmpName)
		return BlobInfo{}, fmt.Errorf("artifact: write temp failed: %w", err)
	}
	if written > limit {
		os.Remove(tmpName)
		return BlobInfo{}, fmt.Errorf("artifact: blob exceeds limit %d", limit)
	}
	digest := BlobDigest("sha256:" + hex.EncodeToString(h.Sum(nil)))
	canonicalPath := s.blobPath(digest)
	if _, err := os.Stat(canonicalPath); err == nil {
		os.Remove(tmpName)
		return BlobInfo{Digest: digest, SizeBytes: written}, nil
	}
	if err := os.MkdirAll(filepath.Dir(canonicalPath), 0700); err != nil {
		os.Remove(tmpName)
		return BlobInfo{}, fmt.Errorf("artifact: mkdir failed: %w", err)
	}
	if err := os.Rename(tmpName, canonicalPath); err != nil {
		os.Remove(tmpName)
		return BlobInfo{}, fmt.Errorf("artifact: rename failed: %w", err)
	}
	verifyFile, err := os.Open(canonicalPath)
	if err != nil {
		os.Remove(canonicalPath)
		return BlobInfo{}, fmt.Errorf("artifact: verify open failed: %w", err)
	}
	verifyHash := sha256.New()
	_, err = io.Copy(verifyHash, verifyFile)
	verifyFile.Close()
	if err != nil {
		os.Remove(canonicalPath)
		return BlobInfo{}, fmt.Errorf("artifact: verify read failed: %w", err)
	}
	verifyDigest := BlobDigest("sha256:" + hex.EncodeToString(verifyHash.Sum(nil)))
	if verifyDigest != digest {
		os.Remove(canonicalPath)
		return BlobInfo{}, fmt.Errorf("artifact: integrity mismatch: expected %s, got %s", digest, verifyDigest)
	}
	return BlobInfo{Digest: digest, SizeBytes: written}, nil
}

func (s *FilesystemBlobStore) Open(ctx context.Context, digest BlobDigest) (io.ReadCloser, BlobInfo, error) {
	p := s.blobPath(digest)
	f, err := os.Open(p)
	if err != nil {
		return nil, BlobInfo{}, fmt.Errorf("artifact: open blob failed: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, BlobInfo{}, fmt.Errorf("artifact: stat blob failed: %w", err)
	}
	return f, BlobInfo{Digest: digest, SizeBytes: info.Size()}, nil
}

func (s *FilesystemBlobStore) Stat(ctx context.Context, digest BlobDigest) (BlobInfo, error) {
	info, err := os.Stat(s.blobPath(digest))
	if err != nil {
		return BlobInfo{}, fmt.Errorf("artifact: stat blob failed: %w", err)
	}
	return BlobInfo{Digest: digest, SizeBytes: info.Size()}, nil
}

func (s *FilesystemBlobStore) Delete(ctx context.Context, digest BlobDigest) error {
	return os.Remove(s.blobPath(digest))
}

func NormalizeDigest(input string) BlobDigest {
	d := strings.TrimSpace(input)
	if !strings.HasPrefix(d, "sha256:") {
		d = "sha256:" + d
	}
	return BlobDigest(strings.ToLower(d))
}
