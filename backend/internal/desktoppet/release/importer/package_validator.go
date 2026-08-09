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
	registry *security.PathRootRegistry
	repo     security.ImportStagingRepository
	reader   *packageformat.ArchiveReader
	validator *packageformat.Validator
}

func NewDefaultPackageValidator(
	registry *security.PathRootRegistry,
	repo security.ImportStagingRepository,
) *DefaultPackageValidator {
	return &DefaultPackageValidator{
		registry: registry,
		repo: repo,
		reader: packageformat.NewArchiveReader(
			packageformat.DefaultArchiveLimits(),
		),
		validator: packageformat.NewValidator(),
	}
}

func (
	v *DefaultPackageValidator,
) ValidatePackage(
	ctx context.Context,
	userID string,
	stagingID string,
) (
	*ImportValidationResult,
	error,
) {
	if v == nil ||
		v.registry == nil ||
		v.repo == nil ||
		v.reader == nil ||
		v.validator == nil {
		return nil, errors.New(
			"package validator dependencies missing",
		)
	}

	staging, err :=
		v.repo.GetForUser(
			ctx,
			stagingID,
			userID,
		)
	if err != nil {
		return nil, err
	}

	if staging.Status !=
		security.StagingStatusReady &&
		staging.Status !=
			security.StagingStatusConsuming {
		return nil, errors.New(
			"staging status is invalid",
		)
	}

	sourcePath, err :=
		v.registry.Resolve(
			security.RootImportQuarantine,
			staging.StorageKey,
		)
	if err != nil {
		return nil, err
	}

	manifest, _, err :=
		v.reader.ReadArchive(sourcePath)
	if err != nil {
		return nil, err
	}

	report :=
		v.validator.ValidateArchive(
			sourcePath,
		)

	if report == nil {
		return nil, errors.New(
			"package validation report is nil",
		)
	}

	if report.Verdict == "invalid" {
		return nil, fmt.Errorf(
			"package validation failed: %s",
			summarizeValidationFindings(
				report.Findings,
			),
		)
	}

	selectedActions :=
		make(
			[]string,
			0,
			len(manifest.Actions),
		)

	for _, action :=
		range manifest.Actions {
		selectedActions = append(
			selectedActions,
			action.Key,
		)
	}

	licenseDecision := "undeclared"
	if strings.TrimSpace(
		manifest.License.SPDX,
	) != "" {
		licenseDecision = "declared"
	}

	archiveValid := report.Verdict == "valid" || report.Verdict == "valid_with_warnings"

	workspaceReport := v.validator.ValidateDirectory(sourcePath, manifest)
	workspaceValid := workspaceReport.Verdict == "valid" || workspaceReport.Verdict == "valid_with_warnings"

	allWarnings := collectValidationWarnings(report.Findings)
	allWarnings = append(allWarnings, collectValidationWarnings(workspaceReport.Findings)...)

	integrityVerified := archiveValid && workspaceValid && staging.SourceContentHash != ""

	compatibility := checkRuntimeCompatibility(manifest.Compatibility)

	return &ImportValidationResult{
		IsValid:             archiveValid,
		IntegrityVerified:   integrityVerified,
		Compatibility:       compatibility,
		SourcePackageHash:   staging.SourceContentHash,
		SourceManifestHash:  manifest.Integrity.ManifestHash,
		SourceSchemaVersion: manifest.SchemaVersion,
		Warnings:            allWarnings,
		BindingDecision:     manifest.Binding.Policy,
		LicenseDecision:     licenseDecision,
		RuntimeCompatibility: string(compatibility),
		SelectedActions:     selectedActions,
		Manifest:            manifest,
		ValidationReport:    report,
	}, nil
}

func checkRuntimeCompatibility(compat packageformat.ManifestCompatibility) release.ReleaseCompatibilityStatus {
	current := parseSemVer(contracts.RuntimeContractVersion)
	minV := parseSemVer(compat.MinRuntimeVersion)
	if minV == nil {
		return release.ReleaseCompatIncompatible
	}
	if compareSemVer(current, minV) < 0 {
		return release.ReleaseCompatIncompatible
	}
	if compat.MaxRuntimeVersion != nil {
		maxV := parseSemVer(*compat.MaxRuntimeVersion)
		if maxV == nil {
			return release.ReleaseCompatIncompatible
		}
		if compareSemVer(current, maxV) > 0 {
			return release.ReleaseCompatIncompatible
		}
	}
	return release.ReleaseCompatCompatible
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
		if err != nil {
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

func summarizeValidationFindings(
	findings []packageformat.Finding,
) string {
	const maxFindings = 8

	parts := make([]string, 0)
	for _, finding := range findings {
		if finding.Severity !=
			packageformat.SeverityError {
			continue
		}

		parts = append(
			parts,
			string(finding.Code),
		)

		if len(parts) >= maxFindings {
			break
		}
	}

	return strings.Join(parts, ",")
}

func collectValidationWarnings(
	findings []packageformat.Finding,
) []string {
	result := make([]string, 0)

	for _, finding := range findings {
		if finding.Severity ==
			packageformat.SeverityWarning {
			result = append(
				result,
				string(finding.Code),
			)
		}
	}

	return result
}
