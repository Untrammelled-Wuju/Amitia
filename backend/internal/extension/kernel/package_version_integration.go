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

func (r *Runtime) runPackageFinalGate(ctx context.Context, operationID string) error {
	_, err := r.VerifyPackageFinalGate(ctx, operationID)
	return err
}
