// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package recovery

import (
	"context"
	"errors"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
)

type StagingRecovery struct {
	worker      *RecoveryWorker
	repo        RecoveryRepo
	stagingRepo StagingRepo
}

type StagingRepo interface {
	CleanupStaging(opID string) error
	ReprepareStaging(opID, stagingPathKey, targetReleaseID string) (string, error)
	VerifyStagingIntegrity(stagingPathKey, targetReleaseID string) (bool, error)
	PublishStaging(stagingPathKey, targetReleaseID string) (string, error)
}

func NewStagingRecovery(worker *RecoveryWorker, repo RecoveryRepo, stagingRepo StagingRepo) *StagingRecovery {
	return &StagingRecovery{
		worker:      worker,
		repo:        repo,
		stagingRepo: stagingRepo,
	}
}

func (r *StagingRecovery) Recover(ctx context.Context, op *operation.InstallationOperation, j *RecoveryCommitJournal) error {
	if r.stagingRepo == nil {
		return errors.New("stagingRecovery: stagingRepo not configured")
	}
	switch j.Stage {
	case operation.OpStageReleaseVerified:
		return r.recoverFromReleaseVerified(ctx, op, j)
	case operation.OpStageStagingPrepared:
		return r.recoverFromStagingPrepared(ctx, op, j)
	case operation.OpStageStagingVerified:
		return r.recoverFromStagingVerified(ctx, op, j)
	default:
		return fmt.Errorf("%w: %s", ErrInvalidStage, j.Stage)
	}
}

func (r *StagingRecovery) recoverFromReleaseVerified(ctx context.Context, op *operation.InstallationOperation, j *RecoveryCommitJournal) error {
	if err := r.stagingRepo.CleanupStaging(op.ID); err != nil {
		return fmt.Errorf("stagingRecovery: cleanup failed op=%s: %w", op.ID, err)
	}
	newStagingKey, err := r.stagingRepo.ReprepareStaging(op.ID, j.StagingPathKey, j.TargetReleaseID)
	if err != nil {
		return fmt.Errorf("stagingRecovery: reprepare failed op=%s: %w", op.ID, err)
	}
	if _, err := r.repo.CASUpdateCommitJournalStage(op.ID, operation.OpStageReleaseVerified, operation.OpStageStagingPrepared, r.worker.executionID); err != nil {
		return fmt.Errorf("stagingRecovery: CAS update to staging_prepared failed: %w", err)
	}
	j.StagingPathKey = newStagingKey
	return r.recoverFromStagingPrepared(ctx, op, j)
}

func (r *StagingRecovery) recoverFromStagingPrepared(ctx context.Context, op *operation.InstallationOperation, j *RecoveryCommitJournal) error {
	valid, err := r.stagingRepo.VerifyStagingIntegrity(j.StagingPathKey, j.TargetReleaseID)
	if err != nil {
		return fmt.Errorf("stagingRecovery: verify integrity failed op=%s: %w", op.ID, err)
	}
	if !valid {
		newStagingKey, reErr := r.stagingRepo.ReprepareStaging(op.ID, j.StagingPathKey, j.TargetReleaseID)
		if reErr != nil {
			return fmt.Errorf("stagingRecovery: reprepare after verify-fail failed op=%s: %w", op.ID, reErr)
		}
		j.StagingPathKey = newStagingKey
	}
	if _, err := r.repo.CASUpdateCommitJournalStage(op.ID, operation.OpStageStagingPrepared, operation.OpStageStagingVerified, r.worker.executionID); err != nil {
		if !errors.Is(err, ErrJournalNotFound) {
			return fmt.Errorf("stagingRecovery: CAS update to staging_verified failed: %w", err)
		}
	}
	return r.recoverFromStagingVerified(ctx, op, j)
}

func (r *StagingRecovery) recoverFromStagingVerified(ctx context.Context, op *operation.InstallationOperation, j *RecoveryCommitJournal) error {
	if err := r.stagingRepo.CleanupStaging(op.ID); err != nil {
		return fmt.Errorf("stagingRecovery: cleanup before skip-staging failed op=%s: %w", op.ID, err)
	}
	if _, err := r.repo.CASUpdateCommitJournalStage(op.ID, operation.OpStageStagingVerified, operation.OpStageOldInstallParked, r.worker.executionID); err != nil {
		if !errors.Is(err, ErrJournalNotFound) {
			return fmt.Errorf("stagingRecovery: CAS update to old_install_parked failed: %w", err)
		}
	}
	return nil
}

var _ = fmt.Errorf
