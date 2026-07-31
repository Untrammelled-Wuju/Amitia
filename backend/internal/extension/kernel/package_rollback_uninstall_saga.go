package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

type PackageUninstallPreviewResult struct {
	ExtensionID    string   `json:"extensionId"`
	CurrentVersion string   `json:"currentVersion"`
	Generation     int64    `json:"generation"`
	Enabled        bool     `json:"enabled"`
	Dependents     []string `json:"dependents"`
	InstalledPath  string   `json:"installedPath"`
	InstalledHash  string   `json:"installedTreeHash"`
	ArtifactID     string   `json:"artifactId"`
	Installable    bool     `json:"uninstallable"`
	GenerationID   string   `json:"generationId"`
	OperationID    string   `json:"operationId"`
}

func (r *Runtime) ExecutePackageRollback(ctx context.Context, extensionID, version, userID, scopeType, scopeID string) (KernelInstallResult, error) {
	point, err := r.container.PackageRepository.GetRollbackPoint(ctx, extensionID, version)
	if err != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback point unavailable: %w", err)
	}
	if err := validatePackageSnapshot(point); err != nil {
		return KernelInstallResult{}, err
	}
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, point.ArtifactID)
	if err != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback artifact unavailable: %w", err)
	}
	pkg, err := r.VerifyStoredPackage(ctx, artifact)
	if err != nil {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback artifact verification failed: %w", err)
	}
	dependencyPreview := InstallPreview{}
	r.evaluatePackageCompatibilityAndDependencies(ctx, pkg, &dependencyPreview)
	if len(dependencyPreview.Issues) > 0 {
		return KernelInstallResult{}, fmt.Errorf("kernel: rollback dependency or compatibility check failed")
	}
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
	idempotencyKey := computeSimplePackageIdempotencyKey("rollback", extensionID, version, userID, scopeType, scopeID)
	operationID := "package-operation-" + uuid.NewString()
	traceID := "package-trace-" + uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rollbackOp := PackageOperationRecord{OperationID: operationID, TraceID: traceID,
		UserID: userID, ScopeType: scopeType, ScopeID: scopeID, ExtensionID: extensionID,
		TargetVersion: version, OperationType: "rollback", Status: "created", CurrentStep: "created",
		ArtifactID: artifact.ArtifactID, ConfirmationsJSON: "{}",
		IdempotencyKey: idempotencyKey, RequestHash: computePackageRequestHash(PackageOperationRecord{
			OperationType: "rollback", ExtensionID: extensionID, TargetVersion: version,
			ArtifactID: artifact.ArtifactID, ScopeType: scopeType, ScopeID: scopeID,
		}), StartedAt: now, UpdatedAt: now}
	existing, created, createErr := r.container.PackageRepository.CreateOrGetOperation(ctx, rollbackOp)
	if createErr != nil {
		return KernelInstallResult{}, createErr
	}
	if !created {
		if result, handled, handleErr := r.handleExistingPackageOperation(ctx, existing); handled {
			return result, handleErr
		}
	}
	lease, leaseErr := r.acquirePackageExtensionLease(ctx, extensionID, operationID)
	if leaseErr != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "acquire_lease", "PACKAGE_OPERATION_LEASE_CONFLICT", leaseErr.Error(), true, PackageWriteGuard{})
		return KernelInstallResult{}, fmt.Errorf("kernel: extension %s has an active operation: %w", extensionID, leaseErr)
	}
	leaseGuard := r.newPackageLeaseGuard(extensionID, operationID)
	sagaCtx, startErr := leaseGuard.Start(ctx)
	if startErr != nil {
		_ = r.releasePackageExtensionLease(context.Background(), extensionID, operationID)
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "start_lease_guard", "PACKAGE_OPERATION_LEASE_CONFLICT", startErr.Error(), true, PackageWriteGuard{})
		return KernelInstallResult{}, fmt.Errorf("kernel: lease guard start failed: %w", startErr)
	}
	defer func() { _ = leaseGuard.Stop(context.Background()) }()
	ctx = sagaCtx
	guard := packageWriteGuard(lease)
	op := PackageOperationRecord{OperationID: operationID, TraceID: traceID}
	forwardPoint, err := r.createPackageRollbackPoint(ctx, op.OperationID, "forward_recovery", current, &currentDefinition, currentModules, currentContributions)
	if err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "create_forward_recovery_point", "PACKAGE_FORWARD_RECOVERY_POINT_FAILED", err.Error(), true, guard)
		return KernelInstallResult{}, errors.Join(err, persistErr)
	}
	if manualReason := packageSnapshotManualRecoveryReason(forwardPoint, point); manualReason != "" {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "requires_manual_recovery", "PACKAGE_MANUAL_RECOVERY_REQUIRED", manualReason, false, guard)
		return KernelInstallResult{}, errors.Join(fmt.Errorf("kernel: requires_manual_recovery: %s", manualReason), persistErr)
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
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "renew_lease", err, guard)
	}
	if err := r.completeSimplePackageStep(ctx, op.OperationID, "validate_rollback_point", 1, guard); err != nil {
		return KernelInstallResult{}, err
	}
	sourcePath := point.InstalledPath
	var rollbackStagingID string
	if info, statErr := os.Stat(sourcePath); statErr != nil || !info.IsDir() {
		staging, extractErr := r.container.PackageSecurity.ExtractFileToStaging(ctx, artifact.ArchivePath, op.OperationID)
		if extractErr != nil {
			return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "rebuild_rollback_generation", extractErr, guard)
		}
		sourcePath = staging.Path
		rollbackStagingID = staging.ID
		defer r.container.PackageSecurity.GetStagingManager().Cleanup(context.Background(), rollbackStagingID)
	}
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "renew_lease", err, guard)
	}
	targetGeneration, stableGeneration, err := r.preparePackageGeneration(ctx, op.OperationID, artifact, sourcePath, guard.FencingToken)
	if err != nil {
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "commit_rollback_generation", err, guard)
	}
	stableFromDB := packageGenerationFromInstallation(current)
	if stableFromDB.GenerationID != "" && stableFromDB.GenerationID != stableGeneration.GenerationID {
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "validate_current_pointer", ErrPackageGenerationCAS, guard)
	}
	if err := r.container.PackageRepository.SetOperationGenerationEvidence(ctx, op.OperationID, stableGeneration.GenerationID, targetGeneration.Current.GenerationID, packageGenerationJSON(targetGeneration.Current), guard); err != nil {
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "persist_generation_evidence", err, guard)
	}
	if err := r.completePackageGenerationStep(ctx, op.OperationID, "commit_rollback_generation", 2, stableGeneration, targetGeneration.Current, packageJSON(map[string]string{"path": targetGeneration.GenerationPath, "treeHash": targetGeneration.Current.TreeHash}), guard); err != nil {
		_ = r.compensatePackageGeneration(context.Background(), stableGeneration, targetGeneration, false)
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "commit_rollback_generation", err, guard)
	}
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "renew_lease", err, guard)
	}
	if err := r.switchPackageGeneration(ctx, stableGeneration, targetGeneration); err != nil {
		_ = r.compensatePackageGeneration(context.Background(), stableGeneration, targetGeneration, false)
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "switch_current_pointer", err, guard)
	}
	if err := r.completePackageGenerationStep(ctx, op.OperationID, "switch_current_pointer", 3, stableGeneration, targetGeneration.Current, packageGenerationJSON(targetGeneration.Current), guard); err != nil {
		_ = r.compensatePackageGeneration(context.Background(), stableGeneration, targetGeneration, true)
		return KernelInstallResult{}, r.failPackageRollbackWithForwardRecovery(op.OperationID, forwardPoint, "switch_current_pointer", err, guard)
	}
	compensateRollback := func(step string, cause error) error {
		generationErr := r.compensatePackageGeneration(context.Background(), stableGeneration, targetGeneration, true)
		forwardErr := r.restoreForwardPackagePoint(context.Background(), forwardPoint)
		if generationErr == nil && forwardErr == nil {
			forwardErr = r.rebindPackageInstallationGeneration(context.Background(), stableGeneration, forwardPoint.InstalledPath)
		}
		if generationErr != nil || forwardErr != nil {
			detail := errors.Join(cause, generationErr, forwardErr)
			persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "forward_recovery_failed", "PACKAGE_RECOVERY_REQUIRED", detail.Error(), false, guard)
			return errors.Join(detail, persistErr)
		}
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", step, "PACKAGE_ROLLBACK_FAILED", cause.Error(), true, guard)
		return errors.Join(cause, persistErr)
	}
	replacedVersion := current.InstalledVersion.String()
	current.InstalledVersion = definition.Version
	current.PackageID = artifact.ArtifactID
	current.Generation++
	current.EnablementState = domain.EnablementDisabled
	current.UpdatedAt = time.Now().UTC()
	current.Metadata = packageInstallationMetadata(map[string]any{"installedPath": targetGeneration.GenerationPath, "artifactId": artifact.ArtifactID,
		"archiveHash": artifact.ArchiveHash, "manifestHash": artifact.ManifestHash,
		"contentTreeHash": artifact.ContentTreeHash, "artifactHash": artifact.ArtifactHash,
		"installedTreeHash": targetGeneration.Current.TreeHash,
		"ownerUserId":       userID, "scopeType": scopeType, "scopeId": scopeID}, targetGeneration.Current, targetGeneration.GenerationPath, op.OperationID)
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		return KernelInstallResult{}, compensateRollback("renew_lease", err)
	}
	err = r.container.TransactionManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := r.container.PackageRepository.VerifyFencingTokenInContext(txCtx, guard); err != nil {
			return err
		}
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
		if err := r.restorePackageRepositorySnapshots(txCtx, current.ExtensionID, point, &current); err != nil {
			return err
		}
		current.Metadata = packageInstallationMetadata(current.Metadata, targetGeneration.Current, targetGeneration.GenerationPath, op.OperationID)
		return r.container.InstallationRepository.PutInstallation(txCtx, current)
	})
	if err != nil {
		return KernelInstallResult{}, compensateRollback("restore_repositories", err)
	}
	if err := r.restorePackageMigrationState(ctx, point); err != nil {
		return KernelInstallResult{}, compensateRollback("restore_migration_state", err)
	}
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		return KernelInstallResult{}, compensateRollback("renew_lease", err)
	}
	if err := r.container.PackageRepository.SetArtifactInstalledPath(ctx, artifact.ArtifactID, targetGeneration.GenerationPath, guard); err != nil {
		return KernelInstallResult{}, compensateRollback("persist_artifact_metadata", err)
	}
	if err := r.completePackageGenerationStep(ctx, op.OperationID, "restore_repositories", 4, stableGeneration, targetGeneration.Current, packageGenerationJSON(targetGeneration.Current), guard); err != nil {
		return KernelInstallResult{}, compensateRollback("record_restore_completion", err)
	}
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		return KernelInstallResult{}, compensateRollback("renew_lease", err)
	}
	if err := r.recordPackageVersionAfterOperation(ctx, op.OperationID, "rollback", extensionID, version, artifact.ArtifactID, targetGeneration.GenerationPath, targetGeneration.Current.TreeHash, artifact.ArchiveHash, artifact.ManifestHash, artifact.ContentTreeHash, targetGeneration.Current.GenerationID); err != nil {
		return KernelInstallResult{}, compensateRollback("record_version", err)
	}
	if replacedVersion != "" && replacedVersion != version {
		if err := r.container.PackageRepository.MarkPackageVersionRollbackAvailable(ctx, extensionID, replacedVersion); err != nil {
			return KernelInstallResult{}, compensateRollback("mark_rollback_available", err)
		}
	}
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		return KernelInstallResult{}, compensateRollback("renew_lease", err)
	}
	if err := r.runPackageFinalGate(ctx, op.OperationID); err != nil {
		return KernelInstallResult{}, compensateRollback("final_gate", err)
	}
	if stopErr := leaseGuard.Stop(context.Background()); stopErr != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "lease_release", "PACKAGE_LEASE_RELEASE_FAILED", stopErr.Error(), false, guard)
		return KernelInstallResult{}, stopErr
	}
	if err := r.container.PackageRepository.SetOperation(ctx, op.OperationID, "completed", "completed", "", "", true, PackageWriteGuard{}); err != nil {
		return KernelInstallResult{}, compensateRollback("complete_operation", err)
	}
	return KernelInstallResult{OperationID: op.OperationID, TraceID: op.TraceID, Operation: "rollback",
		ExtensionID: extensionID, Version: version, InstallationID: current.InstallationID,
		PackageHash: artifact.ArchiveHash, ContentTreeHash: artifact.ContentTreeHash,
		ArtifactPath: artifact.ArchivePath, InstallPath: targetGeneration.GenerationPath,
		DefinitionHash: artifact.ArtifactHash, InstalledAt: time.Now().UTC()}, nil
}

func (r *Runtime) failPackageRollbackWithForwardRecovery(operationID string, forwardPoint PackageRollbackPoint, step string, cause error, guard PackageWriteGuard) error {
	compensationErr := r.restoreForwardPackagePoint(context.Background(), forwardPoint)
	if compensationErr != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), operationID, "requires_recovery", "forward_recovery_failed", "PACKAGE_RECOVERY_REQUIRED", errors.Join(cause, compensationErr).Error(), false, guard)
		return errors.Join(cause, compensationErr, persistErr)
	}
	persistErr := r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", step, "PACKAGE_ROLLBACK_FAILED", cause.Error(), true, guard)
	return errors.Join(cause, persistErr)
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
		Generation:     installation.Generation,
		Enabled:        installation.EnablementState == domain.EnablementEnabled, Installable: true}
	result.InstalledPath, _ = installation.Metadata["installedPath"].(string)
	result.InstalledHash, _ = installation.Metadata["installedTreeHash"].(string)
	result.ArtifactID, _ = installation.Metadata["artifactId"].(string)
	result.GenerationID, _ = installation.Metadata["generationId"].(string)
	result.OperationID, _ = installation.Metadata["lastOperationId"].(string)
	if result.InstalledPath == "" || result.InstalledHash == "" {
		return result, fmt.Errorf("kernel: uninstall installation evidence incomplete")
	}
	if result.GenerationID != "" && r.container.PackageGenerationStore != nil {
		current, currentErr := r.container.PackageGenerationStore.ReadCurrent(extensionID)
		if currentErr != nil || current.GenerationID != result.GenerationID || current.ArtifactID != result.ArtifactID || current.TreeHash != result.InstalledHash {
			return result, fmt.Errorf("kernel: uninstall current pointer mismatch")
		}
		if verifyErr := r.container.PackageGenerationStore.VerifyGeneration(ctx, current); verifyErr != nil {
			return result, fmt.Errorf("kernel: uninstall generation verification failed: %w", verifyErr)
		}
	} else {
		actualHash := package_security.ComputeDirHash(result.InstalledPath, r.container.PackageSecurity.GetHasher())
		if actualHash == "" || actualHash != result.InstalledHash {
			return result, fmt.Errorf("kernel: uninstall installed tree verification failed")
		}
	}
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
	initialPreview, err := r.PreviewPackageUninstall(ctx, extensionID, userID, scopeType, scopeID)
	if err != nil {
		return PackageOperationRecord{}, err
	}
	if !initialPreview.Installable {
		return PackageOperationRecord{}, fmt.Errorf("kernel: uninstall preflight failed")
	}
	releaseInProcessLock := r.acquirePackageInProcessLock(extensionID)
	defer releaseInProcessLock()
	idempotencyKey := computeSimplePackageIdempotencyKey("uninstall", extensionID, initialPreview.CurrentVersion, userID, scopeType, scopeID)
	operationID := "package-operation-" + uuid.NewString()
	traceID := "package-trace-" + uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	uninstallOp := PackageOperationRecord{OperationID: operationID, TraceID: traceID,
		UserID: userID, ScopeType: scopeType, ScopeID: scopeID, ExtensionID: extensionID,
		TargetVersion: initialPreview.CurrentVersion, OperationType: "uninstall", Status: "created", CurrentStep: "created",
		ArtifactID: initialPreview.ArtifactID, ConfirmationsJSON: "{}",
		IdempotencyKey: idempotencyKey, RequestHash: computePackageRequestHash(PackageOperationRecord{
			OperationType: "uninstall", ExtensionID: extensionID, TargetVersion: initialPreview.CurrentVersion,
			ArtifactID: initialPreview.ArtifactID, ScopeType: scopeType, ScopeID: scopeID,
		}), StartedAt: now, UpdatedAt: now}
	existing, created, createErr := r.container.PackageRepository.CreateOrGetOperation(ctx, uninstallOp)
	if createErr != nil {
		return PackageOperationRecord{}, createErr
	}
	if !created {
		switch PackageOperationStatus(existing.Status) {
		case PackageOperationCompleted:
			return existing, nil
		case PackageOperationFailed:
			return existing, fmt.Errorf("kernel: idempotent uninstall previously failed: %s", existing.ErrorDetail)
		case PackageOperationRequiresRecovery:
			return existing, fmt.Errorf("kernel: idempotent uninstall requires recovery: %s", existing.ErrorDetail)
		default:
			return existing, fmt.Errorf("kernel: uninstall operation already in progress: %s (status=%s)", existing.OperationID, existing.Status)
		}
	}
	lease, leaseErr := r.acquirePackageExtensionLease(ctx, extensionID, operationID)
	if leaseErr != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "acquire_lease", "PACKAGE_OPERATION_LEASE_CONFLICT", leaseErr.Error(), true, PackageWriteGuard{})
		return PackageOperationRecord{}, fmt.Errorf("kernel: extension %s has an active operation: %w", extensionID, leaseErr)
	}
	leaseGuard := r.newPackageLeaseGuard(extensionID, operationID)
	sagaCtx, startErr := leaseGuard.Start(ctx)
	if startErr != nil {
		_ = r.releasePackageExtensionLease(context.Background(), extensionID, operationID)
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "start_lease_guard", "PACKAGE_OPERATION_LEASE_CONFLICT", startErr.Error(), true, PackageWriteGuard{})
		return PackageOperationRecord{}, fmt.Errorf("kernel: lease guard start failed: %w", startErr)
	}
	defer func() { _ = leaseGuard.Stop(context.Background()) }()
	ctx = sagaCtx
	uninstallGuard := packageWriteGuard(lease)
	preview, err := r.PreviewPackageUninstall(ctx, extensionID, userID, scopeType, scopeID)
	if err != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "recheck_preflight", "PACKAGE_UNINSTALL_PREFLIGHT_FAILED", err.Error(), true, uninstallGuard)
		return PackageOperationRecord{}, err
	}
	if !preview.Installable || !samePackageUninstallPreview(initialPreview, preview) {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, "failed", "recheck_preflight", "PACKAGE_UNINSTALL_PREFLIGHT_CHANGED", "uninstall preflight changed after acquiring lease", true, uninstallGuard)
		return PackageOperationRecord{}, fmt.Errorf("kernel: uninstall preflight changed")
	}
	op := PackageOperationRecord{OperationID: operationID, TraceID: traceID}
	if err := r.completeSimplePackageStep(ctx, op.OperationID, "validate_uninstall_preflight", 1, uninstallGuard); err != nil {
		return op, err
	}
	installation, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return op, err
	}
	currentPointer, err := r.container.PackageGenerationStore.ReadCurrent(extensionID)
	if err != nil || currentPointer.GenerationID != preview.GenerationID || currentPointer.TreeHash != preview.InstalledHash {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "bind_current_pointer", "PACKAGE_RECOVERY_REQUIRED", fmt.Sprint(err), false, uninstallGuard)
		return op, errors.Join(fmt.Errorf("kernel: uninstall current pointer binding failed"), err, persistErr)
	}
	if err := r.container.PackageRepository.SetOperationGenerationEvidence(ctx, op.OperationID, currentPointer.GenerationID, "", packageGenerationJSON(currentPointer), uninstallGuard); err != nil {
		return op, err
	}
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "renew_lease", "PACKAGE_LEASE_LOST", err.Error(), false, uninstallGuard)
		return op, err
	}
	quarantinedCurrent, err := r.container.PackageGenerationStore.QuarantineCurrent(extensionID, currentPointer.GenerationID, op.OperationID)
	if err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "quarantine_current_pointer", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false, uninstallGuard)
		return op, errors.Join(err, persistErr)
	}
	quarantinePath := ""
	quarantinePath, err = r.container.PackageGenerationStore.QuarantineGeneration(ctx, currentPointer)
	if err != nil {
		return op, r.failPackageUninstallAfterGenerationQuarantine(ctx, op, quarantinedCurrent, currentPointer, preview, uninstallGuard, err)
	}
	qm := PackageQuarantineMetadata{
		QuarantineID:             "quarantine-" + op.OperationID,
		OperationID:              op.OperationID,
		ExtensionID:              extensionID,
		GenerationQuarantinePath:  quarantinePath,
		CurrentQuarantinePath:    quarantinedCurrent.Path,
		OriginalGenerationPath:   preview.InstalledPath,
		OriginalCurrentPath:      filepath.Join(r.container.ExtRoot, "installations", safeDirectoryName(extensionID), "current.json"),
		TreeHash:                 preview.InstalledHash,
		ArtifactID:               preview.ArtifactID,
		State:                    "active",
	}
	if err := r.container.PackageRepository.PutQuarantineMetadata(ctx, qm, uninstallGuard); err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "persist_quarantine_metadata", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false, uninstallGuard)
		return op, errors.Join(err, persistErr)
	}
	if err := r.completePackageGenerationStep(ctx, op.OperationID, "move_to_quarantine", 2, currentPointer, PackageGenerationCurrent{}, packageJSON(map[string]string{"originalPath": preview.InstalledPath, "quarantinePath": quarantinePath, "currentQuarantinePath": quarantinedCurrent.Path, "treeHash": preview.InstalledHash}), uninstallGuard); err != nil {
		return op, r.failPackageUninstallAfterGenerationQuarantine(ctx, op, quarantinedCurrent, currentPointer, preview, uninstallGuard, err)
	}
	definitions, err := r.container.DefinitionRepository.ListExtensions(ctx)
	if err != nil {
		return op, r.failPackageUninstallAfterGenerationQuarantine(ctx, op, quarantinedCurrent, currentPointer, preview, uninstallGuard, err)
	}
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		return op, r.failPackageUninstallAfterGenerationQuarantine(ctx, op, quarantinedCurrent, currentPointer, preview, uninstallGuard, err)
	}
	if err := r.container.PackageRepository.SetArtifactInstalledPath(ctx, preview.ArtifactID, "", uninstallGuard); err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "clear_artifact_installation_path", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false, uninstallGuard)
		return op, errors.Join(err, persistErr)
	}
	err = r.container.TransactionManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := r.container.PackageRepository.VerifyFencingTokenInContext(txCtx, uninstallGuard); err != nil {
			return err
		}
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
		return op, r.failPackageUninstallAfterGenerationQuarantine(ctx, op, quarantinedCurrent, currentPointer, preview, uninstallGuard, err)
	}
	if err := r.completeSimplePackageStep(ctx, op.OperationID, "cleanup_kernel_repositories", 3, uninstallGuard); err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "cleanup_kernel_repositories", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false, uninstallGuard)
		return op, errors.Join(err, persistErr)
	}
	if quarantinePath != "" {
		if err := os.RemoveAll(quarantinePath); err != nil {
			persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "remove_files", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false, uninstallGuard)
			return op, errors.Join(err, persistErr)
		}
	}
	if err := os.RemoveAll(quarantinedCurrent.Path); err != nil {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "remove_current_quarantine", "PACKAGE_RECOVERY_REQUIRED", err.Error(), false, uninstallGuard)
		return op, errors.Join(err, persistErr)
	}
	if err := leaseGuard.AssertAlive(ctx); err != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "renew_lease", "PACKAGE_LEASE_LOST", err.Error(), false, uninstallGuard)
		return op, err
	}
	if finalizeQM, qmErr := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, op.OperationID); qmErr == nil {
		finalizeQM.State = "finalizing"
		_ = r.container.PackageRepository.PutQuarantineMetadata(ctx, finalizeQM, uninstallGuard)
	}
	if err := r.completeSimplePackageStep(ctx, op.OperationID, "finalize_quarantine", 4, uninstallGuard); err != nil {
		return op, err
	}
	if finalizeQM, qmErr := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, op.OperationID); qmErr == nil {
		finalizeQM.State = "finalized"
		_ = r.container.PackageRepository.PutQuarantineMetadata(ctx, finalizeQM, uninstallGuard)
	}
	if err := r.deactivatePackageVersionAfterUninstall(ctx, extensionID, initialPreview.CurrentVersion, op.OperationID); err != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "deactivate_version", "PACKAGE_VERSION_HISTORY_CORRUPTED", err.Error(), false, uninstallGuard)
		return op, err
	}
	if err := r.runPackageFinalGate(ctx, op.OperationID); err != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "final_gate", "PACKAGE_FINAL_GATE_FAILED", err.Error(), false, uninstallGuard)
		return op, err
	}
	_ = r.container.PackageRepository.ReleaseQuarantineMetadata(ctx, "quarantine-"+op.OperationID, uninstallGuard)
	if stopErr := leaseGuard.Stop(context.Background()); stopErr != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "lease_release", "PACKAGE_LEASE_RELEASE_FAILED", stopErr.Error(), false, uninstallGuard)
		return op, stopErr
	}
	if err := r.container.PackageRepository.SetOperation(ctx, op.OperationID, "completed", "completed", "", "", true, PackageWriteGuard{}); err != nil {
		return op, err
	}
	op.Status = "completed"
	op.CurrentStep = "completed"
	return op, nil
}

func samePackageUninstallPreview(left, right PackageUninstallPreviewResult) bool {
	return left.ExtensionID == right.ExtensionID && left.CurrentVersion == right.CurrentVersion &&
		left.Generation == right.Generation && left.Enabled == right.Enabled &&
		left.InstalledPath == right.InstalledPath && left.InstalledHash == right.InstalledHash &&
		left.ArtifactID == right.ArtifactID && left.GenerationID == right.GenerationID && left.OperationID == right.OperationID && strings.Join(left.Dependents, "\x00") == strings.Join(right.Dependents, "\x00")
}

func (r *Runtime) failPackageUninstallAfterGenerationQuarantine(ctx context.Context, op PackageOperationRecord, quarantinedCurrent PackageQuarantinedCurrent, current PackageGenerationCurrent, preview PackageUninstallPreviewResult, guard PackageWriteGuard, cause error) error {
	if quarantinedCurrent.Path == "" {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "remove_repositories", "PACKAGE_UNINSTALL_FAILED", cause.Error(), true, guard)
		return errors.Join(cause, persistErr)
	}
	if existingQM, qmErr := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, op.OperationID); qmErr == nil {
		existingQM.State = "restoring"
		_ = r.container.PackageRepository.PutQuarantineMetadata(ctx, existingQM, guard)
	}
	restoreErr := r.container.PackageGenerationStore.RestoreQuarantinedGeneration(ctx, current)
	if restoreErr == nil {
		restoreErr = r.container.PackageGenerationStore.RestoreQuarantinedCurrent(quarantinedCurrent)
	}
	if restoreErr == nil {
		restoreErr = r.container.PackageGenerationStore.VerifyGeneration(ctx, current)
	}
	if restoreErr != nil {
		detail := errors.Join(cause, fmt.Errorf("restore quarantined installation: %w", restoreErr))
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "restore_quarantine", "PACKAGE_RECOVERY_REQUIRED", detail.Error(), false, guard)
		return errors.Join(detail, persistErr)
	}
	if pathErr := r.container.PackageRepository.SetArtifactInstalledPath(ctx, preview.ArtifactID, preview.InstalledPath, guard); pathErr != nil {
		detail := errors.Join(cause, fmt.Errorf("restore artifact installed path: %w", pathErr))
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "restore_artifact_path", "PACKAGE_RECOVERY_REQUIRED", detail.Error(), false, guard)
		return errors.Join(detail, persistErr)
	}
	if _, refErr := r.container.PackageRepository.AcquireArtifactReference(ctx, preview.ArtifactID, ArtifactReferenceInstallation, op.ExtensionID, time.Time{}); refErr != nil {
		detail := errors.Join(cause, fmt.Errorf("restore artifact installation reference: %w", refErr))
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "restore_artifact_reference", "PACKAGE_RECOVERY_REQUIRED", detail.Error(), false, guard)
		return errors.Join(detail, persistErr)
	}
	if restoredQM, qmErr := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, op.OperationID); qmErr == nil {
		restoredQM.State = "restored"
		_ = r.container.PackageRepository.PutQuarantineMetadata(ctx, restoredQM, guard)
	}
	persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "remove_repositories", "PACKAGE_UNINSTALL_FAILED", cause.Error(), true, guard)
	return errors.Join(cause, persistErr)
}

func (r *Runtime) failPackageUninstallAfterQuarantine(ctx context.Context, op PackageOperationRecord, quarantinePath string, preview PackageUninstallPreviewResult, guard PackageWriteGuard, cause error) error {
	if quarantinePath == "" {
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "remove_repositories", "PACKAGE_UNINSTALL_FAILED", cause.Error(), true, guard)
		return errors.Join(cause, persistErr)
	}
	if existingQM, qmErr := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, op.OperationID); qmErr == nil {
		existingQM.State = "restoring"
		_ = r.container.PackageRepository.PutQuarantineMetadata(ctx, existingQM, guard)
	}
	restoreErr := os.Rename(quarantinePath, preview.InstalledPath)
	if restoreErr == nil {
		actualHash := package_security.ComputeDirHash(preview.InstalledPath, r.container.PackageSecurity.GetHasher())
		if actualHash == "" || actualHash != preview.InstalledHash {
			restoreErr = errors.New("kernel: restored installation tree hash mismatch")
		}
	}
	if restoreErr != nil {
		detail := errors.Join(cause, fmt.Errorf("restore quarantined installation: %w", restoreErr))
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "restore_quarantine", "PACKAGE_RECOVERY_REQUIRED", detail.Error(), false, guard)
		return errors.Join(detail, persistErr)
	}
	if pathErr := r.container.PackageRepository.SetArtifactInstalledPath(ctx, preview.ArtifactID, preview.InstalledPath, guard); pathErr != nil {
		detail := errors.Join(cause, fmt.Errorf("restore artifact installed path: %w", pathErr))
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "restore_artifact_path", "PACKAGE_RECOVERY_REQUIRED", detail.Error(), false, guard)
		return errors.Join(detail, persistErr)
	}
	if _, refErr := r.container.PackageRepository.AcquireArtifactReference(ctx, preview.ArtifactID, ArtifactReferenceInstallation, op.ExtensionID, time.Time{}); refErr != nil {
		detail := errors.Join(cause, fmt.Errorf("restore artifact installation reference: %w", refErr))
		persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "requires_recovery", "restore_artifact_reference", "PACKAGE_RECOVERY_REQUIRED", detail.Error(), false, guard)
		return errors.Join(detail, persistErr)
	}
	if restoredQM, qmErr := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, op.OperationID); qmErr == nil {
		restoredQM.State = "restored"
		_ = r.container.PackageRepository.PutQuarantineMetadata(ctx, restoredQM, guard)
	}
	persistErr := r.container.PackageRepository.SetOperation(context.Background(), op.OperationID, "failed", "remove_repositories", "PACKAGE_UNINSTALL_FAILED", cause.Error(), true, guard)
	return errors.Join(cause, persistErr)
}

func (r *Runtime) completePackageStepWithResult(ctx context.Context, operationID, name string, order int, result string, guard PackageWriteGuard) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := r.container.PackageRepository.PutStep(ctx, PackageOperationStep{
		StepID: "package-step-" + uuid.NewString(), OperationID: operationID, StepName: name,
		StepOrder: order, Status: "completed", AttemptCount: 1, ResultJSON: result, StartedAt: now, CompletedAt: now,
	}, guard); err != nil {
		return err
	}
	return r.container.PackageRepository.SetOperation(ctx, operationID, "in_progress", name, "", "", false, guard)
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

func (r *Runtime) completeSimplePackageStep(ctx context.Context, operationID, name string, order int, guard PackageWriteGuard) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := r.container.PackageRepository.PutStep(ctx, PackageOperationStep{
		StepID: "package-step-" + uuid.NewString(), OperationID: operationID, StepName: name,
		StepOrder: order, Status: "completed", AttemptCount: 1, ResultJSON: "{}", StartedAt: now, CompletedAt: now,
	}, guard); err != nil {
		return err
	}
	return r.container.PackageRepository.SetOperation(ctx, operationID, "in_progress", name, "", "", false, guard)
}
