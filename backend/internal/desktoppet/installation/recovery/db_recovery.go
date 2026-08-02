// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package recovery

import (
	"context"
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
)

type DBRecovery struct {
	worker *RecoveryWorker
	repo   RecoveryRepo
	dbRepo DBRepo
}

type DBRepo interface {
	DBCommitBatch(opID, installationID, targetReleaseID, previousReleaseID string) error
	DBMarkDatabaseCommitted(opID string) error
	GetInstallation(installationID string) (interface{}, error)
}

func NewDBRecovery(worker *RecoveryWorker, repo RecoveryRepo, dbRepo DBRepo) *DBRecovery {
	return &DBRecovery{
		worker: worker,
		repo:   repo,
		dbRepo: dbRepo,
	}
}

func (r *DBRecovery) Recover(ctx context.Context, op *operation.InstallationOperation, j *RecoveryCommitJournal) error {
	if r.dbRepo == nil {
		return errors.New("dbRecovery: dbRepo not configured")
	}
	switch j.Stage {
	case operation.OpStageFilesPublished:
		return r.recoverFromFilesPublished(ctx, op, j)
	case operation.OpStageDatabaseCommitted:
		return r.recoverFromDatabaseCommitted(ctx, op, j)
	case operation.OpStageDesiredStateCommitted:
		return r.recoverFromDesiredStateCommitted(ctx, op, j)
	default:
		return fmt.Errorf("%w: %s", ErrInvalidStage, j.Stage)
	}
}

func (r *DBRecovery) recoverFromFilesPublished(ctx context.Context, op *operation.InstallationOperation, j *RecoveryCommitJournal) error {
	if err := r.dbRepo.DBCommitBatch(op.ID, op.InstallationID, j.TargetReleaseID, op.SourceReleaseID); err != nil {
		return fmt.Errorf("dbRecovery: DB commit batch failed op=%s: %w", op.ID, err)
	}
	if err := r.dbRepo.DBMarkDatabaseCommitted(op.ID); err != nil {
		return fmt.Errorf("dbRecovery: DB mark committed failed op=%s: %w", op.ID, err)
	}
	if _, err := r.repo.CASUpdateCommitJournalStage(op.ID, operation.OpStageFilesPublished, operation.OpStageDatabaseCommitted, r.worker.executionID); err != nil {
		if !errors.Is(err, ErrJournalNotFound) {
			return fmt.Errorf("dbRecovery: CAS update to database_committed failed: %w", err)
		}
	}
	return r.recoverFromDatabaseCommitted(ctx, op, j)
}

func (r *DBRecovery) recoverFromDatabaseCommitted(ctx context.Context, op *operation.InstallationOperation, j *RecoveryCommitJournal) error {
	if _, err := r.repo.CASUpdateCommitJournalStage(op.ID, operation.OpStageDatabaseCommitted, operation.OpStageDesiredStateCommitted, r.worker.executionID); err != nil {
		if !errors.Is(err, ErrJournalNotFound) {
			return fmt.Errorf("dbRecovery: CAS update to desired_state_committed failed: %w", err)
		}
	}
	return r.recoverFromDesiredStateCommitted(ctx, op, j)
}

func (r *DBRecovery) recoverFromDesiredStateCommitted(ctx context.Context, op *operation.InstallationOperation, j *RecoveryCommitJournal) error {
	if _, err := r.repo.CASUpdateCommitJournalStage(op.ID, operation.OpStageDesiredStateCommitted, operation.OpStageRuntimeCommandEnqueued, r.worker.executionID); err != nil {
		if !errors.Is(err, ErrJournalNotFound) {
			return fmt.Errorf("dbRecovery: CAS update to runtime_command_enqueued failed: %w", err)
		}
	}
	return nil
}

var _ = fmt.Errorf
