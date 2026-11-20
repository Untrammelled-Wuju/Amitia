package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

var ErrBlobTooLarge = errors.New("artifact: blob exceeds limit")

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
	hex := d[7:]
	if len(hex) < 4 {
		return filepath.Join(s.root, hex)
	}
	return filepath.Join(s.root, hex[0:2], hex[2:4], hex)
}

func (s *FilesystemBlobStore) Put(ctx context.Context, reader io.Reader, limit int64) (BlobInfo, error) {
	tmp, err := os.CreateTemp(s.root, "blob-*.tmp")
	if err != nil {
		return BlobInfo{}, fmt.Errorf("artifact: create temp failed: %w", err)
	}
	tmpName := tmp.Name()
	h := sha256.New()
	var source io.Reader = &contextReader{ctx: ctx, reader: reader}
	if limit >= 0 && limit < math.MaxInt64 {
		source = io.LimitReader(source, limit+1)
	}
	written, err := io.Copy(io.MultiWriter(tmp, h), source)
	if err != nil {
		closeErr := tmp.Close()
		removeErr := os.Remove(tmpName)
		return BlobInfo{}, fmt.Errorf("artifact: write temp failed: %w", errors.Join(err, closeErr, removeErr))
	}
	if err := tmp.Sync(); err != nil {
		closeErr := tmp.Close()
		removeErr := os.Remove(tmpName)
		return BlobInfo{}, fmt.Errorf("artifact: sync temp failed: %w", errors.Join(err, closeErr, removeErr))
	}
	if err := tmp.Close(); err != nil {
		return BlobInfo{}, fmt.Errorf("artifact: close temp failed: %w", errors.Join(err, os.Remove(tmpName)))
	}
	if limit >= 0 && written > limit {
		return BlobInfo{}, errors.Join(fmt.Errorf("%w: %d", ErrBlobTooLarge, limit), os.Remove(tmpName))
	}
	digest := BlobDigest("sha256:" + hex.EncodeToString(h.Sum(nil)))
	verifyFile, err := os.Open(tmpName)
	if err != nil {
		return BlobInfo{}, fmt.Errorf("artifact: verify open failed: %w", errors.Join(err, os.Remove(tmpName)))
	}
	verifyHash := sha256.New()
	_, readErr := io.Copy(verifyHash, &contextReader{ctx: ctx, reader: verifyFile})
	closeErr := verifyFile.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return BlobInfo{}, fmt.Errorf("artifact: verify read failed: %w", errors.Join(err, os.Remove(tmpName)))
	}
	verifyDigest := BlobDigest("sha256:" + hex.EncodeToString(verifyHash.Sum(nil)))
	if verifyDigest != digest {
		return BlobInfo{}, errors.Join(fmt.Errorf("artifact: integrity mismatch: expected %s, got %s", digest, verifyDigest), os.Remove(tmpName))
	}
	if err := ctx.Err(); err != nil {
		return BlobInfo{}, fmt.Errorf("artifact: upload canceled before commit: %w", errors.Join(err, os.Remove(tmpName)))
	}
	canonicalPath := s.blobPath(digest)
	if _, err := os.Stat(canonicalPath); err == nil {
		if err := os.Remove(tmpName); err != nil {
			return BlobInfo{}, fmt.Errorf("artifact: remove duplicate temp failed: %w", err)
		}
		return BlobInfo{Digest: digest, SizeBytes: written}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return BlobInfo{}, fmt.Errorf("artifact: stat blob failed: %w", errors.Join(err, os.Remove(tmpName)))
	}
	if err := os.MkdirAll(filepath.Dir(canonicalPath), 0700); err != nil {
		return BlobInfo{}, fmt.Errorf("artifact: mkdir failed: %w", errors.Join(err, os.Remove(tmpName)))
	}
	if err := os.Rename(tmpName, canonicalPath); err != nil {
		return BlobInfo{}, fmt.Errorf("artifact: rename failed: %w", errors.Join(err, os.Remove(tmpName)))
	}
	return BlobInfo{Digest: digest, SizeBytes: written}, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func (s *FilesystemBlobStore) Open(ctx context.Context, digest BlobDigest) (io.ReadSeekCloser, BlobInfo, error) {
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
