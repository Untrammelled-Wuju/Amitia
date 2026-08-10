package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/installation/binding"
	"github.com/u-ai/backend/internal/desktoppet/installation/desired"
	"github.com/u-ai/backend/internal/desktoppet/installation/device"
	"github.com/u-ai/backend/internal/desktoppet/installation/journal"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	"github.com/u-ai/backend/internal/desktoppet/installation/projection"
	"github.com/u-ai/backend/internal/desktoppet/release"
	releasecore "github.com/u-ai/backend/internal/desktoppet/release"
	desktoppetsecurity "github.com/u-ai/backend/internal/desktoppet/security"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	runtimev2 "github.com/u-ai/backend/internal/desktoppet/runtime/protocol/v2"
)

type coordinatorReleaseValidator struct {
	releases release.ReleaseRepository
}

func (p *coordinatorReleaseValidator) ValidateRelease(ctx context.Context, userID, releaseID string) (*coordinator.ReleaseValidationResult, error) {
	item, err := p.releases.GetRelease(releaseID)
	if err != nil {
		return nil, err
	}
	if item.OwnerUserID != "" && item.OwnerUserID != userID {
		return &coordinator.ReleaseValidationResult{
			ReleaseID:    item.ID,
			IsInstallable: false,
			ErrorMessage: "release does not belong to user",
		}, nil
	}
	return &coordinator.ReleaseValidationResult{
		ReleaseID:        item.ID,
		IsInstallable:    releasecore.IsInstallable(item.Lifecycle, item.IntegrityStatus, item.CompatibilityStatus),
		HasPublishedCopy: item.StorageKey != "",
		ManifestValid:    item.ManifestHash != "" && item.ContentRootHash != "",
		PublishedPathKey: item.StorageKey,
	}, nil
}

type coordinatorReleaseStager struct {
	registry *desktoppetsecurity.PathRootRegistry
	releases release.ReleaseRepository
}

func (p *coordinatorReleaseStager) PrepareStagingCopy(ctx context.Context, releaseID, installationID string) (string, error) {
	item, err := p.releases.GetRelease(releaseID)
	if err != nil {
		return "", fmt.Errorf("get release: %w", err)
	}
	if item.StorageKey == "" {
		return "", fmt.Errorf("release has no published copy")
	}
	publishedRoot, err := p.registry.Root(desktoppetsecurity.RootReleasePublished)
	if err != nil {
		return "", fmt.Errorf("resolve published root: %w", err)
	}
	srcPath := filepath.Join(publishedRoot, item.StorageKey)
	info, err := os.Lstat(srcPath)
	if err != nil {
		return "", fmt.Errorf("resolve source: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("source path is a symlink")
	}
	stagingRoot, err := p.registry.Root(desktoppetsecurity.RootInstallations)
	if err != nil {
		return "", fmt.Errorf("resolve installations root: %w", err)
	}
	stagingKey := installationID + "_" + uuid.New().String()[:8]
	dstPath := filepath.Join(stagingRoot, stagingKey)
	if err := safeCopy(srcPath, dstPath, info); err != nil {
		return "", fmt.Errorf("copy staging: %w", err)
	}
	return stagingKey, nil
}

func (p *coordinatorReleaseStager) VerifyStagingCopy(ctx context.Context, releaseID, installationID, stagingPathKey string) error {
	files, err := p.releases.GetReleaseFiles(releaseID)
	if err != nil {
		return fmt.Errorf("get release files: %w", err)
	}
	stagingRoot, err := p.registry.Root(desktoppetsecurity.RootInstallations)
	if err != nil {
		return fmt.Errorf("resolve installations root: %w", err)
	}
	for _, f := range files {
		fullPath := filepath.Join(stagingRoot, stagingPathKey, f.Path)
		info, err := os.Lstat(fullPath)
		if err != nil {
			return fmt.Errorf("verify file %s: %w", f.Path, err)
		}
		if info.Size() != f.Bytes {
			return fmt.Errorf("verify file %s: size mismatch (got %d, want %d)", f.Path, info.Size(), f.Bytes)
		}
		if f.SHA256 != "" {
			actualHash, err := hashFile(fullPath)
			if err != nil {
				return fmt.Errorf("hash file %s: %w", f.Path, err)
			}
			if actualHash != f.SHA256 {
				return fmt.Errorf("verify file %s: hash mismatch", f.Path)
			}
		}
	}
	return nil
}

func safeCopy(src, dst string, info os.FileInfo) error {
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst, info.Mode().Perm())
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		info, err := os.Lstat(srcPath)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink detected during copy: %s", srcPath)
		}
		if err := safeCopy(srcPath, dstPath, info); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type coordinatorRuntimePublisher struct {
	facade *runtimev2.RuntimeFacade
}

func (p *coordinatorRuntimePublisher) PublishDesiredState(ctx context.Context, deviceCtx device.DeviceContext, snapshot *coordinator.DesiredStateSnapshot) error {
	if p.facade == nil {
		return fmt.Errorf("runtime v2 unavailable")
	}
	if !deviceCtx.IsValid() {
		return fmt.Errorf("invalid device context")
	}
	seq, err := p.facade.Commands().AllocateDeviceSequence(nil, deviceCtx.UserID, deviceCtx.DeviceID, time.Now())
	if err != nil {
		return fmt.Errorf("allocate sequence: %w", err)
	}
	payload := runtimev2.SyncDesiredStatePayload{
		DesiredRevision:        snapshot.DesiredRevision,
		DesiredHash:            snapshot.DesiredHash,
		EnsureAbsent:           snapshot.EnsureAbsent,
		InstallationID:         snapshot.InstallationID,
		PetID:                  snapshot.PetID,
		CharacterID:            "",
		ReleaseID:              snapshot.ReleaseID,
		RuntimeContractVersion: runtimev2.CurrentSchemaVersion,
		DefaultActionKey:       snapshot.DefaultActionKey,
	}
	commandType := runtimev2.CommandTypeSyncDesiredState
	if snapshot.EnsureAbsent {
		commandType = runtimev2.CommandTypeEnsureAbsent
	}
	_, err = p.facade.Commands().CreateDurableCommand(
		deviceCtx.UserID,
		deviceCtx.DeviceID,
		string(commandType),
		fmt.Sprintf("desired:%s:%d", deviceCtx.DeviceID, snapshot.DesiredRevision),
		fmt.Sprintf("desired:%s", deviceCtx.DeviceID),
		seq,
		payload,
	)
	if err != nil {
		if err == runtimev2.ErrCommandDuplication {
			return nil
		}
		return fmt.Errorf("create durable command: %w", err)
	}
	return nil
}

type coordinatorProjectionService struct {
	installRepo installation.Repository
}

func (s *coordinatorProjectionService) UpdateProjection(ctx context.Context, userID, deviceID string, updateFn func(*coordinator.Projection) error) error {
	proj, err := getRuntimeProjectionTx(s.installRepo.DB().WithContext(ctx), userID, deviceID)
	if err != nil {
		proj = &projection.InstallationRuntimeProjection{
			ID:                    uuid.New().String(),
			UserID:                userID,
			DeviceID:              deviceID,
			AppliedDesiredRevision: 0,
			RuntimeSyncState:      projection.SyncStatePending,
			CreatedAt:             time.Now().Format("2006-01-02 15:04:05"),
			UpdatedAt:             time.Now().Format("2006-01-02 15:04:05"),
		}
	}
	grid := &coordinator.Projection{
		InstallationID:         proj.InstallationID,
		PetID:                  proj.PetID,
		AppliedDesiredRevision: proj.AppliedDesiredRevision,
		ActualReleaseID:        proj.ActualReleaseID,
		RuntimeSyncState:       proj.RuntimeSyncState,
	}
	if err := updateFn(grid); err != nil {
		return err
	}
	proj.InstallationID = grid.InstallationID
	proj.PetID = grid.PetID
	proj.AppliedDesiredRevision = grid.AppliedDesiredRevision
	proj.ActualReleaseID = grid.ActualReleaseID
	proj.RuntimeSyncState = grid.RuntimeSyncState
	proj.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	return upsertRuntimeProjectionTx(s.installRepo.DB().WithContext(ctx), proj)
}

func (s *coordinatorProjectionService) HandleRuntimeHeartbeat(ctx context.Context, userID, deviceID, runtimeID string, heartbeat *coordinator.RuntimeHeartbeat) error {
	proj, err := getRuntimeProjectionTx(s.installRepo.DB().WithContext(ctx), userID, deviceID)
	if err != nil {
		proj = &projection.InstallationRuntimeProjection{
			ID:               uuid.New().String(),
			UserID:           userID,
			DeviceID:         deviceID,
			RuntimeID:        runtimeID,
			RuntimeSyncState: projection.SyncStatePending,
			CreatedAt:        time.Now().Format("2006-01-02 15:04:05"),
			UpdatedAt:        time.Now().Format("2006-01-02 15:04:05"),
		}
	}
	proj.RuntimeID = runtimeID
	proj.AppliedDesiredRevision = heartbeat.AppliedDesiredRevision
	proj.AppliedSettingsRevision = heartbeat.AppliedSettingsRevision
	proj.ActualReleaseID = heartbeat.ActualReleaseID
	proj.ActualVisible = heartbeat.ActualVisible
	proj.ActualActionKey = heartbeat.ActualActionKey
	proj.ActualHealth = heartbeat.ActualHealth
	if heartbeat.AppliedDesiredRevision > 0 && heartbeat.ActualHealth != "failed" {
		proj.RuntimeSyncState = projection.SyncStateApplied
	} else if heartbeat.ActualHealth == "failed" {
		proj.RuntimeSyncState = projection.SyncStateFailed
	}
	proj.LastHeartbeatAt = heartbeat.Timestamp
	proj.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	return upsertRuntimeProjectionTx(s.installRepo.DB().WithContext(ctx), proj)
}

func (s *coordinatorProjectionService) HandleCommandResult(ctx context.Context, userID, deviceID string, result *coordinator.CommandResult) error {
	proj, err := getRuntimeProjectionTx(s.installRepo.DB().WithContext(ctx), userID, deviceID)
	if err != nil {
		proj = &projection.InstallationRuntimeProjection{
			ID:               uuid.New().String(),
			UserID:           userID,
			DeviceID:         deviceID,
			RuntimeSyncState: projection.SyncStatePending,
			CreatedAt:        time.Now().Format("2006-01-02 15:04:05"),
			UpdatedAt:        time.Now().Format("2006-01-02 15:04:05"),
		}
	}
	if result.Success {
		proj.AppliedDesiredRevision = result.AppliedRevision
		proj.RuntimeSyncState = projection.SyncStateApplied
	} else {
		proj.RuntimeSyncState = projection.SyncStateFailed
	}
	proj.LastAppliedAt = result.Timestamp
	proj.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	return upsertRuntimeProjectionTx(s.installRepo.DB().WithContext(ctx), proj)
}

func getRuntimeProjectionTx(db *gorm.DB, userID, deviceID string) (*projection.InstallationRuntimeProjection, error) {
	var p projection.InstallationRuntimeProjection
	err := db.Where("user_id = ? AND device_id = ?", userID, deviceID).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func upsertRuntimeProjectionTx(db *gorm.DB, p *projection.InstallationRuntimeProjection) error {
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "device_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"runtime_id", "installation_id", "pet_id", "applied_desired_revision",
			"applied_settings_revision", "actual_release_id", "actual_visible",
			"actual_action_key", "actual_health", "runtime_sync_state",
			"last_applied_at", "last_heartbeat_at", "updated_at",
		}),
	}).Create(p).Error
}

type coordinatorRepoAdapter struct {
	installRepo installation.Repository
}

func newCoordinatorRepoAdapter(installRepo installation.Repository) *coordinatorRepoAdapter {
	return &coordinatorRepoAdapter{installRepo: installRepo}
}

func (a *coordinatorRepoAdapter) CreateOperation(ctx context.Context, op *operation.InstallationOperation) error {
	if op == nil {
		return fmt.Errorf("coordinator: nil operation")
	}
	return a.installRepo.DB().WithContext(ctx).Create(op).Error
}

func (a *coordinatorRepoAdapter) GetOperation(ctx context.Context, userID, deviceID, operationID string) (*operation.InstallationOperation, error) {
	var op operation.InstallationOperation
	err := a.installRepo.DB().WithContext(ctx).
		Where("id = ? AND user_id = ? AND device_id = ?", operationID, userID, deviceID).
		First(&op).Error
	if err != nil {
		return nil, err
	}
	return &op, nil
}

func (a *coordinatorRepoAdapter) UpdateOperation(ctx context.Context, op *operation.InstallationOperation) error {
	if op == nil {
		return fmt.Errorf("coordinator: nil operation")
	}
	op.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	return a.installRepo.DB().WithContext(ctx).Save(op).Error
}

func (a *coordinatorRepoAdapter) FindOperationByIdempotencyKey(ctx context.Context, userID, deviceID, key, opType string) (*operation.InstallationOperation, error) {
	var op operation.InstallationOperation
	err := a.installRepo.DB().WithContext(ctx).
		Where("idempotency_key = ? AND operation_type = ? AND user_id = ? AND device_id = ?", key, opType, userID, deviceID).
		First(&op).Error
	if err != nil {
		return nil, err
	}
	return &op, nil
}

func (a *coordinatorRepoAdapter) GetInstallation(ctx context.Context, userID, deviceID, installationID string) (*coordinator.InstallationRecord, error) {
	var inst installation.Installation
	err := a.installRepo.DB().WithContext(ctx).
		Where("id = ? AND user_id = ? AND device_id = ?", installationID, userID, deviceID).
		First(&inst).Error
	if err != nil {
		return nil, err
	}
	return &coordinator.InstallationRecord{
		ID:                inst.ID,
		UserID:            inst.UserID,
		DeviceID:          inst.DeviceID,
		PetID:             inst.PetID,
		ReleaseID:         inst.CurrentReleaseID,
		Status:            inst.Status,
		Enabled:           inst.IsEnabled(),
		InstallStorageKey: inst.InstallStorageKey,
		DefaultActionKey:  inst.DefaultActionKey,
	}, nil
}

func (a *coordinatorRepoAdapter) CreateInstallationAndDesiredState(ctx context.Context, op *operation.InstallationOperation, inst *coordinator.InstallationRecord, desired *coordinator.DesiredStateSnapshot, stagingPathKey string) (int64, error) {
	db := a.installRepo.DB()
	var desiredRevision int64
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		instModel := &installation.Installation{
			ID:                   inst.ID,
			UserID:               inst.UserID,
			DeviceID:             inst.DeviceID,
			PetID:                inst.PetID,
			Status:               installation.StatusInstalled,
			IsActive:             1,
			InstallPath:          stagingPathKey,
			DefaultActionKey:     desired.DefaultActionKey,
			CurrentReleaseID:     desired.ReleaseID,
			LifecycleState:       installation.LifecycleInstalled,
			IntegrityStatus:      installation.IntegrityVerified,
			DesiredState:         installation.DesiredEnabled,
			RuntimeSyncState:     installation.SyncPending,
			InstallStorageKey:    stagingPathKey,
			InstalledAt:          time.Now().Format("2006-01-02 15:04:05"),
			CreatedAt:            time.Now().Format("2006-01-02 15:04:05"),
			UpdatedAt:            time.Now().Format("2006-01-02 15:04:05"),
		}
		if err := tx.Create(instModel).Error; err != nil {
			return fmt.Errorf("create installation: %w", err)
		}
		var counter desired.DeviceDesiredRevisionCounter
		err := tx.Where("user_id = ? AND device_id = ?", inst.UserID, inst.DeviceID).First(&counter).Error
		if err != nil {
			counter = desired.DeviceDesiredRevisionCounter{
				UserID:          inst.UserID,
				DeviceID:        inst.DeviceID,
				CurrentRevision: 0,
				UpdatedAt:       time.Now().Format("2006-01-02 15:04:05"),
			}
			if err := tx.Create(&counter).Error; err != nil {
				return fmt.Errorf("create revision counter: %w", err)
			}
		}
		newRev := counter.CurrentRevision + 1
		result := tx.Model(&desired.DeviceDesiredRevisionCounter{}).
			Where("user_id = ? AND device_id = ? AND current_revision = ?", inst.UserID, inst.DeviceID, counter.CurrentRevision).
			Updates(map[string]interface{}{
				"current_revision": newRev,
				"updated_at":       time.Now().Format("2006-01-02 15:04:05"),
			})
		if result.Error != nil {
			return fmt.Errorf("allocate revision: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("revision allocation conflict")
		}
		desiredRevision = newRev
		desiredModel := &desired.RuntimeDesiredState{
			ID:                  uuid.New().String(),
			UserID:              desired.UserID,
			DeviceID:            desired.DeviceID,
			RuntimeID:           desired.RuntimeID,
			InstallationID:      desired.InstallationID,
			PetID:               desired.PetID,
			ReleaseID:           desired.ReleaseID,
			DesiredEnabled:      !desired.EnsureAbsent,
			DesiredVisible:      !desired.EnsureAbsent,
			DesiredActionKey:    desired.DefaultActionKey,
			DesiredRevision:     desiredRevision,
			DesiredHash:         desired.DesiredHash,
			OperationID:         op.ID,
			CreatedAt:           time.Now().Format("2006-01-02 15:04:05"),
			UpdatedAt:           time.Now().Format("2006-01-02 15:04:05"),
		}
		if err := tx.Create(desiredModel).Error; err != nil {
			return fmt.Errorf("create desired state: %w", err)
		}
		bindingEntry := &binding.DeviceActiveInstallationBinding{
			UserID:         inst.UserID,
			DeviceID:       inst.DeviceID,
			InstallationID: inst.ID,
			ReleaseID:      desired.ReleaseID,
			CreatedAt:      time.Now().Format("2006-01-02 15:04:05"),
			UpdatedAt:      time.Now().Format("2006-01-02 15:04:05"),
		}
		if err := tx.Create(bindingEntry).Error; err != nil {
			return fmt.Errorf("create binding: %w", err)
		}
		jrnl := &journal.InstallationCommitJournal{
			ID:             uuid.New().String(),
			OperationID:    op.ID,
			InstallationID: inst.ID,
			Stage:          installation.JournalStateOperationCreatedAt,
			CreatedAt:      time.Now().Format("2006-01-02 15:04:05"),
			UpdatedAt:      time.Now().Format("2006-01-02 15:04:05"),
		}
		if err := tx.Create(jrnl).Error; err != nil {
			return fmt.Errorf("create commit journal: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return desiredRevision, nil
}

func (a *coordinatorRepoAdapter) UpdateDesiredEnabled(ctx context.Context, op *operation.InstallationOperation, installationID string, enabled bool) (int64, error) {
	db := a.installRepo.DB()
	var desiredRevision int64
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var counter desired.DeviceDesiredRevisionCounter
		err := tx.Where("user_id = ? AND device_id = ?", op.UserID, op.DeviceID).First(&counter).Error
		if err != nil {
			counter = desired.DeviceDesiredRevisionCounter{
				UserID:          op.UserID,
				DeviceID:        op.DeviceID,
				CurrentRevision: 0,
				UpdatedAt:       time.Now().Format("2006-01-02 15:04:05"),
			}
			if err := tx.Create(&counter).Error; err != nil {
				return fmt.Errorf("create revision counter: %w", err)
			}
		}
		newRev := counter.CurrentRevision + 1
		result := tx.Model(&desired.DeviceDesiredRevisionCounter{}).
			Where("user_id = ? AND device_id = ? AND current_revision = ?", op.UserID, op.DeviceID, counter.CurrentRevision).
			Updates(map[string]interface{}{
				"current_revision": newRev,
				"updated_at":       time.Now().Format("2006-01-02 15:04:05"),
			})
		if result.Error != nil {
			return fmt.Errorf("allocate revision: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("revision allocation conflict")
		}
		desiredRevision = newRev
		desiredState := &desired.RuntimeDesiredState{
			ID:                    uuid.New().String(),
			UserID:                op.UserID,
			DeviceID:              op.DeviceID,
			RuntimeID:             op.RuntimeID,
			InstallationID:        installationID,
			DesiredEnabled:        enabled,
			DesiredVisible:        enabled,
			DesiredRevision:       desiredRevision,
			OperationID:           op.ID,
			CreatedAt:             time.Now().Format("2006-01-02 15:04:05"),
			UpdatedAt:             time.Now().Format("2006-01-02 15:04:05"),
		}
		if err := tx.Create(desiredState).Error; err != nil {
			return fmt.Errorf("create desired state: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return desiredRevision, nil
}

func (a *coordinatorRepoAdapter) SwitchRelease(ctx context.Context, op *operation.InstallationOperation, installationID, targetReleaseID, stagingPathKey string) (int64, error) {
	db := a.installRepo.DB()
	var desiredRevision int64
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var counter desired.DeviceDesiredRevisionCounter
		err := tx.Where("user_id = ? AND device_id = ?", op.UserID, op.DeviceID).First(&counter).Error
		if err != nil {
			counter = desired.DeviceDesiredRevisionCounter{
				UserID:          op.UserID,
				DeviceID:        op.DeviceID,
				CurrentRevision: 0,
				UpdatedAt:       time.Now().Format("2006-01-02 15:04:05"),
			}
			if err := tx.Create(&counter).Error; err != nil {
				return fmt.Errorf("create revision counter: %w", err)
			}
		}
		newRev := counter.CurrentRevision + 1
		result := tx.Model(&desired.DeviceDesiredRevisionCounter{}).
			Where("user_id = ? AND device_id = ? AND current_revision = ?", op.UserID, op.DeviceID, counter.CurrentRevision).
			Updates(map[string]interface{}{
				"current_revision": newRev,
				"updated_at":       time.Now().Format("2006-01-02 15:04:05"),
			})
		if result.Error != nil {
			return fmt.Errorf("allocate revision: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("revision allocation conflict")
		}
		desiredRevision = newRev
		desiredState := &desired.RuntimeDesiredState{
			ID:                    uuid.New().String(),
			UserID:                op.UserID,
			DeviceID:              op.DeviceID,
			RuntimeID:             op.RuntimeID,
			InstallationID:        installationID,
			ReleaseID:             targetReleaseID,
			DesiredEnabled:        true,
			DesiredVisible:        true,
			DesiredRevision:       desiredRevision,
			OperationID:           op.ID,
			CreatedAt:             time.Now().Format("2006-01-02 15:04:05"),
			UpdatedAt:             time.Now().Format("2006-01-02 15:04:05"),
		}
		if err := tx.Create(desiredState).Error; err != nil {
			return fmt.Errorf("create desired state: %w", err)
		}
		if err := tx.Model(&installation.Installation{}).Where("id = ?", installationID).Updates(map[string]interface{}{
			"current_release_id": targetReleaseID,
			"install_path":       stagingPathKey,
			"install_storage_key": stagingPathKey,
			"updated_at":         time.Now().Format("2006-01-02 15:04:05"),
		}).Error; err != nil {
			return fmt.Errorf("update installation release: %w", err)
		}
		jrnl := &journal.InstallationSwitchJournal{
			ID:             uuid.New().String(),
			OperationID:    op.ID,
			InstallationID: installationID,
			SourceReleaseID: op.SourceReleaseID,
			TargetReleaseID: targetReleaseID,
			Stage:          "switch_initiated",
			CreatedAt:      time.Now().Format("2006-01-02 15:04:05"),
			UpdatedAt:      time.Now().Format("2006-01-02 15:04:05"),
		}
		if err := tx.Create(jrnl).Error; err != nil {
			return fmt.Errorf("create switch journal: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return desiredRevision, nil
}

func (a *coordinatorRepoAdapter) UpdateSettings(ctx context.Context, op *operation.InstallationOperation, installationID string, expectedRevision int, updates map[string]interface{}) (int, int64, error) {
	db := a.installRepo.DB()
	var settingsRevision int
	var desiredRevision int64
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var settings installation.RuntimeSettings
		err := tx.Where("installation_id = ?", installationID).First(&settings).Error
		if err != nil {
			return fmt.Errorf("get settings: %w", err)
		}
		if settings.SettingsRevision != expectedRevision {
			return fmt.Errorf("settings revision conflict: expected %d, got %d", expectedRevision, settings.SettingsRevision)
		}
		newSettingsRev := settings.SettingsRevision + 1
		updates["settings_revision"] = newSettingsRev
		updates["updated_at"] = time.Now().Format("2006-01-02 15:04:05")
		if err := tx.Model(&installation.RuntimeSettings{}).Where("id = ?", settings.ID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update settings: %w", err)
		}
		settingsRevision = newSettingsRev
		var counter desired.DeviceDesiredRevisionCounter
		counterErr := tx.Where("user_id = ? AND device_id = ?", op.UserID, op.DeviceID).First(&counter).Error
		if counterErr != nil {
			counter = desired.DeviceDesiredRevisionCounter{
				UserID:          op.UserID,
				DeviceID:        op.DeviceID,
				CurrentRevision: 0,
				UpdatedAt:       time.Now().Format("2006-01-02 15:04:05"),
			}
			if err := tx.Create(&counter).Error; err != nil {
				return fmt.Errorf("create revision counter: %w", err)
			}
		}
		newRev := counter.CurrentRevision + 1
		result := tx.Model(&desired.DeviceDesiredRevisionCounter{}).
			Where("user_id = ? AND device_id = ? AND current_revision = ?", op.UserID, op.DeviceID, counter.CurrentRevision).
			Updates(map[string]interface{}{
				"current_revision": newRev,
				"updated_at":       time.Now().Format("2006-01-02 15:04:05"),
			})
		if result.Error != nil || result.RowsAffected == 0 {
			return fmt.Errorf("revision allocation conflict")
		}
		desiredRevision = newRev
		desiredState := &desired.RuntimeDesiredState{
			ID:                    uuid.New().String(),
			UserID:                op.UserID,
			DeviceID:              op.DeviceID,
			RuntimeID:             op.RuntimeID,
			InstallationID:        installationID,
			DesiredRevision:       desiredRevision,
			OperationID:           op.ID,
			CreatedAt:             time.Now().Format("2006-01-02 15:04:05"),
			UpdatedAt:             time.Now().Format("2006-01-02 15:04:05"),
		}
		if err := tx.Create(desiredState).Error; err != nil {
			return fmt.Errorf("create desired state: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return settingsRevision, desiredRevision, nil
}

func (a *coordinatorRepoAdapter) ChangeDefaultAction(ctx context.Context, op *operation.InstallationOperation, installationID, actionKey string) (int64, error) {
	db := a.installRepo.DB()
	var desiredRevision int64
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var counter desired.DeviceDesiredRevisionCounter
		counterErr := tx.Where("user_id = ? AND device_id = ?", op.UserID, op.DeviceID).First(&counter).Error
		if counterErr != nil {
			counter = desired.DeviceDesiredRevisionCounter{
				UserID:          op.UserID,
				DeviceID:        op.DeviceID,
				CurrentRevision: 0,
				UpdatedAt:       time.Now().Format("2006-01-02 15:04:05"),
			}
			if err := tx.Create(&counter).Error; err != nil {
				return fmt.Errorf("create revision counter: %w", err)
			}
		}
		newRev := counter.CurrentRevision + 1
		result := tx.Model(&desired.DeviceDesiredRevisionCounter{}).
			Where("user_id = ? AND device_id = ? AND current_revision = ?", op.UserID, op.DeviceID, counter.CurrentRevision).
			Updates(map[string]interface{}{
				"current_revision": newRev,
				"updated_at":       time.Now().Format("2006-01-02 15:04:05"),
			})
		if result.Error != nil || result.RowsAffected == 0 {
			return fmt.Errorf("revision allocation conflict")
		}
		desiredRevision = newRev
		desiredState := &desired.RuntimeDesiredState{
			ID:                    uuid.New().String(),
			UserID:                op.UserID,
			DeviceID:              op.DeviceID,
			RuntimeID:             op.RuntimeID,
			InstallationID:        installationID,
			DesiredActionKey:      actionKey,
			DesiredEnabled:        true,
			DesiredVisible:        true,
			DesiredRevision:       desiredRevision,
			OperationID:           op.ID,
			CreatedAt:             time.Now().Format("2006-01-02 15:04:05"),
			UpdatedAt:             time.Now().Format("2006-01-02 15:04:05"),
		}
		if err := tx.Create(desiredState).Error; err != nil {
			return fmt.Errorf("create desired state: %w", err)
		}
		if err := tx.Model(&installation.Installation{}).Where("id = ?", installationID).Updates(map[string]interface{}{
			"default_action_key": actionKey,
			"updated_at":         time.Now().Format("2006-01-02 15:04:05"),
		}).Error; err != nil {
			return fmt.Errorf("update installation default action: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return desiredRevision, nil
}

func (a *coordinatorRepoAdapter) MarkUninstallDesired(ctx context.Context, op *operation.InstallationOperation, installationID string) (int64, error) {
	db := a.installRepo.DB()
	var desiredRevision int64
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var counter desired.DeviceDesiredRevisionCounter
		counterErr := tx.Where("user_id = ? AND device_id = ?", op.UserID, op.DeviceID).First(&counter).Error
		if counterErr != nil {
			counter = desired.DeviceDesiredRevisionCounter{
				UserID:          op.UserID,
				DeviceID:        op.DeviceID,
				CurrentRevision: 0,
				UpdatedAt:       time.Now().Format("2006-01-02 15:04:05"),
			}
			if err := tx.Create(&counter).Error; err != nil {
				return fmt.Errorf("create revision counter: %w", err)
			}
		}
		newRev := counter.CurrentRevision + 1
		result := tx.Model(&desired.DeviceDesiredRevisionCounter{}).
			Where("user_id = ? AND device_id = ? AND current_revision = ?", op.UserID, op.DeviceID, counter.CurrentRevision).
			Updates(map[string]interface{}{
				"current_revision": newRev,
				"updated_at":       time.Now().Format("2006-01-02 15:04:05"),
			})
		if result.Error != nil || result.RowsAffected == 0 {
			return fmt.Errorf("revision allocation conflict")
		}
		desiredRevision = newRev
		desiredState := &desired.RuntimeDesiredState{
			ID:                    uuid.New().String(),
			UserID:                op.UserID,
			DeviceID:              op.DeviceID,
			RuntimeID:             op.RuntimeID,
			InstallationID:        installationID,
			DesiredEnabled:        false,
			DesiredVisible:        false,
			DesiredRevision:       desiredRevision,
			OperationID:           op.ID,
			CreatedAt:             time.Now().Format("2006-01-02 15:04:05"),
			UpdatedAt:             time.Now().Format("2006-01-02 15:04:05"),
		}
		if err := tx.Create(desiredState).Error; err != nil {
			return fmt.Errorf("create desired state: %w", err)
		}
		if err := tx.Model(&installation.Installation{}).Where("id = ?", installationID).Updates(map[string]interface{}{
			"desired_state":   installation.DesiredDisabled,
			"lifecycle_state": installation.LifecycleUninstalling,
			"updated_at":      time.Now().Format("2006-01-02 15:04:05"),
		}).Error; err != nil {
			return fmt.Errorf("update installation uninstalling: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return desiredRevision, nil
}

func (a *coordinatorRepoAdapter) MarkOperationCancelRequested(ctx context.Context, userID, deviceID, operationID string) error {
	return a.installRepo.DB().WithContext(ctx).
		Model(&operation.InstallationOperation{}).
		Where("id = ? AND user_id = ? AND device_id = ? AND status NOT IN (?, ?, ?)", operationID, userID, deviceID, operation.OpStatusCompleted, operation.OpStatusFailedTerminal, operation.OpStatusCancelled).
		Updates(map[string]interface{}{
			"status":     operation.OpStatusCancelRequested,
			"updated_at": time.Now().Format("2006-01-02 15:04:05"),
		}).Error
}
