// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package recovery

import (
	"context"
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
)

const (
	SwitchStageBindingCommitted = "binding_committed"
	SwitchStageDesiredCommitted = "desired_committed"
	SwitchStageRuntimeApplied   = "switch_runtime_applied"
	SwitchStageSwitchCompleted  = "switch_completed"
)

type SwitchRecovery struct {
	worker     *RecoveryWorker
	repo       RecoveryRepo
	switchRepo SwitchRepo
}

type SwitchRepo interface {
	ResolveDesiredRevision(ctx context.Context, opID, userID, deviceID string) (int64, error)
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
	if op == nil || j == nil {
		return errors.New("switchRecovery: operation and journal are required")
	}
	if isSwitchDesiredRevisionStage(j.Stage) {
		if err := r.ensureAuthoritativeDesiredRevision(ctx, op, j); err != nil {
			return err
		}
	}
	switch j.Stage {
	case SwitchStageBindingCommitted:
		return r.recoverFromBindingCommitted(ctx, op, j)
	case SwitchStageDesiredCommitted:
		return r.recoverFromDesiredCommitted(ctx, op, j)
	case SwitchStageRuntimeApplied:
		return r.recoverFromRuntimeApplied(ctx, op, j)
	case SwitchStageSwitchCompleted:
		return r.recoverFromSwitchCompleted(op)
	default:
		return fmt.Errorf("%w: %s", ErrInvalidStage, j.Stage)
	}
}

func isSwitchDesiredRevisionStage(stage string) bool {
	switch stage {
	case SwitchStageBindingCommitted, SwitchStageDesiredCommitted, SwitchStageRuntimeApplied:
		return true
	default:
		return false
	}
}

func (r *SwitchRecovery) ensureAuthoritativeDesiredRevision(ctx context.Context, op *operation.InstallationOperation, j *RecoverySwitchJournal) error {
	if op.DesiredRevision > 0 && j.NewDesiredRevision > 0 {
		if op.DesiredRevision != j.NewDesiredRevision {
			return fmt.Errorf("switchRecovery: desired revision conflict op=%s operation=%d journal=%d", op.ID, op.DesiredRevision, j.NewDesiredRevision)
		}
		return nil
	}

	desiredRevision := op.DesiredRevision
	if desiredRevision <= 0 {
		desiredRevision = j.NewDesiredRevision
	}
	if desiredRevision <= 0 {
		resolved, err := r.switchRepo.ResolveDesiredRevision(ctx, op.ID, op.UserID, op.DeviceID)
		if err != nil {
			return fmt.Errorf("switchRecovery: resolve desired revision failed op=%s: %w", op.ID, err)
		}
		if resolved <= 0 {
			return fmt.Errorf("switchRecovery: committed switch %s has no authoritative desired revision", op.ID)
		}
		desiredRevision = resolved
	}

	if op.DesiredRevision <= 0 {
		persistedOp, err := r.repo.SetOperationDesiredRevisionIfMissing(op.ID, desiredRevision, r.worker.executionID)
		if err != nil {
			return fmt.Errorf("switchRecovery: persist recovered operation revision failed op=%s: %w", op.ID, err)
		}
		if persistedOp == nil || persistedOp.DesiredRevision <= 0 {
			return fmt.Errorf("switchRecovery: persisted operation revision is invalid op=%s", op.ID)
		}
		op.DesiredRevision = persistedOp.DesiredRevision
		desiredRevision = persistedOp.DesiredRevision
	}
	if j.NewDesiredRevision <= 0 {
		persistedJournal, err := r.repo.SetSwitchJournalDesiredRevisionIfMissing(op.ID, desiredRevision, r.worker.executionID)
		if err != nil {
			return fmt.Errorf("switchRecovery: persist recovered journal revision failed op=%s: %w", op.ID, err)
		}
		if persistedJournal == nil || persistedJournal.NewDesiredRevision <= 0 {
			return fmt.Errorf("switchRecovery: persisted journal revision is invalid op=%s", op.ID)
		}
		j.NewDesiredRevision = persistedJournal.NewDesiredRevision
	}
	if op.DesiredRevision != j.NewDesiredRevision {
		return fmt.Errorf("switchRecovery: desired revision conflict after recovery op=%s operation=%d journal=%d", op.ID, op.DesiredRevision, j.NewDesiredRevision)
	}
	return nil
}

func (r *SwitchRecovery) recoverFromBindingCommitted(ctx context.Context, op *operation.InstallationOperation, j *RecoverySwitchJournal) error {
	desiredRevision := j.NewDesiredRevision
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
	if _, err := r.repo.CompleteOperation(op.ID, op.Stage, op.Status, r.worker.executionID); err != nil {
		return fmt.Errorf("switchRecovery: complete operation op=%s: %w", op.ID, err)
	}
	op.Stage = operation.OpStageCompleted
	op.Status = operation.OpStatusCompleted
	return nil
}

func (r *SwitchRecovery) recoverFromSwitchCompleted(op *operation.InstallationOperation) error {
	if op.Stage == operation.OpStageCompleted && op.Status == operation.OpStatusCompleted {
		return nil
	}
	if op.IsTerminal() && op.Status != operation.OpStatusCompleted {
		return fmt.Errorf("switchRecovery: switch-completed journal conflicts with terminal operation op=%s status=%s", op.ID, op.Status)
	}
	if _, err := r.repo.CompleteOperation(op.ID, op.Stage, op.Status, r.worker.executionID); err != nil {
		return fmt.Errorf("switchRecovery: finish switch-completed operation op=%s: %w", op.ID, err)
	}
	op.Stage = operation.OpStageCompleted
	op.Status = operation.OpStatusCompleted
	return nil
}

var _ = fmt.Errorf
