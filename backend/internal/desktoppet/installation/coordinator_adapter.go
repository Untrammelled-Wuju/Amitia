// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
	"github.com/u-ai/backend/internal/desktoppet/installation/desired"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
)

type CoordinatorRepoAdapter struct {
	repo RepositoryV2
}

func NewCoordinatorRepoAdapter(repo RepositoryV2) *CoordinatorRepoAdapter {
	return &CoordinatorRepoAdapter{repo: repo}
}

func (a *CoordinatorRepoAdapter) CreateOperation(ctx context.Context, op *operation.InstallationOperation) error {
	return a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		return tx.CreateOperationTx(tx.DB(), op)
	})
}

func (a *CoordinatorRepoAdapter) GetOperation(ctx context.Context, userID, deviceID, operationID string) (*operation.InstallationOperation, error) {
	var result *operation.InstallationOperation
	err := a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		op, err := tx.GetOperationTx(tx.DB(), operationID)
		if err != nil {
			return err
		}
		result = op
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a *CoordinatorRepoAdapter) UpdateOperation(ctx context.Context, op *operation.InstallationOperation) error {
	return a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		return tx.CreateOperationTx(tx.DB(), op)
	})
}

func (a *CoordinatorRepoAdapter) FindOperationByIdempotencyKey(ctx context.Context, userID, deviceID, key, operationType string) (*operation.InstallationOperation, error) {
	var result *operation.InstallationOperation
	err := a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		op, err := tx.GetOperationByIdempotencyKeyTx(tx.DB(), userID, deviceID, key, operationType)
		if err != nil {
			return err
		}
		result = op
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a *CoordinatorRepoAdapter) GetInstallation(ctx context.Context, userID, deviceID, installationID string) (*coordinator.InstallationRecord, error) {
	inst, err := a.repo.GetInstallationForUserDevice(userID, deviceID, installationID)
	if err != nil {
		return nil, err
	}
	enabled := inst.DesiredState == "enabled"
	return &coordinator.InstallationRecord{
		ID:                inst.ID,
		UserID:            inst.UserID,
		DeviceID:          inst.DeviceID,
		PetID:             inst.PetID,
		ReleaseID:         inst.CurrentReleaseID,
		Status:            inst.Status,
		Enabled:           enabled,
		InstallStorageKey: inst.InstallStorageKey,
		DefaultActionKey:  inst.DefaultActionKey,
	}, nil
}

func (a *CoordinatorRepoAdapter) CreateInstallationAndDesiredState(ctx context.Context, op *operation.InstallationOperation, install *coordinator.InstallationRecord, desiredSnapshot *coordinator.DesiredStateSnapshot, stagingPathKey string) (int64, error) {
	var desiredRevision int64
	err := a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		if err := tx.CreateOperationTx(tx.DB(), op); err != nil {
			return err
		}
		inst := &Installation{
			ID:                install.ID,
			UserID:            install.UserID,
			DeviceID:          install.DeviceID,
			PetID:             install.PetID,
			CurrentReleaseID:  install.ReleaseID,
			Status:            install.Status,
			DesiredState:      "enabled",
			InstallStorageKey: install.InstallStorageKey,
			DefaultActionKey:  install.DefaultActionKey,
		}
		if err := tx.CreateInstallationTx(tx.DB(), inst); err != nil {
			return err
		}
		ds := &desired.RuntimeDesiredState{
			UserID:           desiredSnapshot.UserID,
			DeviceID:         desiredSnapshot.DeviceID,
			DesiredRevision:  desiredSnapshot.DesiredRevision,
			InstallationID:   desiredSnapshot.InstallationID,
			PetID:            desiredSnapshot.PetID,
			ReleaseID:        desiredSnapshot.ReleaseID,
			RuntimeID:        desiredSnapshot.RuntimeID,
			DesiredActionKey: desiredSnapshot.DefaultActionKey,
		}
		if _, err := tx.UpsertRuntimeDesiredStateCAS(tx.DB(), desiredSnapshot.UserID, desiredSnapshot.DeviceID, ds, 0); err != nil {
			return err
		}
		desiredRevision = desiredSnapshot.DesiredRevision
		return nil
	})
	return desiredRevision, err
}

func (a *CoordinatorRepoAdapter) UpdateDesiredEnabled(ctx context.Context, op *operation.InstallationOperation, installationID string, enabled bool) (int64, error) {
	var desiredRevision int64
	err := a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		inst, err := a.repo.GetInstallationForUserDevice(op.UserID, op.DeviceID, installationID)
		if err != nil {
			return err
		}
		desiredRevision = time.Now().UnixNano()
		ds := &desired.RuntimeDesiredState{
			UserID:          inst.UserID,
			DeviceID:        inst.DeviceID,
			DesiredRevision: desiredRevision,
			InstallationID:  inst.ID,
			PetID:           inst.PetID,
			ReleaseID:       inst.CurrentReleaseID,
			DesiredEnabled:  enabled,
		}
		if _, err := tx.UpsertRuntimeDesiredStateCAS(tx.DB(), inst.UserID, inst.DeviceID, ds, 0); err != nil {
			return err
		}
		return nil
	})
	return desiredRevision, err
}

func (a *CoordinatorRepoAdapter) SwitchRelease(ctx context.Context, op *operation.InstallationOperation, installationID, targetReleaseID, stagingPathKey string) (int64, error) {
	var desiredRevision int64
	err := a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		inst, err := a.repo.GetInstallationForUserDevice(op.UserID, op.DeviceID, installationID)
		if err != nil {
			return err
		}
		desiredRevision = time.Now().UnixNano()
		ds := &desired.RuntimeDesiredState{
			UserID:          inst.UserID,
			DeviceID:        inst.DeviceID,
			DesiredRevision: desiredRevision,
			InstallationID:  inst.ID,
			PetID:           inst.PetID,
			ReleaseID:       targetReleaseID,
		}
		if _, err := tx.UpsertRuntimeDesiredStateCAS(tx.DB(), inst.UserID, inst.DeviceID, ds, 0); err != nil {
			return err
		}
		return nil
	})
	return desiredRevision, err
}

func (a *CoordinatorRepoAdapter) UpdateSettings(ctx context.Context, op *operation.InstallationOperation, installationID string, expectedRevision int, updates map[string]interface{}) (int, int64, error) {
	var settingsRevision int
	var desiredRevision int64
	err := a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		_, err := tx.UpdateRuntimeSettingsCAS(tx.DB(), installationID, op.UserID, op.DeviceID, expectedRevision, updates)
		if err != nil {
			return err
		}
		settingsRevision = expectedRevision + 1
		return nil
	})
	return settingsRevision, desiredRevision, err
}

func (a *CoordinatorRepoAdapter) ChangeDefaultAction(ctx context.Context, op *operation.InstallationOperation, installationID, actionKey string) (int64, error) {
	var desiredRevision int64
	err := a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		inst, err := a.repo.GetInstallationForUserDevice(op.UserID, op.DeviceID, installationID)
		if err != nil {
			return err
		}
		desiredRevision = time.Now().UnixNano()
		ds := &desired.RuntimeDesiredState{
			UserID:           inst.UserID,
			DeviceID:         inst.DeviceID,
			DesiredRevision:  desiredRevision,
			InstallationID:   inst.ID,
			PetID:            inst.PetID,
			ReleaseID:        inst.CurrentReleaseID,
			DesiredActionKey: actionKey,
		}
		if _, err := tx.UpsertRuntimeDesiredStateCAS(tx.DB(), inst.UserID, inst.DeviceID, ds, 0); err != nil {
			return err
		}
		return nil
	})
	return desiredRevision, err
}

func (a *CoordinatorRepoAdapter) MarkUninstallDesired(ctx context.Context, op *operation.InstallationOperation, installationID string) (int64, error) {
	var desiredRevision int64
	err := a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		inst, err := a.repo.GetInstallationForUserDevice(op.UserID, op.DeviceID, installationID)
		if err != nil {
			return err
		}
		desiredRevision = time.Now().UnixNano()
		ds := &desired.RuntimeDesiredState{
			UserID:          inst.UserID,
			DeviceID:        inst.DeviceID,
			DesiredRevision: desiredRevision,
			InstallationID:  inst.ID,
			PetID:           inst.PetID,
			ReleaseID:       inst.CurrentReleaseID,
		}
		if _, err := tx.UpsertRuntimeDesiredStateCAS(tx.DB(), inst.UserID, inst.DeviceID, ds, 0); err != nil {
			return err
		}
		return nil
	})
	return desiredRevision, err
}

func (a *CoordinatorRepoAdapter) MarkOperationCancelRequested(ctx context.Context, userID, deviceID, operationID string) error {
	return a.repo.Transaction(ctx, func(tx RepositoryV2) error {
		op, err := tx.GetOperationTx(tx.DB(), operationID)
		if err != nil {
			return err
		}
		if op.Status == string(operation.OpStatusCancelRequested) {
			return nil
		}
		_, err = tx.UpdateOperationStatusCAS(tx.DB(), op.ID, op.Status, string(operation.OpStatusCancelRequested), "recovery")
		return err
	})
}

var _ coordinator.Repository = (*CoordinatorRepoAdapter)(nil)
