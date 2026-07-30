package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

type PackageUninstallPreviewResult struct {
	ExtensionID    string   `json:"extensionId"`
	CurrentVersion string   `json:"currentVersion"`
	Enabled        bool     `json:"enabled"`
	Dependents     []string `json:"dependents"`
	InstalledPath  string   `json:"installedPath"`
	ArtifactID     string   `json:"artifactId"`
	Installable    bool     `json:"uninstallable"`
}

func (r *Runtime) ExecutePackageRollback(ctx context.Context, extensionID, version, userID, scopeType, scopeID string) (KernelInstallResult, error) {
	point, err := r.container.PackageRepository.GetRollbackPoint(ctx, extensionID, version)
	if err != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback point unavailable: %w", err)
	}
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, point.ArtifactID)
	if err != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback artifact unavailable: %w", err)
	}
	if err := r.container.PackageArtifactStore.VerifyArchive(artifact); err != nil {
		return KernelInstallResult{}, err
	}
	if info, err := os.Stat(point.InstalledPath); err != nil || !info.IsDir() {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback installed tree unavailable")
	}
	lockValue, _ := r.packageLocks.LoadOrStore(extensionID, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	current, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return KernelInstallResult{}, err
	}
	if err := validatePackageOwner(current, userID, scopeType, scopeID); err != nil {
		return KernelInstallResult{}, err
	}
	currentDefinition, err := r.container.DefinitionRepository.GetExtension(ctx, current.ExtensionID, current.InstalledVersion)
	if err != nil {
		return KernelInstallResult{}, err
	}
	currentModules, err := r.container.ModuleRepository.ListModules(ctx, current.ExtensionID)
	if err != nil {
		return KernelInstallResult{}, err
	}
	currentContributions, err := r.container.ContributionRepository.ListContributions(ctx, current.ExtensionID)
	if err != nil {
		return KernelInstallResult{}, err
	}
	if err := r.createPackageRollbackPoint(ctx, current, &currentDefinition, currentModules, currentContributions); err != nil {
		return KernelInstallResult{}, err
	}
	var definition domain.ExtensionDefinition
	var modules []domain.ModuleDefinition
	var contributions []domain.ContributionDefinition
	var permissionSnapshot struct {
		Requirements []sqlite.PermissionRequirement `json:"requirements"`
		Grants       []sqlite.PermissionGrant       `json:"grants"`
	}
	var scopeSnapshot []sqlite.ScopeBinding
	if json.Unmarshal([]byte(point.DefinitionSnapshotJSON), &definition) != nil || json.Unmarshal([]byte(point.ModuleSnapshotJSON), &modules) != nil || json.Unmarshal([]byte(point.ContributionSnapshotJSON), &contributions) != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback point corrupt")
	}
	if json.Unmarshal([]byte(point.PermissionSnapshotJSON), &permissionSnapshot) != nil || json.Unmarshal([]byte(point.ScopeSnapshotJSON), &scopeSnapshot) != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback policy snapshot corrupt")
	}
	op, err := r.beginSimplePackageOperation(ctx, userID, scopeType, scopeID, extensionID, version, "rollback", artifact.ArtifactID)
	if err != nil {
		return KernelInstallResult{}, err
	}
	_ = r.completeSimplePackageStep(ctx, op.OperationID, "validate_rollback_point", 1)
	current.InstalledVersion = definition.Version
	current.PackageID = artifact.ArtifactID
	current.Generation++
	current.EnablementState = domain.EnablementDisabled
	current.UpdatedAt = time.Now().UTC()
	current.Metadata = map[string]any{"installedPath": point.InstalledPath, "artifactId": artifact.ArtifactID,
		"archiveHash": artifact.ArchiveHash, "manifestHash": artifact.ManifestHash,
		"contentTreeHash": artifact.ContentTreeHash, "artifactHash": artifact.ArtifactHash,
		"ownerUserId": userID, "scopeType": scopeType, "scopeId": scopeID}
	err = r.container.TransactionManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := r.container.PermissionRepository.DeleteRequirements(txCtx, current.ExtensionID); err != nil {
			return err
		}
		currentGrants, err := r.container.PermissionRepository.ListGrants(txCtx, current.ExtensionID)
		if err != nil {
			return err
		}
		for _, grant := range currentGrants {
			if err := r.container.PermissionRepository.DeleteGrant(txCtx, current.ExtensionID, grant.PermissionName); err != nil {
				return err
			}
		}
		if err := r.container.ScopeRepository.DeleteBindings(txCtx, current.ExtensionID); err != nil {
			return err
		}
		if err := r.container.ContributionRepository.DeleteContributions(txCtx, current.ExtensionID); err != nil {
			return err
		}
		if err := r.container.ModuleRepository.DeleteModules(txCtx, current.ExtensionID); err != nil {
			return err
		}
		if err := r.container.DefinitionRepository.PutExtension(txCtx, definition); err != nil {
			return err
		}
		for _, module := range modules {
			if err := r.container.ModuleRepository.PutModule(txCtx, module); err != nil {
				return err
			}
		}
		for _, contribution := range contributions {
			if err := r.container.ContributionRepository.PutContribution(txCtx, contribution); err != nil {
				return err
			}
		}
		for _, requirement := range permissionSnapshot.Requirements {
			if err := r.container.PermissionRepository.PutRequirement(txCtx, requirement); err != nil {
				return err
			}
		}
		for _, grant := range permissionSnapshot.Grants {
			if err := r.container.PermissionRepository.PutGrant(txCtx, grant); err != nil {
				return err
			}
		}
		for _, binding := range scopeSnapshot {
			if err := r.container.ScopeRepository.PutBinding(txCtx, binding); err != nil {
				return err
			}
		}
		return r.container.InstallationRepository.PutInstallation(txCtx, current)
	})
	if err != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "restore_repositories", "PACKAGE_ROLLBACK_FAILED", err.Error(), true)
		return KernelInstallResult{}, err
	}
	_ = r.completeSimplePackageStep(ctx, op.OperationID, "restore_repositories", 2)
	_ = r.container.PackageRepository.SetOperation(ctx, op.OperationID, "completed", "completed", "", "", true)
	return KernelInstallResult{OperationID: op.OperationID, TraceID: op.TraceID, Operation: "rollback",
		ExtensionID: extensionID, Version: version, InstallationID: current.InstallationID,
		PackageHash: artifact.ArchiveHash, ContentTreeHash: artifact.ContentTreeHash,
		ArtifactPath: artifact.ArchivePath, InstallPath: point.InstalledPath,
		DefinitionHash: artifact.ArtifactHash, InstalledAt: time.Now().UTC()}, nil
}

func (r *Runtime) PreviewPackageUninstall(ctx context.Context, extensionID, userID, scopeType, scopeID string) (PackageUninstallPreviewResult, error) {
	installation, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return PackageUninstallPreviewResult{}, err
	}
	if err := validatePackageOwner(installation, userID, scopeType, scopeID); err != nil {
		return PackageUninstallPreviewResult{}, err
	}
	result := PackageUninstallPreviewResult{ExtensionID: extensionID,
		CurrentVersion: installation.InstalledVersion.String(),
		Enabled:        installation.EnablementState == domain.EnablementEnabled, Installable: true}
	result.InstalledPath, _ = installation.Metadata["installedPath"].(string)
	result.ArtifactID, _ = installation.Metadata["artifactId"].(string)
	definitions, err := r.container.DefinitionRepository.ListExtensions(ctx)
	if err != nil {
		return result, err
	}
	for _, definition := range definitions {
		if definition.ID == installation.ExtensionID {
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
	result.Installable = !result.Enabled && len(result.Dependents) == 0
	return result, nil
}

func (r *Runtime) ExecutePackageUninstall(ctx context.Context, extensionID, userID, scopeType, scopeID string) (PackageOperationRecord, error) {
	preview, err := r.PreviewPackageUninstall(ctx, extensionID, userID, scopeType, scopeID)
	if err != nil {
		return PackageOperationRecord{}, err
	}
	if !preview.Installable {
		return PackageOperationRecord{}, fmt.Errorf("kernel: uninstall preflight failed")
	}
	lockValue, _ := r.packageLocks.LoadOrStore(extensionID, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	op, err := r.beginSimplePackageOperation(ctx, userID, scopeType, scopeID, extensionID, preview.CurrentVersion, "uninstall", preview.ArtifactID)
	if err != nil {
		return op, err
	}
	_ = r.completeSimplePackageStep(ctx, op.OperationID, "validate_uninstall_preflight", 1)
	quarantinePath := ""
	if preview.InstalledPath != "" {
		quarantineRoot := filepath.Join(r.root, "quarantine", op.OperationID)
		if err := os.MkdirAll(filepath.Dir(quarantineRoot), 0o700); err != nil {
			return op, err
		}
		if err := os.Rename(preview.InstalledPath, quarantineRoot); err != nil {
			_ = r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "move_to_quarantine", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false)
			return op, err
		}
		quarantinePath = quarantineRoot
	}
	_ = r.completeSimplePackageStep(ctx, op.OperationID, "move_to_quarantine", 2)
	installation, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return op, err
	}
	definitions, err := r.container.DefinitionRepository.ListExtensions(ctx)
	if err != nil {
		return op, err
	}
	err = r.container.TransactionManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := r.container.PermissionRepository.DeleteRequirements(txCtx, installation.ExtensionID); err != nil {
			return err
		}
		grants, err := r.container.PermissionRepository.ListGrants(txCtx, installation.ExtensionID)
		if err != nil {
			return err
		}
		for _, grant := range grants {
			if err := r.container.PermissionRepository.DeleteGrant(txCtx, installation.ExtensionID, grant.PermissionName); err != nil {
				return err
			}
		}
		if err := r.container.ScopeRepository.DeleteBindings(txCtx, installation.ExtensionID); err != nil {
			return err
		}
		resources, err := r.container.ResourceRepository.ListResources(txCtx, installation.ExtensionID)
		if err != nil {
			return err
		}
		for _, resource := range resources {
			if err := r.container.ResourceRepository.DeleteResource(txCtx, resource.ResourceID); err != nil {
				return err
			}
		}
		if err := r.container.ContributionRepository.DeleteContributions(txCtx, installation.ExtensionID); err != nil {
			return err
		}
		if err := r.container.ModuleRepository.DeleteModules(txCtx, installation.ExtensionID); err != nil {
			return err
		}
		if err := r.container.InstallationRepository.DeleteInstallation(txCtx, installation.ExtensionID); err != nil {
			return err
		}
		for _, definition := range definitions {
			if definition.ID == installation.ExtensionID {
				if err := r.container.DefinitionRepository.DeleteExtension(txCtx, definition.ID, definition.Version); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		if quarantinePath != "" {
			_ = os.Rename(quarantinePath, preview.InstalledPath)
		}
		_ = r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "remove_repositories", "PACKAGE_UNINSTALL_FAILED", err.Error(), true)
		return op, err
	}
	_ = r.completeSimplePackageStep(ctx, op.OperationID, "cleanup_kernel_repositories", 3)
	if quarantinePath != "" {
		if err := os.RemoveAll(quarantinePath); err != nil {
			_ = r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "remove_files", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false)
			return op, err
		}
	}
	_ = r.container.PackageRepository.SetOperation(ctx, op.OperationID, "completed", "completed", "", "", true)
	op.Status = "completed"
	op.CurrentStep = "completed"
	return op, nil
}

func validatePackageOwner(installation domain.ExtensionInstallation, userID, scopeType, scopeID string) error {
	owner, _ := installation.Metadata["ownerUserId"].(string)
	storedScopeType, _ := installation.Metadata["scopeType"].(string)
	storedScopeID, _ := installation.Metadata["scopeId"].(string)
	if owner == "" || owner != userID || storedScopeType != scopeType || storedScopeID != scopeID {
		return fmt.Errorf("kernel: package scope mismatch")
	}
	return nil
}

func (r *Runtime) beginSimplePackageOperation(ctx context.Context, userID, scopeType, scopeID, extensionID, version, operationType, artifactID string) (PackageOperationRecord, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	op := PackageOperationRecord{OperationID: "package-operation-" + uuid.NewString(), TraceID: "package-trace-" + uuid.NewString(),
		UserID: userID, ScopeType: scopeType, ScopeID: scopeID, ExtensionID: extensionID,
		TargetVersion: version, OperationType: operationType, Status: "created", CurrentStep: "created",
		ArtifactID: artifactID, ConfirmationsJSON: "{}", StartedAt: now, UpdatedAt: now}
	return op, r.container.PackageRepository.CreateOperation(ctx, op)
}

func (r *Runtime) completeSimplePackageStep(ctx context.Context, operationID, name string, order int) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := r.container.PackageRepository.PutStep(ctx, PackageOperationStep{
		StepID: "package-step-" + uuid.NewString(), OperationID: operationID, StepName: name,
		StepOrder: order, Status: "completed", AttemptCount: 1, ResultJSON: "{}", StartedAt: now, CompletedAt: now,
	}); err != nil {
		return err
	}
	return r.container.PackageRepository.SetOperation(ctx, operationID, "in_progress", name, "", "", false)
}
