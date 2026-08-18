// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package recovery

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/log"

	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
)

var (
	ErrRecoveryLeaseLost = errors.New("recovery: lease lost during recovery")
	ErrJournalNotFound   = errors.New("recovery: journal not found for operation")
	ErrInvalidStage      = errors.New("recovery: invalid journal stage")
	ErrCASConflict       = errors.New("recovery: CAS conflict on journal stage")
)

const (
	DefaultLeaseTTL          = 5 * time.Minute
	DefaultScanInterval      = 30 * time.Second
	DefaultMaxRecoverBatch   = 50
	DefaultLeaseSafetyMargin = 30 * time.Second
)

type RecoveryRepo interface {
	ListExpiredLeaseOperations(leaseTimeout string, limit int) ([]*operation.InstallationOperation, error)
	RenewOperationLease(operationID, executionID string) error
	ReleaseOperationLease(operationID, executionID string) error
	ClaimOperationLease(operationID, owner string, ttl time.Duration, expectedStatuses []string) (*operation.InstallationOperation, error)
	UpdateOperationStatus(operationID, oldStatus, newStatus, executionID string) (*operation.InstallationOperation, error)
	CASUpdateOperationStage(operationID, expectedStage, newStage, executionID string) (*operation.InstallationOperation, error)
	CompleteOperation(operationID, expectedStage, expectedStatus, executionID string) (*operation.InstallationOperation, error)
	GetCommitJournal(operationID string) (*RecoveryCommitJournal, error)
	GetSwitchJournal(operationID string) (*RecoverySwitchJournal, error)
	CASUpdateCommitJournalStage(operationID, expectedStage, newStage, executionID string) (*RecoveryCommitJournal, error)
	CASUpdateSwitchJournalStage(operationID, expectedStage, newStage, executionID string) (*RecoverySwitchJournal, error)
}

type RecoveryCommitJournal struct {
	ID               string
	OperationID      string
	Stage            string
	Status           string
	StagingPathKey   string
	TargetReleaseID  string
	PetID            string
	PublishedPathKey string
	ErrorMessage     string
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
	repo         RecoveryRepo
	leaseTTL     time.Duration
	scanInterval time.Duration
	maxBatch     int
	executionID  string

	stagingRecovery       *StagingRecovery
	dbRecovery            *DBRecovery
	runtimeRecovery       *RuntimeRecovery
	switchRecovery        *SwitchRecovery
	desiredStateFinalizer DesiredStateFinalizer
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

func WithDesiredStateFinalizer(f DesiredStateFinalizer) RecoveryWorkerOption {
	return func(w *RecoveryWorker) { w.desiredStateFinalizer = f }
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

func (w *RecoveryWorker) ConfigureRecoveries(staging *StagingRecovery, db *DBRecovery, runtime *RuntimeRecovery, sw *SwitchRecovery, desiredStateFinalizer DesiredStateFinalizer) {
	if w == nil {
		return
	}
	w.stagingRecovery = staging
	w.dbRecovery = db
	w.runtimeRecovery = runtime
	w.switchRecovery = sw
	w.desiredStateFinalizer = desiredStateFinalizer
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
				log.Warn("installation recovery: scan failed: ", err)
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
			log.Warn("installation recovery: operation ", op.ID, " failed: ", err)
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
	if op.Status == operation.OpStatusCancelRequested {
		return w.recoverCancelOperation(ctx, op)
	}
	switch op.OperationType {
	case operation.TypeInstall, operation.TypeRepair:
		return w.recoverCommitOperation(ctx, op, lease)
	case operation.TypeSwitch, operation.TypeUpgrade, operation.TypeDowngrade:
		return w.recoverSwitchOperation(ctx, op, lease)
	case operation.TypeUninstall, operation.TypeEnable, operation.TypeDisable, operation.TypeRecenter, operation.TypeSettings, operation.TypeDefaultAction:
		return w.recoverDesiredStateOperation(ctx, op, lease)
	default:
		return fmt.Errorf("installation recovery: unsupported operation type %s", op.OperationType)
	}
}

func (w *RecoveryWorker) recoverCancelOperation(ctx context.Context, op *operation.InstallationOperation) error {
	if op.Status == operation.OpStatusCompleted || op.Status == operation.OpStatusFailedTerminal || op.Status == operation.OpStatusCancelled {
		return nil
	}
	if op.Stage == operation.OpStageRuntimeCommandEnqueued || op.Stage == operation.OpStageWaitingRuntimeACK || op.Stage == operation.OpStageRuntimeApplied {
		if w.runtimeRecovery == nil {
			return errors.New("installation recovery: runtime recovery is required to cancel an in-flight runtime operation")
		}
		if err := w.runtimeRecovery.CancelOperation(ctx, op); err != nil {
			return err
		}
	}
	if _, err := w.repo.UpdateOperationStatus(op.ID, op.Status, operation.OpStatusCancelled, w.executionID); err != nil {
		if op.Status == operation.OpStatusCancelRequested {
			return nil
		}
		return err
	}
	return nil
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
	if err := w.repo.ReleaseOperationLease(operationID, w.executionID); err != nil {
		log.Warn("installation recovery: release operation lease failed: ", err)
	}
}

func (w *RecoveryWorker) recoverCommitOperation(ctx context.Context, op *operation.InstallationOperation, lease *operation.Lease) error {
	journal, err := w.repo.GetCommitJournal(op.ID)
	if err != nil {
		return fmt.Errorf("installation recovery: load commit journal: %w", err)
	}
	if isStagingStage(journal.Stage) {
		if w.stagingRecovery == nil {
			return errors.New("installation recovery: staging recovery is not configured")
		}
		if err := w.stagingRecovery.Recover(ctx, op, journal); err != nil {
			return err
		}
	}
	journal, err = w.repo.GetCommitJournal(op.ID)
	if err != nil {
		return fmt.Errorf("installation recovery: reload commit journal: %w", err)
	}
	if isDBStage(journal.Stage) {
		if w.dbRecovery == nil {
			return errors.New("installation recovery: database recovery is not configured")
		}
		if err := w.dbRecovery.Recover(ctx, op, journal); err != nil {
			return err
		}
	}
	journal, err = w.repo.GetCommitJournal(op.ID)
	if err != nil {
		return fmt.Errorf("installation recovery: reload commit journal: %w", err)
	}
	if isRuntimeStage(journal.Stage) {
		if w.runtimeRecovery == nil {
			return errors.New("installation recovery: runtime recovery is not configured")
		}
		if err := w.runtimeRecovery.Recover(ctx, op, journal); err != nil {
			return err
		}
	}
	return nil
}

func (w *RecoveryWorker) recoverSwitchOperation(ctx context.Context, op *operation.InstallationOperation, lease *operation.Lease) error {
	journal, err := w.repo.GetSwitchJournal(op.ID)
	if err != nil {
		return fmt.Errorf("installation recovery: load switch journal: %w", err)
	}
	if w.switchRecovery == nil {
		return errors.New("installation recovery: switch recovery is not configured")
	}
	return w.switchRecovery.Recover(ctx, op, journal)
}

type DesiredStateFinalizer interface {
	FinalizeDesiredStateApplied(ctx context.Context, op *operation.InstallationOperation) error
}

func (w *RecoveryWorker) ConfigureDesiredStateFinalizer(finalizer DesiredStateFinalizer) {
	if w == nil {
		return
	}
	w.desiredStateFinalizer = finalizer
}

func (w *RecoveryWorker) recoverDesiredStateOperation(ctx context.Context, op *operation.InstallationOperation, lease *operation.Lease) error {
	switch op.Stage {
	case operation.OpStageDesiredStateCommitted:
		return w.recoverDesiredStateOperationFromCommitted(ctx, op, lease)
	case operation.OpStageRuntimeCommandEnqueued, operation.OpStageWaitingRuntimeACK:
		if w.runtimeRecovery == nil {
			return errors.New("installation recovery: runtime recovery is not configured")
		}
		return w.runtimeRecovery.Recover(ctx, op, &RecoveryCommitJournal{
			OperationID: op.ID,
			Stage:       op.Stage,
		})
	case operation.OpStageRuntimeApplied:
		return w.recoverDesiredStateOperationFromRuntimeApplied(ctx, op, lease)
	default:
		return fmt.Errorf("installation recovery: active desired-state operation %s has unexpected stage %s; manual review required", op.ID, op.Stage)
	}
}

func (w *RecoveryWorker) recoverDesiredStateOperationFromCommitted(ctx context.Context, op *operation.InstallationOperation, lease *operation.Lease) error {
	if w.runtimeRecovery == nil {
		return errors.New("installation recovery: runtime recovery is not configured")
	}
	return w.runtimeRecovery.Recover(ctx, op, &RecoveryCommitJournal{
		OperationID: op.ID,
		Stage:       operation.OpStageDesiredStateCommitted,
	})
}

func (w *RecoveryWorker) recoverDesiredStateOperationFromRuntimeApplied(ctx context.Context, op *operation.InstallationOperation, lease *operation.Lease) error {
	if w.desiredStateFinalizer == nil {
		return errors.New("installation recovery: desired-state finalizer is not configured")
	}
	if err := w.desiredStateFinalizer.FinalizeDesiredStateApplied(ctx, op); err != nil {
		return fmt.Errorf("installation recovery: finalize desired-state operation op=%s: %w", op.ID, err)
	}
	if _, err := w.repo.CompleteOperation(op.ID, operation.OpStageRuntimeApplied, op.Status, w.executionID); err != nil {
		return fmt.Errorf("installation recovery: complete runtime-applied operation op=%s: %w", op.ID, err)
	}
	op.Stage = operation.OpStageCompleted
	op.Status = operation.OpStatusCompleted
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
	return "recv-" + uuid.NewString()
}
