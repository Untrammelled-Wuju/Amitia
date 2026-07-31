package packageformat

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

	if m.Version == "" {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityError,
			Message:  "version is empty",
		})
	}

	if m.Name == "" {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityError,
			Message:  "name is empty",
		})
	}

	if m.Canvas.Width <= 0 || m.Canvas.Height <= 0 {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityError,
			Expected: "positive canvas dimensions",
			Actual:   fmt.Sprintf("%dx%d", m.Canvas.Width, m.Canvas.Height),
			Message:  "invalid canvas dimensions",
		})
	}

	if m.Integrity.Algorithm != IntegrityAlgorithmV2 && m.Integrity.Algorithm != IntegrityAlgorithmV1Legacy {
		report.addFinding(Finding{
			Code:     ErrCodePackageIntegrityAlgorithmUnsupported,
			Severity: SeverityError,
			Expected: IntegrityAlgorithmV2,
			Actual:   m.Integrity.Algorithm,
			Message:  fmt.Sprintf("unsupported integrity algorithm: %s", m.Integrity.Algorithm),
		})
	}

	if m.Compatibility.MinRuntimeVersion != "" && !isValidRuntimeVersion(m.Compatibility.MinRuntimeVersion) {
		report.addFinding(Finding{
			Code:     ErrCodeRuntimeVersionUnsupported,
			Severity: SeverityError,
			Actual:   m.Compatibility.MinRuntimeVersion,
			Message:  fmt.Sprintf("unsupported minRuntimeVersion: %s", m.Compatibility.MinRuntimeVersion),
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

func (v *Validator) validateActionLayer(report *ValidationReport, m *Manifest) {
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
			report.addFinding(Finding{
				Code:      ErrCodePackageManifestInvalid,
				Severity:  SeverityWarning,
				ActionKey: action.Key,
				Message:   "action name is empty",
			})
		}
		if action.Config == "" {
			report.addFinding(Finding{
				Code:      ErrCodePackageManifestInvalid,
				Severity:  SeverityError,
				ActionKey: action.Key,
				Message:   "action config path is empty",
			})
		}
		if action.FrameCount < 0 {
			report.addFinding(Finding{
				Code:      ErrCodePackageManifestInvalid,
				Severity:  SeverityError,
				ActionKey: action.Key,
				Message:   fmt.Sprintf("negative frame count: %d", action.FrameCount),
			})
		}
		if action.FPS < 0 {
			report.addFinding(Finding{
				Code:      ErrCodePackageManifestInvalid,
				Severity:  SeverityError,
				ActionKey: action.Key,
				Message:   fmt.Sprintf("negative FPS: %d", action.FPS),
			})
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
	SchemaVersion int                   `json:"schemaVersion"`
	ActionKey     string                `json:"actionKey"`
	DisplayName   string                `json:"displayName"`
	ActionName    string                `json:"actionName"`
	Fps           int                   `json:"fps"`
	DefaultFps    int                   `json:"defaultFps"`
	PlaybackMode  string                `json:"playbackMode"`
	LoopType      string                `json:"loopType"`
	Frames        []validatedFrameEntry `json:"frames"`
	ReturnTo      validatedReturnTo     `json:"returnTo"`
	Anchor        validatedAnchor       `json:"anchor"`
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
		if jErr := json.Unmarshal(data, &cfg); jErr != nil {
			report.addFinding(Finding{
				Code:      ErrCodeActionConfigInvalid,
				Severity:  SeverityError,
				Path:      action.Config,
				ActionKey: action.Key,
				Message:   fmt.Sprintf("failed to parse action config: %v", jErr),
			})
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

			if frame.ContentHash == "" {
				report.addFinding(Finding{
					Code:      ErrCodeFrameHashMismatch,
					Severity:  SeverityError,
					Path:      action.Config,
					ActionKey: action.Key,
					Message:   fmt.Sprintf("frame %d has empty contentHash", idx),
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
		}

		if cfg.ReturnTo.Type == "default_idle" {
			report.addFinding(Finding{
				Code:      ErrCodeActionConfigInvalid,
				Severity:  SeverityError,
				Path:      action.Config,
				ActionKey: action.Key,
				Actual:    cfg.ReturnTo.Type,
				Message:   "returnTo type default_idle is rejected, use default instead",
			})
		}

		if cfg.ReturnTo.Type == "action" {
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

func isValidRuntimeVersion(version string) bool {
	if version == "" {
		return true
	}
	_, _, _, ok := parseSemver(version)
	return ok
}

func parseSemver(version string) (int, int, int, bool) {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 1 {
		return 0, 0, 0, false
	}
	major, mOk := parseIntStr(parts[0])
	if !mOk {
		return 0, 0, 0, false
	}
	minor := 0
	patch := 0
	if len(parts) >= 2 {
		minor, mOk = parseIntStr(parts[1])
		if !mOk {
			return 0, 0, 0, false
		}
	}
	if len(parts) >= 3 {
		patch, mOk = parseIntStr(stripPreRelease(parts[2]))
		if !mOk {
			return 0, 0, 0, false
		}
	}
	return major, minor, patch, true
}

func parseIntStr(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

func stripPreRelease(s string) string {
	if idx := strings.IndexByte(s, '-'); idx >= 0 {
		return s[:idx]
	}
	return s
}

var forbiddenExecutableExtensions = map[string]bool{
	".exe":   true,
	".dll":   true,
	".so":    true,
	".dylib": true,
	".sh":    true,
	".bat":   true,
	".cmd":   true,
	".com":   true,
	".msi":   true,
	".scr":   true,
	".jar":   true,
	".app":   true,
	".bin":   true,
	".ps1":   true,
}

func isForbiddenExecutable(relPath string) bool {
	lower := strings.ToLower(relPath)
	for ext := range forbiddenExecutableExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}

	if fi, err := os.Stat(relPath); err == nil {
		if fi.Mode()&0o111 != 0 {
			return true
		}
	}
	return false
}
