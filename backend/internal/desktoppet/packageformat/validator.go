package packageformat

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Finding struct {
	Code        ErrorCode
	Severity    string
	Path        string
	ActionKey   string
	Message     string
	Expected    string
	Actual      string
	Remediation string
}

type ValidationReport struct {
	Verdict      string
	Findings     []Finding
	FileCount    int
	ErrorCount   int
	WarningCount int
}

type PackageFileSystem interface {
	Open(path string) (io.ReadCloser, error)
	Stat(path string) (os.FileInfo, error)
	List() ([]string, error)
}

type DirectoryPackageFS struct {
	root string
}

func NewDirectoryPackageFS(root string) *DirectoryPackageFS {
	return &DirectoryPackageFS{root: root}
}

func (fs *DirectoryPackageFS) Open(path string) (io.ReadCloser, error) {
	abs, err := SecureJoinUnderRoot(fs.root, path)
	if err != nil {
		return nil, err
	}
	return os.Open(abs)
}

func (fs *DirectoryPackageFS) Stat(path string) (os.FileInfo, error) {
	abs, err := SecureJoinUnderRoot(fs.root, path)
	if err != nil {
		return nil, err
	}
	return os.Stat(abs)
}

func (fs *DirectoryPackageFS) List() ([]string, error) {
	var paths []string
	err := filepath.WalkDir(fs.root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(fs.root, path)
		if relErr != nil {
			return nil
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	return paths, err
}

type ArchivePackageFS struct {
	reader *zip.Reader
	files  map[string]*zip.File
}

func NewArchivePackageFS(reader *zip.Reader) *ArchivePackageFS {
	files := make(map[string]*zip.File)
	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		normalized := filepath.ToSlash(filepath.Clean(f.Name))
		files[normalized] = f
	}
	return &ArchivePackageFS{reader: reader, files: files}
}

func (fs *ArchivePackageFS) Open(path string) (io.ReadCloser, error) {
	f, ok := fs.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return f.Open()
}

func (fs *ArchivePackageFS) Stat(path string) (os.FileInfo, error) {
	f, ok := fs.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return f.FileInfo(), nil
}

func (fs *ArchivePackageFS) List() ([]string, error) {
	paths := make([]string, 0, len(fs.files))
	for p := range fs.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths, nil
}

type Validator struct{}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) ValidateDirectory(root string, manifest *Manifest) *ValidationReport {
	report := &ValidationReport{Verdict: "valid"}

	if manifest == nil {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityError,
			Message:  "manifest is nil",
		})
		report.Finalize()
		return report
	}

	fs := NewDirectoryPackageFS(root)
	v.validateCore(report, fs, manifest)

	count, _ := countFiles(root)
	report.FileCount = count
	report.Finalize()
	return report
}

func (v *Validator) ValidateArchive(path string) *ValidationReport {
	report := &ValidationReport{Verdict: "valid"}

	reader := NewArchiveReader(DefaultArchiveLimits())
	rc, manifest, err := reader.OpenArchive(path)
	if err != nil {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityError,
			Message:  fmt.Sprintf("failed to read archive: %v", err),
		})
		report.Finalize()
		return report
	}
	defer rc.Close()

	if manifest == nil {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityError,
			Message:  "manifest not found in archive",
		})
		report.Finalize()
		return report
	}

	fs := NewArchivePackageFS(&rc.Reader)
	v.validateCore(report, fs, manifest)

	paths, _ := fs.List()
	report.FileCount = len(paths)
	report.Finalize()
	return report
}

func (v *Validator) validateCore(report *ValidationReport, fs PackageFileSystem, m *Manifest) {
	v.validateSchemaLayer(report, m)
	v.validatePathLayer(report, m)
	v.validateFileLayerFS(report, fs, m)
	v.validateActionLayer(report, m)
	v.validateActionConfigLayer(report, fs, m)
	v.validateSecurityLayerFS(report, fs)
}

func (v *Validator) validateSchemaLayer(report *ValidationReport, m *Manifest) {
	isLegacyV1 := m.Provenance.SourceType == "legacy_v1"
	if m.SchemaVersion == 0 {
		report.addFinding(Finding{
			Code:     ErrCodePackageSchemaMissing,
			Severity: SeverityError,
			Message:  "schemaVersion is missing or zero",
		})
	} else if m.SchemaVersion != ManifestSchemaVersion {
		report.addFinding(Finding{
			Code:     ErrCodePackageSchemaUnsupported,
			Severity: SeverityError,
			Expected: fmt.Sprintf("schemaVersion=%d", ManifestSchemaVersion),
			Actual:   fmt.Sprintf("schemaVersion=%d", m.SchemaVersion),
			Message:  fmt.Sprintf("unsupported schema version: %d", m.SchemaVersion),
		})
	}

	if m.ManifestFormat != ManifestFormatCanonical {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityError,
			Expected: ManifestFormatCanonical,
			Actual:   m.ManifestFormat,
			Message:  fmt.Sprintf("invalid manifestFormat: %s", m.ManifestFormat),
		})
	}

	if m.PetID == "" {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityError,
			Message:  "petId is empty",
		})
	}

	if m.ReleaseID == "" {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityError,
			Message:  "releaseId is empty",
		})
	}

	if !isLegacyV1 && !isValidRuntimeVersion(m.Version) {
		report.addFinding(Finding{
			Code:     ErrCodePackageRuntimeVersionInvalid,
			Severity: SeverityError,
			Expected: "semantic version (for example 1.2.3)",
			Actual:   m.Version,
			Message:  "version must be a valid semantic version",
		})
	}

	if m.Name == "" {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityError,
			Message:  "name is empty",
		})
	}

	if m.Canvas.Width < 1 || m.Canvas.Width > 4096 || m.Canvas.Height < 1 || m.Canvas.Height > 4096 {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityError,
			Expected: "1..4096 canvas dimensions",
			Actual:   fmt.Sprintf("%dx%d", m.Canvas.Width, m.Canvas.Height),
			Message:  "invalid canvas dimensions",
		})
	}
	if m.Canvas.CoordinateSystem != CoordinateSystemTopLeft {
		report.addFinding(Finding{Code: ErrCodePackageManifestInvalid, Severity: SeverityError, Expected: CoordinateSystemTopLeft, Actual: m.Canvas.CoordinateSystem, Message: "canvas.coordinateSystem must be top-left"})
	}
	if m.Compatibility.MinRuntimeVersion == "" || !isValidRuntimeVersion(m.Compatibility.MinRuntimeVersion) {
		report.addFinding(Finding{Code: ErrCodePackageRuntimeVersionInvalid, Severity: SeverityError, Expected: "valid minRuntimeVersion semver", Actual: m.Compatibility.MinRuntimeVersion, Message: "compatibility.minRuntimeVersion is required and must be valid semver"})
	}
	if m.Compatibility.MaxRuntimeVersion != nil && !isValidRuntimeVersion(*m.Compatibility.MaxRuntimeVersion) {
		report.addFinding(Finding{Code: ErrCodePackageRuntimeVersionInvalid, Severity: SeverityError, Expected: "valid maxRuntimeVersion semver or null", Actual: *m.Compatibility.MaxRuntimeVersion, Message: "compatibility.maxRuntimeVersion must be valid semver or null"})
	}
	if m.Compatibility.RenderMode != RenderModeSprite {
		report.addFinding(Finding{Code: ErrCodePackageManifestInvalid, Severity: SeverityError, Expected: RenderModeSprite, Actual: m.Compatibility.RenderMode, Message: "compatibility.renderMode must be sprite"})
	}
	if m.Binding.Policy != BindingPolicyBound && m.Binding.Policy != BindingPolicyUnbound && m.Binding.Policy != BindingPolicyInferred {
		report.addFinding(Finding{Code: ErrCodePackageManifestInvalid, Severity: SeverityError, Expected: "bound/unbound/legacy_inferred", Actual: m.Binding.Policy, Message: "invalid binding.policy"})
	}
	if strings.TrimSpace(m.Provenance.Builder) == "" {
		report.addFinding(Finding{Code: ErrCodePackageManifestInvalid, Severity: SeverityError, Message: "provenance.builder is required"})
	}

	expectedIntegrityAlgorithm := IntegrityAlgorithmV2
	if isLegacyV1 {
		expectedIntegrityAlgorithm = IntegrityAlgorithmV1Legacy
	}
	if m.Integrity.Algorithm != expectedIntegrityAlgorithm {
		report.addFinding(Finding{
			Code:     ErrCodePackageIntegrityAlgorithmUnsupported,
			Severity: SeverityError,
			Expected: expectedIntegrityAlgorithm,
			Actual:   m.Integrity.Algorithm,
			Message:  fmt.Sprintf("unsupported integrity algorithm: %s", m.Integrity.Algorithm),
		})
	}

}

func (v *Validator) validatePathLayer(report *ValidationReport, m *Manifest) {
	if m.Preview != "" {
		if _, err := NormalizePackagePath(m.Preview); err != nil {
			report.addFinding(Finding{
				Code:     ErrCodePackagePathInvalid,
				Severity: SeverityError,
				Path:     m.Preview,
				Message:  fmt.Sprintf("invalid preview path: %v", err),
			})
		}
	}

	if m.License.NoticePath != "" {
		if _, err := NormalizePackagePath(m.License.NoticePath); err != nil {
			report.addFinding(Finding{
				Code:     ErrCodePackagePathInvalid,
				Severity: SeverityError,
				Path:     m.License.NoticePath,
				Message:  fmt.Sprintf("invalid license notice path: %v", err),
			})
		}
	}

	for _, action := range m.Actions {
		if action.Config != "" {
			if _, err := NormalizePackagePath(action.Config); err != nil {
				report.addFinding(Finding{
					Code:      ErrCodePackagePathInvalid,
					Severity:  SeverityError,
					Path:      action.Config,
					ActionKey: action.Key,
					Message:   fmt.Sprintf("invalid action config path: %v", err),
				})
			}
		}
	}

	for _, f := range m.Integrity.Files {
		if _, err := NormalizePackagePath(f.Path); err != nil {
			report.addFinding(Finding{
				Code:     ErrCodePackagePathInvalid,
				Severity: SeverityError,
				Path:     f.Path,
				Message:  fmt.Sprintf("invalid integrity file path: %v", err),
			})
		}
	}
}

func (v *Validator) validateFileLayerFS(report *ValidationReport, fs PackageFileSystem, m *Manifest) {
	if m.Provenance.SourceType == "legacy_v1" {
		v.validateLegacyFileLayerFS(report, fs, m)
		return
	}

	declared := make(map[string]*FileManifestEntry, len(m.Integrity.Files))
	for i := range m.Integrity.Files {
		e := &m.Integrity.Files[i]
		if existing, dup := declared[e.Path]; dup {
			report.addFinding(Finding{
				Code:     ErrCodePackageDuplicateEntry,
				Severity: SeverityError,
				Path:     e.Path,
				Message:  fmt.Sprintf("duplicate manifest entry: %s also declared at %s", e.Path, existing.Path),
			})
			continue
		}
		declared[e.Path] = e
	}

	actualPaths, err := fs.List()
	if err != nil {
		report.addFinding(Finding{
			Code:     ErrCodePackagePathInvalid,
			Severity: SeverityError,
			Message:  fmt.Sprintf("failed to list package files: %v", err),
		})
		return
	}

	actualSet := make(map[string]bool, len(actualPaths))
	for _, p := range actualPaths {
		actualSet[p] = true
		if p == "manifest.json" {
			continue
		}
		if _, ok := declared[p]; !ok {
			report.addFinding(Finding{
				Code:     ErrCodePackageFileUndeclared,
				Severity: SeverityError,
				Path:     p,
				Message:  fmt.Sprintf("file exists but not declared in manifest: %s", p),
			})
		}
	}

	for i := range m.Integrity.Files {
		e := &m.Integrity.Files[i]
		if !actualSet[e.Path] {
			report.addFinding(Finding{
				Code:     ErrCodePackageFileMissing,
				Severity: SeverityError,
				Path:     e.Path,
				Message:  fmt.Sprintf("file declared in manifest but missing: %s", e.Path),
			})
			continue
		}

		rc, openErr := fs.Open(e.Path)
		if openErr != nil {
			report.addFinding(Finding{
				Code:     ErrCodePackageFileMissing,
				Severity: SeverityError,
				Path:     e.Path,
				Message:  fmt.Sprintf("failed to open file: %v", openErr),
			})
			continue
		}
		h := sha256.New()
		size, copyErr := io.Copy(h, rc)
		rc.Close()
		if copyErr != nil {
			report.addFinding(Finding{
				Code:     ErrCodePackageHashMismatch,
				Severity: SeverityError,
				Path:     e.Path,
				Message:  fmt.Sprintf("failed to hash file: %v", copyErr),
			})
			continue
		}

		actualHash := hex.EncodeToString(h.Sum(nil))
		if actualHash != e.SHA256 {
			report.addFinding(Finding{
				Code:     ErrCodePackageHashMismatch,
				Severity: SeverityError,
				Path:     e.Path,
				Expected: e.SHA256,
				Actual:   actualHash,
				Message:  fmt.Sprintf("hash mismatch for %s", e.Path),
			})
		}

		if size != e.Bytes {
			report.addFinding(Finding{
				Code:     ErrCodePackageHashMismatch,
				Severity: SeverityError,
				Path:     e.Path,
				Expected: fmt.Sprintf("bytes=%d", e.Bytes),
				Actual:   fmt.Sprintf("bytes=%d", size),
				Message:  fmt.Sprintf("size mismatch for %s", e.Path),
			})
		}
	}

	if m.Integrity.ManifestHash == "" {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestHashMissing,
			Severity: SeverityError,
			Message:  "integrity.manifestHash is empty",
		})
	}

	if m.Integrity.ContentRootHash == "" {
		report.addFinding(Finding{
			Code:     ErrCodePackageIntegrityMissing,
			Severity: SeverityError,
			Message:  "integrity.contentRootHash is empty",
		})
	}

	var entries []FileEntry
	for _, f := range m.Integrity.Files {
		entries = append(entries, FileEntry{
			Path:   f.Path,
			SHA256: f.SHA256,
			Bytes:  f.Bytes,
		})
	}

	if m.Integrity.ManifestHash != "" {
		recomputedManifestHash, hashErr := CanonicalManifestHash(m)
		if hashErr == nil && recomputedManifestHash != m.Integrity.ManifestHash {
			report.addFinding(Finding{
				Code:     ErrCodePackageManifestHashMismatch,
				Severity: SeverityError,
				Expected: recomputedManifestHash,
				Actual:   m.Integrity.ManifestHash,
				Message:  "manifest hash mismatch",
			})
		}

		if m.Integrity.ContentRootHash != "" && hashErr == nil {
			canonicalData, dataErr := CanonicalManifestData(m)
			if dataErr == nil {
				manifestBytes := int64(len(canonicalData))
				recomputedRootHash := ComputeContentRootHash(entries, recomputedManifestHash, manifestBytes)
				if recomputedRootHash != m.Integrity.ContentRootHash {
					report.addFinding(Finding{
						Code:     ErrCodePackageHashMismatch,
						Severity: SeverityError,
						Expected: recomputedRootHash,
						Actual:   m.Integrity.ContentRootHash,
						Message:  "content root hash mismatch",
					})
				}
			}
		}
	}

	if m.Provenance.SourceType != "legacy_v1" && m.Integrity.FileCount < 1 {
		report.addFinding(Finding{Code: ErrCodePackageManifestInvalid, Severity: SeverityError, Expected: "fileCount >= 1", Actual: fmt.Sprintf("fileCount=%d", m.Integrity.FileCount), Message: "invalid integrity.fileCount"})
	}
	if m.Integrity.TotalBytes < 0 {
		report.addFinding(Finding{Code: ErrCodePackageManifestInvalid, Severity: SeverityError, Expected: "totalBytes >= 0", Actual: fmt.Sprintf("totalBytes=%d", m.Integrity.TotalBytes), Message: "invalid integrity.totalBytes"})
	}
	for _, file := range m.Integrity.Files {
		if !isLowerHexSHA256(file.SHA256) {
			report.addFinding(Finding{Code: ErrCodePackageManifestInvalid, Severity: SeverityError, Path: file.Path, Expected: "64 lowercase hex characters", Actual: file.SHA256, Message: "invalid integrity file sha256"})
		}
		if file.Bytes < 0 {
			report.addFinding(Finding{Code: ErrCodePackageManifestInvalid, Severity: SeverityError, Path: file.Path, Expected: "bytes >= 0", Actual: fmt.Sprintf("bytes=%d", file.Bytes), Message: "invalid integrity file size"})
		}
	}

	declaredCount := len(m.Integrity.Files)
	if m.Integrity.FileCount > 0 && m.Integrity.FileCount != declaredCount {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityError,
			Expected: fmt.Sprintf("fileCount=%d", m.Integrity.FileCount),
			Actual:   fmt.Sprintf("actual entries=%d", declaredCount),
			Message:  "file count mismatch",
		})
	}

	var totalBytes int64
	for _, f := range m.Integrity.Files {
		totalBytes += f.Bytes
	}
	if m.Integrity.TotalBytes > 0 && m.Integrity.TotalBytes != totalBytes {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityError,
			Expected: fmt.Sprintf("totalBytes=%d", m.Integrity.TotalBytes),
			Actual:   fmt.Sprintf("actual total=%d", totalBytes),
			Message:  "total bytes mismatch",
		})
	}
}

func (v *Validator) validateLegacyFileLayerFS(report *ValidationReport, fs PackageFileSystem, m *Manifest) {
	actualPaths, err := fs.List()
	if err != nil {
		report.addFinding(Finding{
			Code:     ErrCodePackagePathInvalid,
			Severity: SeverityError,
			Message:  fmt.Sprintf("failed to list legacy package files: %v", err),
		})
		return
	}

	actualSet := make(map[string]bool, len(actualPaths))
	for _, p := range actualPaths {
		actualSet[p] = true
	}

	required := make([]struct {
		path      string
		actionKey string
	}, 0, len(m.Actions)+1)
	if m.Preview != "" {
		required = append(required, struct {
			path      string
			actionKey string
		}{path: m.Preview})
	}
	for _, action := range m.Actions {
		if action.Config != "" {
			required = append(required, struct {
				path      string
				actionKey string
			}{path: action.Config, actionKey: action.Key})
		}
	}

	for _, ref := range required {
		normalized, pathErr := NormalizePackagePath(ref.path)
		if pathErr != nil {
			continue // validatePathLayer already reports the precise path failure.
		}
		if !actualSet[normalized] {
			report.addFinding(Finding{
				Code:      ErrCodePackageFileMissing,
				Severity:  SeverityError,
				Path:      normalized,
				ActionKey: ref.actionKey,
				Message:   fmt.Sprintf("legacy package referenced file is missing: %s", normalized),
			})
		}
	}
}

func (v *Validator) validateActionLayer(report *ValidationReport, m *Manifest) {
	isLegacyV1 := m.Provenance.SourceType == "legacy_v1"
	if len(m.Actions) == 0 {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityError,
			Message:  "no actions declared",
		})
		return
	}

	keys := make(map[string]bool)
	for _, action := range m.Actions {
		if action.Key == "" {
			report.addFinding(Finding{
				Code:      ErrCodePackageManifestInvalid,
				Severity:  SeverityError,
				ActionKey: action.Key,
				Message:   "action key is empty",
			})
			continue
		}
		if keys[action.Key] {
			report.addFinding(Finding{
				Code:      ErrCodePackageDuplicateEntry,
				Severity:  SeverityError,
				ActionKey: action.Key,
				Message:   fmt.Sprintf("duplicate action key: %s", action.Key),
			})
		}
		keys[action.Key] = true

		if action.Name == "" {
			report.addFinding(Finding{Code: ErrCodePackageManifestInvalid, Severity: SeverityError, ActionKey: action.Key, Message: "action name is empty"})
		}
		if action.Config == "" {
			report.addFinding(Finding{
				Code:      ErrCodePackageManifestInvalid,
				Severity:  SeverityError,
				ActionKey: action.Key,
				Message:   "action config path is empty",
			})
		}
		if !isLegacyV1 && action.FrameCount < 1 {
			report.addFinding(Finding{Code: ErrCodePackageManifestInvalid, Severity: SeverityError, ActionKey: action.Key, Expected: "frameCount >= 1", Actual: fmt.Sprintf("frameCount=%d", action.FrameCount), Message: "invalid frame count"})
		}
		if !isLegacyV1 && (action.FPS < 1 || action.FPS > 120) {
			report.addFinding(Finding{Code: ErrCodePackageManifestInvalid, Severity: SeverityError, ActionKey: action.Key, Expected: "1 <= fps <= 120", Actual: fmt.Sprintf("fps=%d", action.FPS), Message: "invalid FPS"})
		}
		if !isLegacyV1 && !IsValidPlaybackMode(action.PlaybackMode) {
			report.addFinding(Finding{Code: ErrCodePackageManifestInvalid, Severity: SeverityError, ActionKey: action.Key, Expected: "loop/once/hold/ping_pong", Actual: action.PlaybackMode, Message: "invalid action playbackMode"})
		}
		if action.QualityVerdict != "" {
			validQualityVerdict := action.QualityVerdict == QualityVerdictAccepted ||
				action.QualityVerdict == QualityVerdictAcceptedWithWarning ||
				action.QualityVerdict == QualityVerdictNeedsReview ||
				action.QualityVerdict == QualityVerdictRejected ||
				(isLegacyV1 && action.QualityVerdict == QualityVerdictSkipped)
			if !validQualityVerdict {
				report.addFinding(Finding{Code: ErrCodePackageManifestInvalid, Severity: SeverityError, ActionKey: action.Key, Actual: action.QualityVerdict, Message: "invalid action qualityVerdict"})
			}
		}
	}

	if m.DefaultAction == "" {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityError,
			Message:  "defaultAction is empty",
		})
	} else if !keys[m.DefaultAction] {
		report.addFinding(Finding{
			Code:      ErrCodeDefaultActionInvalid,
			Severity:  SeverityError,
			ActionKey: m.DefaultAction,
			Expected:  "an existing action key",
			Actual:    m.DefaultAction,
			Message:   fmt.Sprintf("defaultAction %s not found in actions", m.DefaultAction),
		})
	}
}

type validatedActionConfig struct {
	SchemaVersion          int                   `json:"schemaVersion"`
	ActionKey              string                `json:"actionKey"`
	DisplayName            string                `json:"displayName"`
	Version                int                   `json:"version"`
	Fps                    int                   `json:"fps"`
	PlaybackMode           string                `json:"playbackMode"`
	Interruptible          bool                  `json:"interruptible"`
	InterruptAfterMs       *int                  `json:"interruptAfterMs"`
	Priority               int                   `json:"priority"`
	CooldownMs             int                   `json:"cooldownMs"`
	MinimumPlayMs          int                   `json:"minimumPlayMs"`
	MaximumPlayMs          *int                  `json:"maximumPlayMs"`
	MutexGroup             *string               `json:"mutexGroup"`
	SupportsDefaultIdle    bool                  `json:"supportsDefaultIdle"`
	IsStableStateCandidate bool                  `json:"isStableStateCandidate"`
	IsTransitionOnly       bool                  `json:"isTransitionOnly"`
	Frames                 []validatedFrameEntry `json:"frames"`
	ReturnTo               validatedReturnTo     `json:"returnTo"`
	Anchor                 validatedAnchor       `json:"anchor"`
}

var actionConfigAllowedFields = []string{
	"schemaVersion", "actionKey", "displayName", "version", "playbackMode",
	"fps", "interruptible", "interruptAfterMs", "priority", "cooldownMs",
	"minimumPlayMs", "maximumPlayMs", "mutexGroup", "supportsDefaultIdle",
	"isStableStateCandidate", "isTransitionOnly", "returnTo", "anchor", "frames",
}

var actionConfigRequiredFields = []string{
	"schemaVersion", "actionKey", "displayName", "version", "playbackMode",
	"fps", "interruptible", "priority", "cooldownMs", "minimumPlayMs",
	"maximumPlayMs", "mutexGroup", "supportsDefaultIdle",
	"isStableStateCandidate", "isTransitionOnly", "returnTo", "anchor", "frames",
}

type validatedFrameEntry struct {
	Index       int    `json:"index"`
	FrameID     string `json:"frameId"`
	File        string `json:"file"`
	DurationMs  int    `json:"durationMs"`
	AssetID     string `json:"assetId"`
	ContentHash string `json:"contentHash"`
}

type validatedReturnTo struct {
	Type      string `json:"type"`
	ActionKey string `json:"actionKey"`
}

type validatedAnchor struct {
	X               float64 `json:"x"`
	Y               float64 `json:"y"`
	CoordinateSpace string  `json:"coordinateSpace"`
}

func (v *Validator) validateActionConfigLayer(report *ValidationReport, fs PackageFileSystem, m *Manifest) {
	if m.Provenance.SourceType == "legacy_v1" {
		v.validateLegacyActionConfigLayer(report, fs, m)
		return
	}

	actionKeys := make(map[string]bool, len(m.Actions))
	for _, a := range m.Actions {
		actionKeys[a.Key] = true
	}

	for _, action := range m.Actions {
		if action.Config == "" {
			report.addFinding(Finding{
				Code:      ErrCodeActionConfigMissing,
				Severity:  SeverityError,
				ActionKey: action.Key,
				Message:   "action config path is empty",
			})
			continue
		}

		rc, err := fs.Open(action.Config)
		if err != nil {
			report.addFinding(Finding{
				Code:      ErrCodeActionConfigMissing,
				Severity:  SeverityError,
				Path:      action.Config,
				ActionKey: action.Key,
				Message:   fmt.Sprintf("failed to open action config: %v", err),
			})
			continue
		}
		data, readErr := io.ReadAll(rc)
		rc.Close()
		if readErr != nil {
			report.addFinding(Finding{
				Code:      ErrCodeActionConfigInvalid,
				Severity:  SeverityError,
				Path:      action.Config,
				ActionKey: action.Key,
				Message:   fmt.Sprintf("failed to read action config: %v", readErr),
			})
			continue
		}

		var cfg validatedActionConfig
		if jErr := DecodeStrictTopLevelJSON(data, &cfg, actionConfigAllowedFields); jErr != nil {
			report.addFinding(Finding{
				Code:      ErrCodeActionConfigInvalid,
				Severity:  SeverityError,
				Path:      action.Config,
				ActionKey: action.Key,
				Message:   fmt.Sprintf("failed strict action config validation: %v", jErr),
			})
			continue
		}

		var rawFields map[string]json.RawMessage
		if err := json.Unmarshal(data, &rawFields); err != nil {
			report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Message: fmt.Sprintf("failed to inspect required fields: %v", err)})
			continue
		}
		missingRequired := false
		for _, field := range actionConfigRequiredFields {
			value, ok := rawFields[field]
			if !ok {
				missingRequired = true
				report.addFinding(Finding{
					Code:      ErrCodeActionConfigInvalid,
					Severity:  SeverityError,
					Path:      action.Config,
					ActionKey: action.Key,
					Expected:  field,
					Message:   fmt.Sprintf("required action config field is missing: %s", field),
				})
				continue
			}
			if field != "maximumPlayMs" && field != "mutexGroup" && isJSONNull(value) {
				missingRequired = true
				report.addFinding(Finding{
					Code:      ErrCodeActionConfigInvalid,
					Severity:  SeverityError,
					Path:      action.Config,
					ActionKey: action.Key,
					Expected:  field,
					Actual:    "null",
					Message:   fmt.Sprintf("required action config field must not be null: %s", field),
				})
			}
		}
		if value, ok := rawFields["interruptAfterMs"]; ok && isJSONNull(value) {
			missingRequired = true
			report.addFinding(Finding{
				Code:      ErrCodeActionConfigInvalid,
				Severity:  SeverityError,
				Path:      action.Config,
				ActionKey: action.Key,
				Expected:  "integer >= 0",
				Actual:    "null",
				Message:   "interruptAfterMs must not be null when present",
			})
		}
		if missingRequired {
			continue
		}
		if !v.validateV2ActionNestedRequiredFields(report, action, rawFields) {
			continue
		}

		if cfg.SchemaVersion != ActionConfigSchemaVersion {
			report.addFinding(Finding{
				Code:      ErrCodePackageActionConfigSchemaUnsupported,
				Severity:  SeverityError,
				Path:      action.Config,
				ActionKey: action.Key,
				Expected:  fmt.Sprintf("schemaVersion=%d", ActionConfigSchemaVersion),
				Actual:    fmt.Sprintf("schemaVersion=%d", cfg.SchemaVersion),
				Message:   fmt.Sprintf("action config schemaVersion must be %d, got %d", ActionConfigSchemaVersion, cfg.SchemaVersion),
			})
		}

		if cfg.ActionKey != action.Key {
			report.addFinding(Finding{
				Code:      ErrCodePackageActionKeyMismatch,
				Severity:  SeverityError,
				Path:      action.Config,
				ActionKey: action.Key,
				Expected:  action.Key,
				Actual:    cfg.ActionKey,
				Message:   fmt.Sprintf("action key mismatch: manifest=%s, config=%s", action.Key, cfg.ActionKey),
			})
		}

		if strings.TrimSpace(cfg.DisplayName) == "" {
			report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Message: "displayName must be non-empty"})
		}
		if cfg.Version < 1 {
			report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: "version >= 1", Actual: fmt.Sprintf("version=%d", cfg.Version), Message: "invalid action version"})
		}
		if cfg.Priority < 0 || cfg.Priority > 100 {
			report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: "0 <= priority <= 100", Actual: fmt.Sprintf("priority=%d", cfg.Priority), Message: "invalid action priority"})
		}
		if cfg.CooldownMs < 0 {
			report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: "cooldownMs >= 0", Actual: fmt.Sprintf("cooldownMs=%d", cfg.CooldownMs), Message: "invalid cooldownMs"})
		}
		if cfg.MinimumPlayMs < 0 {
			report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: "minimumPlayMs >= 0", Actual: fmt.Sprintf("minimumPlayMs=%d", cfg.MinimumPlayMs), Message: "invalid minimumPlayMs"})
		}
		if cfg.MaximumPlayMs != nil && *cfg.MaximumPlayMs < 0 {
			report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: "maximumPlayMs is null or >= 0", Actual: fmt.Sprintf("maximumPlayMs=%d", *cfg.MaximumPlayMs), Message: "invalid maximumPlayMs"})
		}
		if cfg.InterruptAfterMs != nil && *cfg.InterruptAfterMs < 0 {
			report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: "interruptAfterMs >= 0", Actual: fmt.Sprintf("interruptAfterMs=%d", *cfg.InterruptAfterMs), Message: "invalid interruptAfterMs"})
		}

		playbackMode := cfg.PlaybackMode
		if playbackMode == "" {
			report.addFinding(Finding{
				Code:      ErrCodeActionConfigInvalid,
				Severity:  SeverityError,
				Path:      action.Config,
				ActionKey: action.Key,
				Message:   "playbackMode is empty",
			})
		} else if !IsValidPlaybackMode(playbackMode) {
			report.addFinding(Finding{
				Code:      ErrCodeActionConfigInvalid,
				Severity:  SeverityError,
				Path:      action.Config,
				ActionKey: action.Key,
				Expected:  "loop/once/hold/ping_pong",
				Actual:    playbackMode,
				Message:   fmt.Sprintf("invalid playbackMode: %s", playbackMode),
			})
		} else if playbackMode != action.PlaybackMode {
			report.addFinding(Finding{
				Code:      ErrCodePackageActionSummaryMismatch,
				Severity:  SeverityError,
				Path:      action.Config,
				ActionKey: action.Key,
				Expected:  action.PlaybackMode,
				Actual:    playbackMode,
				Message:   fmt.Sprintf("playbackMode mismatch: manifest=%s, config=%s", action.PlaybackMode, playbackMode),
			})
		}

		if cfg.Fps <= 0 || cfg.Fps > 120 {
			report.addFinding(Finding{
				Code:      ErrCodeActionConfigInvalid,
				Severity:  SeverityError,
				Path:      action.Config,
				ActionKey: action.Key,
				Expected:  "0 < fps <= 120",
				Actual:    fmt.Sprintf("fps=%d", cfg.Fps),
				Message:   fmt.Sprintf("invalid fps: %d", cfg.Fps),
			})
		} else if cfg.Fps != action.FPS {
			report.addFinding(Finding{
				Code:      ErrCodePackageActionSummaryMismatch,
				Severity:  SeverityError,
				Path:      action.Config,
				ActionKey: action.Key,
				Expected:  fmt.Sprintf("fps=%d", action.FPS),
				Actual:    fmt.Sprintf("fps=%d", cfg.Fps),
				Message:   fmt.Sprintf("fps mismatch: manifest=%d, config=%d", action.FPS, cfg.Fps),
			})
		}

		if len(cfg.Frames) != action.FrameCount {
			report.addFinding(Finding{
				Code:      ErrCodePackageActionSummaryMismatch,
				Severity:  SeverityError,
				Path:      action.Config,
				ActionKey: action.Key,
				Expected:  fmt.Sprintf("frameCount=%d", action.FrameCount),
				Actual:    fmt.Sprintf("frames=%d", len(cfg.Frames)),
				Message:   fmt.Sprintf("frameCount mismatch: manifest=%d, config=%d", action.FrameCount, len(cfg.Frames)),
			})
		}

		if cfg.SupportsDefaultIdle != action.SupportsDefaultIdle {
			report.addFinding(Finding{Code: ErrCodePackageActionSummaryMismatch, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: fmt.Sprintf("supportsDefaultIdle=%t", action.SupportsDefaultIdle), Actual: fmt.Sprintf("supportsDefaultIdle=%t", cfg.SupportsDefaultIdle), Message: "supportsDefaultIdle mismatch between manifest and action config"})
		}
		if cfg.IsStableStateCandidate != action.IsStableStateCandidate {
			report.addFinding(Finding{Code: ErrCodePackageActionSummaryMismatch, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: fmt.Sprintf("isStableStateCandidate=%t", action.IsStableStateCandidate), Actual: fmt.Sprintf("isStableStateCandidate=%t", cfg.IsStableStateCandidate), Message: "isStableStateCandidate mismatch between manifest and action config"})
		}
		if cfg.IsTransitionOnly != action.IsTransitionOnly {
			report.addFinding(Finding{Code: ErrCodePackageActionSummaryMismatch, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: fmt.Sprintf("isTransitionOnly=%t", action.IsTransitionOnly), Actual: fmt.Sprintf("isTransitionOnly=%t", cfg.IsTransitionOnly), Message: "isTransitionOnly mismatch between manifest and action config"})
		}

		if len(cfg.Frames) < 1 {
			report.addFinding(Finding{
				Code:      ErrCodeFrameMissing,
				Severity:  SeverityError,
				Path:      action.Config,
				ActionKey: action.Key,
				Message:   "action config has no frames",
			})
		}

		if cfg.Anchor.CoordinateSpace == "" {
			report.addFinding(Finding{
				Code:      ErrCodeActionConfigInvalid,
				Severity:  SeverityError,
				Path:      action.Config,
				ActionKey: action.Key,
				Message:   "anchor is missing",
			})
		} else {
			if cfg.Anchor.CoordinateSpace != "normalized_canvas" {
				report.addFinding(Finding{
					Code:      ErrCodeActionConfigInvalid,
					Severity:  SeverityError,
					Path:      action.Config,
					ActionKey: action.Key,
					Expected:  "normalized_canvas",
					Actual:    cfg.Anchor.CoordinateSpace,
					Message:   fmt.Sprintf("invalid anchor coordinateSpace: %s", cfg.Anchor.CoordinateSpace),
				})
			}
			if cfg.Anchor.X < 0 || cfg.Anchor.X > 1 {
				report.addFinding(Finding{
					Code:      ErrCodeActionConfigInvalid,
					Severity:  SeverityError,
					Path:      action.Config,
					ActionKey: action.Key,
					Expected:  "0 <= x <= 1",
					Actual:    fmt.Sprintf("x=%f", cfg.Anchor.X),
					Message:   fmt.Sprintf("anchor x out of range: %f", cfg.Anchor.X),
				})
			}
			if cfg.Anchor.Y < 0 || cfg.Anchor.Y > 1 {
				report.addFinding(Finding{
					Code:      ErrCodeActionConfigInvalid,
					Severity:  SeverityError,
					Path:      action.Config,
					ActionKey: action.Key,
					Expected:  "0 <= y <= 1",
					Actual:    fmt.Sprintf("y=%f", cfg.Anchor.Y),
					Message:   fmt.Sprintf("anchor y out of range: %f", cfg.Anchor.Y),
				})
			}
		}

		seenFiles := make(map[string]bool)
		seenFrameIDs := make(map[string]bool)
		declaredResources := make(map[string]FileManifestEntry, len(m.Integrity.Files))
		for _, declared := range m.Integrity.Files {
			declaredResources[declared.Path] = declared
		}
		for idx, frame := range cfg.Frames {
			if frame.Index != idx {
				report.addFinding(Finding{
					Code:      ErrCodePackageFrameIndexInvalid,
					Severity:  SeverityError,
					Path:      action.Config,
					ActionKey: action.Key,
					Expected:  fmt.Sprintf("index=%d", idx),
					Actual:    fmt.Sprintf("index=%d", frame.Index),
					Message:   fmt.Sprintf("frame index not contiguous at position %d", idx),
				})
			}
			if frame.FrameID == "" {
				report.addFinding(Finding{
					Code:      ErrCodeFrameMissing,
					Severity:  SeverityError,
					Path:      action.Config,
					ActionKey: action.Key,
					Message:   fmt.Sprintf("frame %d has empty frameId", idx),
				})
			} else if seenFrameIDs[frame.FrameID] {
				report.addFinding(Finding{
					Code:      ErrCodePackageFrameIdDuplicate,
					Severity:  SeverityError,
					Path:      action.Config,
					ActionKey: action.Key,
					Message:   fmt.Sprintf("duplicate frameId: %s", frame.FrameID),
				})
			}
			seenFrameIDs[frame.FrameID] = true

			if frame.File == "" {
				report.addFinding(Finding{
					Code:      ErrCodeFrameMissing,
					Severity:  SeverityError,
					Path:      action.Config,
					ActionKey: action.Key,
					Message:   fmt.Sprintf("frame %d has empty file", idx),
				})
			} else if seenFiles[frame.File] {
				report.addFinding(Finding{
					Code:      ErrCodeActionConfigInvalid,
					Severity:  SeverityError,
					Path:      action.Config,
					ActionKey: action.Key,
					Message:   fmt.Sprintf("duplicate frame file: %s", frame.File),
				})
			}
			seenFiles[frame.File] = true

			if frame.AssetID == "" {
				report.addFinding(Finding{
					Code:      ErrCodePackageFrameAssetIdMissing,
					Severity:  SeverityError,
					Path:      action.Config,
					ActionKey: action.Key,
					Message:   fmt.Sprintf("frame %d has empty assetId", idx),
				})
			}

			if !isLowerHexSHA256(frame.ContentHash) {
				report.addFinding(Finding{
					Code:      ErrCodeFrameHashMismatch,
					Severity:  SeverityError,
					Path:      action.Config,
					ActionKey: action.Key,
					Expected:  "64 lowercase hex characters",
					Actual:    frame.ContentHash,
					Message:   fmt.Sprintf("frame %d has invalid contentHash", idx),
				})
			}

			if frame.DurationMs < 8 || frame.DurationMs > 60000 {
				report.addFinding(Finding{
					Code:      ErrCodeActionConfigInvalid,
					Severity:  SeverityError,
					Path:      action.Config,
					ActionKey: action.Key,
					Expected:  "8 <= durationMs <= 60000",
					Actual:    fmt.Sprintf("durationMs=%d", frame.DurationMs),
					Message:   fmt.Sprintf("invalid frame durationMs: %d", frame.DurationMs),
				})
			}

			if frame.File != "" {
				framePath, pathErr := resolveActionResourcePath(action.Config, frame.File)
				if pathErr != nil {
					report.addFinding(Finding{Code: ErrCodePackagePathInvalid, Severity: SeverityError, Path: frame.File, ActionKey: action.Key, Message: fmt.Sprintf("invalid frame resource path: %v", pathErr)})
					continue
				}
				declared, ok := declaredResources[framePath]
				if !ok {
					report.addFinding(Finding{Code: ErrCodePackageResourceNotDeclared, Severity: SeverityError, Path: framePath, ActionKey: action.Key, Message: fmt.Sprintf("frame resource not declared in integrity.files: %s", framePath)})
				} else if frame.ContentHash != "" && declared.SHA256 != frame.ContentHash {
					report.addFinding(Finding{Code: ErrCodePackageResourceHashMismatch, Severity: SeverityError, Path: framePath, ActionKey: action.Key, Expected: frame.ContentHash, Actual: declared.SHA256, Message: fmt.Sprintf("frame contentHash does not match integrity.files sha256: %s", framePath)})
				}
				if _, statErr := fs.Stat(framePath); statErr != nil {
					report.addFinding(Finding{Code: ErrCodeFrameMissing, Severity: SeverityError, Path: framePath, ActionKey: action.Key, Message: fmt.Sprintf("frame resource is missing: %s", framePath)})
				}
			}
		}

		switch cfg.ReturnTo.Type {
		case ReturnToDefault, ReturnToPrevious, ReturnToCurrentActivity, ReturnToNone:
		case ReturnToAction:
			if cfg.ReturnTo.ActionKey == "" || !actionKeys[cfg.ReturnTo.ActionKey] {
				report.addFinding(Finding{
					Code:      ErrCodeActionReferenceInvalid,
					Severity:  SeverityError,
					Path:      action.Config,
					ActionKey: action.Key,
					Actual:    cfg.ReturnTo.ActionKey,
					Message:   fmt.Sprintf("returnTo action target not found: %s", cfg.ReturnTo.ActionKey),
				})
			}
		default:
			report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: "default/previous/current_activity/none/action", Actual: cfg.ReturnTo.Type, Message: fmt.Sprintf("invalid returnTo type: %s", cfg.ReturnTo.Type)})
		}

		if action.IsTransitionOnly && action.IsStableStateCandidate {
			report.addFinding(Finding{
				Code:      ErrCodeActionConfigInvalid,
				Severity:  SeverityError,
				Path:      action.Config,
				ActionKey: action.Key,
				Message:   "action cannot be both transitionOnly and stableStateCandidate",
			})
		}

		if action.IsTransitionOnly && action.Key == m.DefaultAction {
			report.addFinding(Finding{
				Code:      ErrCodeDefaultActionInvalid,
				Severity:  SeverityError,
				ActionKey: action.Key,
				Message:   fmt.Sprintf("transition-only action %s cannot be default action", action.Key),
			})
		}
	}

	if m.DefaultAction != "" {
		for i := range m.Actions {
			if m.Actions[i].Key == m.DefaultAction {
				if !m.Actions[i].SupportsDefaultIdle {
					report.addFinding(Finding{
						Code:      ErrCodeDefaultActionInvalid,
						Severity:  SeverityError,
						ActionKey: m.DefaultAction,
						Message:   fmt.Sprintf("default action %s does not support default idle", m.DefaultAction),
					})
				}
				break
			}
		}
	}
}

func (v *Validator) validateV2ActionNestedRequiredFields(report *ValidationReport, action ManifestActionEntry, rawFields map[string]json.RawMessage) bool {
	valid := true
	require := func(raw json.RawMessage, label string, fields ...string) map[string]json.RawMessage {
		var object map[string]json.RawMessage
		if len(raw) == 0 || json.Unmarshal(raw, &object) != nil || object == nil {
			report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Message: fmt.Sprintf("%s must be an object", label)})
			valid = false
			return nil
		}
		for _, field := range fields {
			value, ok := object[field]
			if !ok {
				report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: field, Message: fmt.Sprintf("required action config field is missing: %s.%s", label, field)})
				valid = false
			} else if isJSONNull(value) {
				report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: field, Actual: "null", Message: fmt.Sprintf("required action config field must not be null: %s.%s", label, field)})
				valid = false
			}
		}
		return object
	}

	returnTo := require(rawFields["returnTo"], "returnTo", "type")
	if returnTo != nil {
		var returnType string
		_ = json.Unmarshal(returnTo["type"], &returnType)
		if value, ok := returnTo["actionKey"]; ok && isJSONNull(value) {
			report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: "string", Actual: "null", Message: "returnTo.actionKey must not be null when present"})
			valid = false
		}
		if returnType == ReturnToAction {
			if _, ok := returnTo["actionKey"]; !ok {
				report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: "actionKey", Message: "required action config field is missing: returnTo.actionKey"})
				valid = false
			}
		}
	}
	require(rawFields["anchor"], "anchor", "x", "y", "coordinateSpace")

	var frames []json.RawMessage
	if err := json.Unmarshal(rawFields["frames"], &frames); err != nil {
		report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Message: fmt.Sprintf("frames must be an array: %v", err)})
		return false
	}
	for i, frameRaw := range frames {
		require(frameRaw, fmt.Sprintf("frames[%d]", i), "frameId", "index", "file", "durationMs", "assetId", "contentHash")
	}
	return valid
}

type legacyValidatedActionConfig struct {
	ActionKey       string            `json:"actionKey"`
	Key             string            `json:"key"`
	DisplayName     string            `json:"displayName"`
	ActionName      string            `json:"actionName"`
	Name            string            `json:"name"`
	Fps             int               `json:"fps"`
	DefaultFps      int               `json:"defaultFps"`
	PlaybackMode    string            `json:"playbackMode"`
	LoopType        string            `json:"loopType"`
	FrameDurationMs int               `json:"frameDurationMs"`
	Frames          []json.RawMessage `json:"frames"`
	ReturnAction    string            `json:"returnAction"`
}

type legacyValidatedFrame struct {
	Index      *int   `json:"index"`
	File       string `json:"file"`
	DurationMs *int   `json:"durationMs"`
}

func (v *Validator) validateLegacyActionConfigLayer(report *ValidationReport, fs PackageFileSystem, m *Manifest) {
	actionKeys := make(map[string]bool, len(m.Actions))
	for _, action := range m.Actions {
		actionKeys[action.Key] = true
	}

	for _, action := range m.Actions {
		if action.Config == "" {
			report.addFinding(Finding{Code: ErrCodeActionConfigMissing, Severity: SeverityError, ActionKey: action.Key, Message: "legacy action config path is empty"})
			continue
		}
		rc, err := fs.Open(action.Config)
		if err != nil {
			report.addFinding(Finding{Code: ErrCodeActionConfigMissing, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Message: fmt.Sprintf("failed to open legacy action config: %v", err)})
			continue
		}
		data, readErr := io.ReadAll(rc)
		rc.Close()
		if readErr != nil {
			report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Message: fmt.Sprintf("failed to read legacy action config: %v", readErr)})
			continue
		}

		var cfg legacyValidatedActionConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Message: fmt.Sprintf("failed to parse legacy action config: %v", err)})
			continue
		}
		configKey := strings.TrimSpace(cfg.ActionKey)
		if configKey == "" {
			configKey = strings.TrimSpace(cfg.Key)
		}
		if configKey != "" && configKey != action.Key {
			report.addFinding(Finding{Code: ErrCodePackageActionKeyMismatch, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: action.Key, Actual: configKey, Message: "legacy action key does not match manifest"})
		}

		fps := cfg.Fps
		if fps == 0 {
			fps = cfg.DefaultFps
		}
		if fps < 0 || fps > 120 {
			report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: "0 <= fps <= 120 for legacy packages", Actual: fmt.Sprintf("fps=%d", fps), Message: "invalid legacy fps"})
		}
		mode := NormalizePlaybackMode(cfg.PlaybackMode)
		if mode == "" {
			mode = NormalizePlaybackMode(cfg.LoopType)
		}
		if mode != "" && !IsValidPlaybackMode(mode) {
			report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: "loop/once/hold/ping_pong or legacy alias", Actual: cfg.PlaybackMode, Message: "invalid legacy playback mode"})
		}

		if len(cfg.Frames) == 0 {
			report.addFinding(Finding{Code: ErrCodeFrameMissing, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Message: "legacy action config has no frames"})
			continue
		}
		seenFramePaths := make(map[string]bool, len(cfg.Frames))
		for i, rawFrame := range cfg.Frames {
			frameFile := ""
			if err := json.Unmarshal(rawFrame, &frameFile); err != nil || frameFile == "" {
				var frame legacyValidatedFrame
				if objErr := json.Unmarshal(rawFrame, &frame); objErr != nil {
					report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Message: fmt.Sprintf("invalid legacy frame at index %d: %v", i, objErr)})
					continue
				}
				frameFile = frame.File
				if frame.Index != nil && *frame.Index != i {
					report.addFinding(Finding{Code: ErrCodePackageFrameIndexInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: fmt.Sprintf("index=%d", i), Actual: fmt.Sprintf("index=%d", *frame.Index), Message: "legacy frame index is not contiguous"})
				}
				if frame.DurationMs != nil && (*frame.DurationMs <= 0 || *frame.DurationMs > 60000) {
					report.addFinding(Finding{Code: ErrCodeActionConfigInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Expected: "0 < durationMs <= 60000", Actual: fmt.Sprintf("durationMs=%d", *frame.DurationMs), Message: "invalid legacy frame duration"})
				}
			}
			if strings.TrimSpace(frameFile) == "" {
				report.addFinding(Finding{Code: ErrCodeFrameMissing, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Message: fmt.Sprintf("legacy frame %d has empty file", i)})
				continue
			}
			framePath, pathErr := resolveActionResourcePath(action.Config, frameFile)
			if pathErr != nil {
				report.addFinding(Finding{Code: ErrCodePackagePathInvalid, Severity: SeverityError, Path: frameFile, ActionKey: action.Key, Message: fmt.Sprintf("invalid legacy frame path: %v", pathErr)})
				continue
			}
			if seenFramePaths[framePath] {
				report.addFinding(Finding{Code: ErrCodePackageDuplicateEntry, Severity: SeverityError, Path: framePath, ActionKey: action.Key, Message: "duplicate legacy frame file"})
			}
			seenFramePaths[framePath] = true
			if _, statErr := fs.Stat(framePath); statErr != nil {
				report.addFinding(Finding{Code: ErrCodeFrameMissing, Severity: SeverityError, Path: framePath, ActionKey: action.Key, Message: fmt.Sprintf("legacy frame resource is missing: %s", framePath)})
			}
		}

		if cfg.ReturnAction != "" && !actionKeys[cfg.ReturnAction] {
			report.addFinding(Finding{Code: ErrCodeActionReferenceInvalid, Severity: SeverityError, Path: action.Config, ActionKey: action.Key, Actual: cfg.ReturnAction, Message: fmt.Sprintf("legacy returnAction target not found: %s", cfg.ReturnAction)})
		}
	}
}

func resolveActionResourcePath(configPath, resourcePath string) (string, error) {
	if _, err := NormalizePackagePath(configPath); err != nil {
		return "", err
	}
	if strings.TrimSpace(resourcePath) == "" {
		return "", fmt.Errorf("resource path is empty")
	}
	normalizedResource, err := NormalizePackagePath(resourcePath)
	if err != nil {
		return "", err
	}
	joined := pathpkg.Clean(pathpkg.Join(pathpkg.Dir(configPath), normalizedResource))
	return NormalizePackagePath(joined)
}

func isLowerHexSHA256(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func (v *Validator) validateSecurityLayerFS(report *ValidationReport, fs PackageFileSystem) {
	paths, err := fs.List()
	if err != nil {
		report.addFinding(Finding{
			Code:     ErrCodePackagePathInvalid,
			Severity: SeverityError,
			Message:  fmt.Sprintf("security list failed: %v", err),
		})
		return
	}

	seenCaseFold := make(map[string]string)
	for _, relSlash := range paths {
		fi, statErr := fs.Stat(relSlash)
		if statErr != nil {
			continue
		}

		if fi.Mode()&os.ModeSymlink != 0 {
			report.addFinding(Finding{
				Code:     ErrCodePackageSymlinkForbidden,
				Severity: SeverityError,
				Path:     relSlash,
				Message:  "symlink is not allowed in package",
			})
			continue
		}

		base := filepath.Base(relSlash)
		if strings.HasPrefix(base, ".") {
			report.addFinding(Finding{
				Code:     ErrCodePackagePathInvalid,
				Severity: SeverityError,
				Path:     relSlash,
				Message:  "hidden file or directory is not allowed",
			})
		}

		if isForbiddenExecutable(relSlash) {
			report.addFinding(Finding{
				Code:     ErrCodePackageExecutableForbidden,
				Severity: SeverityError,
				Path:     relSlash,
				Message:  "executable file is not allowed in package",
			})
		}

		folded := CaseFoldPath(relSlash)
		if existing, dup := seenCaseFold[folded]; dup {
			report.addFinding(Finding{
				Code:     ErrCodePackageDuplicateEntry,
				Severity: SeverityError,
				Path:     relSlash,
				Message:  fmt.Sprintf("case-insensitive path collision: %s conflicts with %s", relSlash, existing),
			})
		} else {
			seenCaseFold[folded] = relSlash
		}
	}
}

func (r *ValidationReport) addFinding(f Finding) {
	if f.Severity == "" {
		f.Severity = SeverityError
	}
	r.Findings = append(r.Findings, f)
	switch f.Severity {
	case SeverityError:
		r.ErrorCount++
	case SeverityWarning:
		r.WarningCount++
	}
}

func (r *ValidationReport) Finalize() {
	if r.ErrorCount > 0 {
		r.Verdict = "invalid"
	} else if r.WarningCount > 0 {
		r.Verdict = "valid_with_warnings"
	} else {
		r.Verdict = "valid"
	}

	sort.SliceStable(r.Findings, func(i, j int) bool {
		if r.Findings[i].Severity != r.Findings[j].Severity {
			return severityOrder(r.Findings[i].Severity) < severityOrder(r.Findings[j].Severity)
		}
		return r.Findings[i].Path < r.Findings[j].Path
	})
}

func severityOrder(s string) int {
	switch s {
	case SeverityError:
		return 0
	case SeverityWarning:
		return 1
	case SeverityInfo:
		return 2
	default:
		return 3
	}
}

func countFiles(root string) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}

var strictSemVerPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

func isValidRuntimeVersion(version string) bool {
	return strictSemVerPattern.MatchString(version)
}
