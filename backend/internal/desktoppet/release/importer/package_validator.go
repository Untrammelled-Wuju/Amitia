package importer

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"github.com/u-ai/backend/internal/desktoppet/release"
	"github.com/u-ai/backend/internal/desktoppet/security"
)

type DefaultPackageValidator struct {
	registry  *security.PathRootRegistry
	repo      security.ImportStagingRepository
	reader    *packageformat.ArchiveReader
	validator *packageformat.Validator
}

func NewDefaultPackageValidator(
	registry *security.PathRootRegistry,
	repo security.ImportStagingRepository,
) *DefaultPackageValidator {
	return &DefaultPackageValidator{
		registry: registry,
		repo:     repo,
		reader: packageformat.NewArchiveReader(
			packageformat.DefaultArchiveLimits(),
		),
		validator: packageformat.NewValidator(),
	}
}

func (v *DefaultPackageValidator) ValidatePackage(
	ctx context.Context,
	userID string,
	stagingID string,
) (*ImportValidationResult, error) {
	if v == nil || v.registry == nil || v.repo == nil || v.reader == nil || v.validator == nil {
		return nil, errors.New("package validator dependencies missing")
	}

	staging, err := v.repo.GetForUser(ctx, stagingID, userID)
	if err != nil {
		return nil, err
	}
	if staging.Status != security.StagingStatusReady && staging.Status != security.StagingStatusConsuming {
		return nil, errors.New("staging status is invalid")
	}

	sourcePath, err := v.registry.Resolve(security.RootImportQuarantine, staging.StorageKey)
	if err != nil {
		return nil, err
	}

	manifest, _, err := v.reader.ReadArchive(sourcePath)
	if err != nil {
		return nil, err
	}

	archiveReport := v.validator.ValidateArchive(sourcePath)
	if archiveReport == nil {
		return nil, errors.New("package validation report is nil")
	}
	if archiveReport.Verdict == "invalid" {
		return nil, fmt.Errorf("package validation failed: %s", summarizeValidationFindings(archiveReport.Findings))
	}

	compatibility, err := checkRuntimeCompatibility(manifest.Compatibility)
	if err != nil {
		return nil, err
	}

	selectedActions := make([]string, 0, len(manifest.Actions))
	for _, action := range manifest.Actions {
		selectedActions = append(selectedActions, action.Key)
	}

	licenseDecision := "undeclared"
	if strings.TrimSpace(manifest.License.SPDX) != "" {
		licenseDecision = "declared"
	}

	archiveValid := archiveReport.Verdict == "valid" || archiveReport.Verdict == "valid_with_warnings"

	// IntegrityVerified intentionally remains false here. At this stage only the
	// immutable archive has been validated. The importer promotes integrity to
	// verified only after extraction, directory validation and verified archive
	// storage all succeed.
	return &ImportValidationResult{
		IsValid:              archiveValid,
		IntegrityVerified:    false,
		Compatibility:        compatibility,
		SourcePackageHash:    staging.SourceContentHash,
		SourceManifestHash:   manifest.Integrity.ManifestHash,
		SourceSchemaVersion:  manifest.SchemaVersion,
		Warnings:             collectValidationWarnings(archiveReport.Findings),
		BindingDecision:      manifest.Binding.Policy,
		LicenseDecision:      licenseDecision,
		RuntimeCompatibility: string(compatibility),
		SelectedActions:      selectedActions,
		Manifest:             manifest,
		ValidationReport:     archiveReport,
	}, nil
}

func checkRuntimeCompatibility(compat packageformat.ManifestCompatibility) (release.ReleaseCompatibilityStatus, error) {
	current := parseSemVer(contracts.RuntimeContractVersion)
	if current == nil {
		return release.ReleaseCompatIncompatible, release.NewReleaseError(
			"PACKAGE_RUNTIME_VERSION_INVALID",
			"当前 Runtime Contract Version 非法",
			nil,
		)
	}

	minV := parseSemVer(compat.MinRuntimeVersion)
	if minV == nil {
		return release.ReleaseCompatIncompatible, release.NewReleaseError(
			"PACKAGE_RUNTIME_VERSION_INVALID",
			"桌宠包 minRuntimeVersion 非法",
			nil,
		)
	}
	if compareSemVer(current, minV) < 0 {
		return release.ReleaseCompatIncompatible, nil
	}

	if compat.MaxRuntimeVersion != nil {
		maxV := parseSemVer(*compat.MaxRuntimeVersion)
		if maxV == nil {
			return release.ReleaseCompatIncompatible, release.NewReleaseError(
				"PACKAGE_RUNTIME_VERSION_INVALID",
				"桌宠包 maxRuntimeVersion 非法",
				nil,
			)
		}
		if compareSemVer(current, maxV) > 0 {
			return release.ReleaseCompatIncompatible, nil
		}
	}
	return release.ReleaseCompatCompatible, nil
}

func parseSemVer(s string) []int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 4 {
		return nil
	}
	result := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 {
			return nil
		}
		result[i] = n
	}
	return result
}

func compareSemVer(a, b []int) int {
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	for i := 0; i < maxLen; i++ {
		ai := 0
		if i < len(a) {
			ai = a[i]
		}
		bi := 0
		if i < len(b) {
			bi = b[i]
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return 0
}

func summarizeValidationFindings(findings []packageformat.Finding) string {
	const maxFindings = 8
	parts := make([]string, 0)
	for _, finding := range findings {
		if finding.Severity != packageformat.SeverityError {
			continue
		}
		parts = append(parts, string(finding.Code))
		if len(parts) >= maxFindings {
			break
		}
	}
	return strings.Join(parts, ",")
}

func collectValidationWarnings(findings []packageformat.Finding) []string {
	result := make([]string, 0)
	for _, finding := range findings {
		if finding.Severity == packageformat.SeverityWarning {
			result = append(result, string(finding.Code))
		}
	}
	return result
}
