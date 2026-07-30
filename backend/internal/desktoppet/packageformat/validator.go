package packageformat

import (
	"fmt"
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

	v.validateSchemaLayer(report, manifest)
	v.validatePathLayer(report, manifest)

	v.validateFileLayer(report, root, manifest)
	v.validateActionLayer(report, manifest)
	v.validateSecurityLayer(report, root)

	count, _ := countFiles(root)
	report.FileCount = count
	report.Finalize()
	return report
}

func (v *Validator) ValidateArchive(path string) *ValidationReport {
	report := &ValidationReport{Verdict: "valid"}

	reader := NewArchiveReader(DefaultArchiveLimits())
	manifest, _, err := reader.ReadArchive(path)
	if err != nil {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityError,
			Message:  fmt.Sprintf("failed to read archive: %v", err),
		})
		report.Finalize()
		return report
	}

	if manifest == nil {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityError,
			Message:  "manifest not found in archive",
		})
		report.Finalize()
		return report
	}

	v.validateSchemaLayer(report, manifest)
	v.validatePathLayer(report, manifest)
	v.validateActionLayer(report, manifest)

	report.Finalize()
	return report
}

func (v *Validator) validateSchemaLayer(report *ValidationReport, m *Manifest) {
	if m.SchemaVersion != ManifestSchemaVersion {
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

	if m.Integrity.Algorithm != TreeHashAlgorithm {
		report.addFinding(Finding{
			Code:     ErrCodePackageManifestInvalid,
			Severity: SeverityWarning,
			Expected: TreeHashAlgorithm,
			Actual:   m.Integrity.Algorithm,
			Message:  fmt.Sprintf("unexpected integrity algorithm: %s", m.Integrity.Algorithm),
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

func (v *Validator) validateFileLayer(report *ValidationReport, root string, m *Manifest) {
	fileManifest := &FileManifest{Entries: m.Integrity.Files}
	if err := ValidateAgainstManifest(root, fileManifest); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			finding := Finding{
				Code:     ve.Code,
				Severity: ve.Severity,
				Path:     ve.Path,
				Message:  ve.Message,
			}
			if finding.Severity == "" {
				finding.Severity = SeverityError
			}
			report.addFinding(finding)
		} else {
			report.addFinding(Finding{
				Code:     ErrCodePackageHashMismatch,
				Severity: SeverityError,
				Message:  fmt.Sprintf("file validation failed: %v", err),
			})
		}
		return
	}

	var entries []FileEntry
	for _, f := range m.Integrity.Files {
		entries = append(entries, FileEntry{
			Path:   f.Path,
			SHA256: f.SHA256,
			Bytes:  f.Bytes,
		})
	}
	actualRootHash := ComputeTreeHash(entries)
	if m.Integrity.ContentRootHash != "" && actualRootHash != m.Integrity.ContentRootHash {
		report.addFinding(Finding{
			Code:     ErrCodePackageHashMismatch,
			Severity: SeverityError,
			Expected: m.Integrity.ContentRootHash,
			Actual:   actualRootHash,
			Message:  "content root hash mismatch",
		})
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
			Code:      ErrCodePackageManifestInvalid,
			Severity:  SeverityError,
			ActionKey: m.DefaultAction,
			Expected:  "an existing action key",
			Actual:    m.DefaultAction,
			Message:   fmt.Sprintf("defaultAction %s not found in actions", m.DefaultAction),
		})
	}
}

func (v *Validator) validateSecurityLayer(report *ValidationReport, root string) {
	seenCaseFold := make(map[string]string)

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			report.addFinding(Finding{
				Code:     ErrCodePackagePathInvalid,
				Severity: SeverityError,
				Path:     path,
				Message:  fmt.Sprintf("walk error: %v", walkErr),
			})
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		if rel == "." {
			return nil
		}
		relSlash := filepath.ToSlash(rel)

		fi, statErr := d.Info()
		if statErr != nil {
			return nil
		}

		if fi.Mode()&os.ModeSymlink != 0 {
			report.addFinding(Finding{
				Code:     ErrCodePackageSymlinkForbidden,
				Severity: SeverityError,
				Path:     relSlash,
				Message:  "symlink is not allowed in package",
			})
			return nil
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

		if !d.IsDir() {
			if isForbiddenExecutable(relSlash) {
				report.addFinding(Finding{
					Code:     ErrCodePackageExecutableForbidden,
					Severity: SeverityError,
					Path:     relSlash,
					Message:  "executable file is not allowed in package",
				})
			}
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

		return nil
	})
	if err != nil {
		report.addFinding(Finding{
			Code:     ErrCodePackagePathInvalid,
			Severity: SeverityError,
			Message:  fmt.Sprintf("security walk failed: %v", err),
		})
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
