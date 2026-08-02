package importer

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/release"
)

type PackageImporter struct {
	repo      release.ReleaseRepository
	storage   release.ReleaseStoragePort
	validator PackageValidator
}

type PackageValidator interface {
	ValidatePackage(stagingID string) (*ImportValidationResult, error)
}

type ImportValidationResult struct {
	IsValid              bool
	SourcePackageHash    string
	SourceManifestHash   string
	SourceSchemaVersion  int
	Warnings             []string
	BindingDecision      string
	LicenseDecision      string
	RuntimeCompatibility string
	SelectedActions      []string
}

type ImportPackageRequest struct {
	UserID          string
	ImportStagingID string
	SourceFilePath  string
	PreferPetID     string
	IdempotencyKey  string
}

type ImportPackageResult struct {
	ImportSnapshot *release.ImportPackageSnapshot
	PetID          string
	ReleaseID      string
	OperationID    string
}

func NewPackageImporter(
	repo release.ReleaseRepository,
	storage release.ReleaseStoragePort,
	validator PackageValidator,
) *PackageImporter {
	return &PackageImporter{
		repo:      repo,
		storage:   storage,
		validator: validator,
	}
}

func (pi *PackageImporter) ImportPackage(ctx context.Context, req *ImportPackageRequest) (*ImportPackageResult, error) {
	if req.UserID == "" {
		return nil, release.NewReleaseError("INVALID_USER", "用户 ID 不能为空", nil)
	}
	if req.ImportStagingID == "" {
		return nil, release.NewReleaseError("INVALID_STAGING", "导入暂存 ID 不能为空", nil)
	}

	existing, err := pi.repo.GetImportSnapshot(req.ImportStagingID)
	if err == nil && existing != nil {
		return pi.loadExistingImport(existing)
	}

	validation, err := pi.validator.ValidatePackage(req.ImportStagingID)
	if err != nil {
		return nil, release.NewReleaseError("VALIDATION_FAILED", "包验证失败", err)
	}

	if !validation.IsValid {
		return nil, release.NewReleaseError("PACKAGE_INVALID", fmt.Sprintf("包验证不通过: %v", validation.Warnings), nil)
	}

	snapshot := &release.ImportPackageSnapshot{
		ImportStagingID:       req.ImportStagingID,
		SourcePackageHash:     validation.SourcePackageHash,
		SourceManifestHash:    validation.SourceManifestHash,
		SourceSchemaVersion:   validation.SourceSchemaVersion,
		NormalizationWarnings: formatWarnings(validation.Warnings),
		SelectedActionsJSON:   formatSelectedActions(validation.SelectedActions),
		BindingDecision:       validation.BindingDecision,
		LicenseDecision:       validation.LicenseDecision,
		RuntimeCompatibility:  validation.RuntimeCompatibility,
		UserID:                req.UserID,
	}

	if err := pi.repo.CreateImportSnapshot(snapshot); err != nil {
		return nil, release.NewReleaseError("SNAPSHOT_CREATE_FAILED", "创建导入快照失败", err)
	}

	return &ImportPackageResult{
		ImportSnapshot: snapshot,
		PetID:          req.PreferPetID,
	}, nil
}

func (pi *PackageImporter) loadExistingImport(snapshot *release.ImportPackageSnapshot) (*ImportPackageResult, error) {
	result := &ImportPackageResult{
		ImportSnapshot: snapshot,
	}
	if snapshot.PetID != "" {
		result.PetID = snapshot.PetID
	}
	if snapshot.ReleaseID != "" {
		result.ReleaseID = snapshot.ReleaseID
	}
	return result, nil
}

func formatWarnings(warnings []string) string {
	if len(warnings) == 0 {
		return "[]"
	}
	result := "["
	for i, w := range warnings {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%q", w)
	}
	result += "]"
	return result
}

func formatSelectedActions(actions []string) string {
	if len(actions) == 0 {
		return "[]"
	}
	result := "["
	for i, a := range actions {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%q", a)
	}
	result += "]"
	return result
}
