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
	readers map[int][]SchemaReader
}

func NewSchemaRegistry() *SchemaRegistry {
	registry := &SchemaRegistry{
		readers: make(map[int][]SchemaReader),
	}
	registry.Register(&CanonicalReader{})
	return registry
}

func (r *SchemaRegistry) Register(reader SchemaReader) {
	version := reader.SchemaVersion()
	r.readers[version] = append(r.readers[version], reader)
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

	readers, ok := r.readers[probe.SchemaVersion]
	if !ok {
		return nil, NewPackageError(
			ErrCodePackageSchemaUnsupported,
			fmt.Sprintf("unsupported schemaVersion: %d", probe.SchemaVersion),
			nil,
		)
	}

	var lastErr error
	for _, reader := range readers {
		manifest, err := reader.ReadManifest(data)
		if err == nil {
			return manifest, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (r *SchemaRegistry) SupportedVersions() []int {
	versions := make([]int, 0, len(r.readers))
	for v := range r.readers {
		versions = append(versions, v)
	}
	sort.Ints(versions)
	return versions
}

type CanonicalReader struct{}

var canonicalManifestAllowedFields = []string{
	"schemaVersion", "manifestFormat", "petId", "releaseId", "version", "name",
	"description", "author", "license", "compatibility", "binding", "canvas",
	"defaultAction", "preview", "actions", "capabilities", "integrity", "provenance",
}

var canonicalManifestRequiredFields = []string{
	"schemaVersion", "manifestFormat", "petId", "releaseId", "version", "name",
	"compatibility", "binding", "canvas", "defaultAction", "actions", "capabilities",
	"integrity", "provenance",
}

func (r *CanonicalReader) SchemaVersion() int { return 1 }

func (r *CanonicalReader) ReadManifest(data []byte) (*Manifest, error) {
	var manifest Manifest
	if err := DecodeStrictTopLevelJSON(data, &manifest, canonicalManifestAllowedFields); err != nil {
		return nil, NewPackageError(ErrCodePackageManifestInvalid, "failed strict v1 manifest validation", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, NewPackageError(ErrCodePackageManifestInvalid, "failed to inspect v1 manifest fields", err)
	}
	for _, field := range canonicalManifestRequiredFields {
		value, ok := raw[field]
		if !ok {
			return nil, NewPackageError(ErrCodePackageManifestInvalid, fmt.Sprintf("required v1 manifest field is missing: %s", field), nil)
		}
		if isJSONNull(value) {
			return nil, NewPackageError(ErrCodePackageManifestInvalid, fmt.Sprintf("required v1 manifest field must not be null: %s", field), nil)
		}
	}
	if err := validateCanonicalManifestRequiredNestedFields(raw); err != nil {
		return nil, NewPackageError(ErrCodePackageManifestInvalid, "v1 manifest is missing required nested fields", err)
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

func validateCanonicalManifestRequiredNestedFields(raw map[string]json.RawMessage) error {
	if authorRaw, ok := raw["author"]; ok {
		if err := requireJSONObjectFields(authorRaw, "author", "name"); err != nil {
			return err
		}
		if err := requireOptionalNonNullJSONObjectFields(authorRaw, "author", "id"); err != nil {
			return err
		}
	}
	if licenseRaw, ok := raw["license"]; ok {
		if err := requireJSONObjectFields(licenseRaw, "license"); err != nil {
			return err
		}
		if err := requireOptionalNonNullJSONObjectFields(licenseRaw, "license", "spdx", "noticePath"); err != nil {
			return err
		}
	}
	if descriptionRaw, ok := raw["description"]; ok && isJSONNull(descriptionRaw) {
		return fmt.Errorf("description must not be null")
	}
	for field, required := range map[string][]string{
		"compatibility": {"minRuntimeVersion", "renderMode"},
		"binding":       {"policy"},
		"canvas":        {"width", "height", "coordinateSystem"},
		"capabilities":  {},
		"integrity":     {"algorithm", "manifestHash", "contentRootHash", "fileCount", "totalBytes", "files"},
		"provenance":    {"builder"},
	} {
		if err := requireJSONObjectFields(raw[field], field, required...); err != nil {
			return err
		}
	}

	var actions []json.RawMessage
	if err := json.Unmarshal(raw["actions"], &actions); err != nil {
		return fmt.Errorf("actions must be an array: %w", err)
	}
	for i, actionRaw := range actions {
		if err := requireJSONObjectFields(actionRaw, fmt.Sprintf("actions[%d]", i),
			"key", "name", "config", "playbackMode", "fps", "frameCount",
			"supportsDefaultIdle", "isStableStateCandidate", "isTransitionOnly"); err != nil {
			return err
		}
		if err := requireOptionalNonNullJSONObjectFields(actionRaw, fmt.Sprintf("actions[%d]", i),
			"revisionId", "qualityEvaluationId", "qualityVerdict"); err != nil {
			return err
		}
	}
	if err := requireOptionalNonNullJSONObjectFields(raw["capabilities"], "capabilities",
		"transparentBackground", "frameSequence", "perFrameDuration", "audio"); err != nil {
		return err
	}
	if err := requireOptionalNonNullJSONObjectFields(raw["provenance"], "provenance",
		"sourceType", "generationTaskId", "processingTaskId", "builtAt"); err != nil {
		return err
	}

	var integrity map[string]json.RawMessage
	if err := json.Unmarshal(raw["integrity"], &integrity); err != nil {
		return fmt.Errorf("integrity must be an object: %w", err)
	}
	var files []json.RawMessage
	if err := json.Unmarshal(integrity["files"], &files); err != nil {
		return fmt.Errorf("integrity.files must be an array: %w", err)
	}
	for i, fileRaw := range files {
		if err := requireJSONObjectFields(fileRaw, fmt.Sprintf("integrity.files[%d]", i),
			"path", "sha256", "bytes", "mediaType", "role"); err != nil {
			return err
		}
		if err := requireOptionalNonNullJSONObjectFields(fileRaw, fmt.Sprintf("integrity.files[%d]", i),
			"actionKey", "frameId"); err != nil {
			return err
		}
	}
	return nil
}

func requireJSONObjectFields(raw json.RawMessage, label string, fields ...string) error {
	if len(raw) == 0 {
		return fmt.Errorf("%s is missing", label)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return fmt.Errorf("%s must be an object: %w", label, err)
	}
	if object == nil {
		return fmt.Errorf("%s must be an object", label)
	}
	for _, field := range fields {
		value, ok := object[field]
		if !ok {
			return fmt.Errorf("required field is missing: %s.%s", label, field)
		}
		if isJSONNull(value) {
			return fmt.Errorf("required field must not be null: %s.%s", label, field)
		}
	}
	return nil
}

func requireOptionalNonNullJSONObjectFields(raw json.RawMessage, label string, fields ...string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return fmt.Errorf("%s must be an object", label)
	}
	for _, field := range fields {
		if value, ok := object[field]; ok && isJSONNull(value) {
			return fmt.Errorf("field must not be null: %s.%s", label, field)
		}
	}
	return nil
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
