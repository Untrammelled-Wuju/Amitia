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
	current := parseSemVer(contracts.RuntimeVersion)
	if current == nil {
		return release.ReleaseCompatIncompatible, release.NewReleaseError(
			"PACKAGE_RUNTIME_VERSION_INVALID",
			"当前 Runtime Version 非法",
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

type semVer struct {
	major      int
	minor      int
	patch      int
	prerelease []string
}

func parseSemVer(s string) *semVer {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	withoutBuild := strings.SplitN(s, "+", 2)[0]
	parts := strings.SplitN(withoutBuild, "-", 2)
	core := strings.Split(parts[0], ".")
	if len(core) != 3 {
		return nil
	}

	values := make([]int, 3)
	for i, part := range core {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return nil
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return nil
		}
		values[i] = n
	}

	var prerelease []string
	if len(parts) == 2 {
		if parts[1] == "" {
			return nil
		}
		prerelease = strings.Split(parts[1], ".")
		for _, identifier := range prerelease {
			if identifier == "" {
				return nil
			}
			numeric := true
			for _, r := range identifier {
				if !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '-') {
					return nil
				}
				if r < '0' || r > '9' {
					numeric = false
				}
			}
			if numeric && len(identifier) > 1 && identifier[0] == '0' {
				return nil
			}
		}
	}

	// Validate build metadata separately because SplitN above intentionally
	// ignores it for ordering but SemVer still requires valid identifiers.
	if plus := strings.IndexByte(s, '+'); plus >= 0 {
		build := s[plus+1:]
		if build == "" {
			return nil
		}
		for _, identifier := range strings.Split(build, ".") {
			if identifier == "" {
				return nil
			}
			for _, r := range identifier {
				if !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '-') {
					return nil
				}
			}
		}
	}

	return &semVer{
		major:      values[0],
		minor:      values[1],
		patch:      values[2],
		prerelease: prerelease,
	}
}

func compareSemVer(a, b *semVer) int {
	if a == nil || b == nil {
		panic("compareSemVer requires valid versions")
	}
	for _, pair := range [][2]int{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}

	if len(a.prerelease) == 0 && len(b.prerelease) == 0 {
		return 0
	}
	if len(a.prerelease) == 0 {
		return 1
	}
	if len(b.prerelease) == 0 {
		return -1
	}

	max := len(a.prerelease)
	if len(b.prerelease) < max {
		max = len(b.prerelease)
	}
	for i := 0; i < max; i++ {
		ai, bi := a.prerelease[i], b.prerelease[i]
		an, aErr := strconv.Atoi(ai)
		bn, bErr := strconv.Atoi(bi)
		switch {
		case aErr == nil && bErr == nil:
			if an < bn {
				return -1
			}
			if an > bn {
				return 1
			}
		case aErr == nil:
			return -1
		case bErr == nil:
			return 1
		default:
			if ai < bi {
				return -1
			}
			if ai > bi {
				return 1
			}
		}
	}
	if len(a.prerelease) < len(b.prerelease) {
		return -1
	}
	if len(a.prerelease) > len(b.prerelease) {
		return 1
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
