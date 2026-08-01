// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package recovery

import (
	"context"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
)

var (
	ErrRecoveryLeaseLost = errors.New("recovery: lease lost during recovery")
	ErrJournalNotFound   = errors.New("recovery: journal not found for operation")
	ErrInvalidStage      = errors.New("recovery: invalid journal stage")
	ErrCASConflict       = errors.New("recovery: CAS conflict on journal stage")
)

const (
	DefaultLeaseTTL        = 5 * time.Minute
	DefaultScanInterval    = 30 * time.Second
	DefaultMaxRecoverBatch = 50
	DefaultLeaseSafetyMargin = 30 * time.Second
)

type RecoveryRepo interface {
	ListExpiredLeaseOperations(leaseTimeout string, limit int) ([]*operation.InstallationOperation, error)
	RenewOperationLease(operationID, executionID string) error
	ClaimOperationLease(operationID, owner string, ttl time.Duration, expectedStatuses []string) (*operation.InstallationOperation, error)
	UpdateOperationStatus(operationID, oldStatus, newStatus, executionID string) (*operation.InstallationOperation, error)
	GetCommitJournal(operationID string) (*RecoveryCommitJournal, error)
	GetSwitchJournal(operationID string) (*RecoverySwitchJournal, error)
	CASUpdateCommitJournalStage(operationID, expectedStage, newStage, executionID string) (*RecoveryCommitJournal, error)
	CASUpdateSwitchJournalStage(operationID, expectedStage, newStage, executionID string) (*RecoverySwitchJournal, error)
}

type RecoveryCommitJournal struct {
	ID              string
	OperationID     string
	Stage           string
	Status          string
	StagingPathKey  string
	TargetReleaseID string
	PetID           string
	PublishedPathKey string
	ErrorMessage    string
}

type RecoverySwitchJournal struct {
	ID                 string
	OperationID        string
	Stage              string
	Status             string
	NewInstallationID  string
	NewBindingRevision int64
	NewDesiredRevision int64
}

type RecoveryWorker struct {
	repo          RecoveryRepo
	leaseTTL      time.Duration
	scanInterval  time.Duration
	maxBatch      int
	executionID   string

	stagingRecovery  *StagingRecovery
	dbRecovery       *DBRecovery
	runtimeRecovery  *RuntimeRecovery
	switchRecovery   *SwitchRecovery
}

type RecoveryWorkerOption func(*RecoveryWorker)

func WithStagingRecovery(r *StagingRecovery) RecoveryWorkerOption {
	return func(w *RecoveryWorker) { w.stagingRecovery = r }
}

func WithDBRecovery(r *DBRecovery) RecoveryWorkerOption {
	return func(w *RecoveryWorker) { w.dbRecovery = r }
}

func WithRuntimeRecovery(r *RuntimeRecovery) RecoveryWorkerOption {
	return func(w *RecoveryWorker) { w.runtimeRecovery = r }
}

func WithSwitchRecovery(r *SwitchRecovery) RecoveryWorkerOption {
	return func(w *RecoveryWorker) { w.switchRecovery = r }
}

func NewRecoveryWorker(repo RecoveryRepo, opts ...RecoveryWorkerOption) *RecoveryWorker {
	w := &RecoveryWorker{
		repo:         repo,
		leaseTTL:     DefaultLeaseTTL,
		scanInterval: DefaultScanInterval,
		maxBatch:     DefaultMaxRecoverBatch,
		executionID:  generateRecoveryID(),
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

func (w *RecoveryWorker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.scanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				continue
			}
		}
	}
}

func (w *RecoveryWorker) tick(ctx context.Context) error {
	leaseTimeout := time.Now().Add(-w.leaseTTL).Format("2006-01-02 15:04:05")
	pendingOps, err := w.repo.ListExpiredLeaseOperations(leaseTimeout, w.maxBatch)
	if err != nil {
		return err
	}
	for _, op := range pendingOps {
		if err := w.recoverOperation(ctx, op); err != nil {
			continue
		}
	}
	return nil
}

func (w *RecoveryWorker) recoverOperation(ctx context.Context, op *operation.InstallationOperation) error {
	if !op.IsActive() {
		return nil
	}
	lease, err := w.tryClaimLease(op)
	if err != nil {
		return err
	}
	defer w.safeReleaseLease(op.ID)
	switch op.OperationType {
	case operation.TypeInstall, operation.TypeUpgrade, operation.TypeDowngrade, operation.TypeRepair:
		return w.recoverCommitOperation(ctx, op, lease)
	case operation.TypeSwitch:
		return w.recoverSwitchOperation(ctx, op, lease)
	default:
		return nil
	}
}

func (w *RecoveryWorker) tryClaimLease(op *operation.InstallationOperation) (*operation.Lease, error) {
	claimedOp, err := w.repo.ClaimOperationLease(op.ID, w.executionID, w.leaseTTL, []string{op.Status})
	if err != nil {
		return nil, err
	}
	return &operation.Lease{
		OperationID: claimedOp.ID,
		Owner:       w.executionID,
		ExpiresAt:   time.Now().Add(w.leaseTTL),
		HeartbeatAt: time.Now(),
	}, nil
}

func (w *RecoveryWorker) safeReleaseLease(operationID string) {
	if w.repo == nil {
		return
	}
	_ = w.repo.RenewOperationLease(operationID, "")
}

func (w *RecoveryWorker) recoverCommitOperation(ctx context.Context, op *operation.InstallationOperation, lease *operation.Lease) error {
	journal, err := w.repo.GetCommitJournal(op.ID)
	if err != nil {
		return nil
	}
	if w.stagingRecovery != nil && isStagingStage(journal.Stage) {
		if err := w.stagingRecovery.Recover(ctx, op, journal); err != nil {
			return err
		}
	}
	journal, err = w.repo.GetCommitJournal(op.ID)
	if err != nil {
		return nil
	}
	if w.dbRecovery != nil && isDBStage(journal.Stage) {
		if err := w.dbRecovery.Recover(ctx, op, journal); err != nil {
			return err
		}
	}
	journal, err = w.repo.GetCommitJournal(op.ID)
	if err != nil {
		return nil
	}
	if w.runtimeRecovery != nil && isRuntimeStage(journal.Stage) {
		if err := w.runtimeRecovery.Recover(ctx, op, journal); err != nil {
			return err
		}
	}
	return nil
}

func (w *RecoveryWorker) recoverSwitchOperation(ctx context.Context, op *operation.InstallationOperation, lease *operation.Lease) error {
	journal, err := w.repo.GetSwitchJournal(op.ID)
	if err != nil {
		return nil
	}
	if w.switchRecovery != nil {
		return w.switchRecovery.Recover(ctx, op, journal)
	}
	return nil
}

func isStagingStage(stage string) bool {
	switch stage {
	case operation.OpStageReleaseVerified,
		operation.OpStageStagingPrepared,
		operation.OpStageStagingVerified:
		return true
	}
	return false
}

func isDBStage(stage string) bool {
	switch stage {
	case operation.OpStageFilesPublished,
		operation.OpStageDatabaseCommitted,
		operation.OpStageDesiredStateCommitted:
		return true
	}
	return false
}

func isRuntimeStage(stage string) bool {
	switch stage {
	case operation.OpStageRuntimeCommandEnqueued,
		operation.OpStageWaitingRuntimeACK,
		operation.OpStageRuntimeApplied,
		operation.OpStageCleanupCompleted:
		return true
	}
	return false
}

func generateRecoveryID() string {
	return "recv-" + time.Now().Format("20060102") + "-" + time.Now().Format("150405")
}
