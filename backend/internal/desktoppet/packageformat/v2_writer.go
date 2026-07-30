package packageformat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type V2Writer struct{}

func (w *V2Writer) SchemaVersion() int { return 2 }

func (w *V2Writer) WriteManifest(manifest *Manifest) ([]byte, error) {
	if manifest == nil {
		return nil, NewPackageError(ErrCodePackageManifestInvalid, "manifest is nil", nil)
	}

	if manifest.SchemaVersion != ManifestSchemaVersion {
		return nil, NewPackageError(
			ErrCodePackageSchemaUnsupported,
			fmt.Sprintf("expected schemaVersion %d, got %d", ManifestSchemaVersion, manifest.SchemaVersion),
			nil,
		)
	}

	if manifest.ManifestFormat == "" {
		manifest.ManifestFormat = ManifestFormatCanonical
	}

	if manifest.Integrity.Algorithm == "" {
		manifest.Integrity.Algorithm = TreeHashAlgorithm
	}

	manifest.SchemaVersion = ManifestSchemaVersion

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, NewPackageError(ErrCodePackageManifestInvalid, "failed to marshal manifest", err)
	}

	return data, nil
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
