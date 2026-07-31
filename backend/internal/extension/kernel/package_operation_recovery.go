package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
)

func (r *Runtime) RecoverPackageOperations(ctx context.Context) error {
	if r.container == nil || r.container.PackageRepository == nil {
		return nil
	}
	operations, err := r.container.PackageRepository.ListIncompleteOperations(ctx)
	if err != nil {
		return err
	}
	var failures []error
	for _, operation := range operations {
		if err := r.recoverPackageOperation(ctx, operation); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", operation.OperationID, err))
		}
	}
	return errors.Join(failures...)
}

func (r *Runtime) finalizePackageOperation(ctx context.Context, operation PackageOperationRecord, guard PackageWriteGuard, completionNote string) error {
	if err := r.runPackageFinalGate(ctx, operation.OperationID, guard); err != nil {
		setErr := r.container.PackageRepository.SetOperation(context.Background(), operation.OperationID, StatusRequiresRecovery, completionNote, "PACKAGE_FINAL_GATE_FAILED", err.Error(), false, guard)
		if setErr != nil {
			setErr = r.container.PackageRepository.SetOperation(context.Background(), operation.OperationID, StatusRequiresRecovery, completionNote, "PACKAGE_FINAL_GATE_FAILED", err.Error(), false, PackageWriteGuard{})
		}
		if setErr != nil {
			return errors.Join(err, fmt.Errorf("persist recovery state after final gate failure: %w", setErr))
		}
		return err
	}
	return r.container.PackageRepository.SetOperation(ctx, operation.OperationID, StatusCompleted, completionNote, "", "", true, guard)
}

func (r *Runtime) recoverPackageOperation(ctx context.Context, operation PackageOperationRecord) error {
	lease, leaseErr := r.acquirePackageExtensionLease(ctx, operation.ExtensionID, operation.OperationID)
	if leaseErr != nil {
		return r.requirePackageRecovery(ctx, operation, "recovery lease acquire failed", leaseErr, PackageWriteGuard{})
	}
	guard := packageWriteGuard(lease)
	leaseGuard := r.newPackageLeaseGuard(operation.ExtensionID, operation.OperationID)
	sagaCtx, startErr := leaseGuard.Start(ctx)
	if startErr != nil {
		releaseErr := r.releasePackageExtensionLease(context.Background(), operation.ExtensionID, operation.OperationID)
		if releaseErr != nil {
			if putErr := r.container.PackageRepository.PutConsistencyFinding(context.Background(), PackageConsistencyFinding{
				FindingID:         "stale-lease-" + operation.OperationID,
				Metric:            "stale_extension_leases",
				Count:             1,
				ResourceIDsJSON:   fmt.Sprintf(`["%s"]`, operation.OperationID),
				ErrorDetail:       releaseErr.Error(),
				RecommendedAction: "manual_lease_cleanup",
			}); putErr != nil {
				fmt.Printf("kernel: failed to persist stale lease finding for %s: %v\n", operation.OperationID, errors.Join(releaseErr, putErr))
			}
		}
		return r.requirePackageRecovery(ctx, operation, "recovery lease guard start failed", startErr, guard)
	}
	defer func() {
		if stopErr := leaseGuard.Stop(context.Background()); stopErr != nil {
			if putErr := r.container.PackageRepository.PutConsistencyFinding(context.Background(), PackageConsistencyFinding{
				FindingID:         "stale-lease-" + operation.OperationID,
				Metric:            "stale_extension_leases",
				Count:             1,
				ResourceIDsJSON:   `["` + operation.OperationID + `"]`,
				ErrorDetail:       stopErr.Error(),
				RecommendedAction: "manual_lease_cleanup",
			}); putErr != nil {
				fmt.Printf("kernel: failed to persist stale lease finding for %s: %v\n", operation.OperationID, errors.Join(stopErr, putErr))
			}
		}
	}()
	ctx = sagaCtx
	_, steps, err := r.container.PackageRepository.GetOperation(ctx, operation.UserID, operation.OperationID)
	if err != nil {
		return r.requirePackageRecovery(ctx, operation, "operation journal unavailable", err, guard)
	}
	completed := completedPackageSteps(steps)
	switch operation.OperationType {
	case "install":
		compensated, reconcileErr := r.reconcileInstalledPackageGeneration(ctx, operation)
		if reconcileErr != nil {
			return r.requirePackageRecovery(ctx, operation, "generation reconciliation failed", reconcileErr, guard)
		}
		if compensated {
			return r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "failed", "recovered_compensated", "PACKAGE_INSTALL_FAILED", "generation switch compensated during recovery", true, guard)
		}
		err = r.proveInstalledPackageOperation(ctx, operation, completed)
		if err != nil {
			return r.requirePackageRecovery(ctx, operation, "installed package consistency could not be proven", err, guard)
		}
		if err := r.newPackageRecoveryFinalizer().FinalizeInstallRecovery(ctx, operation, completed, guard); err != nil {
			return r.requirePackageRecovery(ctx, operation, "install recovery finalizer failed", err, guard)
		}
		if err := r.container.PackageRepository.FinalizeOperationAndReleaseLeaseTx(ctx, operation.OperationID, operation.ExtensionID, guard.FencingToken); err != nil {
			return r.requirePackageRecovery(ctx, operation, "recovery lease release failed", err, guard)
		}
		leaseGuard.MarkLeaseReleased()
		return nil
	case "update":
		compensated, reconcileErr := r.reconcileInstalledPackageGeneration(ctx, operation)
		if reconcileErr != nil {
			return r.requirePackageRecovery(ctx, operation, "update generation reconciliation failed", reconcileErr, guard)
		}
		if compensated {
			return r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "failed", "recovered_compensated", "PACKAGE_UPDATE_FAILED", "update generation switch compensated during recovery", true, guard)
		}
		err = r.proveUpdatedPackageOperation(ctx, operation, completed)
		if err != nil {
			return r.requirePackageRecovery(ctx, operation, "updated package consistency could not be proven", err, guard)
		}
		if err := r.newPackageRecoveryFinalizer().FinalizeUpdateRecovery(ctx, operation, completed, guard); err != nil {
			return r.requirePackageRecovery(ctx, operation, "update recovery finalizer failed", err, guard)
		}
		if err := r.container.PackageRepository.FinalizeOperationAndReleaseLeaseTx(ctx, operation.OperationID, operation.ExtensionID, guard.FencingToken); err != nil {
			return r.requirePackageRecovery(ctx, operation, "recovery lease release failed", err, guard)
		}
		leaseGuard.MarkLeaseReleased()
		return nil
	case "rollback":
		compensated, reconcileErr := r.reconcileInstalledPackageGeneration(ctx, operation)
		if reconcileErr != nil {
			return r.requirePackageRecovery(ctx, operation, "rollback generation reconciliation failed", reconcileErr, guard)
		}
		if compensated {
			if err := r.reconcileRollbackResourceQuarantine(ctx, operation, true); err != nil {
				return r.requirePackageRecovery(ctx, operation, "rollback resource quarantine restore failed", err, guard)
			}
			return r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "failed", "recovered_compensated", "PACKAGE_ROLLBACK_FAILED", "rollback generation switch compensated during recovery", true, guard)
		}
		if err := r.reconcileRollbackResourceQuarantine(ctx, operation, false); err != nil {
			return r.requirePackageRecovery(ctx, operation, "rollback resource quarantine purge failed", err, guard)
		}
		err = r.proveRollbackPackageOperation(ctx, operation, completed)
		if err != nil {
			return r.requirePackageRecovery(ctx, operation, "rollback consistency could not be proven", err, guard)
		}
		if err := r.newPackageRecoveryFinalizer().FinalizeRollbackRecovery(ctx, operation, completed, guard); err != nil {
			return r.requirePackageRecovery(ctx, operation, "rollback recovery finalizer failed", err, guard)
		}
		if err := r.container.PackageRepository.FinalizeOperationAndReleaseLeaseTx(ctx, operation.OperationID, operation.ExtensionID, guard.FencingToken); err != nil {
			return r.requirePackageRecovery(ctx, operation, "recovery lease release failed", err, guard)
		}
		leaseGuard.MarkLeaseReleased()
		return nil
	case "uninstall":
		outcome, reconcileErr := r.reconcileUninstallPackageGeneration(ctx, operation)
		if reconcileErr != nil {
			return r.requirePackageRecovery(ctx, operation, "uninstall generation reconciliation failed", reconcileErr, guard)
		}
		if outcome == "compensated" {
			if err := r.reconcileUninstallCompensatedState(ctx, operation, guard); err != nil {
				return r.requirePackageRecovery(ctx, operation, "uninstall compensated state reconciliation failed", err, guard)
			}
			return r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "failed", "recovered_compensated", "PACKAGE_UNINSTALL_FAILED", "uninstall quarantine restored during recovery", true, guard)
		}
		if err := r.reconcileUninstallCompletedState(ctx, operation, guard); err != nil {
			return r.requirePackageRecovery(ctx, operation, "uninstall completed state reconciliation failed", err, guard)
		}
		err = r.proveUninstalledPackageOperation(ctx, operation, completed)
		if err != nil {
			return r.requirePackageRecovery(ctx, operation, "uninstall consistency could not be proven", err, guard)
		}
		if err := r.newPackageRecoveryFinalizer().FinalizeUninstallRecovery(ctx, operation, completed, guard); err != nil {
			return r.requirePackageRecovery(ctx, operation, "uninstall recovery finalizer failed", err, guard)
		}
		if err := r.container.PackageRepository.FinalizeOperationAndReleaseLeaseTx(ctx, operation.OperationID, operation.ExtensionID, guard.FencingToken); err != nil {
			return r.requirePackageRecovery(ctx, operation, "recovery lease release failed", err, guard)
		}
		leaseGuard.MarkLeaseReleased()
		return nil
	default:
		return r.requirePackageRecovery(ctx, operation, "unsupported package operation type", nil, guard)
	}
}

func (r *Runtime) reconcileUninstallPackageGeneration(ctx context.Context, operation PackageOperationRecord) (string, error) {
	if r.container.PackageGenerationStore == nil {
		return "", errors.New("generation store unavailable")
	}
	var stable PackageGenerationCurrent
	if err := json.Unmarshal([]byte(operation.CurrentPointerJSON), &stable); err != nil || stable.GenerationID == "" {
		return "", errors.New("stable generation evidence unavailable")
	}
	_, dbErr := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	current, currentErr := r.container.PackageGenerationStore.ReadCurrent(operation.ExtensionID)
	if dbErr == nil {
		if currentErr == nil {
			if current.GenerationID != stable.GenerationID {
				return "", fmt.Errorf("unexpected current generation %s", current.GenerationID)
			}
			return "compensated", nil
		}
		if !errors.Is(currentErr, ErrPackageGenerationNotFound) {
			return "", currentErr
		}
		currentQuarantinePath, pathErr := r.resolveQuarantineCurrentPath(ctx, operation)
		if pathErr != nil {
			return "", pathErr
		}
		state := PackageQuarantinedCurrent{Current: stable, Path: currentQuarantinePath}
		if err := r.container.PackageGenerationStore.RestoreQuarantinedGeneration(ctx, stable); err != nil {
			return "", err
		}
		if err := r.container.PackageGenerationStore.RestoreQuarantinedCurrent(state); err != nil {
			return "", err
		}
		return "compensated", r.container.PackageGenerationStore.VerifyGeneration(ctx, stable)
	}
	if !errors.Is(dbErr, domain.ErrInvalidExtensionID) {
		return "", dbErr
	}
	if currentErr == nil {
		return "", errors.New("database removed while current pointer remains")
	}
	if !errors.Is(currentErr, ErrPackageGenerationNotFound) {
		return "", currentErr
	}
	quarantinePath, currentQuarantinePath, pathErr := r.resolveQuarantinePathsForCleanup(ctx, operation, stable)
	if pathErr != nil {
		return "", pathErr
	}
	if quarantinePath != "" {
		if err := os.RemoveAll(quarantinePath); err != nil {
			return "", err
		}
	}
	if currentQuarantinePath != "" {
		if err := os.RemoveAll(currentQuarantinePath); err != nil {
			return "", err
		}
	}
	return "completed", nil
}

func (r *Runtime) reconcileUninstallArtifactPath(ctx context.Context, operation PackageOperationRecord, guard PackageWriteGuard) error {
	if operation.ArtifactID == "" {
		return nil
	}
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("kernel: artifact unavailable for path reconciliation: %w", err)
	}
	if artifact.InstalledPath == "" {
		return nil
	}
	if _, statErr := os.Stat(artifact.InstalledPath); statErr == nil || !os.IsNotExist(statErr) {
		return fmt.Errorf("kernel: installed path still exists during uninstall recovery: %s", artifact.InstalledPath)
	}
	if err := r.container.PackageRepository.SetArtifactInstalledPath(ctx, operation.ArtifactID, "", guard); err != nil {
		return fmt.Errorf("kernel: failed to clear stale artifact installed path during recovery: %w", err)
	}
	return nil
}

func (r *Runtime) reconcileUninstallCompensatedState(ctx context.Context, operation PackageOperationRecord, guard PackageWriteGuard) error {
	qm, qmErr := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, operation.OperationID)
	if qmErr != nil {
		kind := RepositoryErrorKindOf(qmErr)
		switch kind {
		case RepositoryErrorNotFound:
			return NewPackageErrorWithRecovery(
				PackageErrCodeQuarantineMetadataMissing,
				409,
				false,
				true,
				"Run manual recovery inspection",
				qmErr,
			)
		case RepositoryErrorUnavailable:
			return NewPackageErrorWithRecovery(
				PackageErrCodeQuarantineMetadataUnavailable,
				503,
				true,
				true,
				"Retry operation recovery",
				qmErr,
			)
		case RepositoryErrorCorrupt:
			return NewPackageErrorWithRecovery(
				PackageErrCodeQuarantineMetadataIncomplete,
				409,
				false,
				true,
				"Inspect persisted quarantine metadata",
				qmErr,
			)
		default:
			return qmErr
		}
	}
	if err := validateQuarantineMetadataIntegrity(qm, operation, r.container.ExtRoot); err != nil {
		return err
	}
	if operation.ArtifactID != "" {
		artifact, err := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
		if err != nil {
			if !IsRepositoryErrorKind(err, RepositoryErrorNotFound) {
				return fmt.Errorf("kernel: artifact unavailable for compensated recovery: %w", err)
			}
		} else if artifact.InstalledPath == "" && qm.OriginalGenerationPath != "" {
			if err := r.container.PackageRepository.SetArtifactInstalledPath(ctx, operation.ArtifactID, qm.OriginalGenerationPath, guard); err != nil {
				return fmt.Errorf("kernel: restore artifact installed path failed: %w", err)
			}
		} else if artifact.InstalledPath != "" && qm.OriginalGenerationPath != "" && artifact.InstalledPath != qm.OriginalGenerationPath {
			return fmt.Errorf("kernel: artifact installed path conflict: current=%s expected=%s", artifact.InstalledPath, qm.OriginalGenerationPath)
		}
		if _, refErr := r.container.PackageRepository.AcquireArtifactReference(ctx, operation.ArtifactID, ArtifactReferenceInstallation, operation.ExtensionID, time.Time{}); refErr != nil {
			return fmt.Errorf("kernel: restore artifact installation reference failed: %w", refErr)
		}
	}
	if operation.TargetVersion != "" && operation.ExtensionID != "" {
		versionRecord, vErr := r.container.PackageRepository.GetPackageVersion(ctx, operation.ExtensionID, operation.TargetVersion)
		if vErr == nil {
			if versionRecord.VersionState == string(PackageVersionStateRetained) || versionRecord.VersionState == string(PackageVersionStateRemoved) {
				db := r.container.PackageRepository.DB()
				if db == nil {
					return errors.New("kernel: package version database unavailable for compensated recovery")
				}
				tx, txErr := db.BeginTx(ctx, nil)
				if txErr != nil {
					return fmt.Errorf("kernel: begin version state restore transaction: %w", txErr)
				}
				defer tx.Rollback()
				if _, err := tx.ExecContext(ctx, `UPDATE package_versions SET version_state=?, retained_until='', uninstalled_at='', uninstall_operation_id='' WHERE version_id=?`,
					string(PackageVersionStateCurrent), versionRecord.VersionID); err != nil {
					return fmt.Errorf("kernel: restore version state failed: %w", err)
				}
				if _, err := tx.ExecContext(ctx, `UPDATE extension_installations SET current_version_id=?, current_generation_id=? WHERE extension_id=?`,
					versionRecord.VersionID, versionRecord.GenerationID, operation.ExtensionID); err != nil {
					return fmt.Errorf("kernel: restore installation current version failed: %w", err)
				}
				if err := tx.Commit(); err != nil {
					return fmt.Errorf("kernel: commit version state restore: %w", err)
				}
			} else if versionRecord.VersionState == string(PackageVersionStateCurrent) {
				if versionRecord.ArtifactID != operation.ArtifactID {
					return NewPackageErrorWithRecovery(
						PackageErrCodeQuarantineMetadataIncomplete,
						409,
						false,
						true,
						"Version state conflict: current version artifact mismatch",
						fmt.Errorf("kernel: version state conflict: current version artifact %s != operation artifact %s", versionRecord.ArtifactID, operation.ArtifactID),
					)
				}
			}
		}
	}
	qm.State = "restored"
	if err := r.container.PackageRepository.PutQuarantineMetadata(ctx, qm, guard); err != nil {
		return fmt.Errorf("kernel: persist quarantine restored state failed: %w", err)
	}
	return nil
}

func validateQuarantineMetadataIntegrity(qm PackageQuarantineMetadata, operation PackageOperationRecord, extRoot string) error {
	var missing []string
	if qm.QuarantineID == "" {
		missing = append(missing, "QuarantineID")
	}
	if qm.OperationID == "" {
		missing = append(missing, "OperationID")
	}
	if qm.ExtensionID == "" {
		missing = append(missing, "ExtensionID")
	}
	if qm.TreeHash == "" {
		missing = append(missing, "TreeHash")
	}
	if qm.State == "" {
		missing = append(missing, "State")
	}
	if len(missing) > 0 {
		return NewPackageErrorWithRecovery(
			PackageErrCodeQuarantineMetadataIncomplete,
			409,
			false,
			true,
			"Inspect persisted quarantine metadata",
			fmt.Errorf("quarantine metadata missing fields: %s", strings.Join(missing, ", ")),
		)
	}
	if qm.OperationID != operation.OperationID {
		return NewPackageErrorWithRecovery(
			PackageErrCodeQuarantineMetadataIncomplete,
			409,
			false,
			true,
			"Inspect persisted quarantine metadata",
			fmt.Errorf("quarantine metadata operation id mismatch: %s != %s", qm.OperationID, operation.OperationID),
		)
	}
	if qm.ExtensionID != operation.ExtensionID {
		return NewPackageErrorWithRecovery(
			PackageErrCodeQuarantineMetadataIncomplete,
			409,
			false,
			true,
			"Inspect persisted quarantine metadata",
			fmt.Errorf("quarantine metadata extension id mismatch: %s != %s", qm.ExtensionID, operation.ExtensionID),
		)
	}
	validStates := map[string]bool{"active": true, "finalized": true, "restored": true}
	if !validStates[qm.State] {
		return NewPackageErrorWithRecovery(
			PackageErrCodeQuarantineMetadataIncomplete,
			409,
			false,
			true,
			"Inspect persisted quarantine metadata",
			fmt.Errorf("quarantine metadata state invalid for compensation: %s", qm.State),
		)
	}
	if extRoot != "" {
		paths := map[string]string{
			"GenerationQuarantinePath": qm.GenerationQuarantinePath,
			"CurrentQuarantinePath":    qm.CurrentQuarantinePath,
			"OriginalGenerationPath":   qm.OriginalGenerationPath,
			"OriginalCurrentPath":      qm.OriginalCurrentPath,
		}
		absExtRoot, _ := filepath.Abs(extRoot)
		for name, p := range paths {
			if p == "" {
				continue
			}
			absPath, _ := filepath.Abs(p)
			if !strings.HasPrefix(absPath, absExtRoot+string(filepath.Separator)) && absPath != absExtRoot {
				return NewPackageErrorWithRecovery(
					PackageErrCodeQuarantineMetadataIncomplete,
					409,
					false,
					true,
					"Inspect persisted quarantine metadata",
					fmt.Errorf("quarantine metadata %s escapes package generation root: %s", name, p),
				)
			}
		}
	}
	return nil
}

func (r *Runtime) reconcileUninstallCompletedState(ctx context.Context, operation PackageOperationRecord, guard PackageWriteGuard) error {
	if err := r.reconcileUninstallArtifactPath(ctx, operation, guard); err != nil {
		return err
	}
	if operation.ArtifactID != "" && operation.ExtensionID != "" {
		if err := r.container.PackageRepository.ReleaseArtifactReference(ctx, operation.ArtifactID, ArtifactReferenceInstallation, operation.ExtensionID); err != nil {
			return fmt.Errorf("kernel: release artifact installation reference failed: %w", err)
		}
	}
	if operation.TargetVersion != "" && operation.ExtensionID != "" {
		if err := r.container.PackageRepository.RemovePackageVersion(ctx, operation.ExtensionID, operation.TargetVersion); err != nil {
			if !strings.Contains(err.Error(), "not found for remove") {
				return fmt.Errorf("kernel: remove package version state failed: %w", err)
			}
		}
	}
	qm, qmErr := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, operation.OperationID)
	if qmErr != nil {
		kind := RepositoryErrorKindOf(qmErr)
		if kind != RepositoryErrorNotFound {
			return NewPackageErrorWithRecovery(
				PackageErrCodeQuarantineMetadataUnavailable,
				503,
				true,
				true,
				"Retry quarantine metadata release",
				qmErr,
			)
		}
	} else if qm.QuarantineID != "" {
		if err := r.container.PackageRepository.ReleaseQuarantineMetadata(ctx, qm.QuarantineID, guard); err != nil {
			return fmt.Errorf("kernel: release quarantine metadata failed: %w", err)
		}
	} else {
		return NewPackageErrorWithRecovery(
			PackageErrCodeQuarantineMetadataIncomplete,
			409,
			false,
			true,
			"Inspect persisted quarantine metadata",
			fmt.Errorf("quarantine metadata missing quarantine id"),
		)
	}
	return nil
}

func (r *Runtime) reconcileRollbackResourceQuarantine(ctx context.Context, operation PackageOperationRecord, isCompensated bool) error {
	if r.container.ResourceSnapshotStore == nil {
		return nil
	}
	if isCompensated {
		return r.container.ResourceSnapshotStore.RestoreQuarantinedResources(ctx, operation.OperationID)
	}
	return r.container.ResourceSnapshotStore.PurgeQuarantinedResources(ctx, operation.OperationID)
}

func (r *Runtime) resolveQuarantineCurrentPath(ctx context.Context, operation PackageOperationRecord) (string, error) {
	qm, err := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, operation.OperationID)
	if err != nil {
		return "", fmt.Errorf("kernel: quarantine metadata unavailable for current path resolution: %w", err)
	}
	if qm.CurrentQuarantinePath == "" {
		return "", errors.New("kernel: quarantine metadata missing current quarantine path")
	}
	return qm.CurrentQuarantinePath, nil
}

func (r *Runtime) resolveQuarantinePathsForCleanup(ctx context.Context, operation PackageOperationRecord, stable PackageGenerationCurrent) (string, string, error) {
	qm, err := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, operation.OperationID)
	if err != nil {
		return "", "", fmt.Errorf("kernel: quarantine metadata unavailable for cleanup path resolution: %w", err)
	}
	if qm.GenerationQuarantinePath == "" {
		return "", "", errors.New("kernel: quarantine metadata missing generation quarantine path")
	}
	if qm.CurrentQuarantinePath == "" {
		return "", "", errors.New("kernel: quarantine metadata missing current quarantine path")
	}
	return qm.GenerationQuarantinePath, qm.CurrentQuarantinePath, nil
}

func completedPackageSteps(steps []PackageOperationStep) map[string]PackageOperationStep {
	result := make(map[string]PackageOperationStep, len(steps))
	for _, step := range steps {
		if step.Status == "completed" {
			result[NormalizePackageStepName(step.StepName)] = step
		}
	}
	return result
}

func (r *Runtime) reconcileInstalledPackageGeneration(ctx context.Context, operation PackageOperationRecord) (bool, error) {
	if r.container.PackageGenerationStore == nil {
		return false, errors.New("generation store unavailable")
	}
	var target PackageGenerationCurrent
	if err := json.Unmarshal([]byte(operation.CurrentPointerJSON), &target); err != nil || target.GenerationID == "" {
		return false, errors.New("target generation evidence unavailable")
	}
	installation, dbErr := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	dbGeneration := ""
	if dbErr == nil {
		dbGeneration, _ = installation.Metadata["generationId"].(string)
	}
	current, currentErr := r.container.PackageGenerationStore.ReadCurrent(operation.ExtensionID)
	currentGeneration := ""
	if currentErr == nil {
		currentGeneration = current.GenerationID
	} else if !errors.Is(currentErr, ErrPackageGenerationNotFound) {
		return false, currentErr
	}
	if dbGeneration == target.GenerationID {
		if currentGeneration == target.GenerationID {
			verifyErr := r.container.PackageGenerationStore.VerifyGeneration(ctx, target)
			if verifyErr == nil {
				return false, nil
			}
			if !errors.Is(verifyErr, ErrPackageGenerationNotFound) {
				return false, verifyErr
			}
		}
		artifact, err := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
		if err != nil {
			return false, err
		}
		if _, err := r.VerifyStoredPackage(ctx, artifact); err != nil {
			return false, err
		}
		staging, err := r.container.PackageSecurity.ExtractFileToStaging(ctx, artifact.ArchivePath, operation.OperationID+"-recovery")
		if err != nil {
			return false, err
		}
		defer r.container.PackageSecurity.GetStagingManager().Cleanup(context.Background(), staging.ID)
		prepared, err := r.container.PackageGenerationStore.PrepareGeneration(ctx, PackageGenerationPrepareRequest{ExtensionID: target.ExtensionID, GenerationID: target.GenerationID, Version: target.Version, ArtifactID: target.ArtifactID, OperationID: target.OperationID, SourcePath: staging.Path, ExpectedTreeHash: target.TreeHash, FencingToken: target.FencingToken})
		if err != nil {
			return false, err
		}
		committed, err := r.container.PackageGenerationStore.CommitGeneration(ctx, prepared)
		if err != nil {
			return false, err
		}
		expected := operation.StableGeneration
		if currentGeneration != "" {
			expected = currentGeneration
		}
		if err := r.container.PackageGenerationStore.SwitchCurrent(operation.ExtensionID, expected, committed.Current); err != nil {
			return false, err
		}
		return false, nil
	}
	if currentGeneration != "" && currentGeneration != operation.StableGeneration && currentGeneration != target.GenerationID {
		return false, fmt.Errorf("unexpected current generation %s", currentGeneration)
	}
	prepared := PackagePreparedGeneration{Current: target}
	if currentGeneration == target.GenerationID {
		if err := r.compensatePackageGeneration(ctx, PackageGenerationCurrent{GenerationID: operation.StableGeneration}, prepared, true); err != nil {
			return false, err
		}
	} else {
		verifyErr := r.container.PackageGenerationStore.VerifyGeneration(ctx, target)
		if verifyErr == nil {
			if _, err := r.container.PackageGenerationStore.QuarantineGeneration(ctx, target); err != nil {
				return false, err
			}
		} else if !errors.Is(verifyErr, ErrPackageGenerationNotFound) {
			return false, verifyErr
		}
	}
	return true, nil
}

type CommitGenerationStepResult struct {
	Path             string `json:"path"`
	TreeHash         string `json:"treeHash"`
	StableGeneration string `json:"stableGeneration"`
	TargetGeneration string `json:"targetGeneration"`
	ArtifactHash     string `json:"artifactHash"`
}

type CommitRepositoryResult struct {
	InstallationID string `json:"installationId"`
	VersionID      string `json:"versionId"`
	ArtifactID     string `json:"artifactId"`
	GenerationID   string `json:"generationId"`
}

func (r *Runtime) proveInstalledPackageOperation(ctx context.Context, operation PackageOperationRecord, completed map[string]PackageOperationStep) error {
	commitTree, ok := completed[StepCommitInstalledTree]
	if !ok {
		return errors.New("installed tree commit step missing")
	}
	commitRepoStep, ok := completed[StepCommitKernelRepositories]
	if !ok {
		return errors.New("kernel repository commit step missing")
	}
	installation, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	if err != nil {
		return fmt.Errorf("installation unavailable: %w", err)
	}
	if installation.PackageID != operation.ArtifactID || installation.InstalledVersion.String() != operation.TargetVersion || installation.InstallationState != domain.InstallationStateInstalled {
		return errors.New("installation identity mismatch")
	}
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
	if err != nil {
		return fmt.Errorf("artifact unavailable: %w", err)
	}
	if artifact.ExtensionID != operation.ExtensionID || artifact.Version != operation.TargetVersion {
		return errors.New("artifact identity mismatch")
	}
	if _, err := r.VerifyStoredPackage(ctx, artifact); err != nil {
		return fmt.Errorf("artifact verification failed: %w", err)
	}
	installedPath, _ := installation.Metadata["installedPath"].(string)
	if installedPath == "" || artifact.InstalledPath == "" || filepath.Clean(installedPath) != filepath.Clean(artifact.InstalledPath) {
		return errors.New("installed path identity mismatch")
	}
	var commitResult CommitGenerationStepResult
	if err := json.Unmarshal([]byte(commitTree.ResultJSON), &commitResult); err != nil || filepath.Clean(commitResult.Path) != filepath.Clean(installedPath) {
		return errors.New("installed path journal mismatch")
	}
	var commitRepoResult CommitRepositoryResult
	if err := json.Unmarshal([]byte(commitRepoStep.ResultJSON), &commitRepoResult); err != nil || commitRepoResult.InstallationID != installation.InstallationID || commitRepoResult.ArtifactID != operation.ArtifactID {
		return errors.New("kernel repository commit result mismatch")
	}
	if err := r.proveInstalledTree(installedPath, installation, commitResult.TreeHash); err != nil {
		return err
	}
	definition, err := r.container.DefinitionRepository.GetExtension(ctx, installation.ExtensionID, installation.InstalledVersion)
	if err != nil {
		return fmt.Errorf("definition unavailable: %w", err)
	}
	modules, err := r.container.ModuleRepository.ListModules(ctx, installation.ExtensionID)
	if err != nil {
		return fmt.Errorf("modules unavailable: %w", err)
	}
	contributions, err := r.container.ContributionRepository.ListContributions(ctx, installation.ExtensionID)
	if err != nil {
		return fmt.Errorf("contributions unavailable: %w", err)
	}
	expectedContributions := 0
	for _, module := range definition.Modules {
		expectedContributions += len(module.Contributions)
	}
	if len(modules) != len(definition.Modules) || len(contributions) != expectedContributions {
		return errors.New("installed definitions incomplete")
	}
	return nil
}

func (r *Runtime) proveUpdatedPackageOperation(ctx context.Context, operation PackageOperationRecord, completed map[string]PackageOperationStep) error {
	commitTree, ok := completed[StepCommitTargetGeneration]
	if !ok {
		return errors.New("update tree commit step missing")
	}
	commitUpdateStateStep, ok := completed[StepCommitUpdateState]
	if !ok {
		return errors.New("update state commit step missing")
	}
	installation, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	if err != nil {
		return fmt.Errorf("installation unavailable: %w", err)
	}
	if installation.PackageID != operation.ArtifactID || installation.InstalledVersion.String() != operation.TargetVersion || installation.InstallationState != domain.InstallationStateInstalled {
		return errors.New("update installation identity mismatch")
	}
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
	if err != nil {
		return fmt.Errorf("update artifact unavailable: %w", err)
	}
	if artifact.ExtensionID != operation.ExtensionID || artifact.Version != operation.TargetVersion {
		return errors.New("update artifact identity mismatch")
	}
	if _, err := r.VerifyStoredPackage(ctx, artifact); err != nil {
		return fmt.Errorf("update artifact verification failed: %w", err)
	}
	installedPath, _ := installation.Metadata["installedPath"].(string)
	if installedPath == "" || artifact.InstalledPath == "" || filepath.Clean(installedPath) != filepath.Clean(artifact.InstalledPath) {
		return errors.New("update installed path identity mismatch")
	}
	var commitResult CommitGenerationStepResult
	if err := json.Unmarshal([]byte(commitTree.ResultJSON), &commitResult); err != nil || filepath.Clean(commitResult.Path) != filepath.Clean(installedPath) {
		return errors.New("update installed path journal mismatch")
	}
	var commitRepoResult CommitRepositoryResult
	if err := json.Unmarshal([]byte(commitUpdateStateStep.ResultJSON), &commitRepoResult); err != nil || commitRepoResult.InstallationID != installation.InstallationID || commitRepoResult.ArtifactID != operation.ArtifactID {
		return errors.New("update repository commit result mismatch")
	}
	if err := r.proveInstalledTree(installedPath, installation, commitResult.TreeHash); err != nil {
		return err
	}
	if _, ok := completed[StepCreateRollbackPoint]; !ok {
		return errors.New("update rollback point step missing")
	}
	if _, ok := completed[StepExecuteMigrations]; !ok {
		return errors.New("update migration step missing")
	}
	definition, err := r.container.DefinitionRepository.GetExtension(ctx, installation.ExtensionID, installation.InstalledVersion)
	if err != nil {
		return fmt.Errorf("update definition unavailable: %w", err)
	}
	modules, err := r.container.ModuleRepository.ListModules(ctx, installation.ExtensionID)
	if err != nil {
		return fmt.Errorf("update modules unavailable: %w", err)
	}
	contributions, err := r.container.ContributionRepository.ListContributions(ctx, installation.ExtensionID)
	if err != nil {
		return fmt.Errorf("update contributions unavailable: %w", err)
	}
	expectedContributions := 0
	for _, module := range definition.Modules {
		expectedContributions += len(module.Contributions)
	}
	if len(modules) != len(definition.Modules) || len(contributions) != expectedContributions {
		return errors.New("update definitions incomplete")
	}
	return nil
}

func (r *Runtime) proveRollbackPackageOperation(ctx context.Context, operation PackageOperationRecord, completed map[string]PackageOperationStep) error {
	if _, ok := completed[StepValidateRollbackPoint]; !ok {
		return errors.New("rollback point validation step missing")
	}
	if _, ok := completed[StepRestoreRepositories]; !ok {
		return errors.New("repository restore step missing")
	}
	installation, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	if err != nil {
		return fmt.Errorf("installation unavailable: %w", err)
	}
	if installation.InstalledVersion.String() != operation.TargetVersion || installation.PackageID != operation.ArtifactID {
		return errors.New("rollback installation identity mismatch")
	}
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
	if err != nil {
		return fmt.Errorf("rollback artifact unavailable: %w", err)
	}
	if _, err := r.VerifyStoredPackage(ctx, artifact); err != nil {
		return fmt.Errorf("rollback artifact verification failed: %w", err)
	}
	installedPath, _ := installation.Metadata["installedPath"].(string)
	if err := r.proveInstalledTree(installedPath, installation, ""); err != nil {
		return err
	}
	point, pointErr := r.container.PackageRepository.GetRollbackPoint(ctx, operation.ExtensionID, operation.TargetVersion)
	if pointErr == nil {
		if r.container.ResourceSnapshotStore != nil && point.ResourceSnapshotJSON != "" {
			if err := r.container.ResourceSnapshotStore.VerifyResourceSnapshotEntries(ctx, point.ResourceSnapshotJSON); err != nil {
				return fmt.Errorf("rollback resource snapshot verification failed: %w", err)
			}
			if err := r.container.ResourceSnapshotStore.VerifyNoActiveQuarantine(ctx, operation.OperationID); err != nil {
				return fmt.Errorf("rollback resource quarantine verification failed: %w", err)
			}
		}
		if r.container.UserDataSnapshotStore != nil && point.UserDataMigrationStateJSON != "" {
			restoreOperationID := point.SourceOperationID
			if restoreOperationID == "" {
				restoreOperationID = "restore-" + point.RollbackPointID
			}
			if err := r.container.UserDataSnapshotStore.VerifyUserDataRestore(ctx, restoreOperationID); err != nil {
				return fmt.Errorf("rollback user data restore verification failed: %w", err)
			}
		}
	}
	return nil
}

func (r *Runtime) proveUninstalledPackageOperation(ctx context.Context, operation PackageOperationRecord, completed map[string]PackageOperationStep) error {
	if _, ok := completed[StepMoveToQuarantine]; !ok {
		return errors.New("quarantine move step missing")
	}
	if _, ok := completed[StepCleanupKernelRepositories]; !ok {
		return errors.New("repository cleanup step missing")
	}
	_, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	if err == nil {
		return errors.New("installation still exists")
	}
	if !errors.Is(err, domain.ErrInvalidExtensionID) {
		return fmt.Errorf("installation state unavailable: %w", err)
	}
	artifact, err := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
	if err != nil {
		return fmt.Errorf("artifact unavailable: %w", err)
	}
	if artifact.InstalledPath != "" {
		if _, statErr := os.Stat(artifact.InstalledPath); statErr == nil || !os.IsNotExist(statErr) {
			return errors.New("installed path absence could not be proven")
		}
	}
	qm, qmErr := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, operation.OperationID)
	if qmErr != nil {
		return fmt.Errorf("kernel: quarantine metadata unavailable for finalization proof: %w", qmErr)
	}
	if qm.GenerationQuarantinePath == "" {
		return errors.New("kernel: quarantine metadata missing generation quarantine path")
	}
	if _, statErr := os.Stat(qm.GenerationQuarantinePath); statErr == nil || !os.IsNotExist(statErr) {
		return errors.New("quarantine finalization could not be proven")
	}
	return nil
}

func (r *Runtime) proveInstalledTree(installedPath string, installation domain.ExtensionInstallation, journalHash string) error {
	if installedPath == "" {
		return errors.New("installed path missing")
	}
	info, err := os.Stat(installedPath)
	if err != nil || !info.IsDir() {
		return errors.New("installed tree unavailable")
	}
	expectedHash, _ := installation.Metadata["installedTreeHash"].(string)
	generationID, _ := installation.Metadata["generationId"].(string)
	if generationID != "" && r.container.PackageGenerationStore != nil {
		current, err := r.container.PackageGenerationStore.ReadCurrent(string(installation.ExtensionID))
		if err != nil || current.GenerationID != generationID || current.TreeHash != expectedHash || filepath.Clean(installedPath) != filepath.Clean(filepath.Join(r.container.ExtRoot, "installations", safeDirectoryName(string(installation.ExtensionID)), "generations", generationID)) {
			return errors.New("installed generation current pointer mismatch")
		}
		if err := r.container.PackageGenerationStore.VerifyGeneration(context.Background(), current); err != nil {
			return err
		}
		if journalHash != "" && journalHash != expectedHash {
			return errors.New("installed tree journal hash mismatch")
		}
		return nil
	}
	actualHash := package_security.ComputeDirHash(installedPath, r.container.PackageSecurity.GetHasher())
	if actualHash == "" || expectedHash == "" || actualHash != expectedHash {
		return errors.New("installed tree hash mismatch")
	}
	if journalHash != "" && journalHash != actualHash {
		return errors.New("installed tree journal hash mismatch")
	}
	return nil
}

func (r *Runtime) requirePackageRecovery(ctx context.Context, operation PackageOperationRecord, detail string, cause error, guard PackageWriteGuard) error {
	if cause != nil {
		detail = detail + ": " + cause.Error()
	}
	setErr := r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "requires_recovery", "recovery_manual", "PACKAGE_RECOVERY_REQUIRED", detail, false, guard)
	if setErr != nil {
		setErr = r.container.PackageRepository.SetOperation(ctx, operation.OperationID, "requires_recovery", "recovery_manual", "PACKAGE_RECOVERY_REQUIRED", detail, false, PackageWriteGuard{})
	}
	if setErr != nil {
		return errors.Join(errors.New(detail), fmt.Errorf("persist recovery state: %w", setErr))
	}
	return errors.New(detail)
}
