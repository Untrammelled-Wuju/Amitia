package packageformat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

type SchemaReader interface {
	SchemaVersion() int
	ReadManifest(data []byte) (*Manifest, error)
}

type SchemaRegistry struct {
	readers map[int]SchemaReader
}

func NewSchemaRegistry() *SchemaRegistry {
	registry := &SchemaRegistry{
		readers: make(map[int]SchemaReader),
	}
	registry.Register(&V1Reader{})
	registry.Register(&V2Reader{})
	return registry
}

func (r *SchemaRegistry) Register(reader SchemaReader) {
	r.readers[reader.SchemaVersion()] = reader
}

func (r *SchemaRegistry) ReadManifest(data []byte) (*Manifest, error) {
	if len(data) == 0 {
		return nil, NewPackageError(ErrCodePackageManifestInvalid, "manifest data is empty", nil)
	}

	var probe struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, NewPackageError(ErrCodePackageManifestInvalid, "failed to parse schemaVersion", err)
	}

	if probe.SchemaVersion == 0 {
		return nil, NewPackageError(
			ErrCodePackageSchemaMissing,
			"schemaVersion is missing or zero",
			nil,
		)
	}

	reader, ok := r.readers[probe.SchemaVersion]
	if !ok {
		return nil, NewPackageError(
			ErrCodePackageSchemaUnsupported,
			fmt.Sprintf("unsupported schemaVersion: %d", probe.SchemaVersion),
			nil,
		)
	}

	return reader.ReadManifest(data)
}

func (r *SchemaRegistry) SupportedVersions() []int {
	versions := make([]int, 0, len(r.readers))
	for v := range r.readers {
		versions = append(versions, v)
	}
	sort.Ints(versions)
	return versions
}

type V2Reader struct{}

func (r *V2Reader) SchemaVersion() int { return 2 }

func (r *V2Reader) ReadManifest(data []byte) (*Manifest, error) {
	var manifest Manifest
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&manifest); err != nil {
		return nil, NewPackageError(ErrCodePackageManifestInvalid, "failed to unmarshal v2 manifest", err)
	}
	if manifest.SchemaVersion != ManifestSchemaVersion {
		return nil, NewPackageError(
			ErrCodePackageSchemaUnsupported,
			fmt.Sprintf("expected schemaVersion %d, got %d", ManifestSchemaVersion, manifest.SchemaVersion),
			nil,
		)
	}
	return &manifest, nil
}
