package package_security

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
)

type SecureExtractor struct {
	policy       ArchivePolicy
	pathResolver *SafePathResolver
	hasher       *ContentHasher
}

func NewSecureExtractor(policy ArchivePolicy) *SecureExtractor {
	return &SecureExtractor{
		policy:       policy,
		pathResolver: NewSafePathResolver(policy.MaxPathLength, policy.MaxDirectoryDepth),
		hasher:       NewContentHasher(),
	}
}

func (e *SecureExtractor) Extract(ctx context.Context, raw []byte, targetRoot string) ([]ArchiveEntryInfo, error) {
	if err := os.MkdirAll(targetRoot, 0o700); err != nil {
		return nil, err
	}

	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}

	var entries []ArchiveEntryInfo

	for _, item := range reader.File {
		if item.NonUTF8 {
			continue
		}

		normalized, err := e.pathResolver.NormalizeArchivePath(item.Name)
		if err != nil {
			return nil, err
		}

		if item.FileInfo().IsDir() || item.Mode()&os.ModeDir != 0 {
			continue
		}

		if item.Mode()&os.ModeSymlink != 0 && !e.policy.AllowSymlink {
			return nil, ErrSymlinkNotAllowed
		}

		if item.Mode()&os.ModeDevice != 0 || item.Mode()&os.ModeCharDevice != 0 {
			return nil, ErrSpecialFileNotAllowed
		}

		resolved, err := e.pathResolver.ResolveWithinRoot(targetRoot, normalized)
		if err != nil {
			return nil, err
		}

		parentDir := filepath.Dir(resolved)
		if err := os.MkdirAll(parentDir, 0o700); err != nil {
			return nil, err
		}

		rc, err := item.Open()
		if err != nil {
			return nil, err
		}

		content, err := io.ReadAll(io.LimitReader(rc, e.policy.MaxSingleEntryBytes+1))
		rc.Close()
		if err != nil {
			return nil, err
		}

		if int64(len(content)) > e.policy.MaxSingleEntryBytes {
			return nil, ErrSizeLimitExceeded
		}

		f, err := os.OpenFile(resolved, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o400)
		if err != nil {
			return nil, err
		}

		if _, err := f.Write(content); err != nil {
			f.Close()
			os.Remove(resolved)
			return nil, err
		}
		f.Close()

		if err := os.Chmod(resolved, 0o400); err != nil {
			return nil, err
		}

		entryHash := e.hasher.HashEntry(content)

		entries = append(entries, ArchiveEntryInfo{
			Path:             item.Name,
			NormalizedPath:   string(normalized),
			Kind:             EntryKindFile,
			CompressedSize:   int64(item.CompressedSize64),
			UncompressedSize: int64(item.UncompressedSize64),
			Mode:             uint32(item.Mode()),
			Hash:             entryHash,
			CRC32:            item.CRC32,
		})
	}

	return entries, nil
}
