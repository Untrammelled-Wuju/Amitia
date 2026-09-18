package package_security

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type SecureExtractor struct {
	policy         ArchivePolicy
	pathResolver   *SafePathResolver
	hasher         *ContentHasher
	entryValidator *EntryValidator
}

func NewSecureExtractor(policy ArchivePolicy) *SecureExtractor {
	return &SecureExtractor{
		policy:         policy,
		pathResolver:   NewSafePathResolver(policy.MaxPathLength, policy.MaxDirectoryDepth),
		hasher:         NewContentHasher(),
		entryValidator: NewEntryValidator(policy),
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
	return e.extractReader(ctx, reader, targetRoot)
}

func (e *SecureExtractor) ExtractFile(ctx context.Context, archivePath, targetRoot string) ([]ArchiveEntryInfo, error) {
	if err := os.MkdirAll(targetRoot, 0o700); err != nil {
		return nil, err
	}
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return e.extractReader(ctx, &reader.Reader, targetRoot)
}

func (e *SecureExtractor) extractReader(ctx context.Context, reader *zip.Reader, targetRoot string) ([]ArchiveEntryInfo, error) {

	var entries []ArchiveEntryInfo
	declaredExecutables := discoverDeclaredServiceExecutables(reader, e.policy)
	entryValidator := NewEntryValidatorWithDeclaredExecutables(e.policy, declaredExecutables)

	for _, item := range reader.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
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

		content, err := io.ReadAll(limitReader(rc, e.policy.MaxSingleEntryBytes))
		rc.Close()
		if err != nil {
			return nil, err
		}

		if int64(len(content)) > e.policy.MaxSingleEntryBytes {
			return nil, ErrSizeLimitExceeded
		}

		entry := ArchiveEntryInfo{
			Path:             item.Name,
			NormalizedPath:   string(normalized),
			Kind:             EntryKindFile,
			CompressedSize:   int64(item.CompressedSize64),
			UncompressedSize: int64(item.UncompressedSize64),
			Mode:             uint32(item.Mode()),
			CRC32:            item.CRC32,
		}
		validation := entryValidator.Validate(entry, content)
		if !validation.Passed {
			return nil, ErrForbiddenFileType
		}

		mode := os.FileMode(0o400)
		if _, ok := declaredExecutables[strings.ToLower(strings.ReplaceAll(entry.NormalizedPath, "\\", "/"))]; ok {
			mode = 0o500
		}
		f, err := os.OpenFile(resolved, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			return nil, err
		}

		if _, err := f.Write(content); err != nil {
			f.Close()
			os.Remove(resolved)
			return nil, err
		}
		f.Close()

		if err := os.Chmod(resolved, mode); err != nil {
			return nil, err
		}

		entryHash := e.hasher.HashEntry(content)

		entry.Hash = entryHash
		entries = append(entries, entry)
	}

	return entries, nil
}
