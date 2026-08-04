package kernel

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type PackageRollbackPreviewResult struct {
	ExtensionID               string   `json:"extensionId"`
	CurrentVersion            string   `json:"currentVersion"`
	TargetVersion             string   `json:"targetVersion"`
	CurrentGeneration         int64    `json:"currentGeneration"`
	TargetArtifactID          string   `json:"targetArtifactId"`
	InstalledPath             string   `json:"installedPath"`
	InstalledHash             string   `json:"installedTreeHash"`
	RollbackPointID           string   `json:"rollbackPointId"`
	SnapshotHash              string   `json:"snapshotHash"`
	RetentionState            string   `json:"retentionState"`
	RetentionUntil            string   `json:"retentionUntil"`
	ManualRequired            bool     `json:"manualRequired"`
	ManualReason              string   `json:"manualReason,omitempty"`
	Dependents                []string `json:"dependents"`
	Installable               bool     `json:"installable"`
	GenerationID              string   `json:"generationId"`
	SourceGenerationID        string   `json:"sourceGenerationId"`
	TargetGenerationID        string   `json:"targetGenerationId"`
	OperationID               string   `json:"operationId"`
	PreviewSessionID          string   `json:"previewSessionId,omitempty"`
	PreviewHash               string   `json:"previewHash,omitempty"`
	SecurityPolicyHash        string   `json:"securityPolicyHash,omitempty"`
	SnapshotRequirementHash   string   `json:"snapshotRequirementHash,omitempty"`
	RequiredConfirmationsHash string   `json:"requiredConfirmationsHash,omitempty"`
	RequiredConfirmations     []string `json:"requiredConfirmations,omitempty"`
	DependenciesHash          string   `json:"dependenciesHash,omitempty"`
}

type PackageRollbackConfirmation struct {
	ConfirmationToken string    `json:"confirmationToken"`
	ExpiresAt         time.Time `json:"expiresAt"`
	PreviewSessionID  string    `json:"previewSessionId"`
}

func (r *Runtime) PreviewPackageRollback(ctx context.Context, extensionID, version, userID, scopeType, scopeID string) (PackageRollbackPreviewResult, error) {
	result := PackageRollbackPreviewResult{
		ExtensionID:   extensionID,
		TargetVersion: version,
		Dependents:    []string{},
		Installable:   true,
	}

	point, err := r.container.PackageRepository.GetRollbackPoint(ctx, extensionID, version)
	if err != nil {
		return result, fmt.Errorf("kernel: rollback point unavailable: %w", err)
	}
	if versionRecord, verErr := r.container.PackageRepository.GetPackageVersion(ctx, extensionID, version); verErr != nil {
		return PackageRollbackPreviewResult{}, fmt.Errorf("kernel: version record unavailable, fail closed: %w", verErr)
	} else {
		switch versionRecord.VersionState {
		case string(PackageVersionStateBlocked), string(PackageVersionStateCorrupted), string(PackageVersionStateRemoved):
			return result, fmt.Errorf("kernel: rollback target version state %s not allowed", versionRecord.VersionState)
		}
	}
	result.RollbackPointID = point.RollbackPointID
	result.TargetArtifactID = point.ArtifactID
	result.SnapshotHash = point.SnapshotHash
	result.RetentionState = point.RetentionState
	result.RetentionUntil = point.RetentionUntil

	if err := validatePackageSnapshot(point); err != nil {
		return result, err
	}

	current, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return result, err
	}
	result.CurrentVersion = current.InstalledVersion.String()
	result.CurrentGeneration = current.Generation

	if err := validatePackageOwner(current, userID, scopeType, scopeID); err != nil {
		return result, err
	}

	result.InstalledPath, _ = current.Metadata["installedPath"].(string)
	result.InstalledHash, _ = current.Metadata["installedTreeHash"].(string)
	result.OperationID, _ = current.Metadata["lastOperationId"].(string)

	generation := packageGenerationFromInstallation(current)
	result.GenerationID = generation.GenerationID
	result.SourceGenerationID = generation.GenerationID

	targetVersionRecord, tvErr := r.container.PackageRepository.GetPackageVersion(ctx, extensionID, version)
	if tvErr != nil || targetVersionRecord.GenerationID == "" {
		return result, fmt.Errorf("kernel: rollback target version generation unavailable: %w", tvErr)
	}
	result.TargetGenerationID = targetVersionRecord.GenerationID

	artifact, err := r.container.PackageRepository.GetArtifact(ctx, point.ArtifactID)
	if err != nil {
		return result, fmt.Errorf("kernel: rollback artifact unavailable: %w", err)
	}
	if _, err := r.VerifyStoredPackage(ctx, artifact); err != nil {
		return result, fmt.Errorf("kernel: rollback artifact verification failed: %w", err)
	}

	definitions, err := r.container.DefinitionRepository.ListExtensions(ctx)
	if err != nil {
		return result, err
	}
	for _, definition := range definitions {
		if definition.ID == current.ExtensionID {
			continue
		}
		dependentInstallation, dependentErr := r.container.InstallationRepository.GetInstallation(ctx, definition.ID)
		if dependentErr != nil || dependentInstallation.InstalledVersion.Compare(definition.Version) != 0 {
			continue
		}
		for _, dependency := range definition.Dependencies {
			if dependency.Type == domain.DependencyTypeExtension && dependency.ID == extensionID {
				result.Dependents = append(result.Dependents, string(definition.ID))
			}
		}
		for _, module := range definition.Modules {
			for _, dependency := range module.Dependencies {
				if dependency.Type == domain.DependencyTypeExtension && dependency.ID == extensionID {
					result.Dependents = append(result.Dependents, string(definition.ID))
				}
			}
			for _, contribution := range module.Contributions {
				for _, dependency := range contribution.Dependencies {
					if dependency.Type == domain.DependencyTypeExtension && dependency.ID == extensionID {
						result.Dependents = append(result.Dependents, string(definition.ID))
					}
				}
			}
		}
	}
	result.Dependents = uniquePackageStrings(result.Dependents)

	_, _, _, migrationJSON, _, snapshotErr := r.capturePackageStateSnapshots(ctx, current)
	if snapshotErr == nil && migrationJSON != "" {
		forwardPoint := PackageRollbackPoint{
			MigrationStateSnapshotJSON: migrationJSON,
		}
		if manualReason := packageSnapshotManualRecoveryReason(forwardPoint, point); manualReason != "" {
			result.ManualRequired = true
			result.ManualReason = manualReason
		}
	}

	previewSessionID := "rollback-preview-" + uuid.NewString()
	result.PreviewSessionID = previewSessionID
	result.SecurityPolicyHash = computeSecurityPolicyHash()

	requirementInput, requirement, err := r.buildRollbackPackageSnapshotRequirement(ctx, current, point, targetVersionRecord)
	if err != nil {
		return result, fmt.Errorf("kernel: build rollback snapshot requirement: %w", err)
	}

	result.SnapshotRequirementHash = requirement.Hash
	result.RequiredConfirmations = []string{"confirm.rollback", PackageConfirmationSnapshotExempt}

	hasMigrationPlan := requirementInput.MigrationPlanHash != "" || requirementInput.MigrationDefinitionHash != ""
	if hasMigrationPlan {
		result.RequiredConfirmations = append(result.RequiredConfirmations, "confirm.rollback.migration_reverse")
	}
	result.RequiredConfirmations = normalizeConfirmedItems(result.RequiredConfirmations)
	result.RequiredConfirmationsHash = computePackageRequiredConfirmationsHash(result.RequiredConfirmations)
	result.DependenciesHash = computePackageDependenciesHash(result.Dependents)

	if result.RequiredConfirmationsHash == "" {
		return result,
			fmt.Errorf(
				"kernel: rollback required confirmations hash missing",
			)
	}

	if result.DependenciesHash == "" {
		return result,
			fmt.Errorf(
				"kernel: rollback dependencies hash missing",
			)
	}

	if result.SecurityPolicyHash == "" {
		return result,
			fmt.Errorf(
				"kernel: rollback security policy hash missing",
			)
	}

	result.PreviewHash = computeRollbackPreviewHash(
		PackageRollbackPreviewHashInput{
			ExtensionID: result.ExtensionID,

			CurrentVersion: result.CurrentVersion,

			TargetVersion: result.TargetVersion,

			RollbackPointID: result.RollbackPointID,

			ArtifactID: result.TargetArtifactID,

			SnapshotHash: result.SnapshotHash,

			SnapshotRequirementHash: result.SnapshotRequirementHash,

			RequiredConfirmationsHash: result.RequiredConfirmationsHash,

			DependenciesHash: result.DependenciesHash,

			SecurityPolicyHash: result.SecurityPolicyHash,

			InstalledPath: result.InstalledPath,

			InstalledTreeHash: result.InstalledHash,

			SourceGenerationID: result.SourceGenerationID,

			TargetGenerationID: result.TargetGenerationID,

			ScopeType: scopeType,

			ScopeID: scopeID,
		},
	)

	if result.PreviewHash == "" {
		return result,
			fmt.Errorf(
				"kernel: rollback preview identity incomplete",
			)
	}

	return result, nil
}

func samePackageRollbackPreview(claims PackageRollbackConfirmationClaims, current PackageRollbackPreviewResult) error {
	if claims.ExtensionID != current.ExtensionID {
		return fmt.Errorf("extensionId mismatch: claims=%s current=%s", claims.ExtensionID, current.ExtensionID)
	}
	if claims.ArtifactID != current.TargetArtifactID {
		return fmt.Errorf("artifactId mismatch: claims=%s current=%s", claims.ArtifactID, current.TargetArtifactID)
	}
	if claims.SourceVersionID != current.CurrentVersion {
		return fmt.Errorf("sourceVersionId mismatch: claims=%s current=%s", claims.SourceVersionID, current.CurrentVersion)
	}
	if claims.SourceGenerationID == "" || claims.TargetGenerationID == "" {
		return fmt.Errorf("%w: claims must bind sourceGenerationId and targetGenerationId", ErrPackageConfirmationStale)
	}
	if claims.SourceGenerationID != current.SourceGenerationID {
		return fmt.Errorf("%w: sourceGenerationId mismatch: claims=%s current=%s", ErrPackageConfirmationStale, claims.SourceGenerationID, current.SourceGenerationID)
	}
	if claims.TargetGenerationID != current.TargetGenerationID {
		return fmt.Errorf("%w: targetGenerationId mismatch: claims=%s current=%s", ErrPackageConfirmationStale, claims.TargetGenerationID, current.TargetGenerationID)
	}
	if claims.TargetVersionID != current.TargetVersion {
		return fmt.Errorf("targetVersionId mismatch: claims=%s current=%s", claims.TargetVersionID, current.TargetVersion)
	}
	if claims.RollbackPointID != current.RollbackPointID {
		return fmt.Errorf("rollbackPointId mismatch: claims=%s current=%s", claims.RollbackPointID, current.RollbackPointID)
	}
	if claims.SecurityPolicyHash != current.SecurityPolicyHash {
		return fmt.Errorf("securityPolicyHash mismatch: claims=%s current=%s", claims.SecurityPolicyHash, current.SecurityPolicyHash)
	}
	if claims.SnapshotRequirementHash != current.SnapshotRequirementHash {
		return fmt.Errorf("snapshotRequirementHash mismatch: claims=%s current=%s", claims.SnapshotRequirementHash, current.SnapshotRequirementHash)
	}
	if claims.RequiredConfirmationsHash == "" {
		return fmt.Errorf("%w: claims must bind requiredConfirmationsHash", ErrPackageConfirmationStale)
	}
	if claims.RequiredConfirmationsHash != current.RequiredConfirmationsHash {
		return fmt.Errorf("requiredConfirmationsHash mismatch: claims=%s current=%s", claims.RequiredConfirmationsHash, current.RequiredConfirmationsHash)
	}
	if claims.DependenciesHash == "" {
		return fmt.Errorf("%w: claims must bind dependenciesHash", ErrPackageConfirmationStale)
	}
	if current.DependenciesHash == "" {
		return fmt.Errorf("%w: current preview dependenciesHash missing", ErrPackageConfirmationStale)
	}
	if claims.DependenciesHash != current.DependenciesHash {
		return fmt.Errorf("%w: dependenciesHash mismatch: claims=%s current=%s", ErrPackageConfirmationStale, claims.DependenciesHash, current.DependenciesHash)
	}
	if claims.PreviewHash != current.PreviewHash {
		return fmt.Errorf("previewHash mismatch: claims=%s current=%s", claims.PreviewHash, current.PreviewHash)
	}
	return nil
}

func (r *Runtime) ConfirmPackageRollback(ctx context.Context, request PackageRollbackConfirmationRequest) (PackageRollbackConfirmation, error) {
	if r.container == nil || r.container.PackageRepository == nil {
		return PackageRollbackConfirmation{}, fmt.Errorf("kernel: package services unavailable")
	}

	preview, err := r.PreviewPackageRollback(ctx, request.ExtensionID, request.TargetVersion, request.UserID, request.ScopeType, request.ScopeID)
	if err != nil {
		return PackageRollbackConfirmation{}, err
	}

	if !preview.Installable {
		return PackageRollbackConfirmation{}, NewPackageError(PackageErrCodeConfirmationStale, 409, fmt.Errorf("kernel: rollback preview is not installable"))
	}

	if preview.PreviewHash == "" ||
		preview.SecurityPolicyHash == "" ||
		preview.SnapshotRequirementHash == "" ||
		preview.RequiredConfirmationsHash == "" ||
		preview.DependenciesHash == "" {
		return PackageRollbackConfirmation{}, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 409, fmt.Errorf("kernel: rollback preview evidence incomplete"))
	}

	required := normalizeConfirmedItems(preview.RequiredConfirmations)
	requiredSet := make(map[string]struct{}, len(required))
	confirmed := make(map[string]bool, len(required))
	for _, item := range required {
		requiredSet[item] = struct{}{}
		if !request.Confirmations[item] {
			return PackageRollbackConfirmation{}, NewPackageError(PackageErrCodeConfirmationItemsMissing, 403, fmt.Errorf("%w: %s", ErrPackageConfirmationItemsMissing, item))
		}
		confirmed[item] = true
	}
	for item, value := range request.Confirmations {
		if !value {
			continue
		}
		if _, expected := requiredSet[item]; !expected {
			return PackageRollbackConfirmation{}, NewPackageError(PackageErrCodeConfirmationItemsMismatch, 403, fmt.Errorf("%w: unexpected confirmation %s", ErrPackageConfirmationItemsMismatch, item))
		}
	}

	requiredHash := computePackageRequiredConfirmationsHash(required)
	if requiredHash != preview.RequiredConfirmationsHash {
		return PackageRollbackConfirmation{}, NewPackageError(PackageErrCodeConfirmationStale, 409, fmt.Errorf("kernel: rollback required confirmations changed"))
	}

	previewSessionID := strings.TrimSpace(request.PreviewSessionID)
	if previewSessionID == "" {
		previewSessionID = preview.PreviewSessionID
	}
	if previewSessionID == "" {
		return PackageRollbackConfirmation{}, NewPackageError(PackageErrCodeConfirmationClaimsInvalid, 400, fmt.Errorf("kernel: rollback preview session id missing"))
	}

	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(10 * time.Minute)

	claims := PackageRollbackConfirmationClaims{
		SchemaVersion:             PackageRollbackConfirmationClaimsSchemaVersion,
		OperationType:             string(PackageOperationTypeRollback),
		PolicyVersion:             packagePolicyVersion,
		ExtensionID:               preview.ExtensionID,
		ArtifactID:                preview.TargetArtifactID,
		SourceVersionID:           preview.CurrentVersion,
		SourceGenerationID:        preview.SourceGenerationID,
		TargetVersionID:           preview.TargetVersion,
		TargetGenerationID:        preview.TargetGenerationID,
		RollbackPointID:           preview.RollbackPointID,
		PreviewSessionID:          previewSessionID,
		PreviewHash:               preview.PreviewHash,
		SecurityPolicyHash:        preview.SecurityPolicyHash,
		SnapshotRequirementHash:   preview.SnapshotRequirementHash,
		RequiredConfirmationsHash: preview.RequiredConfirmationsHash,
		DependenciesHash:          preview.DependenciesHash,
		UserID:                    request.UserID,
		ScopeType:                 request.ScopeType,
		ScopeID:                   request.ScopeID,
		ConfirmedItems:            confirmedItemsFromMap(confirmed),
		Confirmations:             confirmed,
		IssuedAt:                  issuedAt.Unix(),
		ExpiresAt:                 expiresAt.Unix(),
		Nonce:                     uuid.NewString(),
	}

	token, err := signPackageRollbackConfirmation(claims)
	if err != nil {
		return PackageRollbackConfirmation{}, err
	}

	return PackageRollbackConfirmation{
		ConfirmationToken: token,
		ExpiresAt:         expiresAt,
		PreviewSessionID:  previewSessionID,
	}, nil
}

type PackageRollbackConfirmationRequest struct {
	ExtensionID      string          `json:"extensionId"`
	TargetVersion    string          `json:"targetVersion"`
	UserID           string          `json:"-"`
	ScopeType        string          `json:"scopeType"`
	ScopeID          string          `json:"scopeId"`
	PreviewSessionID string          `json:"previewSessionId,omitempty"`
	Confirmations    map[string]bool `json:"confirmations"`
}
