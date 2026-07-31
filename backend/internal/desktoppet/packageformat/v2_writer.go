package packageformat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type V2Writer struct{}

func (w *V2Writer) SchemaVersion() int { return 2 }

func (w *V2Writer) WriteManifest(manifest *Manifest) ([]byte, error) {
	_, data, err := w.FinalizeManifest(manifest)
	return data, err
}

func (w *V2Writer) FinalizeManifest(manifest *Manifest) (*Manifest, []byte, error) {
	if manifest == nil {
		return nil, nil, NewPackageError(ErrCodePackageManifestInvalid, "manifest is nil", nil)
	}

	clone := deepCopyManifest(manifest)
	clone.SchemaVersion = ManifestSchemaVersion
	clone.ManifestFormat = ManifestFormatCanonical
	clone.Integrity.Algorithm = IntegrityAlgorithmV2

	canonicalData, err := CanonicalManifestData(clone)
	if err != nil {
		return nil, nil, NewPackageError(ErrCodePackageManifestInvalid, "failed to compute canonical manifest data", err)
	}
	hashDigest := sha256.Sum256(canonicalData)
	manifestHash := hex.EncodeToString(hashDigest[:])
	manifestBytes := int64(len(canonicalData))

	var entries []FileEntry
	for _, f := range clone.Integrity.Files {
		entries = append(entries, FileEntry{
			Path:   f.Path,
			SHA256: f.SHA256,
			Bytes:  f.Bytes,
		})
	}
	contentRootHash := ComputeContentRootHash(entries, manifestHash, manifestBytes)

	clone.Integrity.ManifestHash = manifestHash
	clone.Integrity.ContentRootHash = contentRootHash

	data, err := json.MarshalIndent(clone, "", "  ")
	if err != nil {
		return nil, nil, NewPackageError(ErrCodePackageManifestInvalid, "failed to marshal manifest", err)
	}

	return clone, data, nil
}

func deepCopyManifest(m *Manifest) *Manifest {
	clone := *m
	if m.Actions != nil {
		clone.Actions = make([]ManifestActionEntry, len(m.Actions))
		copy(clone.Actions, m.Actions)
	}
	if m.Integrity.Files != nil {
		clone.Integrity.Files = make([]FileManifestEntry, len(m.Integrity.Files))
		copy(clone.Integrity.Files, m.Integrity.Files)
	}
	if m.Compatibility.MaxRuntimeVersion != nil {
		v := *m.Compatibility.MaxRuntimeVersion
		clone.Compatibility.MaxRuntimeVersion = &v
	}
	return &clone
}

func (w *V2Writer) WriteManifestToFile(manifest *Manifest, path string) error {
	data, err := w.WriteManifest(manifest)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("failed to create directory: %s", dir), err)
		}
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return NewPackageError(ErrCodePackagePathInvalid, fmt.Sprintf("failed to write manifest file: %s", path), err)
	}

	return nil
}
