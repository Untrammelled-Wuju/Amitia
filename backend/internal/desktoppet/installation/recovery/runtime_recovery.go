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

type RuntimeRecovery struct {
	worker      *RecoveryWorker
	repo        RecoveryRepo
	runtimeRepo RuntimeRepo
	finalizer   RuntimeAppliedFinalizer
}

type RuntimeAppliedFinalizer interface {
	FinalizeRuntimeApplied(ctx context.Context, op *operation.InstallationOperation) error
}

type RuntimeRepo interface {
	SendDesiredCommand(ctx context.Context, opID, userID, deviceID, runtimeID, installationID string, desiredRevision int64) error
	CancelDesiredCommand(ctx context.Context, opID, userID, deviceID, runtimeID string) error
	QueryRuntimeAppliedState(ctx context.Context, userID, deviceID, runtimeID string) (appliedRevision int64, actualReleaseID string, err error)
	MarkRuntimeApplied(opID string, appliedRevision int64) error
	QueryCommandTerminalStatus(ctx context.Context, commandID string) (status string, found bool, err error)
}

func NewRuntimeRecovery(worker *RecoveryWorker, repo RecoveryRepo, runtimeRepo RuntimeRepo, finalizers ...RuntimeAppliedFinalizer) *RuntimeRecovery {
	r := &RuntimeRecovery{
		worker:      worker,
		repo:        repo,
		runtimeRepo: runtimeRepo,
	}
	if len(finalizers) > 0 {
		r.finalizer = finalizers[0]
	}
	return r
}

func (r *RuntimeRecovery) Recover(ctx context.Context, op *operation.InstallationOperation, j *RecoveryCommitJournal) error {
	if r.runtimeRepo == nil {
		return errors.New("runtimeRecovery: runtimeRepo not configured")
	}
	switch j.Stage {
	case operation.OpStageDesiredStateCommitted, operation.OpStageRuntimeCommandEnqueued:
		return r.recoverFromDesiredStateCommitted(ctx, op, j)
	case operation.OpStageWaitingRuntimeACK:
		return r.recoverFromWaitingRuntimeAck(ctx, op, j)
	case operation.OpStageRuntimeApplied:
		return r.recoverFromRuntimeApplied(ctx, op, j)
	default:
		return fmt.Errorf("%w: %s", ErrInvalidStage, j.Stage)
	}
}

func (r *RuntimeRecovery) CancelOperation(ctx context.Context, op *operation.InstallationOperation) error {
	if r == nil || r.runtimeRepo == nil {
		return errors.New("runtimeRecovery: runtimeRepo not configured")
	}
	return r.runtimeRepo.CancelDesiredCommand(ctx, op.ID, op.UserID, op.DeviceID, op.RuntimeID)
}

func (r *RuntimeRecovery) recoverFromDesiredStateCommitted(ctx context.Context, op *operation.InstallationOperation, j *RecoveryCommitJournal) error {
	desiredRevision := op.DesiredRevision
	if desiredRevision == 0 {
		desiredRevision = time.Now().UnixNano()
		op.DesiredRevision = desiredRevision
	}
	if err := r.runtimeRepo.SendDesiredCommand(ctx, op.ID, op.UserID, op.DeviceID, op.RuntimeID, op.InstallationID, desiredRevision); err != nil {
		return fmt.Errorf("runtimeRecovery: send command failed op=%s: %w", op.ID, err)
	}
	if _, err := r.repo.CASUpdateCommitJournalStage(op.ID, j.Stage, operation.OpStageWaitingRuntimeACK, r.worker.executionID); err != nil {
		if !errors.Is(err, ErrJournalNotFound) {
			return fmt.Errorf("runtimeRecovery: CAS update to waiting_runtime_ack failed: %w", err)
		}
	}
	return nil
}

func (r *RuntimeRecovery) recoverFromWaitingRuntimeAck(ctx context.Context, op *operation.InstallationOperation, j *RecoveryCommitJournal) error {
	if op.OperationType == operation.TypeRecenter {
		return r.recoverRecenterFromWaitingRuntimeAck(ctx, op, j)
	}

	appliedRevision, _, err := r.runtimeRepo.QueryRuntimeAppliedState(ctx, op.UserID, op.DeviceID, op.RuntimeID)
	if err != nil {
		if err := r.runtimeRepo.SendDesiredCommand(ctx, op.ID, op.UserID, op.DeviceID, op.RuntimeID, op.InstallationID, op.DesiredRevision); err != nil {
			return fmt.Errorf("runtimeRecovery: re-send command failed op=%s: %w", op.ID, err)
		}
		return nil
	}
	if appliedRevision >= op.DesiredRevision && op.DesiredRevision > 0 {
		if err := r.runtimeRepo.MarkRuntimeApplied(op.ID, appliedRevision); err != nil {
			return fmt.Errorf("runtimeRecovery: mark applied failed op=%s: %w", op.ID, err)
		}
		if _, err := r.repo.CASUpdateCommitJournalStage(op.ID, operation.OpStageWaitingRuntimeACK, operation.OpStageRuntimeApplied, r.worker.executionID); err != nil {
			if !errors.Is(err, ErrJournalNotFound) {
				return fmt.Errorf("runtimeRecovery: CAS update to runtime_applied failed: %w", err)
			}
		}
		return r.recoverFromRuntimeApplied(ctx, op, j)
	}
	if err := r.runtimeRepo.SendDesiredCommand(ctx, op.ID, op.UserID, op.DeviceID, op.RuntimeID, op.InstallationID, op.DesiredRevision); err != nil {
		return fmt.Errorf("runtimeRecovery: re-send command failed op=%s: %w", op.ID, err)
	}
	return nil
}

func (r *RuntimeRecovery) recoverRecenterFromWaitingRuntimeAck(ctx context.Context, op *operation.InstallationOperation, j *RecoveryCommitJournal) error {
	if op.ExpectedAppliedRevision > 0 {
		status, found, err := r.runtimeRepo.QueryCommandTerminalStatus(ctx, fmt.Sprintf("%s", op.ExpectedAppliedRevision))
		if err != nil {
			if sendErr := r.runtimeRepo.SendDesiredCommand(ctx, op.ID, op.UserID, op.DeviceID, op.RuntimeID, op.InstallationID, op.DesiredRevision); sendErr != nil {
				return fmt.Errorf("runtimeRecovery: re-send recenter command failed op=%s: %w", op.ID, sendErr)
			}
			return nil
		}
		if found {
			if status == "completed" {
				if _, err := r.repo.CASUpdateCommitJournalStage(op.ID, operation.OpStageWaitingRuntimeACK, operation.OpStageRuntimeApplied, r.worker.executionID); err != nil {
					if !errors.Is(err, ErrJournalNotFound) {
						return fmt.Errorf("runtimeRecovery: CAS update to runtime_applied failed: %w", err)
					}
				}
				return r.recoverFromRuntimeApplied(ctx, op, j)
			}
			if status == "failed_terminal" {
				if _, err := r.repo.UpdateOperationStatus(op.ID, op.Status, operation.OpStatusFailedTerminal, r.worker.executionID); err != nil {
					return fmt.Errorf("runtimeRecovery: update op to failed_terminal failed op=%s: %w", op.ID, err)
				}
				return nil
			}
		}
	}
	if err := r.runtimeRepo.SendDesiredCommand(ctx, op.ID, op.UserID, op.DeviceID, op.RuntimeID, op.InstallationID, op.DesiredRevision); err != nil {
		return fmt.Errorf("runtimeRecovery: re-send recenter command failed op=%s: %w", op.ID, err)
	}
	return nil
}

func (r *RuntimeRecovery) recoverFromRuntimeApplied(ctx context.Context, op *operation.InstallationOperation, j *RecoveryCommitJournal) error {
	if r.finalizer != nil {
		if err := r.finalizer.FinalizeRuntimeApplied(ctx, op); err != nil {
			return fmt.Errorf("runtimeRecovery: finalize runtime-applied operation op=%s: %w", op.ID, err)
		}
	}
	if _, err := r.repo.CASUpdateCommitJournalStage(op.ID, operation.OpStageRuntimeApplied, operation.OpStageCleanupCompleted, r.worker.executionID); err != nil {
		if !errors.Is(err, ErrJournalNotFound) {
			return fmt.Errorf("runtimeRecovery: CAS update to cleanup_completed failed: %w", err)
		}
	}
	if _, err := r.repo.UpdateOperationStatus(op.ID, op.Status, operation.OpStatusCompleted, r.worker.executionID); err != nil {
		return fmt.Errorf("runtimeRecovery: update op to completed failed op=%s: %w", op.ID, err)
	}
	return nil
}

var _ = fmt.Errorf
