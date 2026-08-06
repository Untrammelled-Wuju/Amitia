package importer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/desktoppet/packageformat"
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

	warnings :=
		collectValidationWarnings(
			report.Findings,
		)

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

	return &ImportValidationResult{
		IsValid:
			report.Verdict == "valid" ||
				report.Verdict ==
					"valid_with_warnings",
		SourcePackageHash:
			staging.SourceContentHash,
		SourceManifestHash:
			manifest.Integrity.
				ManifestHash,
		SourceSchemaVersion:
			manifest.SchemaVersion,
		Warnings:
			warnings,
		BindingDecision:
			manifest.Binding.Policy,
		LicenseDecision:
			licenseDecision,
		RuntimeCompatibility:
			manifest.Compatibility.
				MinRuntimeVersion,
		SelectedActions:
			selectedActions,
		Manifest:
			manifest,
		ValidationReport:
			report,
	}, nil
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
