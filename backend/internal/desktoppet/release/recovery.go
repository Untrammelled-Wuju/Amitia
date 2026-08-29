package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"github.com/u-ai/backend/internal/desktoppet/security"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

type ReleaseRecoveryWorker struct {
	repo           ReleaseRepository
	stagingRepo    security.ImportStagingRepository
	leaseManager   LeaseManagerPort
	journalManager JournalManagerPort
	storage        ReleaseStoragePort
	eventPublisher EventPublisher
	checkInterval  time.Duration
	stopCh         chan struct{}
	wg             sync.WaitGroup
	mu             sync.Mutex
	running        bool
	alive          atomic.Bool
}

type LeaseManagerPort interface {
	IsLeaseExpired(op *ReleaseBuildOperation) bool
	ReleaseLease(op *ReleaseBuildOperation)
}

type JournalManagerPort interface {
	GetByOperation(operationID string) (*ReleasePublishJournal, error)
	MarkFailed(journal *ReleasePublishJournal, errMsg string) error
	UpdateStage(journal *ReleasePublishJournal, stage, contentRootHash, stagingPath, publishedPath string) error
}

func NewReleaseRecoveryWorker(
	repo ReleaseRepository,
	stagingRepo security.ImportStagingRepository,
	leaseManager LeaseManagerPort,
	journalManager JournalManagerPort,
	storage ReleaseStoragePort,
	eventPublisher EventPublisher,
) *ReleaseRecoveryWorker {
	return &ReleaseRecoveryWorker{
		repo:           repo,
		stagingRepo:    stagingRepo,
		leaseManager:   leaseManager,
		journalManager: journalManager,
		storage:        storage,
		eventPublisher: eventPublisher,
		checkInterval:  30 * time.Second,
		stopCh:         make(chan struct{}),
	}
}

func (w *ReleaseRecoveryWorker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.stopCh = make(chan struct{})
	w.alive.Store(true)
	w.wg.Add(1)
	w.mu.Unlock()

	go w.run(ctx)
	log.Logger.Info("Release recovery worker started")
}

func (w *ReleaseRecoveryWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	close(w.stopCh)
	w.wg.Wait()
	w.running = false
	w.alive.Store(false)
	log.Logger.Info("Release recovery worker stopped")
}

func (w *ReleaseRecoveryWorker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running && w.alive.Load()
}

func (w *ReleaseRecoveryWorker) run(ctx context.Context) {
	defer w.wg.Done()
	defer w.alive.Store(false)
	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.RecoverStaleOperations(ctx); err != nil {
				log.Logger.Warnf("Release recovery scan failed: %v", err)
			}
			if err := w.RecoverPendingJournals(ctx); err != nil {
				log.Logger.Warnf("Journal recovery scan failed: %v", err)
			}
			if err := w.RecoverImportOperations(ctx); err != nil {
				log.Logger.Warnf("Import recovery scan failed: %v", err)
			}
		}
	}
}

func (w *ReleaseRecoveryWorker) RecoverStaleOperations(ctx context.Context) error {
	staleOps, err := w.repo.ListStaleBuildOperations(formatRecoveryTimestamp(time.Now()))
	if err != nil {
		return fmt.Errorf("查询过期构建操作失败: %w", err)
	}
	for _, op := range staleOps {
		if op.State == BuildOpStateCompleted || op.State == BuildOpStateCancelled {
			continue
		}
		if !w.leaseManager.IsLeaseExpired(op) {
			continue
		}
		journal, journalErr := w.journalManager.GetByOperation(op.ID)
		if journalErr == nil && journal != nil && journal.OperationKind == string(JournalOperationImport) {
			continue
		}
		if err := w.recoverOperation(ctx, op); err != nil {
			log.Logger.Warnf("Failed to recover operation %s: %v", op.ID, err)
		}
	}
	return nil
}

func (w *ReleaseRecoveryWorker) recoverOperation(ctx context.Context, op *ReleaseBuildOperation) error {
	log.Logger.Infof("Recovering stale build operation %s (state=%s, stage=%s)", op.ID, op.State, op.Stage)

	switch op.State {
	case BuildOpStateSnapshotting, BuildOpStateCreated:
		op.State = BuildOpStateFailedRetryable
		op.ErrorCode = "LEASE_EXPIRED_DURING_SNAPSHOT"
		op.ErrorMessage = "租约在快照阶段过期"
		w.leaseManager.ReleaseLease(op)
		if err := w.updateOp(op); err != nil {
			return err
		}

	case BuildOpStateBuilding:
		op.State = BuildOpStateFailedRetryable
		op.ErrorCode = "LEASE_EXPIRED_DURING_BUILD"
		op.ErrorMessage = "租约在构建阶段过期"
		w.leaseManager.ReleaseLease(op)
		if err := w.updateOp(op); err != nil {
			return err
		}
		if op.StagingPathKey != "" && op.PetID != "" {
			if err := w.storage.RemoveStagingDir(op.StagingPathKey); err != nil {
				return fmt.Errorf("remove stale staging directory: %w", err)
			}
		}

	case BuildOpStateValidating:
		op.State = BuildOpStateFailedRetryable
		op.ErrorCode = "LEASE_EXPIRED_DURING_VALIDATION"
		op.ErrorMessage = "租约在验证阶段过期"
		w.leaseManager.ReleaseLease(op)
		if err := w.updateOp(op); err != nil {
			return err
		}

	case BuildOpStatePublishing:
		journal, err := w.journalManager.GetByOperation(op.ID)
		if err != nil || journal == nil {
			op.State = BuildOpStateFailedRetryable
			op.ErrorCode = "JOURNAL_NOT_FOUND"
			op.ErrorMessage = "发布日志未找到"
			w.leaseManager.ReleaseLease(op)
			if err := w.updateOp(op); err != nil {
				return err
			}
			return nil
		}
		switch journal.Stage {
		case JournalStageStagingBuilt, JournalStageValidated:
			if op.StagingPathKey != "" && op.PetID != "" {
				if err := w.storage.RemoveStagingDir(op.StagingPathKey); err != nil {
					return fmt.Errorf("remove stale staging directory: %w", err)
				}
			}
			op.State = BuildOpStateFailedRetryable
			op.ErrorCode = "LEASE_EXPIRED_BEFORE_PUBLISH"
			op.ErrorMessage = "租约在文件发布前过期"
			w.leaseManager.ReleaseLease(op)
			if err := w.updateOp(op); err != nil {
				return err
			}
			if err := w.journalManager.MarkFailed(journal, "租约在文件发布前过期"); err != nil {
				return fmt.Errorf("mark publish journal failed: %w", err)
			}

		case JournalStageFilesPublished:
			releaseData, err := w.repo.GetRelease(op.ReleaseID)
			if err == nil && releaseData != nil {
				op.State = BuildOpStateCompleted
				op.Stage = BuildOpStageDatabaseCommitted
				op.CompletedAt = formatRecoveryTimestamp(time.Now())
				w.leaseManager.ReleaseLease(op)
				if err := w.updateOp(op); err != nil {
					return err
				}
				if err := w.journalManager.UpdateStage(journal, JournalStageDatabaseCommitted,
					journal.ContentRootHash, "", journal.PublishedPath); err != nil {
					return fmt.Errorf("persist recovered database-committed journal stage: %w", err)
				}
				if err := w.journalManager.UpdateStage(journal, JournalStageCompleted,
					journal.ContentRootHash, "", journal.PublishedPath); err != nil {
					return fmt.Errorf("persist recovered completed journal stage: %w", err)
				}
				log.Logger.Infof("Operation %s recovered to completed", op.ID)
			} else {
				if op.PetID != "" && op.ReleaseID != "" {
					if cleanupErr := w.storage.RemovePublishedDir(op.PetID, op.ReleaseID); cleanupErr != nil {
						return fmt.Errorf("remove orphan published directory: %w", cleanupErr)
					}
				}
				op.State = BuildOpStateFailedTerminal
				op.ErrorCode = "RELEASE_RECORD_MISSING"
				op.ErrorMessage = "Release 记录不存在"
				w.leaseManager.ReleaseLease(op)
				if err := w.updateOp(op); err != nil {
					return err
				}
				if err := w.journalManager.MarkFailed(journal, "Release 记录不存在"); err != nil {
					return fmt.Errorf("mark missing-release journal failed: %w", err)
				}
			}

		case JournalStageDatabaseCommitted, JournalStageCompleted:
			op.State = BuildOpStateCompleted
			op.Stage = BuildOpStageDatabaseCommitted
			op.CompletedAt = formatRecoveryTimestamp(time.Now())
			w.leaseManager.ReleaseLease(op)
			if err := w.updateOp(op); err != nil {
				return err
			}

		default:
			op.State = BuildOpStateFailedRetryable
			op.ErrorCode = "UNKNOWN_JOURNAL_STAGE"
			op.ErrorMessage = fmt.Sprintf("未知日志阶段: %s", journal.Stage)
			w.leaseManager.ReleaseLease(op)
			if err := w.updateOp(op); err != nil {
				return err
			}
		}

	case BuildOpStateFailedRetryable:
		if op.RetryCount < 3 {
			op.RetryCount++
			op.State = BuildOpStateCreated
			op.ErrorCode = ""
			op.ErrorMessage = ""
			op.LeaseOwner = ""
			op.LeaseExpiresAt = ""
			if err := w.updateOp(op); err != nil {
				return err
			}
			log.Logger.Infof("Operation %s queued for retry (attempt %d)", op.ID, op.RetryCount)
		} else {
			op.State = BuildOpStateFailedTerminal
			op.CompletedAt = formatRecoveryTimestamp(time.Now())
			if err := w.updateOp(op); err != nil {
				return err
			}
			if op.PetID != "" && op.ReleaseID != "" {
				if err := w.storage.RemovePublishedDir(op.PetID, op.ReleaseID); err != nil {
					return fmt.Errorf("remove published directory after retry exhaustion: %w", err)
				}
			}
			if op.StagingPathKey != "" {
				if err := w.storage.RemoveStagingDir(op.StagingPathKey); err != nil {
					return fmt.Errorf("remove staging directory after retry exhaustion: %w", err)
				}
			}
			if w.eventPublisher != nil {
				if err := w.eventPublisher.PublishReleaseEvent(ReleaseEvent{
					EventType:  EventReleaseBuildFailed,
					UserID:     op.UserID,
					PetID:      op.PetID,
					ReleaseID:  op.ReleaseID,
					OccurredAt: formatRecoveryTimestamp(time.Now()),
				}); err != nil {
					return fmt.Errorf("publish terminal build failure event: %w", err)
				}
			}
		}
	}

	return nil
}

func (w *ReleaseRecoveryWorker) RecoverPendingJournals(ctx context.Context) error {
	journals, err := w.repo.ListPendingPublishJournals()
	if err != nil {
		return fmt.Errorf("查询未完成发布日志失败: %w", err)
	}
	for _, journal := range journals {
		if journal.Stage == JournalStageCompleted || journal.Stage == JournalStageFailed {
			continue
		}
		op, err := w.repo.GetBuildOperation(journal.OperationID)
		if err != nil {
			return fmt.Errorf("load operation %s for pending journal: %w", journal.OperationID, err)
		}
		if op == nil {
			return fmt.Errorf("operation %s for pending journal not found", journal.OperationID)
		}
		if op.State == BuildOpStateCompleted || op.State == BuildOpStateCancelled {
			if err := w.journalManager.UpdateStage(journal, JournalStageCompleted,
				journal.ContentRootHash, "", journal.PublishedPath); err != nil {
				return fmt.Errorf("mark pending journal completed: %w", err)
			}
			continue
		}
		if op.State == BuildOpStateFailedTerminal {
			if err := w.journalManager.MarkFailed(journal, fmt.Sprintf("关联操作已终态失败: %s", op.ErrorMessage)); err != nil {
				return fmt.Errorf("mark pending journal failed: %w", err)
			}
		}
	}
	return nil
}

func (w *ReleaseRecoveryWorker) RecoverImportOperations(ctx context.Context) error {
	staleOps, err := w.repo.ListStaleBuildOperations(formatRecoveryTimestamp(time.Now()))
	if err != nil {
		return fmt.Errorf("查询过期导入操作失败: %w", err)
	}
	for _, op := range staleOps {
		if op.State == BuildOpStateCompleted || op.State == BuildOpStateCancelled {
			continue
		}
		if !w.leaseManager.IsLeaseExpired(op) {
			continue
		}
		journal, err := w.journalManager.GetByOperation(op.ID)
		if err != nil {
			log.Logger.Warnf("Failed to get journal for operation %s: %v", op.ID, err)
			continue
		}
		if journal == nil || journal.OperationKind != string(JournalOperationImport) {
			continue
		}
		if !isImportJournalStage(journal.Stage) {
			continue
		}
		if err := w.recoverImportOperation(ctx, op, journal); err != nil {
			log.Logger.Warnf("Failed to recover import operation %s: %v", op.ID, err)
		}
	}
	return nil
}

func isImportJournalStage(stage string) bool {
	switch stage {
	case ImportJournalStageCreated,
		ImportJournalStageValidated,
		ImportJournalStageWorkspaceBuilt,
		ImportJournalStageDatabasePrepared,
		ImportJournalStageFilesPublished,
		ImportJournalStageDatabaseFinalized,
		ImportJournalStageSnapshotCommitted,
		ImportJournalStageCompleted,
		ImportJournalStageFailed,
		ImportJournalStageManualReview:
		return true
	default:
		return false
	}
}

func (w *ReleaseRecoveryWorker) recoverImportOperation(ctx context.Context, op *ReleaseBuildOperation, journal *ReleasePublishJournal) error {
	log.Logger.Infof("Recovering stale import operation %s (state=%s, journalStage=%s)", op.ID, op.State, journal.Stage)

	switch journal.Stage {
	case ImportJournalStageCreated, ImportJournalStageValidated:
		return w.recoverImportBeforePrepare(ctx, op, journal)

	case ImportJournalStageDatabasePrepared, ImportJournalStageWorkspaceBuilt:
		return w.recoverImportPrepared(ctx, op, journal)

	case ImportJournalStageFilesPublished:
		return w.recoverImportPublished(ctx, op, journal)

	case ImportJournalStageDatabaseFinalized, ImportJournalStageSnapshotCommitted:
		return w.recoverImportFinalized(ctx, op, journal)

	case ImportJournalStageCompleted:
		return w.verifyCompletedImport(ctx, op, journal)

	case ImportJournalStageFailed, ImportJournalStageManualReview:
		return nil

	default:
		return w.markImportManualReview(op, journal, "UNKNOWN_IMPORT_STAGE")
	}
}

func (w *ReleaseRecoveryWorker) recoverImportBeforePrepare(ctx context.Context, op *ReleaseBuildOperation, journal *ReleasePublishJournal) error {
	_ = ctx
	_ = journal
	var recoveryErrors []error
	if err := w.storage.RemoveWorkspaceDir(op.ID); err != nil {
		recoveryErrors = append(recoveryErrors, fmt.Errorf("remove import workspace: %w", err))
	}
	if err := w.storage.RemoveStagingDir(op.ReleaseID); err != nil {
		recoveryErrors = append(recoveryErrors, fmt.Errorf("remove import staging: %w", err))
	}
	if err := w.markImportOperationFailed(op, "LEASE_EXPIRED_BEFORE_PREPARE", "租约在导入准备前过期"); err != nil {
		recoveryErrors = append(recoveryErrors, err)
	}
	return errors.Join(recoveryErrors...)
}

func (w *ReleaseRecoveryWorker) recoverImportPrepared(ctx context.Context, op *ReleaseBuildOperation, journal *ReleasePublishJournal) error {
	_ = ctx
	_ = journal
	var recoveryErrors []error
	if err := w.storage.RemoveWorkspaceDir(op.ID); err != nil {
		recoveryErrors = append(recoveryErrors, fmt.Errorf("remove import workspace: %w", err))
	}
	if err := w.storage.RemoveStagingDir(op.ReleaseID); err != nil {
		recoveryErrors = append(recoveryErrors, fmt.Errorf("remove import staging: %w", err))
	}
	if op.PetID != "" && op.ReleaseID != "" {
		if err := w.storage.RemovePublishedDir(op.PetID, op.ReleaseID); err != nil {
			recoveryErrors = append(recoveryErrors, fmt.Errorf("remove import published directory: %w", err))
		}
	}
	releaseRecord, err := w.repo.GetRelease(op.ReleaseID)
	if err != nil {
		recoveryErrors = append(recoveryErrors, fmt.Errorf("load release during import recovery: %w", err))
	} else if releaseRecord != nil {
		releaseRecord.Lifecycle = string(ReleaseLifecycleFailed)
		releaseRecord.IntegrityStatus = string(ReleaseIntegrityUnknown)
		releaseRecord.UpdatedAt = formatRecoveryTimestamp(time.Now())
		if err := w.repo.UpdateRelease(releaseRecord); err != nil {
			recoveryErrors = append(recoveryErrors, fmt.Errorf("persist failed release during recovery: %w", err))
		}
	}
	if err := w.markImportOperationFailed(op, "LEASE_EXPIRED_PREPARED", "租约在导入准备后过期"); err != nil {
		recoveryErrors = append(recoveryErrors, err)
	}
	return errors.Join(recoveryErrors...)
}

func (w *ReleaseRecoveryWorker) recoverImportPublished(ctx context.Context, op *ReleaseBuildOperation, journal *ReleasePublishJournal) error {
	if op.PetID == "" || op.ReleaseID == "" {
		return w.markImportManualReview(op, journal, "PUBLISH_IDENTITY_MISSING")
	}

	publishedDir, err := w.storage.PublishedDir(op.PetID, op.ReleaseID)
	if err != nil {
		return w.markImportManualReview(op, journal, "PUBLISHED_PATH_INVALID")
	}
	info, err := os.Stat(publishedDir)
	if err != nil || !info.IsDir() {
		return w.markImportManualReview(op, journal, "PUBLISHED_DIRECTORY_MISSING")
	}

	releaseRecord, err := w.repo.GetRelease(op.ReleaseID)
	if err != nil || releaseRecord == nil {
		return w.markImportManualReview(op, journal, "RELEASE_RECORD_MISSING")
	}
	if err := w.verifyPublishedArtifacts(ctx, op, releaseRecord, publishedDir); err != nil {
		log.Logger.Warnf("published import artifacts inconsistent for operation %s: %v", op.ID, err)
		return w.markImportManualReview(op, journal, "PUBLISH_INCONSISTENT")
	}

	if err := w.finalizeRecoveredPublishedImport(ctx, op, releaseRecord, journal); err != nil {
		return err
	}
	log.Logger.Infof("Import operation %s forward-recovered from files_published to completed", op.ID)
	return nil
}

func (w *ReleaseRecoveryWorker) finalizeRecoveredPublishedImport(
	ctx context.Context,
	op *ReleaseBuildOperation,
	releaseRecord *ReleaseData,
	journal *ReleasePublishJournal,
) error {
	snapshot, err := w.repo.GetImportSnapshotByReleaseID(op.ReleaseID)
	if err != nil || snapshot == nil {
		return w.markImportManualReview(op, journal, "IMPORT_SNAPSHOT_MISSING")
	}
	if snapshot.OperationID != op.ID || snapshot.ReleaseID != releaseRecord.ID || snapshot.PetID != releaseRecord.PetID {
		return w.markImportManualReview(op, journal, "IMPORT_SNAPSHOT_MISMATCH")
	}
	if w.stagingRepo == nil || snapshot.ImportStagingID == "" {
		return w.markImportManualReview(op, journal, "IMPORT_STAGING_MISSING")
	}
	staging, err := w.stagingRepo.GetForUser(ctx, snapshot.ImportStagingID, releaseRecord.OwnerUserID)
	if err != nil || staging == nil {
		return w.markImportManualReview(op, journal, "IMPORT_STAGING_NOT_FOUND")
	}
	if staging.Status != security.StagingStatusConsuming && staging.Status != security.StagingStatusConsumed {
		return w.markImportManualReview(op, journal, "IMPORT_STAGING_STATE_INVALID")
	}

	now := formatRecoveryTimestamp(time.Now())
	txErr := w.repo.Transaction(func(tx *gorm.DB) error {
		releaseRecord.Lifecycle = string(ReleaseLifecycleReady)
		releaseRecord.IntegrityStatus = string(ReleaseIntegrityVerified)
		releaseRecord.CompatibilityStatus = string(ReleaseCompatCompatible)
		if releaseRecord.PublishedAt == "" {
			releaseRecord.PublishedAt = now
		}
		releaseRecord.UpdatedAt = now
		if err := w.repo.UpdateReleaseTx(tx, releaseRecord); err != nil {
			return err
		}

		op.State = BuildOpStateCompleted
		op.Stage = ImportJournalStageCompleted
		op.CompletedAt = now
		op.UpdatedAt = now
		if err := w.repo.UpdateBuildOperationTx(tx, op); err != nil {
			return err
		}

		snapshot.Status = ImportSnapshotCompleted
		snapshot.UpdatedAt = now
		if err := w.repo.UpdateImportSnapshotTx(tx, snapshot); err != nil {
			return err
		}

		if staging.Status == security.StagingStatusConsuming {
			if err := w.stagingRepo.CompleteConsumptionTx(
				tx,
				staging.ID,
				releaseRecord.OwnerUserID,
				staging.StateRevision,
				now,
			); err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		return fmt.Errorf("forward finalize published import: %w", txErr)
	}

	if w.journalManager != nil && journal != nil {
		if err := w.journalManager.UpdateStage(journal, ImportJournalStageSnapshotCommitted, releaseRecord.ContentRootHash, "", ""); err != nil {
			return fmt.Errorf("update recovered import snapshot journal: %w", err)
		}
		if err := w.journalManager.UpdateStage(journal, ImportJournalStageCompleted, releaseRecord.ContentRootHash, "", ""); err != nil {
			return fmt.Errorf("update recovered import completed journal: %w", err)
		}
	}

	w.leaseManager.ReleaseLease(op)
	return nil
}

func (w *ReleaseRecoveryWorker) recoverImportFinalized(ctx context.Context, op *ReleaseBuildOperation, journal *ReleasePublishJournal) error {
	if !w.isPublishedConsistent(ctx, op) {
		return w.markImportManualReview(op, journal, "FINALIZE_INCONSISTENT")
	}
	op.State = BuildOpStateCompleted
	op.Stage = ImportJournalStageCompleted
	op.CompletedAt = formatRecoveryTimestamp(time.Now())
	w.leaseManager.ReleaseLease(op)
	if err := w.repo.UpdateBuildOperation(op); err != nil {
		return fmt.Errorf("persist recovered completed operation: %w", err)
	}
	if w.journalManager != nil && journal != nil {
		if err := w.journalManager.UpdateStage(journal, ImportJournalStageCompleted, journal.ContentRootHash, "", ""); err != nil {
			return fmt.Errorf("persist recovered completed journal: %w", err)
		}
	}
	log.Logger.Infof("Import operation %s recovered to completed", op.ID)
	return nil
}

func (w *ReleaseRecoveryWorker) verifyCompletedImport(ctx context.Context, op *ReleaseBuildOperation, journal *ReleasePublishJournal) error {
	if !w.isPublishedConsistent(ctx, op) {
		log.Logger.Warnf("Import operation %s marked completed but published data inconsistent", op.ID)
		return w.markImportManualReview(op, journal, "COMPLETED_BUT_INCONSISTENT")
	}
	op.State = BuildOpStateCompleted
	op.Stage = ImportJournalStageCompleted
	op.CompletedAt = formatRecoveryTimestamp(time.Now())
	w.leaseManager.ReleaseLease(op)
	if err := w.repo.UpdateBuildOperation(op); err != nil {
		return fmt.Errorf("persist verified completed import: %w", err)
	}
	return nil
}

func (w *ReleaseRecoveryWorker) markImportManualReview(op *ReleaseBuildOperation, journal *ReleasePublishJournal, code string) error {
	if w.journalManager != nil && journal != nil {
		if err := w.journalManager.UpdateStage(journal, ImportJournalStageManualReview, journal.ContentRootHash, "", ""); err != nil {
			return fmt.Errorf("mark import journal manual review: %w", err)
		}
	}
	op.State = BuildOpStateFailedRetryable
	op.Stage = ImportJournalStageManualReview
	op.ErrorCode = code
	op.ErrorMessage = "导入进入人工审核"
	w.leaseManager.ReleaseLease(op)
	if err := w.repo.UpdateBuildOperation(op); err != nil {
		return fmt.Errorf("persist import manual review operation: %w", err)
	}
	return nil
}

func (w *ReleaseRecoveryWorker) isPublishedConsistent(ctx context.Context, op *ReleaseBuildOperation) bool {
	if op.PetID == "" || op.ReleaseID == "" {
		return false
	}
	publishedDir, err := w.storage.PublishedDir(op.PetID, op.ReleaseID)
	if err != nil {
		return false
	}
	info, err := os.Stat(publishedDir)
	if err != nil || !info.IsDir() {
		return false
	}
	releaseRecord, err := w.repo.GetRelease(op.ReleaseID)
	if err != nil || releaseRecord == nil {
		return false
	}
	if releaseRecord.Lifecycle != string(ReleaseLifecycleReady) {
		return false
	}
	if err := w.verifyImportConsistency(ctx, op, releaseRecord, publishedDir); err != nil {
		log.Logger.Warnf("import consistency check failed for operation %s: %v", op.ID, err)
		return false
	}
	return true
}

func (w *ReleaseRecoveryWorker) verifyPublishedArtifacts(
	ctx context.Context,
	op *ReleaseBuildOperation,
	releaseData *ReleaseData,
	publishedDir string,
) error {
	_ = ctx
	if releaseData == nil {
		return errors.New("release data is nil")
	}
	if releaseData.ContentRootHash == "" || releaseData.ManifestHash == "" {
		return errors.New("release integrity hashes missing")
	}
	if releaseData.IntegrityStatus != string(ReleaseIntegrityVerified) {
		return errors.New("release integrity is not verified")
	}
	if releaseData.CompatibilityStatus != string(ReleaseCompatCompatible) {
		return errors.New("release compatibility is not compatible")
	}

	files, err := w.repo.GetReleaseFiles(op.ReleaseID)
	if err != nil {
		return fmt.Errorf("get release files: %w", err)
	}
	if len(files) == 0 {
		return errors.New("no release files found")
	}
	if len(files) != releaseData.FileCount {
		return fmt.Errorf("release file count mismatch: expected=%d actual=%d", releaseData.FileCount, len(files))
	}
	for _, item := range files {
		fullPath, err := packageformat.SecureJoinUnderRoot(publishedDir, item.Path)
		if err != nil {
			return fmt.Errorf("secure join %s: %w", item.Path, err)
		}
		info, err := os.Lstat(fullPath)
		if err != nil {
			return fmt.Errorf("stat file %s: %w", item.Path, err)
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("release file is not a regular file: %s", item.Path)
		}
		hash, size, err := HashFile(fullPath)
		if err != nil {
			return fmt.Errorf("hash file %s: %w", item.Path, err)
		}
		if hash != item.SHA256 || size != item.Bytes {
			return fmt.Errorf("file mismatch: %s (expected %s/%d, got %s/%d)", item.Path, item.SHA256, item.Bytes, hash, size)
		}
	}

	if releaseData.ArchiveStorageKey == "" || releaseData.ArchiveHash == "" || releaseData.ArchiveBytes <= 0 {
		return errors.New("verified archive metadata missing")
	}
	archivePath, err := w.storage.ArchivePath(releaseData.PetID, releaseData.ID)
	if err != nil {
		return fmt.Errorf("resolve archive path: %w", err)
	}
	hash, bytes, err := HashArchiveFile(archivePath)
	if err != nil {
		return fmt.Errorf("hash archive: %w", err)
	}
	if !strings.EqualFold(hash, releaseData.ArchiveHash) {
		return errors.New("archive hash mismatch")
	}
	if bytes != releaseData.ArchiveBytes {
		return errors.New("archive size mismatch")
	}

	var manifest packageformat.Manifest
	if err := json.Unmarshal([]byte(releaseData.ManifestJSON), &manifest); err != nil {
		return fmt.Errorf("unmarshal manifest: %w", err)
	}
	if manifest.Integrity.ManifestHash != releaseData.ManifestHash {
		return errors.New("manifest hash mismatch")
	}
	if manifest.Integrity.ContentRootHash != releaseData.ContentRootHash {
		return errors.New("content root hash mismatch")
	}
	report := packageformat.NewValidator().ValidateDirectory(publishedDir, &manifest)
	if report == nil || report.Verdict == "invalid" {
		return errors.New("published release validation failed")
	}
	return nil
}

func (w *ReleaseRecoveryWorker) verifyImportConsistency(ctx context.Context, op *ReleaseBuildOperation, releaseData *ReleaseData, publishedDir string) error {
	if err := w.verifyPublishedArtifacts(ctx, op, releaseData, publishedDir); err != nil {
		return err
	}

	snapshot, err := w.repo.GetImportSnapshotByReleaseID(op.ReleaseID)
	if err != nil {
		return fmt.Errorf("get import snapshot: %w", err)
	}
	if snapshot == nil || snapshot.OperationID != op.ID || snapshot.ReleaseID != releaseData.ID {
		return errors.New("import snapshot mismatch")
	}
	if snapshot.PetID != releaseData.PetID {
		return errors.New("snapshot pet_id mismatch")
	}
	if snapshot.Status != ImportSnapshotCompleted {
		return errors.New("import snapshot not completed")
	}

	if w.stagingRepo == nil || snapshot.ImportStagingID == "" {
		return errors.New("import staging repository or staging id missing")
	}
	staging, err := w.stagingRepo.GetForUser(ctx, snapshot.ImportStagingID, releaseData.OwnerUserID)
	if err != nil {
		return fmt.Errorf("get import staging: %w", err)
	}
	if staging.Status == security.StagingStatusConsuming {
		completed, err := w.stagingRepo.CompleteConsumptionCAS(ctx, staging.ID, releaseData.OwnerUserID, staging.StateRevision)
		if err != nil {
			return fmt.Errorf("complete import staging consumption: %w", err)
		}
		if !completed {
			return errors.New("complete import staging consumption CAS did not update a row")
		}
		staging, err = w.stagingRepo.GetForUser(ctx, snapshot.ImportStagingID, releaseData.OwnerUserID)
		if err != nil {
			return fmt.Errorf("re-read import staging: %w", err)
		}
	}
	if staging.Status != security.StagingStatusConsumed {
		return fmt.Errorf("import staging status expected=consumed actual=%s", staging.Status)
	}
	return nil
}

func HashArchiveFile(archivePath string) (string, int64, error) {
	hash, size, err := HashFile(archivePath)
	if err != nil {
		return "", 0, err
	}
	return hash, size, nil
}

func (w *ReleaseRecoveryWorker) markImportOperationFailed(op *ReleaseBuildOperation, code string, msg string) error {
	op.State = BuildOpStateFailedRetryable
	op.ErrorCode = code
	op.ErrorMessage = msg
	w.leaseManager.ReleaseLease(op)
	op.UpdatedAt = formatRecoveryTimestamp(time.Now())
	if err := w.repo.UpdateBuildOperation(op); err != nil {
		return fmt.Errorf("persist failed import operation: %w", err)
	}
	return nil
}

func (w *ReleaseRecoveryWorker) updateOp(op *ReleaseBuildOperation) error {
	op.UpdatedAt = formatRecoveryTimestamp(time.Now())
	if err := w.repo.UpdateBuildOperation(op); err != nil {
		return fmt.Errorf("persist recovered build operation: %w", err)
	}
	return nil
}

type ReleaseEventPublisher struct {
	repo   ReleaseRepository
	events []ReleaseEvent
	mu     sync.Mutex
}

func NewReleaseEventPublisher(repo ReleaseRepository) *ReleaseEventPublisher {
	return &ReleaseEventPublisher{repo: repo}
}

func (o *ReleaseEventPublisher) PublishReleaseEvent(event ReleaseEvent) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = append(o.events, event)
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	log.Logger.Infof("Release event published: %s - %s", event.EventType, string(data))
	return nil
}

func (o *ReleaseEventPublisher) Flush(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.events) == 0 {
		return nil
	}
	flushed := 0
	for _, event := range o.events {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshal release event %s: %w", event.EventType, err)
		}
		log.Logger.Infof("Release event flushed: %s - %s", event.EventType, string(data))
		flushed++
	}
	o.events = o.events[:0]
	log.Logger.Infof("Flushed %d release events", flushed)
	return nil
}

func (o *ReleaseEventPublisher) PendingCount() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.events)
}

func formatRecoveryTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}
