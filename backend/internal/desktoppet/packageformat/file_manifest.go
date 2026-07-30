package packageformat

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FileManifest struct {
	Entries []FileManifestEntry
}

func BuildFileManifestFromDir(root string) (*FileManifest, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("root directory not accessible: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root is not a directory: %s", root)
	}

	var entries []FileManifestEntry
	var collectedPaths []string
	fileData := make(map[string]os.FileInfo)

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		relSlash := filepath.ToSlash(rel)

		normalized, normErr := NormalizePackagePath(relSlash)
		if normErr != nil {
			return fmt.Errorf("unsafe path %q: %w", relSlash, normErr)
		}

		fi, statErr := d.Info()
		if statErr != nil {
			return statErr
		}

		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink not allowed: %s", normalized)
		}

		collectedPaths = append(collectedPaths, normalized)
		fileData[normalized] = fi
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(collectedPaths)

	for _, relPath := range collectedPaths {
		absPath := filepath.Join(root, filepath.FromSlash(relPath))
		data, readErr := os.ReadFile(absPath)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read %s: %w", relPath, readErr)
		}

		h := sha256.Sum256(data)
		hashHex := hex.EncodeToString(h[:])

		mediaType := DetectMediaType(relPath, data)
		role := inferFileRole(relPath)

		entry := FileManifestEntry{
			Path:      relPath,
			SHA256:    hashHex,
			Bytes:     int64(len(data)),
			MediaType: mediaType,
			Role:      role,
		}

		if strings.HasPrefix(relPath, "actions/") {
			parts := strings.Split(relPath, "/")
			if len(parts) >= 2 {
				entry.ActionKey = parts[1]
			}
			if strings.HasPrefix(relPath, "actions/") && strings.Contains(relPath, "/frames/") {
				parts := strings.Split(relPath, "/")
				if len(parts) >= 4 {
					entry.Role = FileRoleFrame
					entry.FrameID = strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(parts[len(parts)-1]))
				}
			}
		}

		entries = append(entries, entry)
	}

	return &FileManifest{Entries: entries}, nil
}

func inferFileRole(relPath string) string {
	if relPath == "manifest.json" {
		return FileRoleManifest
	}
	if relPath == "preview.png" || relPath == "preview.jpg" || relPath == "preview.webp" {
		return FileRolePreview
	}
	if strings.HasSuffix(relPath, "action.json") {
		return FileRoleActionConfig
	}
	if strings.HasPrefix(relPath, "actions/") && strings.Contains(relPath, "/frames/") {
		return FileRoleFrame
	}
	if strings.HasSuffix(relPath, ".json") {
		return FileRoleMetadata
	}
	return FileRoleAsset
}

func ValidateAgainstManifest(root string, manifest *FileManifest) error {
	if manifest == nil {
		return fmt.Errorf("manifest is nil")
	}

	declared := make(map[string]*FileManifestEntry, len(manifest.Entries))
	for i := range manifest.Entries {
		e := &manifest.Entries[i]
		if existing, dup := declared[e.Path]; dup {
			return NewValidationError(
				ErrCodePackageDuplicateEntry,
				SeverityError,
				e.Path,
				fmt.Sprintf("duplicate manifest entry: %s also declared at %s", e.Path, existing.Path),
			)
		}
		declared[e.Path] = e
	}

	actualPaths := make(map[string]bool)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		relSlash := filepath.ToSlash(rel)
		actualPaths[relSlash] = true
		return nil
	})
	if err != nil {
		return err
	}

	for relPath := range actualPaths {
		if _, ok := declared[relPath]; !ok {
			return NewValidationError(
				ErrCodePackageFileUndeclared,
				SeverityError,
				relPath,
				fmt.Sprintf("file exists on disk but not declared in manifest: %s", relPath),
			)
		}
	}

	for i := range manifest.Entries {
		e := &manifest.Entries[i]
		absPath := filepath.Join(root, filepath.FromSlash(e.Path))
		if !actualPaths[e.Path] {
			return NewValidationError(
				ErrCodePackageFileMissing,
				SeverityError,
				e.Path,
				fmt.Sprintf("file declared in manifest but missing on disk: %s", e.Path),
			)
		}

		f, openErr := os.Open(absPath)
		if openErr != nil {
			return fmt.Errorf("failed to open %s: %w", e.Path, openErr)
		}
		h := sha256.New()
		size, copyErr := io.Copy(h, f)
		f.Close()
		if copyErr != nil {
			return fmt.Errorf("failed to hash %s: %w", e.Path, copyErr)
		}

		actualHash := hex.EncodeToString(h.Sum(nil))
		if actualHash != e.SHA256 {
			return NewValidationError(
				ErrCodePackageHashMismatch,
				SeverityError,
				e.Path,
				fmt.Sprintf("hash mismatch for %s: expected %s, got %s", e.Path, e.SHA256, actualHash),
			)
		}

		if size != e.Bytes {
			return NewValidationError(
				ErrCodePackageHashMismatch,
				SeverityError,
				e.Path,
				fmt.Sprintf("size mismatch for %s: expected %d, got %d", e.Path, e.Bytes, size),
			)
		}
	}

	return nil
}

func DetectMediaType(path string, data []byte) string {
	if len(data) >= 8 {
		if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 &&
			data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A {
			return "image/png"
		}
	}

	if len(data) >= 12 {
		if data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
			data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
			return "image/webp"
		}
	}

	if len(data) >= 3 {
		if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
			return "image/jpeg"
		}
	}

	if len(data) >= 4 {
		if data[0] == 0x7B || (data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF && data[3] == 0x7B) {
			trimmed := strings.TrimSpace(string(data))
			if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
				return "application/json"
			}
		}
	}

	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".json"):
		return "application/json"
	default:
		return "application/octet-stream"
	}
}
