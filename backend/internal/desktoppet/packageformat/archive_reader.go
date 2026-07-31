package packageformat

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const archiveManifestName = "manifest.json"

type ArchiveLimits struct {
	MaxCompressedSize   int64
	MaxUncompressedSize int64
	MaxSingleFileSize   int64
	MaxFileCount        int
	MaxDirDepth         int
	MaxPathLength       int
	MaxCompressionRatio float64
}

func DefaultArchiveLimits() ArchiveLimits {
	return ArchiveLimits{
		MaxCompressedSize:   500 * 1024 * 1024,
		MaxUncompressedSize: 2 * 1024 * 1024 * 1024,
		MaxSingleFileSize:   100 * 1024 * 1024,
		MaxFileCount:        10000,
		MaxDirDepth:         32,
		MaxPathLength:       512,
		MaxCompressionRatio: 100.0,
	}
}

type ArchiveReader struct {
	limits ArchiveLimits
}

func NewArchiveReader(limits ArchiveLimits) *ArchiveReader {
	return &ArchiveReader{limits: limits}
}

func (r *ArchiveReader) ReadArchive(path string) (*Manifest, []byte, error) {
	rc, manifestData, manifest, err := r.openArchiveInternal(path)
	if err != nil {
		return nil, nil, err
	}
	if rc != nil {
		rc.Close()
	}
	return manifest, manifestData, nil
}

func (r *ArchiveReader) OpenArchive(path string) (*zip.ReadCloser, *Manifest, error) {
	rc, _, manifest, err := r.openArchiveInternal(path)
	if err != nil {
		return nil, nil, err
	}
	return rc, manifest, nil
}

func (r *ArchiveReader) openArchiveInternal(path string) (*zip.ReadCloser, []byte, *Manifest, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, nil, NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("archive not found: %s", path), err)
	}
	if info.Size() > r.limits.MaxCompressedSize {
		return nil, nil, nil, NewPackageError(ErrCodePackageArchiveBomb, "compressed size exceeds limit", nil)
	}

	zipReader, err := zip.OpenReader(path)
	if err != nil {
		return nil, nil, nil, NewPackageError(ErrCodePackageManifestInvalid, "failed to open zip archive", err)
	}

	if err := r.validateArchiveStructure(&zipReader.Reader); err != nil {
		zipReader.Close()
		return nil, nil, nil, err
	}

	var manifestData []byte
	var totalUncompressed int64
	fileCount := 0
	seenPaths := make(map[string]bool)
	seenCaseFold := make(map[string]string)

	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			continue
		}

		fileCount++
		if fileCount > r.limits.MaxFileCount {
			zipReader.Close()
			return nil, nil, nil, NewPackageError(ErrCodePackageArchiveBomb, "file count exceeds limit", nil)
		}

		if err := r.validateZipEntry(f, seenPaths, seenCaseFold); err != nil {
			zipReader.Close()
			return nil, nil, nil, err
		}

		normalized := filepath.ToSlash(filepath.Clean(f.Name))

		if len(normalized) > r.limits.MaxPathLength {
			zipReader.Close()
			return nil, nil, nil, NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("path too long: %s", normalized), nil)
		}

		depth := strings.Count(normalized, "/") + 1
		if depth > r.limits.MaxDirDepth {
			zipReader.Close()
			return nil, nil, nil, NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("directory depth exceeds limit: %s", normalized), nil)
		}

		if f.UncompressedSize64 > uint64(r.limits.MaxSingleFileSize) {
			zipReader.Close()
			return nil, nil, nil, NewPackageError(ErrCodePackageArchiveBomb, fmt.Sprintf("single file too large: %s", normalized), nil)
		}

		totalUncompressed += int64(f.UncompressedSize64)
		if totalUncompressed > r.limits.MaxUncompressedSize {
			zipReader.Close()
			return nil, nil, nil, NewPackageError(ErrCodePackageArchiveBomb, "total uncompressed size exceeds limit", nil)
		}

		if r.limits.MaxCompressionRatio > 0 && f.CompressedSize64 > 0 && f.UncompressedSize64 > 0 {
			ratio := float64(f.UncompressedSize64) / float64(f.CompressedSize64)
			if ratio > r.limits.MaxCompressionRatio {
				zipReader.Close()
				return nil, nil, nil, NewPackageError(ErrCodePackageArchiveBomb, fmt.Sprintf("compression ratio too high: %s (%.1f:1)", normalized, ratio), nil)
			}
		}

		if normalized == archiveManifestName {
			rc, openErr := f.Open()
			if openErr != nil {
				zipReader.Close()
				return nil, nil, nil, NewPackageError(ErrCodePackageManifestInvalid, "failed to open manifest in archive", openErr)
			}
			data, readErr := io.ReadAll(rc)
			rc.Close()
			if readErr != nil {
				zipReader.Close()
				return nil, nil, nil, NewPackageError(ErrCodePackageManifestInvalid, "failed to read manifest from archive", readErr)
			}
			manifestData = data
		}
	}

	if manifestData == nil {
		zipReader.Close()
		return nil, nil, nil, NewPackageError(ErrCodePackageManifestInvalid, "manifest.json not found in archive", nil)
	}

	registry := NewSchemaRegistry()
	manifest, err := registry.ReadManifest(manifestData)
	if err != nil {
		zipReader.Close()
		return nil, nil, nil, err
	}

	return zipReader, manifestData, manifest, nil
}

func (r *ArchiveReader) validateArchiveStructure(zr *zip.Reader) error {
	for _, f := range zr.File {
		if f.Flags&0x01 != 0 {
			return NewPackageError(ErrCodePackageManifestInvalid, fmt.Sprintf("encrypted entry not allowed: %s", f.Name), nil)
		}
	}

	if len(zr.File) == 0 {
		return NewPackageError(ErrCodePackageManifestInvalid, "archive is empty", nil)
	}

	return nil
}

func (r *ArchiveReader) validateZipEntry(f *zip.File, seenPaths map[string]bool, seenCaseFold map[string]string) error {
	name := f.Name
	if name == "" {
		return NewPackageError(ErrCodePackagePathInvalid, "empty entry name in archive", nil)
	}

	if filepath.IsAbs(name) {
		return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("absolute path in archive: %s", name), nil)
	}

	cleaned := filepath.Clean(name)
	if strings.HasPrefix(cleaned, "..") || cleaned == ".." {
		return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("path traversal in archive: %s", name), nil)
	}

	if strings.Contains(name, "\\") {
		return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("backslash in archive path: %s", name), nil)
	}

	normalized := filepath.ToSlash(cleaned)
	if _, err := NormalizePackagePath(normalized); err != nil {
		return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("unsafe path in archive: %s", name), err)
	}

	if seenPaths[normalized] {
		return NewPackageError(ErrCodePackageDuplicateEntry, fmt.Sprintf("duplicate entry in archive: %s", normalized), nil)
	}
	seenPaths[normalized] = true

	folded := CaseFoldPath(normalized)
	if existing, dup := seenCaseFold[folded]; dup {
		return NewPackageError(ErrCodePackageDuplicateEntry, fmt.Sprintf("case-insensitive path collision: %s conflicts with %s", normalized, existing), nil)
	}
	seenCaseFold[folded] = normalized

	mode := f.FileInfo().Mode()
	if mode&os.ModeSymlink != 0 {
		return NewPackageError(ErrCodePackageSymlinkForbidden, fmt.Sprintf("symlink in archive: %s", normalized), nil)
	}
	if mode&os.ModeDevice != 0 {
		return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("device file in archive: %s", normalized), nil)
	}
	if mode&os.ModeNamedPipe != 0 {
		return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("named pipe in archive: %s", normalized), nil)
	}
	if mode&os.ModeSocket != 0 {
		return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("socket in archive: %s", normalized), nil)
	}

	if isForbiddenExecutable(normalized) {
		return NewPackageError(ErrCodePackageExecutableForbidden, fmt.Sprintf("executable file in archive: %s", normalized), nil)
	}

	return nil
}

func (r *ArchiveReader) ExtractArchive(path, destDir string) error {
	zipReader, err := zip.OpenReader(path)
	if err != nil {
		return NewPackageError(ErrCodePackageManifestInvalid, "failed to open zip archive for extraction", err)
	}
	defer zipReader.Close()

	if err := r.validateArchiveStructure(&zipReader.Reader); err != nil {
		return err
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("failed to create destination: %s", destDir), err)
	}

	seenPaths := make(map[string]bool)
	seenCaseFold := make(map[string]string)
	var totalUncompressed int64
	fileCount := 0

	for _, f := range zipReader.File {
		if err := r.validateZipEntry(f, seenPaths, seenCaseFold); err != nil {
			return err
		}

		normalized := filepath.ToSlash(filepath.Clean(f.Name))

		if f.FileInfo().IsDir() {
			dirPath, secErr := SecureJoinUnderRoot(destDir, normalized)
			if secErr != nil {
				return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("unsafe directory path: %s", normalized), secErr)
			}
			if err := os.MkdirAll(dirPath, 0o755); err != nil {
				return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("failed to create directory: %s", normalized), err)
			}
			continue
		}

		fileCount++
		if fileCount > r.limits.MaxFileCount {
			return NewPackageError(ErrCodePackageArchiveBomb, "file count exceeds limit during extraction", nil)
		}

		totalUncompressed += int64(f.UncompressedSize64)
		if totalUncompressed > r.limits.MaxUncompressedSize {
			return NewPackageError(ErrCodePackageArchiveBomb, "total uncompressed size exceeds limit during extraction", nil)
		}

		targetPath, secErr := SecureJoinUnderRoot(destDir, normalized)
		if secErr != nil {
			return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("unsafe file path: %s", normalized), secErr)
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("failed to create parent directory for: %s", normalized), err)
		}

		rc, openErr := f.Open()
		if openErr != nil {
			return NewPackageError(ErrCodePackageManifestInvalid, fmt.Sprintf("failed to open entry: %s", normalized), openErr)
		}

		outFile, createErr := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if createErr != nil {
			rc.Close()
			return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("failed to create file: %s", normalized), createErr)
		}

		_, copyErr := io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if copyErr != nil {
			return NewPackageError(ErrCodePackageManifestInvalid, fmt.Sprintf("failed to extract file: %s", normalized), copyErr)
		}
	}

	return nil
}

func readManifestFromArchive(zr *zip.Reader) ([]byte, error) {
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		normalized := filepath.ToSlash(filepath.Clean(f.Name))
		if normalized == archiveManifestName {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("manifest.json not found")
}

func parseManifestFromBytes(data []byte) (*Manifest, error) {
	registry := NewSchemaRegistry()
	return registry.ReadManifest(data)
}
