// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package recovery

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
)

const (
	SwitchStageBindingCommitted  = "binding_committed"
	SwitchStageDesiredCommitted  = "desired_committed"
	SwitchStageRuntimeApplied    = "switch_runtime_applied"
	SwitchStageSwitchCompleted   = "switch_completed"
)

type SwitchRecovery struct {
	worker      *RecoveryWorker
	repo        RecoveryRepo
	switchRepo  SwitchRepo
}

type SwitchRepo interface {
	PublishSwitchDesired(ctx context.Context, opID, userID, deviceID, runtimeID, newInstallationID string, newDesiredRevision int64) error
	SendSwitchCommand(ctx context.Context, opID, userID, deviceID, runtimeID, newInstallationID string, newDesiredRevision int64) error
	QuerySwitchApplied(ctx context.Context, userID, deviceID, runtimeID string, newDesiredRevision int64) (bool, error)
}

func NewSwitchRecovery(worker *RecoveryWorker, repo RecoveryRepo, switchRepo SwitchRepo) *SwitchRecovery {
	return &SwitchRecovery{
		worker:     worker,
		repo:       repo,
		switchRepo: switchRepo,
	}
}

func (r *SwitchRecovery) Recover(ctx context.Context, op *operation.InstallationOperation, j *RecoverySwitchJournal) error {
	if r.switchRepo == nil {
		return errors.New("switchRecovery: switchRepo not configured")
	}
	switch j.Stage {
	case SwitchStageBindingCommitted:
		return r.recoverFromBindingCommitted(ctx, op, j)
	case SwitchStageDesiredCommitted:
		return r.recoverFromDesiredCommitted(ctx, op, j)
	case SwitchStageRuntimeApplied:
		return r.recoverFromRuntimeApplied(ctx, op, j)
	default:
		return fmt.Errorf("%w: %s", ErrInvalidStage, j.Stage)
	}
}

func (r *SwitchRecovery) recoverFromBindingCommitted(ctx context.Context, op *operation.InstallationOperation, j *RecoverySwitchJournal) error {
	desiredRevision := j.NewDesiredRevision
	if desiredRevision == 0 {
		desiredRevision = time.Now().UnixNano()
		j.NewDesiredRevision = desiredRevision
	}
	if err := r.switchRepo.PublishSwitchDesired(ctx, op.ID, op.UserID, op.DeviceID, op.RuntimeID, j.NewInstallationID, desiredRevision); err != nil {
		return fmt.Errorf("switchRecovery: publish desired failed op=%s: %w", op.ID, err)
	}
	if _, err := r.repo.CASUpdateSwitchJournalStage(op.ID, SwitchStageBindingCommitted, SwitchStageDesiredCommitted, r.worker.executionID); err != nil {
		if !errors.Is(err, ErrJournalNotFound) {
			return fmt.Errorf("switchRecovery: CAS update to desired_committed failed: %w", err)
		}
	}
	return r.recoverFromDesiredCommitted(ctx, op, j)
}

func (r *SwitchRecovery) recoverFromDesiredCommitted(ctx context.Context, op *operation.InstallationOperation, j *RecoverySwitchJournal) error {
	if err := r.switchRepo.SendSwitchCommand(ctx, op.ID, op.UserID, op.DeviceID, op.RuntimeID, j.NewInstallationID, j.NewDesiredRevision); err != nil {
		return fmt.Errorf("switchRecovery: send switch command failed op=%s: %w", op.ID, err)
	}
	if _, err := r.repo.CASUpdateSwitchJournalStage(op.ID, SwitchStageDesiredCommitted, SwitchStageRuntimeApplied, r.worker.executionID); err != nil {
		if !errors.Is(err, ErrJournalNotFound) {
			return fmt.Errorf("switchRecovery: CAS update to switch_runtime_applied failed: %w", err)
		}
	}
	return nil
}

func (r *SwitchRecovery) recoverFromRuntimeApplied(ctx context.Context, op *operation.InstallationOperation, j *RecoverySwitchJournal) error {
	applied, err := r.switchRepo.QuerySwitchApplied(ctx, op.UserID, op.DeviceID, op.RuntimeID, j.NewDesiredRevision)
	if err != nil {
		if reErr := r.switchRepo.SendSwitchCommand(ctx, op.ID, op.UserID, op.DeviceID, op.RuntimeID, j.NewInstallationID, j.NewDesiredRevision); reErr != nil {
			return fmt.Errorf("switchRecovery: re-send switch command failed op=%s: %w", op.ID, reErr)
		}
		return nil
	}
	if !applied {
		if reErr := r.switchRepo.SendSwitchCommand(ctx, op.ID, op.UserID, op.DeviceID, op.RuntimeID, j.NewInstallationID, j.NewDesiredRevision); reErr != nil {
			return fmt.Errorf("switchRecovery: re-send switch command failed op=%s: %w", op.ID, reErr)
		}
		return nil
	}
	if _, err := r.repo.CASUpdateSwitchJournalStage(op.ID, SwitchStageRuntimeApplied, SwitchStageSwitchCompleted, r.worker.executionID); err != nil {
		if !errors.Is(err, ErrJournalNotFound) {
			return fmt.Errorf("switchRecovery: CAS update to switch_completed failed: %w", err)
		}
	}
	if _, err := r.repo.UpdateOperationStatus(op.ID, op.Status, operation.OpStatusCompleted, r.worker.executionID); err != nil {
		return fmt.Errorf("switchRecovery: update op to completed failed op=%s: %w", op.ID, err)
	}
	return nil
}

var _ = fmt.Errorf
