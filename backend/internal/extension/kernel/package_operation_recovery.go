package kernel

import (
	"context"
	"crypto/sha256"
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

func (r *Runtime) recoverPackageOperation(ctx context.Context, operation PackageOperationRecord) error {
	releaseInProcessLock := r.acquirePackageInProcessLock(operation.ExtensionID + ":" + operation.OperationID)
	defer releaseInProcessLock()

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
		if err := r.container.PackageRepository.FinalizeOperationAndReleaseLeaseTx(ctx, operation.OperationID, operation.ExtensionID, guard.FencingToken, RecoveryStepFinalizeOperation); err != nil {
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
		if err := r.container.PackageRepository.FinalizeOperationAndReleaseLeaseTx(ctx, operation.OperationID, operation.ExtensionID, guard.FencingToken, RecoveryStepFinalizeOperation); err != nil {
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
		if err := r.container.PackageRepository.FinalizeOperationAndReleaseLeaseTx(ctx, operation.OperationID, operation.ExtensionID, guard.FencingToken, RecoveryStepFinalizeOperation); err != nil {
			return r.requirePackageRecovery(ctx, operation, "recovery lease release failed", err, guard)
		}
		leaseGuard.MarkLeaseReleased()
		return nil
	case "uninstall":
		if err := r.executeUninstallRecoveryChain(ctx, operation, completed, guard); err != nil {
			return err
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
	if err := r.validateQuarantineMetadataIntegrity(qm, operation, guard); err != nil {
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

type quarantineLeaseQuerier func(ctx context.Context, extensionID string) (PackageExtensionLease, error)

func validateQuarantineMetadataFence(qm PackageQuarantineMetadata, operation PackageOperationRecord, recoveryGuardToken int64, leaseQuerier quarantineLeaseQuerier) error {
	if recoveryGuardToken <= 0 {
		return operationStateError(OperationErrProofUnavailable, "recovery guard token unavailable for quarantine fence proof", nil)
	}
	if leaseQuerier == nil {
		return operationStateError(OperationErrProofUnavailable, "lease querier unavailable for quarantine fence proof", nil)
	}
	if qm.FencingToken <= 0 {
		return operationStateError(OperationErrProofUnavailable,
			fmt.Sprintf("quarantine metadata fencing_token must be greater than zero: %d", qm.FencingToken), nil)
	}
	if qm.FencingToken > recoveryGuardToken {
		return operationStateError(OperationErrTokenStale,
			fmt.Sprintf("quarantine metadata fencing_token exceeds recovery guard: metadata=%d recovery=%d", qm.FencingToken, recoveryGuardToken), nil)
	}
	liveLease, liveErr := leaseQuerier(context.Background(), operation.ExtensionID)
	if liveErr != nil {
		return operationStateError(OperationErrProofUnavailable,
			"lease repository unavailable for quarantine fence proof", liveErr)
	}
	if liveLease.OperationID != operation.OperationID {
		return operationStateError(OperationErrLeaseProofMismatch,
			fmt.Sprintf("lease operation mismatch for quarantine fence proof: lease=%s operation=%s", liveLease.OperationID, operation.OperationID), nil)
	}
	if liveLease.FencingToken != recoveryGuardToken {
		return operationStateError(OperationErrLeaseProofMismatch,
			fmt.Sprintf("lease fencing token mismatch for quarantine fence proof: lease=%d recovery=%d", liveLease.FencingToken, recoveryGuardToken), nil)
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
			if !IsRepositoryErrorKind(err, RepositoryErrorNotFound) {
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
			if err := r.container.UserDataSnapshotStore.VerifyUserDataRestore(ctx, restoreOperationID, point.UserDataMigrationStateJSON); err != nil {
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
	if _, ok := completed[StepUninstallRecoveryFinalGate]; !ok {
		return errors.New("final gate step missing")
	}
	if _, ok := completed[StepUninstallRecoveryFinalize]; !ok {
		return errors.New("finalize step missing")
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

func (r *Runtime) validateQuarantineMetadataIntegrity(qm PackageQuarantineMetadata, operation PackageOperationRecord, guard PackageWriteGuard) error {
	if qm.QuarantineID == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeQuarantineMetadataIncomplete, 409, false, true, "Reload quarantine metadata", fmt.Errorf("quarantine_id is empty"))
	}
	if qm.OperationID != operation.OperationID {
		return NewPackageErrorWithRecovery(PackageErrCodeQuarantineMetadataIncomplete, 409, false, true, "Reload quarantine metadata", fmt.Errorf("operation_id mismatch: expected %s got %s", operation.OperationID, qm.OperationID))
	}
	if qm.ExtensionID != operation.ExtensionID {
		return NewPackageErrorWithRecovery(PackageErrCodeQuarantineMetadataIncomplete, 409, false, true, "Reload quarantine metadata", fmt.Errorf("extension_id mismatch: expected %s got %s", operation.ExtensionID, qm.ExtensionID))
	}
	if qm.ArtifactID == "" || qm.ArtifactID != operation.ArtifactID {
		return NewPackageErrorWithRecovery(PackageErrCodeQuarantineMetadataIncomplete, 409, false, true, "Reload quarantine metadata", fmt.Errorf("artifact_id mismatch: expected %s got %s", operation.ArtifactID, qm.ArtifactID))
	}
	if err := validateQuarantineMetadataFence(qm, operation, guard.FencingToken, r.container.PackageRepository.getExtensionLease); err != nil {
		return err
	}
	if qm.GenerationQuarantinePath == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeQuarantineMetadataIncomplete, 409, false, true, "Reload quarantine metadata", fmt.Errorf("generation_quarantine_path is empty"))
	}
	if qm.CurrentQuarantinePath == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeQuarantineMetadataIncomplete, 409, false, true, "Reload quarantine metadata", fmt.Errorf("current_quarantine_path is empty"))
	}
	if qm.OriginalGenerationPath == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeQuarantineMetadataIncomplete, 409, false, true, "Reload quarantine metadata", fmt.Errorf("original_generation_path is empty"))
	}
	if qm.OriginalCurrentPath == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeQuarantineMetadataIncomplete, 409, false, true, "Reload quarantine metadata", fmt.Errorf("original_current_path is empty"))
	}
	if qm.TreeHash == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeQuarantineMetadataIncomplete, 409, false, true, "Reload quarantine metadata", fmt.Errorf("tree_hash is empty"))
	}
	if qm.State != "active" && qm.State != "finalized" && qm.State != "restored" && qm.State != "released" {
		return NewPackageErrorWithRecovery(PackageErrCodeQuarantineMetadataIncomplete, 409, false, true, "Reload quarantine metadata", fmt.Errorf("invalid state: %s", qm.State))
	}
	expectedGenerationID := qm.ExpectedGenerationID
	if expectedGenerationID == "" {
		expectedGenerationID = operation.StableGeneration
	}
	expected, err := r.container.PackageGenerationStore.ExpectedUninstallRecoveryPaths(operation.ExtensionID, expectedGenerationID, operation.OperationID)
	if err != nil {
		return NewPackageErrorWithRecovery(PackageErrCodeQuarantineMetadataCorrupt, 409, false, true, "Reload quarantine metadata", fmt.Errorf("resolve expected recovery paths: %w", err))
	}
	if !r.pathsMatchExactly(qm.OriginalCurrentPath, expected.OriginalCurrentPath) {
		return NewPackageErrorWithRecovery(PackageErrCodeQuarantinePathMismatch, 409, false, true, "Reload quarantine metadata",
			fmt.Errorf("original_current_path mismatch: metadata=%s expected=%s", qm.OriginalCurrentPath, expected.OriginalCurrentPath))
	}
	if !r.pathsMatchExactly(qm.OriginalGenerationPath, expected.OriginalGenerationPath) {
		return NewPackageErrorWithRecovery(PackageErrCodeQuarantinePathMismatch, 409, false, true, "Reload quarantine metadata",
			fmt.Errorf("original_generation_path mismatch: metadata=%s expected=%s", qm.OriginalGenerationPath, expected.OriginalGenerationPath))
	}
	if !r.pathsMatchExactly(qm.CurrentQuarantinePath, expected.CurrentQuarantinePath) {
		return NewPackageErrorWithRecovery(PackageErrCodeQuarantinePathMismatch, 409, false, true, "Reload quarantine metadata",
			fmt.Errorf("current_quarantine_path mismatch: metadata=%s expected=%s", qm.CurrentQuarantinePath, expected.CurrentQuarantinePath))
	}
	if !r.pathsMatchExactly(qm.GenerationQuarantinePath, expected.GenerationQuarantinePath) {
		return NewPackageErrorWithRecovery(PackageErrCodeQuarantinePathMismatch, 409, false, true, "Reload quarantine metadata",
			fmt.Errorf("generation_quarantine_path mismatch: metadata=%s expected=%s", qm.GenerationQuarantinePath, expected.GenerationQuarantinePath))
	}
	return nil
}

func (r *Runtime) pathsMatchExactly(metadataPath, expectedPath string) bool {
	if metadataPath == "" || expectedPath == "" {
		return false
	}
	absMeta, err := filepath.Abs(metadataPath)
	if err != nil {
		return false
	}
	absExpected, err := filepath.Abs(expectedPath)
	if err != nil {
		return false
	}
	cleanMeta := filepath.Clean(absMeta)
	cleanExpected := filepath.Clean(absExpected)
	if cleanMeta != cleanExpected {
		return false
	}
	if info, statErr := os.Stat(cleanMeta); statErr == nil {
		if evalPath, evalErr := filepath.EvalSymlinks(cleanMeta); evalErr == nil {
			if evalExpected, evalErr2 := filepath.EvalSymlinks(cleanExpected); evalErr2 == nil {
				return evalPath == evalExpected
			}
		}
		_ = info
	}
	return true
}

func (r *Runtime) runUninstallRecoveryStep(ctx context.Context, operation PackageOperationRecord, completed map[string]PackageOperationStep, stepName string, order int, guard PackageWriteGuard, action func() (string, error)) error {
	if existing, ok := completed[stepName]; ok {
		if existing.ResultJSON != "" {
			actualHash := fmt.Sprintf("%x", sha256.Sum256([]byte(existing.ResultJSON)))
			if actualHash != existing.ResultHash {
				return NewPackageErrorWithRecovery(PackageErrCodeRecoveryStepPersistFailed, 409, false, true, "Reload and re-verify step", fmt.Errorf("step %s result_hash mismatch", stepName))
			}
		}
		return nil
	}
	resultJSON, actionErr := action()
	if actionErr != nil {
		return actionErr
	}
	resultHash := fmt.Sprintf("%x", sha256.Sum256([]byte(resultJSON)))
	step := PackageOperationStep{
		OperationID:  operation.OperationID,
		StepName:     stepName,
		StepOrder:    order,
		Status:       StatusCompleted,
		ResultJSON:   resultJSON,
		ResultHash:   resultHash,
		AttemptCount: 1,
		StartedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		CompletedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := r.container.PackageRepository.PutStep(ctx, step, guard); err != nil {
		return NewPackageErrorWithRecovery(PackageErrCodeRecoveryStepPersistFailed, 500, false, true, "Retry recovery", fmt.Errorf("put step %s: %w", stepName, err))
	}
	completed[stepName] = step
	return nil
}

type UninstallRestoredIdentityEvidence struct {
	SchemaVersion int `json:"schemaVersion"`

	OperationID string `json:"operationId"`
	ExtensionID string `json:"extensionId"`
	ArtifactID  string `json:"artifactId"`

	ExpectedVersionID    string `json:"expectedVersionId"`
	RestoredVersion      string `json:"restoredVersion"`
	ExpectedGenerationID string `json:"expectedGenerationId"`

	InstallationVersion      string `json:"installationVersion"`
	InstallationGenerationID string `json:"installationGenerationId"`

	VersionRecordID           string `json:"versionRecordId"`
	VersionRecordVersion      string `json:"versionRecordVersion"`
	VersionRecordGenerationID string `json:"versionRecordGenerationId"`

	CurrentVersion      string `json:"currentVersion"`
	CurrentArtifactID   string `json:"currentArtifactId"`
	CurrentGenerationID string `json:"currentGenerationId"`
	CurrentTreeHash     string `json:"currentTreeHash"`

	MetadataTreeHash         string `json:"metadataTreeHash"`
	ActualGenerationTreeHash string `json:"actualGenerationTreeHash"`

	EvidenceHash string `json:"evidenceHash"`
}

type UninstallReleaseQuarantineStepResult struct {
	SchemaVersion int `json:"schemaVersion"`

	OperationID  string `json:"operationId"`
	QuarantineID string `json:"quarantineId"`
	ExtensionID  string `json:"extensionId"`
	ArtifactID   string `json:"artifactId"`

	ReleasedAt     string `json:"releasedAt"`
	SnapshotHash   string `json:"snapshotHash"`
	GenerationHash string `json:"generationHash"`
	MetadataHash   string `json:"metadataHash"`
}

type UninstallRecoveryContext struct {
	operation          PackageOperationRecord
	quarantineMetadata PackageQuarantineMetadata
	guard              PackageWriteGuard
	completed          map[string]PackageOperationStep
	container          *Container
}

func (rc *UninstallRecoveryContext) reloadQuarantineMetadata(ctx context.Context) error {
	if rc.container == nil || rc.container.PackageRepository == nil {
		return errors.New("kernel: recovery context container unavailable")
	}
	qm, err := rc.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, rc.operation.OperationID)
	if err != nil {
		kind := RepositoryErrorKindOf(err)
		switch kind {
		case RepositoryErrorNotFound:
			return NewPackageErrorWithRecovery(PackageErrCodeQuarantineMetadataMissing, 409, false, true, "Reload quarantine metadata", err)
		case RepositoryErrorUnavailable:
			return NewPackageErrorWithRecovery(PackageErrCodeQuarantineMetadataUnavailable, 503, true, true, "Retry recovery", err)
		case RepositoryErrorCorrupt:
			return NewPackageErrorWithRecovery(PackageErrCodeQuarantineMetadataIncomplete, 409, false, true, "Inspect quarantine metadata", err)
		default:
			return err
		}
	}
	rc.quarantineMetadata = qm
	return nil
}

func (rc *UninstallRecoveryContext) crossValidateLoadStep() error {
	loadStep, exists := rc.completed[StepUninstallRecoveryLoadQuarantineMetadata]
	if !exists || loadStep.ResultJSON == "" {
		return nil
	}
	var loadResult struct {
		QuarantineID string `json:"quarantine_id"`
		State        string `json:"state"`
	}
	if err := json.Unmarshal([]byte(loadStep.ResultJSON), &loadResult); err != nil {
		return fmt.Errorf("kernel: load step result corrupted: %w", err)
	}
	if loadResult.QuarantineID != "" && loadResult.QuarantineID != rc.quarantineMetadata.QuarantineID {
		return fmt.Errorf("kernel: load step quarantine_id mismatch: load=%s current=%s", loadResult.QuarantineID, rc.quarantineMetadata.QuarantineID)
	}
	return nil
}

type RecoveryAction string

const (
	RecoveryActionNone                 RecoveryAction = "none"
	RecoveryActionContinueCompensation RecoveryAction = "continue_compensation"
	RecoveryActionResumeFinalization   RecoveryAction = "resume_finalization"
	RecoveryActionVerifyCompleted      RecoveryAction = "verify_completed"
)

type PackageFinalGateMode string

const (
	PackageFinalGateModeOperationCompleted   PackageFinalGateMode = "operation_completed"
	PackageFinalGateModeUninstallCompensated PackageFinalGateMode = "uninstall_compensated"
)

func (r *Runtime) prepareUninstallRecoveryOperation(ctx context.Context, operation PackageOperationRecord, guard PackageWriteGuard) (PackageOperationRecord, RecoveryAction, error) {
	authoritative, _, err := r.container.PackageRepository.GetOperation(ctx, operation.UserID, operation.OperationID)
	if err != nil {
		return operation, RecoveryActionNone, fmt.Errorf("kernel: read authoritative operation for recovery preparation: %w", err)
	}
	if authoritative.OperationType != "uninstall" {
		return operation, RecoveryActionNone, fmt.Errorf("kernel: recovery preparation requires uninstall operation, got %s", authoritative.OperationType)
	}

	switch PackageOperationStatus(authoritative.Status) {
	case PackageOperationRequiresRecovery:
		return r.transitionRequiresRecoveryToInProgress(ctx, authoritative, guard)

	case PackageOperationInProgress:
		if err := r.verifyLeaseForRecovery(ctx, authoritative, guard); err != nil {
			return operation, RecoveryActionNone, err
		}
		return authoritative, RecoveryActionContinueCompensation, nil

	case PackageOperationFinalizing:
		if err := r.verifyLeaseForRecovery(ctx, authoritative, guard); err != nil {
			return operation, RecoveryActionNone, err
		}
		return authoritative, RecoveryActionResumeFinalization, nil

	case PackageOperationCompleted:
		return authoritative, RecoveryActionVerifyCompleted, nil

	case PackageOperationFailed:
		return operation, RecoveryActionNone, operationStateError(OperationErrRecoveryNotAllowed,
			"cannot recover failed operation", nil)

	case PackageOperationCancelled:
		return operation, RecoveryActionNone, operationStateError(OperationErrRecoveryNotAllowed,
			"cannot recover cancelled operation", nil)

	default:
		return operation, RecoveryActionNone,
			NewPackageErrorWithRecovery(PackageErrCodeRecoveryStepEvidenceInvalid, 409, false, true,
				"Inspect operation state",
				fmt.Errorf("unsupported uninstall recovery state: %s", authoritative.Status))
	}
}

func (r *Runtime) transitionRequiresRecoveryToInProgress(ctx context.Context, authoritative PackageOperationRecord, guard PackageWriteGuard) (PackageOperationRecord, RecoveryAction, error) {
	lease, leaseErr := r.container.PackageRepository.getExtensionLease(ctx, authoritative.ExtensionID)
	if leaseErr != nil {
		return authoritative, RecoveryActionNone, fmt.Errorf("kernel: recovery preparation lease proof failed: %w", leaseErr)
	}
	if lease.OperationID != authoritative.OperationID {
		return authoritative, RecoveryActionNone, operationStateError(PackageErrCodeLeaseFenced,
			fmt.Sprintf("lease held by different operation: lease=%s authoritative=%s", lease.OperationID, authoritative.OperationID), nil)
	}
	if lease.FencingToken != guard.FencingToken {
		return authoritative, RecoveryActionNone, operationStateError(PackageErrCodeLeaseFenced,
			fmt.Sprintf("lease fencing token mismatch: lease=%d guard=%d", lease.FencingToken, guard.FencingToken), nil)
	}

	allowedSourceStates := []PackageOperationStatus{PackageOperationRequiresRecovery}
	transitionErr := r.container.PackageRepository.TransitionOperation(ctx,
		authoritative.OperationID,
		allowedSourceStates,
		PackageOperationInProgress,
		PackageOperationTransition{CurrentStep: "uninstall_recovery_compensating"},
		guard)
	if transitionErr != nil {
		return authoritative, RecoveryActionNone, fmt.Errorf("kernel: CAS transition requires_recovery→in_progress failed: %w", transitionErr)
	}

	updatedOp, _, reReadErr := r.container.PackageRepository.GetOperation(ctx, authoritative.UserID, authoritative.OperationID)
	if reReadErr != nil {
		return authoritative, RecoveryActionNone, fmt.Errorf("kernel: re-read operation after transition: %w", reReadErr)
	}
	return updatedOp, RecoveryActionContinueCompensation, nil
}

func (r *Runtime) verifyLeaseForRecovery(ctx context.Context, operation PackageOperationRecord, guard PackageWriteGuard) error {
	lease, leaseErr := r.container.PackageRepository.getExtensionLease(ctx, operation.ExtensionID)
	if leaseErr != nil {
		return fmt.Errorf("kernel: recovery lease proof failed: %w", leaseErr)
	}
	if lease.OperationID != operation.OperationID {
		return operationStateError(PackageErrCodeLeaseFenced,
			fmt.Sprintf("lease held by different operation: lease=%s operation=%s", lease.OperationID, operation.OperationID), nil)
	}
	if lease.FencingToken != guard.FencingToken {
		return operationStateError(PackageErrCodeLeaseFenced,
			fmt.Sprintf("lease fencing token mismatch: lease=%d guard=%d", lease.FencingToken, guard.FencingToken), nil)
	}
	return nil
}

func (r *Runtime) executeUninstallRecoveryChain(ctx context.Context, operation PackageOperationRecord, completed map[string]PackageOperationStep, guard PackageWriteGuard) error {
	preparedOp, action, prepErr := r.prepareUninstallRecoveryOperation(ctx, operation, guard)
	if prepErr != nil {
		if IsPackageOperationError(prepErr, PackageErrCodeLeaseFenced) {
			return prepErr
		}
		return r.requirePackageRecovery(ctx, operation, "recovery preparation failed: "+prepErr.Error(), prepErr, guard)
	}

	switch action {
	case RecoveryActionVerifyCompleted:
		if verifyErr := r.verifyUninstallFinalizedState(ctx, preparedOp); verifyErr != nil {
			return r.requirePackageRecovery(ctx, operation, "finalized state verification failed", verifyErr, guard)
		}
		leaseGuard := r.newPackageLeaseGuard(preparedOp.ExtensionID, preparedOp.OperationID)
		leaseGuard.MarkLeaseReleased()
		return nil

	case RecoveryActionResumeFinalization:
		operation = preparedOp
		return r.finalizeUninstallRecovery(ctx, operation, completed, guard)

	case RecoveryActionContinueCompensation:
		operation = preparedOp

	case RecoveryActionNone:
		return r.requirePackageRecovery(ctx, operation, "recovery preparation returned no action", nil, guard)
	}

	rc := &UninstallRecoveryContext{
		operation: operation,
		guard:     guard,
		completed: completed,
		container: r.container,
	}
	if err := rc.reloadQuarantineMetadata(ctx); err != nil {
		return r.requirePackageRecovery(ctx, operation, "reload quarantine metadata failed", err, guard)
	}
	if err := rc.crossValidateLoadStep(); err != nil {
		return r.requirePackageRecovery(ctx, operation, "cross validate load step failed", err, guard)
	}

	order := 1
	qm := &rc.quarantineMetadata
	var cachedQM *PackageQuarantineMetadata = qm

	err := r.runUninstallRecoveryStep(ctx, operation, completed, StepUninstallRecoveryLoadQuarantineMetadata, order, guard, func() (string, error) {
		order++
		if err := rc.reloadQuarantineMetadata(ctx); err != nil {
			return "", fmt.Errorf("load quarantine metadata: %w", err)
		}
		cachedQM = &rc.quarantineMetadata
		result := map[string]string{"quarantine_id": rc.quarantineMetadata.QuarantineID, "state": rc.quarantineMetadata.State}
		resultBytes, _ := json.Marshal(result)
		return string(resultBytes), nil
	})
	if err != nil {
		return r.requirePackageRecovery(ctx, operation, "uninstall recovery load_quarantine_metadata failed", err, guard)
	}

	err = r.runUninstallRecoveryStep(ctx, operation, completed, StepUninstallRecoveryVerifyQuarantineMetadata, order, guard, func() (string, error) {
		order++
		if err := rc.reloadQuarantineMetadata(ctx); err != nil {
			return "", err
		}
		cachedQM = &rc.quarantineMetadata
		if err := r.validateQuarantineMetadataIntegrity(*cachedQM, operation, guard); err != nil {
			return "", err
		}
		return `{"verified":true}`, nil
	})
	if err != nil {
		if isLeaseRelatedError(err) {
			return err
		}
		return r.requirePackageRecovery(ctx, operation, "uninstall recovery verify_quarantine_metadata failed", err, guard)
	}

	err = r.runUninstallRecoveryStep(ctx, operation, completed, StepUninstallRecoveryRestoreGeneration, order, guard, func() (string, error) {
		order++
		if err := rc.reloadQuarantineMetadata(ctx); err != nil {
			return "", err
		}
		cachedQM = &rc.quarantineMetadata
		if err := r.restoreQuarantinedGeneration(ctx, operation, *cachedQM, guard); err != nil {
			return "", err
		}
		return `{"generation_restored":true}`, nil
	})
	if err != nil {
		if isLeaseRelatedError(err) {
			return err
		}
		return r.requirePackageRecovery(ctx, operation, "uninstall recovery restore_generation failed", err, guard)
	}

	err = r.runUninstallRecoveryStep(ctx, operation, completed, StepUninstallRecoveryRestoreCurrent, order, guard, func() (string, error) {
		order++
		if err := rc.reloadQuarantineMetadata(ctx); err != nil {
			return "", err
		}
		cachedQM = &rc.quarantineMetadata
		if err := r.restoreQuarantinedCurrent(ctx, operation, *cachedQM, guard); err != nil {
			return "", err
		}
		return `{"current_restored":true}`, nil
	})
	if err != nil {
		return r.requirePackageRecovery(ctx, operation, "uninstall recovery restore_current failed", err, guard)
	}

	err = r.runUninstallRecoveryStep(ctx, operation, completed, StepUninstallRecoveryRestoreInstallation, order, guard, func() (string, error) {
		order++
		if err := rc.reloadQuarantineMetadata(ctx); err != nil {
			return "", err
		}
		cachedQM = &rc.quarantineMetadata
		if err := r.restoreQuarantinedInstallation(ctx, operation, *cachedQM, guard); err != nil {
			return "", err
		}
		if err := r.validateRestoredInstallationIdentity(*cachedQM, guard); err != nil {
			return "", err
		}
		return `{"installation_restored":true}`, nil
	})
	if err != nil {
		return r.requirePackageRecovery(ctx, operation, "uninstall recovery restore_installation failed", err, guard)
	}

	err = r.runUninstallRecoveryStep(ctx, operation, completed, StepUninstallRecoveryRestoreVersionState, order, guard, func() (string, error) {
		order++
		if err := rc.reloadQuarantineMetadata(ctx); err != nil {
			return "", err
		}
		cachedQM = &rc.quarantineMetadata
		if err := r.restoreVersionStateToCurrent(ctx, operation, *cachedQM, guard); err != nil {
			return "", err
		}
		return `{"version_state_restored":true}`, nil
	})
	if err != nil {
		return r.requirePackageRecovery(ctx, operation, "uninstall recovery restore_version_state failed", err, guard)
	}

	err = r.runUninstallRecoveryStep(ctx, operation, completed, StepUninstallRecoveryRestoreArtifactPath, order, guard, func() (string, error) {
		order++
		if err := rc.reloadQuarantineMetadata(ctx); err != nil {
			return "", err
		}
		cachedQM = &rc.quarantineMetadata
		if err := r.restoreArtifactInstalledPath(ctx, operation, *cachedQM, guard); err != nil {
			return "", err
		}
		return `{"artifact_path_restored":true}`, nil
	})
	if err != nil {
		return r.requirePackageRecovery(ctx, operation, "uninstall recovery restore_artifact_path failed", err, guard)
	}

	err = r.runUninstallRecoveryStep(ctx, operation, completed, StepUninstallRecoveryRestoreArtifactReference, order, guard, func() (string, error) {
		order++
		if err := rc.reloadQuarantineMetadata(ctx); err != nil {
			return "", err
		}
		cachedQM = &rc.quarantineMetadata
		if err := r.restoreArtifactInstallationReference(ctx, operation, *cachedQM, guard); err != nil {
			return "", err
		}
		return `{"artifact_reference_restored":true}`, nil
	})
	if err != nil {
		return r.requirePackageRecovery(ctx, operation, "uninstall recovery restore_artifact_reference failed", err, guard)
	}

	err = r.runUninstallRecoveryStep(ctx, operation, completed, StepUninstallRecoveryVerifyRestoredState, order, guard, func() (string, error) {
		order++
		if err := rc.reloadQuarantineMetadata(ctx); err != nil {
			return "", err
		}
		cachedQM = &rc.quarantineMetadata
		if err := r.verifyUninstallRestoredState(ctx, operation, *cachedQM, guard); err != nil {
			return "", err
		}
		return `{"restored_state_verified":true}`, nil
	})
	if err != nil {
		return r.requirePackageRecovery(ctx, operation, "uninstall recovery verify_restored_state failed", err, guard)
	}

	err = r.runUninstallRecoveryStep(ctx, operation, completed, StepUninstallRecoveryReleaseQuarantineMetadata, order, guard, func() (string, error) {
		order++
		if err := rc.reloadQuarantineMetadata(ctx); err != nil {
			return "", err
		}
		cachedQM = &rc.quarantineMetadata
		if err := r.container.PackageRepository.ReleaseQuarantineMetadata(ctx, cachedQM.QuarantineID, guard); err != nil {
			return "", err
		}
		releasedQM, reReadErr := r.container.PackageRepository.GetQuarantineMetadataByOperation(ctx, operation.OperationID)
		if reReadErr != nil {
			return "", fmt.Errorf("re-read quarantine metadata after release: %w", reReadErr)
		}
		if releasedQM.State != "released" {
			return "", fmt.Errorf("quarantine metadata state is not released after release: %s", releasedQM.State)
		}
		if _, genErr := r.container.PackageGenerationStore.ReadCurrent(operation.ExtensionID); genErr != nil {
			return "", fmt.Errorf("read current for release validation failed: %w", genErr)
		}
		identityEvidence, identityErr := r.verifyUninstallRestoredIdentity(ctx, operation, releasedQM)
		var actualTreeHash string
		if identityErr == nil {
			actualTreeHash = identityEvidence.ActualGenerationTreeHash
		}
		rc.quarantineMetadata = releasedQM
		releaseResultJSON := UninstallReleaseQuarantineStepResult{
			SchemaVersion:  1,
			OperationID:    releasedQM.OperationID,
			QuarantineID:   releasedQM.QuarantineID,
			ExtensionID:    releasedQM.ExtensionID,
			ArtifactID:     releasedQM.ArtifactID,
			ReleasedAt:     releasedQM.ReleasedAt,
			SnapshotHash:   releasedQM.SnapshotHash,
			GenerationHash: actualTreeHash,
		}
		metadataCanonical := packageCanonicalJSON(releasedQM)
		releaseResultJSON.MetadataHash = fmt.Sprintf("%x", sha256.Sum256([]byte(metadataCanonical)))
		stepResultBytes, marshalErr := json.Marshal(releaseResultJSON)
		if marshalErr != nil {
			return "", fmt.Errorf("marshal release step result: %w", marshalErr)
		}
		return string(stepResultBytes), nil
	})
	if err != nil {
		if isLeaseRelatedError(err) {
			return err
		}
		return r.requirePackageRecovery(ctx, operation, "uninstall recovery release_quarantine_metadata failed", err, guard)
	}

	err = r.runUninstallRecoveryStep(ctx, operation, completed, StepUninstallRecoveryFinalGate, order, guard, func() (string, error) {
		order++
		finalGateResult, gateErr := r.verifyUninstallCompensationFinalGate(ctx, operation, *rc, guard)
		if gateErr != nil {
			return "", gateErr
		}
		if !finalGateResult.Passed {
			return "", NewPackageErrorWithRecovery(PackageErrCodeFinalGateFailed, 409, false, true, "Inspect compensation final gate result",
				fmt.Errorf("compensation final gate not passed for operation %s", operation.OperationID))
		}
		resultBytes, marshalErr := json.Marshal(finalGateResult)
		if marshalErr != nil {
			return "", fmt.Errorf("marshal final gate result: %w", marshalErr)
		}
		return string(resultBytes), nil
	})
	if err != nil {
		return r.requirePackageRecovery(ctx, operation, "uninstall recovery final_gate failed", err, guard)
	}

	if err := r.finalizeUninstallRecovery(ctx, operation, completed, guard); err != nil {
		return err
	}

	return nil
}

func (r *Runtime) verifyUninstallCompensationFinalGate(ctx context.Context, operation PackageOperationRecord, rc UninstallRecoveryContext, guard PackageWriteGuard) (PackageFinalGateResult, error) {
	result := PackageFinalGateResult{
		OperationID:   operation.OperationID,
		OperationType: operation.OperationType,
		ExtensionID:   operation.ExtensionID,
		Version:       operation.TargetVersion,
		Mode:          string(PackageFinalGateModeUninstallCompensated),
		Checks:        make([]PackageFinalGateCheck, 0, 10),
		VerifiedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}
	qm := rc.quarantineMetadata

	checkOpType := PackageFinalGateCheck{Name: "operation_type_uninstall"}
	if operation.OperationType == "uninstall" {
		checkOpType.Passed = true
	} else {
		checkOpType.Detail = fmt.Sprintf("expected uninstall, got %s", operation.OperationType)
	}
	result.Checks = append(result.Checks, checkOpType)

	checkOpStatus := PackageFinalGateCheck{Name: "operation_in_progress"}
	if operation.Status == string(PackageOperationInProgress) || operation.Status == string(PackageOperationFinalizing) {
		checkOpStatus.Passed = true
	} else {
		checkOpStatus.Detail = fmt.Sprintf("status is %s, expected in_progress/finalizing", operation.Status)
	}
	result.Checks = append(result.Checks, checkOpStatus)

	checkLease := PackageFinalGateCheck{Name: "lease_proof"}
	lease, leaseErr := r.container.PackageRepository.getExtensionLease(ctx, operation.ExtensionID)
	if leaseErr != nil {
		checkLease.Detail = fmt.Sprintf("lease not found: %v", leaseErr)
	} else if lease.OperationID != operation.OperationID {
		checkLease.Detail = fmt.Sprintf("lease held by %s", lease.OperationID)
	} else if lease.FencingToken != guard.FencingToken {
		checkLease.Detail = "fencing token mismatch"
	} else {
		checkLease.Passed = true
	}
	result.Checks = append(result.Checks, checkLease)

	identityEvidence, identityErr := r.verifyUninstallRestoredIdentity(ctx, operation, qm)
	if identityErr == nil {
		result.RestoredIdentityEvidence = &identityEvidence
		result.RestoredIdentityEvidenceHash = identityEvidence.EvidenceHash
	}

	checkInstall := PackageFinalGateCheck{Name: "installation_identity", Passed: identityErr == nil}
	if identityErr != nil {
		checkInstall.Detail = identityErr.Error()
	}
	result.Checks = append(result.Checks, checkInstall)

	checkVersion := PackageFinalGateCheck{Name: "version_record", Passed: identityErr == nil}
	if identityErr != nil {
		checkVersion.Detail = identityErr.Error()
	}
	result.Checks = append(result.Checks, checkVersion)

	checkCurrent := PackageFinalGateCheck{Name: "current_pointer", Passed: identityErr == nil}
	if identityErr != nil {
		checkCurrent.Detail = identityErr.Error()
	}
	result.Checks = append(result.Checks, checkCurrent)

	checkGenTree := PackageFinalGateCheck{Name: "generation_tree"}
	if identityErr != nil {
		checkGenTree.Detail = identityErr.Error()
	} else {
		checkGenTree.Passed = true
		checkGenTree.Detail = fmt.Sprintf("current=%s metadata=%s actual=%s", identityEvidence.CurrentTreeHash, identityEvidence.MetadataTreeHash, identityEvidence.ActualGenerationTreeHash)
	}
	result.Checks = append(result.Checks, checkGenTree)

	checkArtifact := PackageFinalGateCheck{Name: "artifact_not_deleted"}
	artifact, artErr := r.container.PackageRepository.GetArtifact(ctx, operation.ArtifactID)
	if artErr != nil {
		checkArtifact.Detail = fmt.Sprintf("artifact unavailable: %v", artErr)
	} else if artifact.RetentionState == "deleted" || artifact.DeletedAt != "" {
		checkArtifact.Detail = "artifact was deleted during compensation"
	} else {
		checkArtifact.Passed = true
	}
	result.Checks = append(result.Checks, checkArtifact)

	checkInstallRef := PackageFinalGateCheck{Name: "installation_reference"}
	hasInstallRef, installRefErr := r.container.PackageRepository.HasArtifactReference(ctx, operation.ArtifactID, ArtifactReferenceInstallation, operation.ExtensionID)
	if installRefErr != nil {
		checkInstallRef.Detail = fmt.Sprintf("installation reference query failed: %v", installRefErr)
	} else if !hasInstallRef {
		checkInstallRef.Detail = "installation reference missing"
	} else {
		checkInstallRef.Passed = true
	}
	result.Checks = append(result.Checks, checkInstallRef)

	checkQM := PackageFinalGateCheck{Name: "quarantine_released_identity"}
	var qmTreeHash string
	if identityErr == nil {
		qmTreeHash = identityEvidence.ActualGenerationTreeHash
	}
	qmValidateErr := r.validateReleasedQuarantineMetadata(qm, operation, rc, qmTreeHash, guard)
	if qmValidateErr != nil {
		checkQM.Detail = qmValidateErr.Error()
	} else {
		checkQM.Passed = true
	}
	result.Checks = append(result.Checks, checkQM)

	allPassed := true
	for _, check := range result.Checks {
		if !check.Passed {
			allPassed = false
			result.Findings = append(result.Findings, PackageFinalGateFinding{
				FindingID:   "comp-gate-finding-" + check.Name,
				OperationID: operation.OperationID,
				ExtensionID: operation.ExtensionID,
				FindingType: check.Name,
				Severity:    "error",
				Expected:    "passed",
				Actual:      check.Detail,
				DetectedAt:  result.VerifiedAt,
			})
		}
	}
	result.Passed = allPassed
	return result, nil
}

func (r *Runtime) validateRestoredInstallationIdentity(qm PackageQuarantineMetadata, _ PackageWriteGuard) error {
	installation, installErr := r.container.InstallationRepository.GetInstallation(context.Background(), domain.ExtensionID(qm.ExtensionID))
	if installErr != nil {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored installation",
			fmt.Errorf("installation unavailable for identity validation: %w", installErr))
	}

	if string(installation.ExtensionID) != qm.ExtensionID {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored installation",
			fmt.Errorf("installation extension_id mismatch: installation=%s quarantine=%s", string(installation.ExtensionID), qm.ExtensionID))
	}

	if installation.PackageID != qm.ArtifactID {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored installation",
			fmt.Errorf("installation artifact_id mismatch: installation=%s quarantine=%s", installation.PackageID, qm.ArtifactID))
	}

	generationID, _ := installation.Metadata["generationId"].(string)
	if qm.ExpectedGenerationID != "" && generationID != qm.ExpectedGenerationID {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored installation",
			fmt.Errorf("installation generation_id mismatch: installation=%s quarantine=%s", generationID, qm.ExpectedGenerationID))
	}

	if installation.InstallationState != domain.InstallationStateInstalled {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored installation",
			fmt.Errorf("installation state invalid: %s, expected installed", installation.InstallationState))
	}

	if qm.ExpectedVersionID != "" {
		versionRecord, vErr := r.container.PackageRepository.GetPackageVersionByID(context.Background(), qm.ExtensionID, qm.ExpectedVersionID)
		if vErr != nil {
			return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect version record",
				fmt.Errorf("%w: expected version %s not found", ErrPackageRecoveryExpectedVersionNotFound, qm.ExpectedVersionID))
		}

		if versionRecord.VersionID != qm.ExpectedVersionID {
			return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect version record",
				fmt.Errorf("version record version_id mismatch: version=%s quarantine=%s", versionRecord.VersionID, qm.ExpectedVersionID))
		}

		if versionRecord.ArtifactID != qm.ArtifactID {
			return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect version record",
				fmt.Errorf("version artifact_id mismatch: version=%s quarantine=%s", versionRecord.ArtifactID, qm.ArtifactID))
		}

		if qm.ExpectedGenerationID != "" && versionRecord.GenerationID != qm.ExpectedGenerationID {
			return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect version record",
				fmt.Errorf("version generation_id mismatch: version=%s quarantine=%s", versionRecord.GenerationID, qm.ExpectedGenerationID))
		}

		if installation.InstalledVersion.String() != versionRecord.Version {
			return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect version identity",
				fmt.Errorf("installed_version mismatch: installation=%s version=%s", installation.InstalledVersion.String(), versionRecord.Version))
		}
	}

	if r.container.PackageGenerationStore != nil {
		current, currentErr := r.container.PackageGenerationStore.ReadCurrent(qm.ExtensionID)
		if currentErr != nil {
			return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored current",
				fmt.Errorf("current read failed: %w", currentErr))
		}
		if qm.ExpectedGenerationID != "" && current.GenerationID != qm.ExpectedGenerationID {
			return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored current",
				fmt.Errorf("current generation_id mismatch: current=%s quarantine=%s", current.GenerationID, qm.ExpectedGenerationID))
		}
		if current.ArtifactID != qm.ArtifactID {
			return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored current",
				fmt.Errorf("current artifact_id mismatch: current=%s quarantine=%s", current.ArtifactID, qm.ArtifactID))
		}
		if current.TreeHash == "" {
			return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored current",
				fmt.Errorf("current tree_hash is empty after restore"))
		}
	}
	return nil
}

func (r *Runtime) verifyUninstallRestoredIdentity(
	ctx context.Context,
	operation PackageOperationRecord,
	metadata PackageQuarantineMetadata,
) (
	UninstallRestoredIdentityEvidence,
	error,
) {
	var evidence UninstallRestoredIdentityEvidence
	evidence.SchemaVersion = 1
	evidence.OperationID = operation.OperationID
	evidence.ExtensionID = operation.ExtensionID
	evidence.ArtifactID = operation.ArtifactID
	evidence.ExpectedVersionID = metadata.ExpectedVersionID
	evidence.ExpectedGenerationID = metadata.ExpectedGenerationID

	if strings.TrimSpace(metadata.ExpectedVersionID) == "" {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect quarantine metadata",
			fmt.Errorf("%w: expected_version_id is empty", ErrPackageRecoveryExpectedVersionNotFound))
	}

	installation, installErr := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(operation.ExtensionID))
	if installErr != nil {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored installation",
			fmt.Errorf("installation unavailable: %w", installErr))
	}

	if string(installation.ExtensionID) != operation.ExtensionID {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored installation",
			fmt.Errorf("installation extension_id mismatch: installation=%s operation=%s", string(installation.ExtensionID), operation.ExtensionID))
	}

	if installation.PackageID != operation.ArtifactID {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored installation",
			fmt.Errorf("installation artifact_id mismatch: installation=%s operation=%s", installation.PackageID, operation.ArtifactID))
	}

	if installation.InstallationState != domain.InstallationStateInstalled {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored installation",
			fmt.Errorf("installation state is %s, expected installed", installation.InstallationState))
	}

	installedVersion := installation.InstalledVersion.String()
	if installedVersion == "" {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored installation",
			fmt.Errorf("installation installed_version is empty"))
	}
	evidence.InstallationVersion = installedVersion

	installationGenerationID, _ := installation.Metadata["generationId"].(string)
	evidence.InstallationGenerationID = installationGenerationID

	versionRecord, vErr := r.container.PackageRepository.GetPackageVersionByID(ctx, operation.ExtensionID, metadata.ExpectedVersionID)
	if vErr != nil {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect version record",
			fmt.Errorf("%w: expected version %s not found", ErrPackageRecoveryExpectedVersionNotFound, metadata.ExpectedVersionID))
	}

	if versionRecord.VersionID != metadata.ExpectedVersionID {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect version record",
			fmt.Errorf("version record version_id mismatch: version=%s quarantine=%s", versionRecord.VersionID, metadata.ExpectedVersionID))
	}

	if versionRecord.ExtensionID != operation.ExtensionID {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect version record",
			fmt.Errorf("version record extension_id mismatch: %s != %s", versionRecord.ExtensionID, operation.ExtensionID))
	}

	if versionRecord.ArtifactID != operation.ArtifactID {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect version record",
			fmt.Errorf("version record artifact_id mismatch: %s != %s", versionRecord.ArtifactID, operation.ArtifactID))
	}

	if metadata.ExpectedGenerationID != "" && versionRecord.GenerationID != metadata.ExpectedGenerationID {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect version record",
			fmt.Errorf("version record generation_id mismatch: %s != %s", versionRecord.GenerationID, metadata.ExpectedGenerationID))
	}

	evidence.VersionRecordID = versionRecord.VersionID
	evidence.VersionRecordVersion = versionRecord.Version
	evidence.VersionRecordGenerationID = versionRecord.GenerationID
	evidence.RestoredVersion = versionRecord.Version

	if installedVersion != versionRecord.Version {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect version identity",
			fmt.Errorf("installed_version mismatch: installation=%s version=%s", installedVersion, versionRecord.Version))
	}

	current, currentErr := r.container.PackageGenerationStore.ReadCurrent(operation.ExtensionID)
	if currentErr != nil {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored current",
			fmt.Errorf("current read failed: %w", currentErr))
	}

	if current.ExtensionID != operation.ExtensionID {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored current",
			fmt.Errorf("current extension_id mismatch: %s != %s", current.ExtensionID, operation.ExtensionID))
	}

	if current.ArtifactID != operation.ArtifactID {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored current",
			fmt.Errorf("current artifact_id mismatch: %s != %s", current.ArtifactID, operation.ArtifactID))
	}

	if current.Version == "" {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored current",
			fmt.Errorf("current version is empty"))
	}

	if current.Version != versionRecord.Version {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored current",
			fmt.Errorf("current version mismatch: current=%s version=%s", current.Version, versionRecord.Version))
	}

	if current.Version != installedVersion {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored current",
			fmt.Errorf("current version mismatch with installation: current=%s installation=%s", current.Version, installedVersion))
	}

	evidence.CurrentVersion = current.Version
	evidence.CurrentArtifactID = current.ArtifactID
	evidence.CurrentGenerationID = current.GenerationID
	evidence.CurrentTreeHash = current.TreeHash
	evidence.MetadataTreeHash = metadata.TreeHash

	if current.TreeHash == "" {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored current",
			fmt.Errorf("current tree_hash is empty"))
	}

	if metadata.TreeHash == "" {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored current",
			fmt.Errorf("metadata tree_hash is empty"))
	}

	if current.TreeHash != metadata.TreeHash {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect restored current",
			fmt.Errorf("tree_hash mismatch: current=%s metadata=%s", current.TreeHash, metadata.TreeHash))
	}

	actualTreeHash, computeErr := r.container.PackageGenerationStore.ComputeGenerationTreeHash(ctx, operation.ExtensionID, current.GenerationID)
	if computeErr != nil {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect generation",
			fmt.Errorf("compute generation tree_hash failed: %w", computeErr))
	}

	evidence.ActualGenerationTreeHash = actualTreeHash

	if actualTreeHash != metadata.TreeHash {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect generation",
			fmt.Errorf("actual generation tree_hash mismatch: actual=%s metadata=%s", actualTreeHash, metadata.TreeHash))
	}

	if actualTreeHash != current.TreeHash {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect generation",
			fmt.Errorf("actual generation tree_hash mismatch with current: actual=%s current=%s", actualTreeHash, current.TreeHash))
	}

	evidenceForHash := evidence
	evidenceForHash.EvidenceHash = ""
	canonicalJSON, marshalErr := json.Marshal(evidenceForHash)
	if marshalErr != nil {
		return evidence, NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Compute evidence hash",
			fmt.Errorf("marshal evidence for hash: %w", marshalErr))
	}
	evidence.EvidenceHash = fmt.Sprintf("%x", sha256.Sum256(canonicalJSON))

	return evidence, nil
}

func (r *Runtime) validateReleasedQuarantineMetadata(qm PackageQuarantineMetadata, operation PackageOperationRecord, rc UninstallRecoveryContext, actualGenerationTreeHash string, guard PackageWriteGuard) error {
	freshQM, err := r.container.PackageRepository.GetQuarantineMetadataByOperation(context.Background(), operation.OperationID)
	if err != nil {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("quarantine metadata unavailable: %w", err))
	}

	if strings.TrimSpace(freshQM.SnapshotJSON) == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("snapshot_json is missing"))
	}

	if strings.TrimSpace(freshQM.SnapshotHash) == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("snapshot_hash is missing"))
	}

	recomputedSnapshotHash := fmt.Sprintf("%x", sha256.Sum256([]byte(freshQM.SnapshotJSON)))
	if recomputedSnapshotHash != freshQM.SnapshotHash {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("snapshot_hash mismatch: recomputed=%s stored=%s", recomputedSnapshotHash, freshQM.SnapshotHash))
	}

	if actualGenerationTreeHash == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("actual_generation_tree_hash is empty"))
	}

	if freshQM.TreeHash == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("tree_hash is empty"))
	}

	if freshQM.TreeHash != actualGenerationTreeHash {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("tree_hash mismatch: metadata=%s actual=%s", freshQM.TreeHash, actualGenerationTreeHash))
	}

	if freshQM.QuarantineID == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("quarantine_id is empty"))
	}

	if freshQM.OperationID != operation.OperationID {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("operation_id mismatch: %s != %s", freshQM.OperationID, operation.OperationID))
	}

	if freshQM.ExtensionID != operation.ExtensionID {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("extension_id mismatch: %s != %s", freshQM.ExtensionID, operation.ExtensionID))
	}

	if freshQM.ArtifactID != operation.ArtifactID {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("artifact_id mismatch: %s != %s", freshQM.ArtifactID, operation.ArtifactID))
	}

	if freshQM.ExpectedVersionID != qm.ExpectedVersionID {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("expected_version_id mismatch: fresh=%s cached=%s", freshQM.ExpectedVersionID, qm.ExpectedVersionID))
	}

	if freshQM.ExpectedGenerationID != qm.ExpectedGenerationID {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("expected_generation_id mismatch: fresh=%s cached=%s", freshQM.ExpectedGenerationID, qm.ExpectedGenerationID))
	}

	if !r.pathsMatchExactly(freshQM.OriginalCurrentPath, qm.OriginalCurrentPath) {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("original_current_path mismatch: fresh=%s cached=%s", freshQM.OriginalCurrentPath, qm.OriginalCurrentPath))
	}

	if !r.pathsMatchExactly(freshQM.OriginalGenerationPath, qm.OriginalGenerationPath) {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("original_generation_path mismatch: fresh=%s cached=%s", freshQM.OriginalGenerationPath, qm.OriginalGenerationPath))
	}

	if !r.pathsMatchExactly(freshQM.CurrentQuarantinePath, qm.CurrentQuarantinePath) {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("current_quarantine_path mismatch: fresh=%s cached=%s", freshQM.CurrentQuarantinePath, qm.CurrentQuarantinePath))
	}

	if !r.pathsMatchExactly(freshQM.GenerationQuarantinePath, qm.GenerationQuarantinePath) {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("generation_quarantine_path mismatch: fresh=%s cached=%s", freshQM.GenerationQuarantinePath, qm.GenerationQuarantinePath))
	}

	if freshQM.State != "released" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("state is %s, expected released", freshQM.State))
	}

	if freshQM.ReleasedAt == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("released_at is empty"))
	}

	if freshQM.FencingToken <= 0 {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("fencing_token invalid: %d", freshQM.FencingToken))
	}

	if freshQM.FencingToken > guard.FencingToken {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect released quarantine",
			fmt.Errorf("fencing_token exceeds guard: metadata=%d guard=%d", freshQM.FencingToken, guard.FencingToken))
	}

	releaseStep, hasReleaseStep := rc.completed[StepUninstallRecoveryReleaseQuarantineMetadata]
	if !hasReleaseStep || releaseStep.Status != StatusCompleted || releaseStep.ResultJSON == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release quarantine step missing or incomplete"))
	}

	var releaseResult UninstallReleaseQuarantineStepResult
	if err := json.Unmarshal([]byte(releaseStep.ResultJSON), &releaseResult); err != nil {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step result_json is invalid: %w", err))
	}

	if releaseResult.OperationID == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step result missing operation_id"))
	}

	if releaseResult.QuarantineID == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step result missing quarantine_id"))
	}

	if releaseResult.ExtensionID == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step result missing extension_id"))
	}

	if releaseResult.ArtifactID == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step result missing artifact_id"))
	}

	if releaseResult.ReleasedAt == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step result missing released_at"))
	}

	if releaseResult.SnapshotHash == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step result missing snapshot_hash"))
	}

	if releaseResult.GenerationHash == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step result missing generation_hash"))
	}

	if releaseResult.MetadataHash == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step result missing metadata_hash"))
	}

	if releaseResult.OperationID != operation.OperationID {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step operation_id mismatch: %s != %s", releaseResult.OperationID, operation.OperationID))
	}

	if releaseResult.QuarantineID != freshQM.QuarantineID {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step quarantine_id mismatch: step=%s metadata=%s", releaseResult.QuarantineID, freshQM.QuarantineID))
	}

	if releaseResult.ExtensionID != freshQM.ExtensionID {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step extension_id mismatch: %s != %s", releaseResult.ExtensionID, freshQM.ExtensionID))
	}

	if releaseResult.ArtifactID != freshQM.ArtifactID {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step artifact_id mismatch: %s != %s", releaseResult.ArtifactID, freshQM.ArtifactID))
	}

	if releaseResult.ReleasedAt != freshQM.ReleasedAt {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step released_at mismatch: %s != %s", releaseResult.ReleasedAt, freshQM.ReleasedAt))
	}

	if releaseResult.SnapshotHash != freshQM.SnapshotHash {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step snapshot_hash mismatch: %s != %s", releaseResult.SnapshotHash, freshQM.SnapshotHash))
	}

	if releaseResult.GenerationHash != actualGenerationTreeHash {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step generation_hash mismatch: %s != %s", releaseResult.GenerationHash, actualGenerationTreeHash))
	}

	metadataCanonical := packageCanonicalJSON(freshQM)
	expectedMetadataHash := fmt.Sprintf("%x", sha256.Sum256([]byte(metadataCanonical)))
	if releaseResult.MetadataHash != expectedMetadataHash {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step metadata_hash mismatch: stored=%s recomputed=%s", releaseResult.MetadataHash, expectedMetadataHash))
	}

	stepResultHash := fmt.Sprintf("%x", sha256.Sum256([]byte(releaseStep.ResultJSON)))
	if stepResultHash != releaseStep.ResultHash {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect release step",
			fmt.Errorf("release step result_hash mismatch: recomputed=%s stored=%s", stepResultHash, releaseStep.ResultHash))
	}

	return nil
}

func (r *Runtime) finalizeUninstallRecovery(ctx context.Context, operation PackageOperationRecord, completed map[string]PackageOperationStep, guard PackageWriteGuard) error {
	authoritativeOp, _, err := r.container.PackageRepository.GetOperation(ctx, operation.UserID, operation.OperationID)
	if err != nil {
		return r.requirePackageRecovery(ctx, operation, "finalize failed: cannot read authoritative operation", err, guard)
	}
	if string(authoritativeOp.Status) == string(PackageOperationCompleted) {
		if err := r.verifyUninstallFinalizedState(ctx, operation); err != nil {
			return r.requirePackageRecovery(ctx, operation, "finalize verification failed", err, guard)
		}
		existing := PackageOperationStep{
			StepID:      operation.OperationID + ":" + StepUninstallRecoveryFinalize,
			OperationID: operation.OperationID,
			StepName:    StepUninstallRecoveryFinalize,
			Status:      StatusCompleted,
		}
		completed[StepUninstallRecoveryFinalize] = existing
		return nil
	}
	if existing, ok := completed[StepUninstallRecoveryFinalize]; ok && existing.Status == StatusCompleted {
		if err := r.verifyUninstallFinalizedState(ctx, operation); err != nil {
			return r.requirePackageRecovery(ctx, operation, "finalize verification failed", err, guard)
		}
		return nil
	}
	gateStep, hasGate := completed[StepUninstallRecoveryFinalGate]
	if !hasGate || gateStep.Status != StatusCompleted || gateStep.ResultJSON == "" {
		return r.requirePackageRecovery(ctx, operation, "finalize blocked: final gate step missing or incomplete", nil, guard)
	}
	if err := r.container.PackageRepository.TransitionOperation(ctx,
		operation.OperationID,
		[]PackageOperationStatus{PackageOperationInProgress},
		PackageOperationFinalizing,
		PackageOperationTransition{CurrentStep: StepUninstallRecoveryFinalize},
		guard); err != nil {
		return r.requirePackageRecovery(ctx, operation, "uninstall recovery transition to finalizing failed", err, guard)
	}
	finalGateResultHash := fmt.Sprintf("%x", sha256.Sum256([]byte(gateStep.ResultJSON)))
	finalizeResultJSON := fmt.Sprintf(`{"finalized":true,"atomic":true,"operationId":%q,"extensionId":%q,"fencingToken":%d,"finalGateStep":%q,"finalGateResultHash":%q}`,
		operation.OperationID, operation.ExtensionID, guard.FencingToken, StepUninstallRecoveryFinalGate, finalGateResultHash)
	finalizeResultHash := fmt.Sprintf("%x", sha256.Sum256([]byte(finalizeResultJSON)))
	finalizeStep := PackageOperationStep{
		StepID:       operation.OperationID + ":" + StepUninstallRecoveryFinalize,
		OperationID:  operation.OperationID,
		StepName:     StepUninstallRecoveryFinalize,
		StepOrder:    9999,
		Status:       StatusCompleted,
		ResultJSON:   finalizeResultJSON,
		ResultHash:   finalizeResultHash,
		AttemptCount: 1,
		InputHash:    finalGateResultHash,
		StartedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		CompletedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := r.container.PackageRepository.FinalizeOperationAndReleaseLeaseTxWithStep(ctx, operation.OperationID, operation.ExtensionID, guard.FencingToken, StepUninstallRecoveryFinalize, finalizeStep); err != nil {
		return r.requirePackageRecovery(ctx, operation, "uninstall recovery finalize failed", err, guard)
	}
	completed[StepUninstallRecoveryFinalize] = finalizeStep
	return nil
}

func (r *Runtime) verifyUninstallFinalizedState(ctx context.Context, operation PackageOperationRecord) error {
	op, _, err := r.container.PackageRepository.GetOperation(ctx, operation.UserID, operation.OperationID)
	if err != nil {
		return fmt.Errorf("verify finalized operation: %w", err)
	}
	if string(op.Status) != string(PackageOperationCompleted) {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalized operation",
			fmt.Errorf("operation not completed after finalize: status=%s", op.Status))
	}
	if op.CompletedAt == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalized operation",
			fmt.Errorf("operation completed but completed_at is empty"))
	}
	steps, listErr := r.container.PackageRepository.ListOperationSteps(ctx, operation.OperationID)
	if listErr != nil {
		return fmt.Errorf("list steps for finalize verification: %w", listErr)
	}
	var finalizeStepPtr *PackageOperationStep
	var finalGateStepRaw *PackageOperationStep
	for _, step := range steps {
		if step.StepName == StepUninstallRecoveryFinalize && step.Status == StatusCompleted && step.ResultJSON != "" {
			expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(step.ResultJSON)))
			if expectedHash != step.ResultHash {
				return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalized operation",
					fmt.Errorf("finalize step result_hash mismatch"))
			}
			s := step
			finalizeStepPtr = &s
		}
		if step.StepName == StepUninstallRecoveryFinalGate && step.Status == StatusCompleted && step.ResultJSON != "" {
			gateExpectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(step.ResultJSON)))
			if gateExpectedHash != step.ResultHash {
				return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect final gate step",
					fmt.Errorf("final gate step result_hash mismatch"))
			}
			s := step
			finalGateStepRaw = &s
		}
	}
	if finalizeStepPtr == nil {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalized operation",
			fmt.Errorf("operation %s completed but finalize step missing or invalid", operation.OperationID))
	}
	if finalGateStepRaw == nil {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalized operation",
			fmt.Errorf("operation %s completed but final gate step missing or invalid", operation.OperationID))
	}

	var gateResult PackageFinalGateResult
	if unmarshalErr := json.Unmarshal([]byte(finalGateStepRaw.ResultJSON), &gateResult); unmarshalErr != nil {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalGateEvidenceInvalid, 409, false, true, "Inspect final gate step",
			fmt.Errorf("final gate result corrupted: %w", unmarshalErr))
	}
	if !gateResult.Passed {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalized operation",
			fmt.Errorf("operation %s completed but final gate did not pass", operation.OperationID))
	}

	var finalizeResult UninstallFinalizeStepResult
	if unmarshalFzErr := json.Unmarshal([]byte(finalizeStepPtr.ResultJSON), &finalizeResult); unmarshalFzErr != nil {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalize step",
			fmt.Errorf("finalize result corrupted: %w", unmarshalFzErr))
	}
	if finalizeResult.OperationID != operation.OperationID {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalize step",
			fmt.Errorf("finalize result operation_id mismatch: %s != %s", finalizeResult.OperationID, operation.OperationID))
	}
	if finalizeResult.FinalGateStepID != StepUninstallRecoveryFinalGate {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalize step",
			fmt.Errorf("finalize result final_gate_step_id mismatch: %s", finalizeResult.FinalGateStepID))
	}
	actualGateResultHash := fmt.Sprintf("%x", sha256.Sum256([]byte(finalGateStepRaw.ResultJSON)))
	if finalizeResult.FinalGateResultHash != actualGateResultHash {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalize step",
			fmt.Errorf("finalize result final_gate_result_hash mismatch: stored=%s actual=%s", finalizeResult.FinalGateResultHash, actualGateResultHash))
	}

	_, leaseErr := r.container.PackageRepository.getExtensionLease(ctx, operation.ExtensionID)
	if leaseErr == nil {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalized operation",
			fmt.Errorf("operation %s completed but lease still exists", operation.OperationID))
	}
	if !IsPackageOperationError(leaseErr, OperationErrNotFound) {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalized operation",
			fmt.Errorf("lease verification failed: %w", leaseErr))
	}

	if operation.ArtifactID != "" {
		hasOpRef, opRefErr := r.container.PackageRepository.HasArtifactReference(ctx, operation.ArtifactID, ArtifactReferenceOperation, operation.OperationID)
		if opRefErr != nil {
			return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalized operation",
				fmt.Errorf("operation reference check failed: %w", opRefErr))
		}
		if hasOpRef {
			return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalized operation",
				fmt.Errorf("operation %s completed but operation artifact reference remains", operation.OperationID))
		}
	}

	if operation.PreviewSessionID == "" {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalized operation",
			fmt.Errorf("operation %s completed but preview_session_id is empty", operation.OperationID))
	}
	if operation.ArtifactID != "" {
		hasPreviewRef, previewRefErr := r.container.PackageRepository.HasArtifactReference(ctx, operation.ArtifactID, ArtifactReferencePreview, operation.PreviewSessionID)
		if previewRefErr != nil {
			return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalized operation",
				fmt.Errorf("preview reference check failed: %w", previewRefErr))
		}
		if hasPreviewRef {
			return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalized operation",
				fmt.Errorf("operation %s completed but preview artifact reference remains", operation.OperationID))
		}
	}

	finalizeResultHash := fmt.Sprintf("%x", sha256.Sum256([]byte(finalizeStepPtr.ResultJSON)))
	if finalizeResultHash != finalizeStepPtr.ResultHash {
		return NewPackageErrorWithRecovery(PackageErrCodeFinalizationEvidenceMissing, 409, false, true, "Inspect finalized operation",
			fmt.Errorf("finalize step result_hash inconsistent"))
	}
	return nil
}

type UninstallFinalizeStepResult struct {
	OperationID         string `json:"operationId"`
	FinalGateStepID     string `json:"finalGateStepId"`
	FinalGateResultHash string `json:"finalGateResultHash"`
}

func isLeaseRelatedError(err error) bool {
	if err == nil {
		return false
	}
	for _, code := range []string{
		PackageErrCodeLeaseFenced,
		PackageErrCodeLeaseLost,
		OperationErrProofUnavailable,
		OperationErrLeaseProofMismatch,
	} {
		if IsPackageOperationError(err, code) {
			return true
		}
	}
	return false
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
