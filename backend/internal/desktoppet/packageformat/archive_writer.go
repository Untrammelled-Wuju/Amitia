package packageformat

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const archiveFixedTimestamp = 0

type ArchiveWriter struct{}

func NewArchiveWriter() *ArchiveWriter {
	return &ArchiveWriter{}
}

func (w *ArchiveWriter) WriteArchive(root string, outputPath string) error {
	info, err := os.Stat(root)
	if err != nil {
		return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("root directory not found: %s", root), err)
	}
	if !info.IsDir() {
		return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("root is not a directory: %s", root), nil)
	}

	var relPaths []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		relSlash := filepath.ToSlash(rel)

		normalized, normErr := NormalizePackagePath(relSlash)
		if normErr != nil {
			return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("unsafe path: %s", relSlash), normErr)
		}

		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		fi, statErr := d.Info()
		if statErr != nil {
			return statErr
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return NewPackageError(ErrCodePackageSymlinkForbidden, fmt.Sprintf("symlink not allowed: %s", normalized), nil)
		}
		if isForbiddenExecutable(normalized) {
			return NewPackageError(ErrCodePackageExecutableForbidden, fmt.Sprintf("executable not allowed: %s", normalized), nil)
		}

		relPaths = append(relPaths, normalized)
		return nil
	})
	if err != nil {
		return err
	}

	sort.Strings(relPaths)

	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("failed to create output directory: %s", dir), err)
		}
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("failed to create archive: %s", outputPath), err)
	}
	defer outFile.Close()

	zw := zip.NewWriter(outFile)
	defer zw.Close()

	for _, relPath := range relPaths {
		absPath := filepath.Join(root, filepath.FromSlash(relPath))

		data, err := os.ReadFile(absPath)
		if err != nil {
			return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("failed to read file: %s", relPath), err)
		}

		method := zip.Deflate
		if len(data) < 512 {
			method = zip.Store
		}

		header := &zip.FileHeader{
			Name:   relPath,
			Method: method,
		}
		header.SetMode(0o644)
		header.SetModTime(time.Unix(archiveFixedTimestamp, 0))

		writer, err := zw.CreateHeader(header)
		if err != nil {
			return NewPackageError(ErrCodePackageManifestInvalid, fmt.Sprintf("failed to create zip entry: %s", relPath), err)
		}

		if _, err := writer.Write(data); err != nil {
			return NewPackageError(ErrCodePackageManifestInvalid, fmt.Sprintf("failed to write zip entry: %s", relPath), err)
		}
	}

	return nil
}

func (w *ArchiveWriter) BuildManifestForArchive(root string, baseManifest *Manifest) (*Manifest, error) {
	fileManifest, err := BuildFileManifestFromDir(root)
	if err != nil {
		return nil, err
	}

	var entries []FileEntry
	for _, e := range fileManifest.Entries {
		entries = append(entries, FileEntry{
			Path:   e.Path,
			SHA256: e.SHA256,
			Bytes:  e.Bytes,
		})
	}

	var totalBytes int64
	for _, e := range fileManifest.Entries {
		totalBytes += e.Bytes
	}

	if baseManifest == nil {
		baseManifest = NewManifest()
	}

	baseManifest.Integrity.Files = fileManifest.Entries
	baseManifest.Integrity.FileCount = len(fileManifest.Entries)
	baseManifest.Integrity.TotalBytes = totalBytes
	baseManifest.Integrity.Algorithm = IntegrityAlgorithmV2
	baseManifest.Integrity.ManifestHash = ""
	baseManifest.Integrity.ContentRootHash = ""

	manifestHash, err := CanonicalManifestHash(baseManifest)
	if err != nil {
		return nil, err
	}

	canonicalData, err := CanonicalManifestData(baseManifest)
	if err != nil {
		return nil, err
	}
	manifestBytes := int64(len(canonicalData))

	contentRootHash := ComputeContentRootHash(entries, manifestHash, manifestBytes)

	baseManifest.Integrity.ManifestHash = manifestHash
	baseManifest.Integrity.ContentRootHash = contentRootHash

	return baseManifest, nil
}

func (w *ArchiveWriter) ComputeFileSHA256(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
