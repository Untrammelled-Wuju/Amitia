package package_security

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"sort"
	"strings"
)

type ArchiveInspector struct {
	policy         ArchivePolicy
	pathResolver   *SafePathResolver
	entryValidator *EntryValidator
}

func NewArchiveInspector(policy ArchivePolicy) *ArchiveInspector {
	return &ArchiveInspector{
		policy:         policy,
		pathResolver:   NewSafePathResolver(policy.MaxPathLength, policy.MaxDirectoryDepth),
		entryValidator: NewEntryValidator(policy),
	}
}

type ArchiveInspectionResult struct {
	TotalCompressed   int64
	TotalUncompressed int64
	CompressionRatio  float64
	EntryCount        int
	Entries           []ArchiveEntryInfo
	PathCollisions    []PathCollision
	Errors            []string
	Warnings          []string
	Passed            bool
}

func (i *ArchiveInspector) Inspect(ctx context.Context, raw []byte) (*ArchiveInspectionResult, error) {
	if int64(len(raw)) > i.policy.MaxArchiveBytes {
		return &ArchiveInspectionResult{Passed: false, Errors: []string{"archive exceeds max size"}}, ErrSizeLimitExceeded
	}

	if len(raw) < 4 || !bytes.Equal(raw[:2], []byte{'P', 'K'}) {
		return &ArchiveInspectionResult{Passed: false, Errors: []string{"not a valid ZIP archive"}}, ErrInvalidArchive
	}

	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return &ArchiveInspectionResult{Passed: false, Errors: []string{err.Error()}}, ErrInvalidArchive
	}

	if len(reader.File) > i.policy.MaxEntryCount {
		return &ArchiveInspectionResult{
			Passed:     false,
			EntryCount: len(reader.File),
			Errors:     []string{"entry count exceeds limit"},
		}, ErrEntryCountExceeded
	}

	result := &ArchiveInspectionResult{Passed: true}
	canonical := make(map[string]bool)

	for _, item := range reader.File {
		if item.NonUTF8 {
			result.Errors = append(result.Errors, "non-UTF8 path: "+item.Name)
			result.Passed = false
			continue
		}

		normalized, err := i.pathResolver.NormalizeArchivePath(item.Name)
		if err != nil {
			result.Errors = append(result.Errors, "path validation failed for "+item.Name+": "+err.Error())
			result.Passed = false
			continue
		}

		if strings.HasSuffix(item.Name, "/") {
			continue
		}

		mode := item.Mode()
		if mode&os.ModeType != 0 || mode.IsDir() {
			result.Errors = append(result.Errors, "special file or directory: "+string(normalized))
			result.Passed = false
			continue
		}

		key := strings.ToLower(string(normalized))
		if canonical[key] {
			result.PathCollisions = append(result.PathCollisions, PathCollision{
				PathA:  string(normalized),
				PathB:  string(normalized),
				Reason: "duplicate_path",
			})
			result.Passed = false
			continue
		}
		canonical[key] = true

		if item.UncompressedSize64 > uint64(i.policy.MaxSingleEntryBytes) {
			result.Errors = append(result.Errors, "entry exceeds max size: "+string(normalized))
			result.Passed = false
			continue
		}

		if item.CompressedSize64 > 0 && item.UncompressedSize64 > 0 {
			ratio := float64(item.UncompressedSize64) / float64(item.CompressedSize64)
			if ratio > i.policy.MaxCompressionRatio {
				result.Errors = append(result.Errors, "compression ratio exceeded: "+string(normalized))
				result.Passed = false
				continue
			}
		}

		result.TotalCompressed += int64(item.CompressedSize64)
		result.TotalUncompressed += int64(item.UncompressedSize64)

		entry := ArchiveEntryInfo{
			Path:             item.Name,
			NormalizedPath:   string(normalized),
			Kind:             EntryKindFile,
			CompressedSize:   int64(item.CompressedSize64),
			UncompressedSize: int64(item.UncompressedSize64),
			Mode:             uint32(mode),
			CRC32:            item.CRC32,
		}
		result.Entries = append(result.Entries, entry)
	}

	if result.TotalUncompressed > i.policy.MaxTotalUncompressedBytes {
		result.Errors = append(result.Errors, "total uncompressed size exceeds limit")
		result.Passed = false
	}

	result.EntryCount = len(result.Entries)

	if result.TotalCompressed > 0 {
		result.CompressionRatio = float64(result.TotalUncompressed) / float64(result.TotalCompressed)
	}

	collisions := i.pathResolver.DetectCollision(normalizedPaths(result.Entries), PlatformWindows)
	result.PathCollisions = append(result.PathCollisions, collisions...)
	if len(collisions) > 0 {
		result.Errors = append(result.Errors, "path collisions detected")
		result.Passed = false
	}

	sort.Slice(result.Entries, func(i, j int) bool {
		return result.Entries[i].NormalizedPath < result.Entries[j].NormalizedPath
	})

	return result, nil
}

func (i *ArchiveInspector) ReadEntry(raw []byte, entry ArchiveEntryInfo) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}

	for _, item := range reader.File {
		if item.Name == entry.Path {
			rc, openErr := item.Open()
			if openErr != nil {
				return nil, openErr
			}
			defer rc.Close()
			return io.ReadAll(io.LimitReader(rc, i.policy.MaxSingleEntryBytes+1))
		}
	}

	return nil, ErrResourceNotFound
}

func normalizedPaths(entries []ArchiveEntryInfo) []NormalizedPath {
	paths := make([]NormalizedPath, len(entries))
	for i, e := range entries {
		paths[i] = NormalizedPath(e.NormalizedPath)
	}
	return paths
}
