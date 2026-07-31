package kernel

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (r *Runtime) recordPackageVersionAfterOperation(ctx context.Context, operationID, operationType, extensionID, version, artifactID, installedPath, installedTreeHash, archiveHash, manifestHash, contentTreeHash, generationID string, guard PackageWriteGuard) error {
	if r.container == nil || r.container.PackageRepository == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	candidate := PackageVersionRecord{
		VersionID:          "version-" + uuid.NewString(),
		ExtensionID:        extensionID,
		Version:            version,
		ArtifactID:         artifactID,
		InstallOperationID: operationID,
		InstalledAt:        now,
		IsActive:           true,
		VersionState:       string(PackageVersionStateCurrent),
		InstalledPath:      installedPath,
		InstalledTreeHash:  installedTreeHash,
		ArchiveHash:        archiveHash,
		ManifestHash:       manifestHash,
		ContentTreeHash:    contentTreeHash,
		GenerationID:       generationID,
	}
	db := r.container.PackageRepository.DB()
	if db == nil {
		return fmt.Errorf("kernel: package version database unavailable")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("kernel: begin package version transaction: %w", err)
	}
	defer tx.Rollback()
	upsertResult, err := r.container.PackageRepository.UpsertPackageVersionTx(ctx, tx, guard, candidate)
	if err != nil {
		return fmt.Errorf("kernel: upsert package version record: %w", err)
	}
	actualVersionID := upsertResult.Record.VersionID
	if err := r.container.PackageRepository.ActivatePackageVersionTx(ctx, tx, guard, extensionID, actualVersionID, generationID); err != nil {
		return fmt.Errorf("kernel: activate package version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("kernel: commit package version transaction: %w", err)
	}
	return nil
}

func (r *Runtime) deactivatePackageVersionAfterUninstall(ctx context.Context, extensionID, version, operationID string, guard PackageWriteGuard) error {
	if r.container == nil || r.container.PackageRepository == nil {
		return nil
	}
	db := r.container.PackageRepository.DB()
	if db == nil {
		return fmt.Errorf("kernel: package version database unavailable")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("kernel: begin deactivate version transaction: %w", err)
	}
	defer tx.Rollback()
	if err := r.container.PackageRepository.DeactivatePackageVersionTx(ctx, tx, guard, extensionID, version, operationID); err != nil {
		return fmt.Errorf("kernel: deactivate package version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("kernel: commit deactivate version transaction: %w", err)
	}
	return nil
}

func (r *Runtime) runPackageFinalGate(ctx context.Context, operationID string, guard PackageWriteGuard) error {
	_, err := r.verifyPackageFinalGateWithGuard(ctx, operationID, guard)
	return err
}

func (r *Runtime) FinalizePackageOperation(ctx context.Context, operationID, extensionID string, leaseGuard *PackageLeaseGuard, guard PackageWriteGuard) error {
	if leaseGuard != nil {
		if err := leaseGuard.AssertAlive(ctx); err != nil {
			_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, string(PackageOperationRequiresRecovery), "finalize_assert_lease", PackageErrCodeLeaseLost, err.Error(), false, guard)
			return fmt.Errorf("kernel: lease assert failed during finalization: %w", err)
		}
	}
	if err := r.container.PackageRepository.SetOperation(ctx, operationID, string(PackageOperationFinalizing), "finalizing", "", "", false, guard); err != nil {
		return fmt.Errorf("kernel: failed to transition operation to finalizing: %w", err)
	}
	if err := r.runPackageFinalGate(ctx, operationID, guard); err != nil {
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, string(PackageOperationRequiresRecovery), "final_gate", PackageErrCodeFinalGateFailed, err.Error(), false, guard)
		return fmt.Errorf("kernel: final gate failed during finalization: %w", err)
	}
	if err := r.container.PackageRepository.FinalizeOperationAndReleaseLeaseTx(ctx, operationID, extensionID, guard.FencingToken); err != nil {
		if IsPackageOperationError(err, PackageErrCodeLeaseFenced) || IsPackageOperationError(err, OperationErrTransitionConflict) {
			_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, string(PackageOperationRequiresRecovery), "finalize_lease_release", PackageErrCodeLeaseFenced, err.Error(), false, guard)
			return fmt.Errorf("kernel: lease conflict during finalization: %w", err)
		}
		_ = r.container.PackageRepository.SetOperation(context.Background(), operationID, string(PackageOperationReleasePending), "finalize_lease_release", "PACKAGE_LEASE_RELEASE_FAILED", err.Error(), false, guard)
		return fmt.Errorf("kernel: lease release failed during finalization: %w", err)
	}
	if leaseGuard != nil {
		leaseGuard.MarkLeaseReleased()
	}
	return nil
}
