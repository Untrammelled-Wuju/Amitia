package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type PackageRollbackPreviewResult struct {
	ExtensionID             string   `json:"extensionId"`
	CurrentVersion          string   `json:"currentVersion"`
	TargetVersion           string   `json:"targetVersion"`
	CurrentGeneration       int64    `json:"currentGeneration"`
	TargetArtifactID        string   `json:"targetArtifactId"`
	InstalledPath           string   `json:"installedPath"`
	InstalledHash           string   `json:"installedTreeHash"`
	RollbackPointID         string   `json:"rollbackPointId"`
	SnapshotHash            string   `json:"snapshotHash"`
	RetentionState          string   `json:"retentionState"`
	RetentionUntil          string   `json:"retentionUntil"`
	ManualRequired          bool     `json:"manualRequired"`
	ManualReason            string   `json:"manualReason,omitempty"`
	Dependents              []string `json:"dependents"`
	Installable             bool     `json:"installable"`
	GenerationID            string   `json:"generationId"`
	SourceGenerationID      string   `json:"sourceGenerationId"`
	TargetGenerationID      string   `json:"targetGenerationId"`
	OperationID             string   `json:"operationId"`
	PreviewSessionID        string   `json:"previewSessionId,omitempty"`
	PreviewHash             string   `json:"previewHash,omitempty"`
	SecurityPolicyHash      string   `json:"securityPolicyHash,omitempty"`
	SnapshotRequirementHash string   `json:"snapshotRequirementHash,omitempty"`
	RequiredConfirmationsHash string `json:"requiredConfirmationsHash,omitempty"`
	RequiredConfirmations   []string `json:"requiredConfirmations,omitempty"`
	DependenciesHash        string   `json:"dependenciesHash,omitempty"`
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
	reqInput := RollbackSnapshotRequirementInput{ManifestNoDataChange: true}
	if point.ConfigSnapshotJSON != "" {
		reqInput.ConfigBeforeHash = packageSnapshotDigest([]byte(point.ConfigSnapshotJSON))
	}
	if point.ResourceSnapshotJSON != "" {
		reqInput.ResourceBeforeTreeHash = packageSnapshotDigest([]byte(point.ResourceSnapshotJSON))
	}
	if point.UserDataMigrationStateJSON != "" {
		reqInput.UserDataBeforeHash = packageSnapshotDigest([]byte(point.UserDataMigrationStateJSON))
	}
	if point.MigrationStateSnapshotJSON != "" {
		var migrationState packageMigrationStateSnapshot
		if json.Unmarshal([]byte(point.MigrationStateSnapshotJSON), &migrationState) == nil {
			if migrationState.Mode != "" && migrationState.Mode != "none" {
				reqInput.MigrationDefinitions = migrationState.Definitions
			}
			for i := range migrationState.Operations {
				reqInput.MigrationOperations = append(reqInput.MigrationOperations, migrationState.Operations[i].Operation)
			}
		}
	}
	snapshotReq := ComputeRollbackSnapshotRequirement(reqInput)
	result.SnapshotRequirementHash = snapshotReq.RequirementHash
	reqInputForHash := reqInput
	reqInputForHash.ManifestNoDataChange = true
	snapshotReqForHash := ComputeRollbackSnapshotRequirement(reqInputForHash)
	result.RequiredConfirmations = []string{"confirm.rollback"}
	hasMigrationPlan := len(reqInput.MigrationOperations) > 0
	if hasMigrationPlan {
		result.RequiredConfirmations = append(result.RequiredConfirmations, "confirm.rollback.migration_reverse")
	}
	sort.Strings(result.RequiredConfirmations)
	result.RequiredConfirmationsHash = computePackageRequiredConfirmationsHash(result.RequiredConfirmations)
	result.DependenciesHash = computePackageDependenciesHash(result.Dependents)
	result.PreviewHash = computeRollbackPreviewHash(extensionID, result.CurrentVersion, version, point.RollbackPointID, point.ArtifactID, point.SnapshotHash, snapshotReqForHash.RequirementHash, result.RequiredConfirmationsHash, result.InstalledPath, result.InstalledHash, scopeType, scopeID, result.SourceGenerationID, result.TargetGenerationID)

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
	if claims.PreviewHash != current.PreviewHash {
		return fmt.Errorf("previewHash mismatch: claims=%s current=%s", claims.PreviewHash, current.PreviewHash)
	}
	return nil
}

func (r *Runtime) ConfirmPackageRollback(ctx context.Context, request PackageRollbackConfirmationRequest) (PackageRollbackConfirmation, error) {
	if r.container == nil || r.container.PackageRepository == nil {
		return PackageRollbackConfirmation{}, fmt.Errorf("kernel: package services unavailable")
	}
	point, err := r.container.PackageRepository.GetRollbackPoint(ctx, request.ExtensionID, request.TargetVersion)
	if err != nil {
		return PackageRollbackConfirmation{}, fmt.Errorf("kernel: rollback point unavailable: %w", err)
	}
	if err := validatePackageSnapshot(point); err != nil {
		return PackageRollbackConfirmation{}, err
	}
	current, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(request.ExtensionID))
	if err != nil {
		return PackageRollbackConfirmation{}, err
	}
	if err := validatePackageOwner(current, request.UserID, request.ScopeType, request.ScopeID); err != nil {
		return PackageRollbackConfirmation{}, err
	}
	snapshotReq := computeRollbackSnapshotRequirementFromPoint(point)
	snapshotInput := RollbackSnapshotRequirementInput{ManifestNoDataChange: true}
	if point.MigrationStateSnapshotJSON != "" {
		var migrationState packageMigrationStateSnapshot
		if json.Unmarshal([]byte(point.MigrationStateSnapshotJSON), &migrationState) == nil {
			for i := range migrationState.Operations {
				snapshotInput.MigrationOperations = append(snapshotInput.MigrationOperations, migrationState.Operations[i].Operation)
			}
		}
	}
	required := []string{"confirm.rollback"}
	hasMigrationPlan := len(snapshotInput.MigrationOperations) > 0
	if hasMigrationPlan {
		required = append(required, "confirm.rollback.migration_reverse")
	}
	sort.Strings(required)
	confirmed := make(map[string]bool, len(required))
	for _, confirmation := range required {
		if !request.Confirmations[confirmation] {
			return PackageRollbackConfirmation{}, NewPackageError(PackageErrCodeConfirmationItemsMissing, 403, ErrPackageConfirmationItemsMissing)
		}
		confirmed[confirmation] = true
	}
	installedPath, _ := current.Metadata["installedPath"].(string)
	installedTreeHash, _ := current.Metadata["installedTreeHash"].(string)
	currentGeneration := packageGenerationFromInstallation(current)
	sourceGenerationID := currentGeneration.GenerationID
	targetVersionRecord, tvErr := r.container.PackageRepository.GetPackageVersion(ctx, request.ExtensionID, request.TargetVersion)
	if tvErr != nil || targetVersionRecord.GenerationID == "" {
		return PackageRollbackConfirmation{}, fmt.Errorf("kernel: rollback target version generation unavailable: %w", tvErr)
	}
	targetGenerationID := targetVersionRecord.GenerationID
	previewHash := computeRollbackPreviewHash(request.ExtensionID, current.InstalledVersion.String(), request.TargetVersion, point.RollbackPointID, point.ArtifactID, point.SnapshotHash, snapshotReq.RequirementHash, computePackageRequiredConfirmationsHash(required), installedPath, installedTreeHash, request.ScopeType, request.ScopeID, sourceGenerationID, targetGenerationID)
	tokenExpiry := time.Now().UTC().Add(10 * time.Minute)
	rollbackClaims := PackageRollbackConfirmationClaims{
		SchemaVersion:           PackageRollbackConfirmationClaimsSchemaVersion,
		OperationType:           string(PackageOperationTypeRollback),
		PolicyVersion:           packagePolicyVersion,
		ExtensionID:             request.ExtensionID,
		ArtifactID:              point.ArtifactID,
		SourceVersionID:         current.InstalledVersion.String(),
		SourceGenerationID:      sourceGenerationID,
		TargetVersionID:         request.TargetVersion,
		TargetGenerationID:      targetGenerationID,
		RollbackPointID:         point.RollbackPointID,
		PreviewSessionID:        request.PreviewSessionID,
		PreviewHash:             previewHash,
		SecurityPolicyHash:      computeSecurityPolicyHash(),
		SnapshotRequirementHash: snapshotReq.RequirementHash,
		RequiredConfirmationsHash: computePackageRequiredConfirmationsHash(required),
		UserID:                  request.UserID,
		ScopeType:               request.ScopeType,
		ScopeID:                 request.ScopeID,
		ConfirmedItems:          confirmedItemsFromMap(confirmed),
		Confirmations:           confirmed,
		IssuedAt:                time.Now().UTC().Unix(),
		ExpiresAt:               tokenExpiry.Unix(),
		Nonce:                   uuid.NewString(),
	}
	token, err := signPackageRollbackConfirmation(rollbackClaims)
	if err != nil {
		return PackageRollbackConfirmation{}, err
	}
	return PackageRollbackConfirmation{ConfirmationToken: token, ExpiresAt: tokenExpiry, PreviewSessionID: request.PreviewSessionID}, nil
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
