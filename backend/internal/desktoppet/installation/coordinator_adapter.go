// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"context"
	"encoding/json"
	"errors"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/installation/binding"
	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
	"github.com/u-ai/backend/internal/desktoppet/installation/desired"
	"github.com/u-ai/backend/internal/desktoppet/installation/journal"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"github.com/u-ai/backend/internal/desktoppet/security"
	"gorm.io/gorm"
)

type CoordinatorRepoAdapter struct {
	repo RepositoryV2
}

func NewCoordinatorRepoAdapter(repo RepositoryV2) *CoordinatorRepoAdapter {
	return &CoordinatorRepoAdapter{repo: repo}
}

func (a *CoordinatorRepoAdapter) CreateOperation(ctx context.Context, op *operation.InstallationOperation) error {
	return a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		if err := tx.CreateOperationTx(tx.DB(), op); err != nil {
			return err
		}
		switch op.OperationType {
		case operation.TypeInstall, operation.TypeRepair:
			entityID := op.InstallationID
			if entityID == "" {
				entityID = op.ID
			}
			return a.ensureCommitJournalTx(tx, op, op.InstallationID, op.PetID, op.SourceReleaseID, op.TargetReleaseID, path.Join(".staging", entityID), op.Stage)
		case operation.TypeSwitch, operation.TypeUpgrade, operation.TypeDowngrade:
			return a.ensureSwitchJournalTx(tx, op, op.InstallationID, op.SourceReleaseID, op.TargetReleaseID, 0)
		default:
			return nil
		}
	})
}

func (a *CoordinatorRepoAdapter) GetOperation(ctx context.Context, userID, deviceID, operationID string) (*operation.InstallationOperation, error) {
	var result *operation.InstallationOperation
	err := a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		op, err := tx.GetOperationTx(tx.DB(), operationID)
		if err != nil {
			return err
		}
		if op.UserID != userID || op.DeviceID != deviceID {
			return ErrOperationNotFound
		}
		result = op
		return nil
	})
	return result, err
}

func (a *CoordinatorRepoAdapter) UpdateOperation(ctx context.Context, op *operation.InstallationOperation) error {
	if op == nil {
		return errors.New("installation coordinator repo: nil operation")
	}
	return a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		if err := tx.UpdateOperationTx(tx.DB(), op); err != nil {
			return err
		}
		switch op.OperationType {
		case operation.TypeInstall, operation.TypeRepair:
			j, err := tx.GetCommitJournalTx(tx.DB(), op.ID)
			if err == nil && j != nil {
				j.Stage = op.Stage
				j.ExecutionID = op.ExecutionID
				j.UpdatedAt = nowInstallation()
				return tx.DB().Save(j).Error
			}
			if err != nil && !errors.Is(err, ErrJournalNotFound) {
				return err
			}
		}
		return nil
	})
}

func (a *CoordinatorRepoAdapter) FindOperationByIdempotencyKey(ctx context.Context, userID, deviceID, key, operationType string) (*operation.InstallationOperation, error) {
	var result *operation.InstallationOperation
	err := a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		op, err := tx.GetOperationByIdempotencyKeyTx(tx.DB(), userID, deviceID, key, operationType)
		if err != nil {
			if errors.Is(err, ErrOperationNotFound) {
				result = nil
				return nil
			}
			return err
		}
		result = op
		return nil
	})
	return result, err
}

func (a *CoordinatorRepoAdapter) GetInstallation(ctx context.Context, userID, deviceID, installationID string) (*coordinator.InstallationRecord, error) {
	inst, err := a.repo.GetInstallationForUserDevice(userID, deviceID, installationID)
	if err != nil {
		return nil, err
	}
	return installationRecord(inst), nil
}

func (a *CoordinatorRepoAdapter) GetDesiredStateSnapshot(ctx context.Context, userID, deviceID string) (*coordinator.DesiredStateSnapshot, error) {
	state, err := a.repo.GetRuntimeDesiredStateTx(a.repo.DB().WithContext(ctx), userID, deviceID)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, ErrDesiredStateRevisionNotFound
	}
	return &coordinator.DesiredStateSnapshot{
		DesiredRevision:      state.DesiredRevision,
		DesiredHash:          state.DesiredHash,
		InstallationID:       state.InstallationID,
		PetID:                state.PetID,
		ReleaseID:            state.ReleaseID,
		UserID:               state.UserID,
		DeviceID:             state.DeviceID,
		RuntimeID:            state.RuntimeID,
		DefaultActionKey:     state.DesiredActionKey,
		SettingsRevision:     state.SettingsRevision,
		SettingsSnapshotJSON: state.SettingsSnapshotJSON,
	}, nil
}

func (a *CoordinatorRepoAdapter) CreateInstallationAndDesiredState(ctx context.Context, op *operation.InstallationOperation, install *coordinator.InstallationRecord, snapshot *coordinator.DesiredStateSnapshot, stagingPathKey string) (int64, error) {
	var desiredRevision int64
	err := a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		now := nowInstallation()
		var existing Installation
		existingErr := tx.DB().Where("id = ? AND user_id = ? AND device_id = ?", install.ID, install.UserID, install.DeviceID).First(&existing).Error
		if existingErr != nil && !errors.Is(existingErr, gorm.ErrRecordNotFound) {
			return existingErr
		}
		isNewInstallation := errors.Is(existingErr, gorm.ErrRecordNotFound)
		presentation, err := loadReleasePresentationTx(tx.DB(), install.ReleaseID)
		if err != nil {
			return err
		}
		installRel, _ := security.DefaultRelativePath(security.RootInstallations)
		installPath := filepath.ToSlash(filepath.Join(installRel, filepath.FromSlash(stagingPathKey)))
		manifestPath := filepath.ToSlash(filepath.Join(installPath, "manifest.json"))
		previewPath := ""
		if presentation.Manifest.Preview != "" {
			previewPath = filepath.ToSlash(filepath.Join(installPath, filepath.FromSlash(presentation.Manifest.Preview)))
		}
		characterID := install.CharacterID
		if characterID == "" {
			characterID = presentation.Manifest.Binding.SourceCharacterID
		}
		model := &Installation{
			ID:                     install.ID,
			UserID:                 install.UserID,
			DeviceID:               install.DeviceID,
			CharacterID:            characterID,
			PackageID:              install.ReleaseID,
			PackageVersion:         presentation.Version,
			Name:                   presentation.Manifest.Name,
			PetID:                  install.PetID,
			CurrentReleaseID:       install.ReleaseID,
			Status:                 StatusEnabled,
			IsActive:               1,
			InstallPath:            installPath,
			ManifestPath:           manifestPath,
			PreviewPath:            previewPath,
			PreviewArtifactPath:    previewPath,
			InstallStorageKey:      stagingPathKey,
			DefaultActionKey:       install.DefaultActionKey,
			DefaultActionReleaseID: install.ReleaseID,
			CanvasWidth:            presentation.Manifest.Canvas.Width,
			CanvasHeight:           presentation.Manifest.Canvas.Height,
			PackageHash:            presentation.ContentRootHash,
			InstalledContentHash:   presentation.ContentRootHash,
			LifecycleState:         LifecycleInstalled,
			IntegrityStatus:        IntegrityVerified,
			DesiredState:           DesiredEnabled,
			RuntimeSyncState:       SyncPending,
			UpdatedAt:              now,
		}
		if err := tx.DB().Model(&Installation{}).Where("user_id = ? AND device_id = ? AND id <> ? AND is_active = 1", install.UserID, install.DeviceID, install.ID).Updates(map[string]interface{}{
			"is_active": 0, "status": StatusDisabled, "desired_state": DesiredDisabled, "last_disabled_at": now, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		if isNewInstallation {
			model.CreatedAt = now
			model.InstalledAt = now
			if err := tx.CreateInstallationTx(tx.DB(), model); err != nil {
				return err
			}
		} else {
			model.CreatedAt = existing.CreatedAt
			model.InstalledAt = existing.InstalledAt
			if model.InstalledAt == "" {
				model.InstalledAt = now
			}
			if err := tx.UpdateInstallationTx(tx.DB(), model); err != nil {
				return err
			}
		}

		settings, err := ensureRuntimeSettingsTx(tx, install.ID, now)
		if err != nil {
			return err
		}
		settingsJSON, err := json.Marshal(settings)
		if err != nil {
			return err
		}
		state := &desired.RuntimeDesiredState{
			UserID:               snapshot.UserID,
			DeviceID:             snapshot.DeviceID,
			RuntimeID:            snapshot.RuntimeID,
			InstallationID:       snapshot.InstallationID,
			PetID:                snapshot.PetID,
			ReleaseID:            snapshot.ReleaseID,
			DesiredEnabled:       !snapshot.EnsureAbsent,
			DesiredVisible:       !snapshot.EnsureAbsent,
			DesiredActionKey:     snapshot.DefaultActionKey,
			SettingsSnapshotJSON: string(settingsJSON),
			SettingsRevision:     int64(settings.SettingsRevision),
			OperationID:          op.ID,
		}
		revision, err := a.writeDesiredStateTx(tx, state, false)
		if err != nil {
			return err
		}
		desiredRevision = revision

		if err := a.upsertBindingTx(tx, install.UserID, install.DeviceID, install.ID, install.PetID, install.ReleaseID, revision, op.ID, binding.BoundReasonInstall); err != nil {
			return err
		}
		if err := a.ensureCommitJournalTx(tx, op, install.ID, install.PetID, op.SourceReleaseID, install.ReleaseID, stagingPathKey, operation.OpStageDesiredStateCommitted); err != nil {
			return err
		}
		return nil
	})
	return desiredRevision, err
}

func (a *CoordinatorRepoAdapter) UpdateDesiredEnabled(ctx context.Context, op *operation.InstallationOperation, installationID string, enabled bool) (int64, error) {
	var desiredRevision int64
	err := a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		inst, err := loadInstallationTx(tx.DB(), op.UserID, op.DeviceID, installationID)
		if err != nil {
			return err
		}
		state, err := a.desiredFromCurrentTx(tx, inst, op)
		if err != nil {
			return err
		}
		state.DesiredEnabled = enabled
		state.DesiredVisible = enabled
		revision, err := a.writeDesiredStateTx(tx, state, false)
		if err != nil {
			return err
		}
		desiredRevision = revision
		inst.DesiredState = DesiredDisabled
		inst.Status = StatusDisabled
		inst.IsActive = 0
		if enabled {
			now := nowInstallation()
			if err := tx.DB().Model(&Installation{}).Where("user_id = ? AND device_id = ? AND id <> ? AND is_active = 1", inst.UserID, inst.DeviceID, inst.ID).Updates(map[string]interface{}{
				"is_active": 0, "status": StatusDisabled, "desired_state": DesiredDisabled, "last_disabled_at": now, "updated_at": now,
			}).Error; err != nil {
				return err
			}
			inst.DesiredState = DesiredEnabled
			inst.Status = StatusEnabled
			inst.IsActive = 1
			inst.LastEnabledAt = now
			if err := a.upsertBindingTx(tx, inst.UserID, inst.DeviceID, inst.ID, inst.PetID, inst.CurrentReleaseID, revision, op.ID, binding.BoundReasonEnable); err != nil {
				return err
			}
		} else {
			inst.LastDisabledAt = nowInstallation()
			active, bindErr := tx.GetActiveBindingForUserDeviceTx(tx.DB(), inst.UserID, inst.DeviceID)
			if bindErr == nil && active != nil && active.InstallationID == inst.ID {
				if err := tx.DeleteActiveBindingTx(tx.DB(), inst.UserID, inst.DeviceID); err != nil {
					return err
				}
			}
		}
		return tx.UpdateInstallationTx(tx.DB(), inst)
	})
	return desiredRevision, err
}

func (a *CoordinatorRepoAdapter) SwitchRelease(ctx context.Context, op *operation.InstallationOperation, installationID, targetReleaseID, stagingPathKey, defaultActionKey string) (int64, error) {
	var desiredRevision int64
	err := a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		inst, err := loadInstallationTx(tx.DB(), op.UserID, op.DeviceID, installationID)
		if err != nil {
			return err
		}
		oldRelease := inst.CurrentReleaseID
		state, err := a.desiredFromCurrentTx(tx, inst, op)
		if err != nil {
			return err
		}
		state.ReleaseID = targetReleaseID
		state.DesiredActionKey = defaultActionKey
		state.DesiredEnabled = true
		state.DesiredVisible = true
		revision, err := a.writeDesiredStateTx(tx, state, false)
		if err != nil {
			return err
		}
		desiredRevision = revision

		presentation, err := loadReleasePresentationTx(tx.DB(), targetReleaseID)
		if err != nil {
			return err
		}
		installRel, _ := security.DefaultRelativePath(security.RootInstallations)
		installPath := filepath.ToSlash(filepath.Join(installRel, filepath.FromSlash(stagingPathKey)))
		inst.CurrentReleaseID = targetReleaseID
		inst.PackageID = targetReleaseID
		inst.PackageVersion = presentation.Version
		inst.Name = presentation.Manifest.Name
		inst.DefaultActionKey = defaultActionKey
		inst.DefaultActionReleaseID = targetReleaseID
		inst.InstallPath = installPath
		inst.ManifestPath = filepath.ToSlash(filepath.Join(installPath, "manifest.json"))
		inst.PreviewPath = ""
		if presentation.Manifest.Preview != "" {
			inst.PreviewPath = filepath.ToSlash(filepath.Join(installPath, filepath.FromSlash(presentation.Manifest.Preview)))
		}
		inst.PreviewArtifactPath = inst.PreviewPath
		inst.InstallStorageKey = stagingPathKey
		inst.CanvasWidth = presentation.Manifest.Canvas.Width
		inst.CanvasHeight = presentation.Manifest.Canvas.Height
		inst.PackageHash = presentation.ContentRootHash
		inst.InstalledContentHash = presentation.ContentRootHash
		inst.RuntimeSyncState = SyncPending
		if err := tx.UpdateInstallationTx(tx.DB(), inst); err != nil {
			return err
		}
		if err := a.upsertBindingTx(tx, inst.UserID, inst.DeviceID, inst.ID, inst.PetID, targetReleaseID, revision, op.ID, binding.BoundReasonSwitch); err != nil {
			return err
		}
		return a.ensureSwitchJournalTx(tx, op, installationID, oldRelease, targetReleaseID, revision)
	})
	return desiredRevision, err
}

func (a *CoordinatorRepoAdapter) UpdateSettings(ctx context.Context, op *operation.InstallationOperation, installationID string, expectedRevision int, updates map[string]interface{}) (int, int64, error) {
	var settingsRevision int
	var desiredRevision int64
	err := a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		inst, err := loadInstallationTx(tx.DB(), op.UserID, op.DeviceID, installationID)
		if err != nil {
			return err
		}
		settings, err := tx.UpdateRuntimeSettingsCAS(tx.DB(), installationID, op.UserID, op.DeviceID, expectedRevision, updates)
		if err != nil {
			return err
		}
		settingsRevision = settings.SettingsRevision
		state, err := a.desiredFromCurrentTx(tx, inst, op)
		if err != nil {
			return err
		}
		state.SettingsRevision = int64(settings.SettingsRevision)
		settingsJSON, err := json.Marshal(settings)
		if err != nil {
			return err
		}
		state.SettingsSnapshotJSON = string(settingsJSON)
		revision, err := a.writeDesiredStateTx(tx, state, false)
		if err != nil {
			return err
		}
		desiredRevision = revision
		return nil
	})
	return settingsRevision, desiredRevision, err
}

func (a *CoordinatorRepoAdapter) ChangeDefaultAction(ctx context.Context, op *operation.InstallationOperation, installationID, actionKey string) (int64, error) {
	var desiredRevision int64
	err := a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		inst, err := loadInstallationTx(tx.DB(), op.UserID, op.DeviceID, installationID)
		if err != nil {
			return err
		}
		state, err := a.desiredFromCurrentTx(tx, inst, op)
		if err != nil {
			return err
		}
		state.DesiredActionKey = actionKey
		revision, err := a.writeDesiredStateTx(tx, state, false)
		if err != nil {
			return err
		}
		desiredRevision = revision
		inst.DefaultActionKey = actionKey
		return tx.UpdateInstallationTx(tx.DB(), inst)
	})
	return desiredRevision, err
}

func (a *CoordinatorRepoAdapter) MarkUninstallDesired(ctx context.Context, op *operation.InstallationOperation, installationID string) (int64, error) {
	var desiredRevision int64
	err := a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		inst, err := loadInstallationTx(tx.DB(), op.UserID, op.DeviceID, installationID)
		if err != nil {
			return err
		}
		state, err := a.desiredFromCurrentTx(tx, inst, op)
		if err != nil {
			return err
		}
		state.DesiredEnabled = false
		state.DesiredVisible = false
		revision, err := a.writeDesiredStateTx(tx, state, true)
		if err != nil {
			return err
		}
		desiredRevision = revision
		inst.Status = StatusUninstalling
		inst.LifecycleState = LifecycleUninstalling
		inst.DesiredState = DesiredDisabled
		inst.IsActive = 0
		inst.RuntimeSyncState = SyncPending
		return tx.UpdateInstallationTx(tx.DB(), inst)
	})
	return desiredRevision, err
}

func (a *CoordinatorRepoAdapter) MarkOperationCancelRequested(ctx context.Context, userID, deviceID, operationID string) error {
	return a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		op, err := tx.GetOperationTx(tx.DB(), operationID)
		if err != nil {
			return err
		}
		if op.UserID != userID || op.DeviceID != deviceID {
			return ErrOperationNotFound
		}
		if op.IsTerminal() {
			return operation.ErrInvalidTransition
		}
		if op.Status == operation.OpStatusCancelRequested {
			return nil
		}
		_, err = tx.UpdateOperationStatusCAS(tx.DB(), op.ID, op.Status, operation.OpStatusCancelRequested, "")
		return err
	})
}

func (a *CoordinatorRepoAdapter) writeDesiredStateTx(tx RepositoryV2, state *desired.RuntimeDesiredState, ensureAbsent bool) (int64, error) {
	existing, err := tx.GetRuntimeDesiredStateTx(tx.DB(), state.UserID, state.DeviceID)
	if err != nil {
		return 0, err
	}
	expected := int64(-1)
	if existing != nil {
		expected = existing.DesiredRevision
		if state.InstallationID == "" {
			state.InstallationID = existing.InstallationID
		}
		if state.PetID == "" {
			state.PetID = existing.PetID
		}
		if state.ReleaseID == "" {
			state.ReleaseID = existing.ReleaseID
		}
		if state.RuntimeID == "" {
			state.RuntimeID = existing.RuntimeID
		}
		if state.DesiredActionKey == "" {
			state.DesiredActionKey = existing.DesiredActionKey
		}
		if state.SettingsRevision == 0 {
			state.SettingsRevision = existing.SettingsRevision
			state.SettingsSnapshotJSON = existing.SettingsSnapshotJSON
		}
	}
	state.DesiredHash = desired.ComputeDesiredHash(desired.DesiredHashFields{
		InstallationLabels: map[string]string{
			"installation_id": state.InstallationID,
			"pet_id":          state.PetID,
			"release_id":      state.ReleaseID,
		},
		DesiredLabels: map[string]string{
			"enabled":    strconv.FormatBool(state.DesiredEnabled),
			"visible":    strconv.FormatBool(state.DesiredVisible),
			"action_key": state.DesiredActionKey,
		},
		SettingsLabels: map[string]string{
			"revision": strconv.FormatInt(state.SettingsRevision, 10),
			"snapshot": state.SettingsSnapshotJSON,
		},
	})
	revision, err := tx.AllocateDeviceDesiredRevisionCAS(tx.DB(), state.UserID, state.DeviceID)
	if err != nil {
		return 0, err
	}
	state.DesiredRevision = revision
	state.UpdatedAt = nowInstallation()
	if state.CreatedAt == "" {
		state.CreatedAt = state.UpdatedAt
	}
	if _, err := tx.UpsertRuntimeDesiredStateCAS(tx.DB(), state.UserID, state.DeviceID, state, expected); err != nil {
		return 0, err
	}
	snapshot := coordinator.DesiredStateSnapshot{
		DesiredRevision:      state.DesiredRevision,
		DesiredHash:          state.DesiredHash,
		InstallationID:       state.InstallationID,
		PetID:                state.PetID,
		ReleaseID:            state.ReleaseID,
		UserID:               state.UserID,
		DeviceID:             state.DeviceID,
		RuntimeID:            state.RuntimeID,
		EnsureAbsent:         ensureAbsent,
		DefaultActionKey:     state.DesiredActionKey,
		SettingsRevision:     state.SettingsRevision,
		SettingsSnapshotJSON: state.SettingsSnapshotJSON,
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return 0, err
	}
	now := nowInstallation()
	if err := tx.CreateOutboxEventTx(tx.DB(), &desired.DesiredStateOutboxEvent{
		EventID:         uuid.NewString(),
		EventType:       "desired_state_changed",
		UserID:          state.UserID,
		DeviceID:        state.DeviceID,
		RuntimeID:       state.RuntimeID,
		InstallationID:  state.InstallationID,
		DesiredRevision: state.DesiredRevision,
		DesiredHash:     state.DesiredHash,
		OperationID:     state.OperationID,
		PayloadJSON:     string(payload),
		Status:          "pending",
		AttemptCount:    0,
		AvailableAt:     now,
		CreatedAt:       now,
	}); err != nil {
		return 0, err
	}
	return revision, nil
}

func (a *CoordinatorRepoAdapter) desiredFromCurrentTx(tx RepositoryV2, inst *Installation, op *operation.InstallationOperation) (*desired.RuntimeDesiredState, error) {
	current, err := tx.GetRuntimeDesiredStateTx(tx.DB(), inst.UserID, inst.DeviceID)
	if err != nil {
		return nil, err
	}
	state := &desired.RuntimeDesiredState{
		UserID:         inst.UserID,
		DeviceID:       inst.DeviceID,
		RuntimeID:      op.RuntimeID,
		InstallationID: inst.ID,
		PetID:          inst.PetID,
		ReleaseID:      inst.CurrentReleaseID,
		DesiredEnabled: inst.DesiredState == DesiredEnabled,
		DesiredVisible: inst.DesiredState == DesiredEnabled,
		OperationID:    op.ID,
		CreatedAt:      nowInstallation(),
		UpdatedAt:      nowInstallation(),
	}
	if current != nil {
		state.RuntimeID = current.RuntimeID
		if op.RuntimeID != "" {
			state.RuntimeID = op.RuntimeID
		}
		state.DesiredEnabled = current.DesiredEnabled
		state.DesiredVisible = current.DesiredVisible
		state.DesiredActionKey = current.DesiredActionKey
		state.SettingsSnapshotJSON = current.SettingsSnapshotJSON
		state.SettingsRevision = current.SettingsRevision
		state.DesiredHash = current.DesiredHash
	}
	return state, nil
}

func (a *CoordinatorRepoAdapter) upsertBindingTx(tx RepositoryV2, userID, deviceID, installationID, petID, releaseID string, revision int64, operationID, reason string) error {
	previous, err := tx.GetActiveBindingForUserDeviceTx(tx.DB(), userID, deviceID)
	if err != nil && !errors.Is(err, ErrBindingNotFound) {
		return err
	}
	now := nowInstallation()
	entry := &binding.DeviceActiveInstallationBinding{
		UserID:          userID,
		DeviceID:        deviceID,
		InstallationID:  installationID,
		PetID:           petID,
		ReleaseID:       releaseID,
		BindingRevision: revision,
		BoundReason:     reason,
		BoundAt:         now,
		BoundBy:         "coordinator",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := tx.UpsertActiveBindingTx(tx.DB(), entry); err != nil {
		return err
	}
	previousID := ""
	if previous != nil {
		previousID = previous.InstallationID
	}
	return tx.InsertBindingHistoryTx(tx.DB(), &binding.BindingHistoryEntry{
		ID:                     uuid.NewString(),
		UserID:                 userID,
		DeviceID:               deviceID,
		PreviousInstallationID: previousID,
		NewInstallationID:      installationID,
		BindingRevision:        revision,
		Reason:                 reason,
		Actor:                  "coordinator",
		OperationID:            operationID,
		OccurredAt:             now,
	})
}

func (a *CoordinatorRepoAdapter) ensureCommitJournalTx(tx RepositoryV2, op *operation.InstallationOperation, installationID, petID, sourceReleaseID, targetReleaseID, stagingPathKey, stage string) error {
	j, err := tx.GetCommitJournalTx(tx.DB(), op.ID)
	if err == nil && j != nil {
		j.Stage = stage
		j.StagingPathKey = stagingPathKey
		j.TargetReleaseID = targetReleaseID
		j.UpdatedAt = nowInstallation()
		return tx.DB().Save(j).Error
	}
	if err != nil && !errors.Is(err, ErrJournalNotFound) {
		return err
	}
	now := nowInstallation()
	return tx.CreateCommitJournalTx(tx.DB(), &journal.InstallationCommitJournal{
		ID:              uuid.NewString(),
		OperationID:     op.ID,
		UserID:          op.UserID,
		DeviceID:        op.DeviceID,
		RuntimeID:       op.RuntimeID,
		InstallationID:  installationID,
		PetID:           petID,
		SourceReleaseID: sourceReleaseID,
		TargetReleaseID: targetReleaseID,
		Stage:           stage,
		Status:          "active",
		StagingPathKey:  stagingPathKey,
		CreatedAt:       now,
		UpdatedAt:       now,
	})
}

func (a *CoordinatorRepoAdapter) ensureSwitchJournalTx(tx RepositoryV2, op *operation.InstallationOperation, installationID, oldRelease, newRelease string, desiredRevision int64) error {
	j, err := tx.GetSwitchJournalTx(tx.DB(), op.ID)
	if err == nil && j != nil {
		j.NewReleaseID = newRelease
		j.NewDesiredRevision = desiredRevision
		j.Stage = journal.SwitchJournalDesiredCommitted
		j.UpdatedAt = nowInstallation()
		return tx.DB().Save(j).Error
	}
	if err != nil && !errors.Is(err, ErrJournalNotFound) {
		return err
	}
	now := nowInstallation()
	stage := journal.SwitchJournalDesiredCommitted
	if desiredRevision == 0 {
		stage = journal.SwitchJournalCreated
	}
	return tx.CreateSwitchJournalTx(tx.DB(), &journal.InstallationSwitchJournal{
		ID:                 uuid.NewString(),
		OperationID:        op.ID,
		UserID:             op.UserID,
		DeviceID:           op.DeviceID,
		RuntimeID:          op.RuntimeID,
		OldInstallationID:  installationID,
		NewInstallationID:  installationID,
		OldReleaseID:       oldRelease,
		NewReleaseID:       newRelease,
		NewDesiredRevision: desiredRevision,
		Stage:              stage,
		Status:             "active",
		CreatedAt:          now,
		UpdatedAt:          now,
	})
}

func ensureRuntimeSettingsTx(tx RepositoryV2, installationID, now string) (*RuntimeSettings, error) {
	var settings RuntimeSettings
	err := tx.DB().Where("installation_id = ?", installationID).First(&settings).Error
	if err == nil {
		return &settings, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	settings = RuntimeSettings{
		ID:                     runtimeSettingsIDPrefix + uuid.NewString(),
		InstallationID:         installationID,
		AlwaysOnTop:            1,
		Scale:                  runtimeSettingsDefaultScale,
		IdleEnabled:            1,
		IdleIntervalMinSeconds: runtimeSettingsDefaultIdleIntervalMinSeconds,
		IdleIntervalMaxSeconds: runtimeSettingsDefaultIdleIntervalMaxSeconds,
		ClickThroughMode:       runtimeSettingsDefaultClickThroughMode,
		SettingsRevision:       0,
		RestoreOnAppStart:      1,
		PositionMode:           positionModeAbsolute,
		RelativeX:              0.5,
		RelativeY:              0.5,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := tx.CreateRuntimeSettingsTx(tx.DB(), &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

func loadInstallationTx(db *gorm.DB, userID, deviceID, installationID string) (*Installation, error) {
	var inst Installation
	if err := db.Where("id = ? AND user_id = ? AND device_id = ?", installationID, userID, deviceID).First(&inst).Error; err != nil {
		return nil, err
	}
	return &inst, nil
}

type releasePresentation struct {
	Version         string
	ContentRootHash string
	Manifest        packageformat.Manifest
}

func loadReleasePresentationTx(db *gorm.DB, releaseID string) (*releasePresentation, error) {
	var row struct {
		Version         string `gorm:"column:version"`
		ContentRootHash string `gorm:"column:content_root_hash"`
		ManifestJSON    string `gorm:"column:manifest_json"`
	}
	if err := db.Table("desktop_pet_package_releases").Select("version, content_root_hash, manifest_json").Where("id = ?", releaseID).Take(&row).Error; err != nil {
		return nil, err
	}
	var manifest packageformat.Manifest
	if err := json.Unmarshal([]byte(row.ManifestJSON), &manifest); err != nil {
		return nil, err
	}
	return &releasePresentation{Version: row.Version, ContentRootHash: row.ContentRootHash, Manifest: manifest}, nil
}

func installationRecord(inst *Installation) *coordinator.InstallationRecord {
	return &coordinator.InstallationRecord{
		ID:                inst.ID,
		UserID:            inst.UserID,
		DeviceID:          inst.DeviceID,
		CharacterID:       inst.CharacterID,
		PetID:             inst.PetID,
		ReleaseID:         inst.CurrentReleaseID,
		Status:            inst.Status,
		Enabled:           inst.IsEnabled(),
		InstallStorageKey: inst.InstallStorageKey,
		DefaultActionKey:  inst.DefaultActionKey,
	}
}

func nowInstallation() string { return time.Now().UTC().Format("2006-01-02 15:04:05") }

var _ coordinator.Repository = (*CoordinatorRepoAdapter)(nil)
