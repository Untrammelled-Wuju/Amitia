package kernel

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type PackageRollbackPreviewResult struct {
	ExtensionID       string   `json:"extensionId"`
	CurrentVersion    string   `json:"currentVersion"`
	TargetVersion     string   `json:"targetVersion"`
	CurrentGeneration int64    `json:"currentGeneration"`
	TargetArtifactID  string   `json:"targetArtifactId"`
	InstalledPath     string   `json:"installedPath"`
	InstalledHash     string   `json:"installedHash"`
	RollbackPointID   string   `json:"rollbackPointId"`
	SnapshotHash      string   `json:"snapshotHash"`
	RetentionState    string   `json:"retentionState"`
	RetentionUntil    string   `json:"retentionUntil"`
	ManualRequired    bool     `json:"manualRequired"`
	ManualReason      string   `json:"manualReason,omitempty"`
	Dependents        []string `json:"dependents"`
	Installable       bool     `json:"installable"`
	GenerationID      string   `json:"generationId"`
	OperationID       string   `json:"operationId"`
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
	if versionRecord, verErr := r.container.PackageRepository.GetPackageVersion(ctx, extensionID, version); verErr == nil {
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

	return result, nil
}
