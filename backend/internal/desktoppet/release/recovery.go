package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"github.com/u-ai/backend/log"
)

type ReleaseRecoveryWorker struct {
	repo           ReleaseRepository
	leaseManager   LeaseManagerPort
	journalManager JournalManagerPort
	storage        ReleaseStoragePort
	eventPublisher EventPublisher
	checkInterval  time.Duration
	stopCh         chan struct{}
	wg             sync.WaitGroup
	mu             sync.Mutex
	running        bool
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
	leaseManager LeaseManagerPort,
	journalManager JournalManagerPort,
	storage ReleaseStoragePort,
	eventPublisher EventPublisher,
) *ReleaseRecoveryWorker {
	return &ReleaseRecoveryWorker{
		repo:           repo,
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
	w.mu.Unlock()

	w.wg.Add(1)
	go w.run(ctx)
	log.Logger.Info("Release recovery worker started")
}

func (w *ReleaseRecoveryWorker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	close(w.stopCh)
	w.mu.Unlock()
	w.wg.Wait()
	log.Logger.Info("Release recovery worker stopped")
}

func (w *ReleaseRecoveryWorker) run(ctx context.Context) {
	defer w.wg.Done()
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
		w.updateOp(op)

	case BuildOpStateBuilding:
		op.State = BuildOpStateFailedRetryable
		op.ErrorCode = "LEASE_EXPIRED_DURING_BUILD"
		op.ErrorMessage = "租约在构建阶段过期"
		w.leaseManager.ReleaseLease(op)
		w.updateOp(op)
		if op.StagingPathKey != "" && op.PetID != "" {
			w.storage.RemoveStagingDir(op.StagingPathKey)
		}

	case BuildOpStateValidating:
		op.State = BuildOpStateFailedRetryable
		op.ErrorCode = "LEASE_EXPIRED_DURING_VALIDATION"
		op.ErrorMessage = "租约在验证阶段过期"
		w.leaseManager.ReleaseLease(op)
		w.updateOp(op)

	case BuildOpStatePublishing:
		journal, err := w.journalManager.GetByOperation(op.ID)
		if err != nil || journal == nil {
			op.State = BuildOpStateFailedRetryable
			op.ErrorCode = "JOURNAL_NOT_FOUND"
			op.ErrorMessage = "发布日志未找到"
			w.leaseManager.ReleaseLease(op)
			w.updateOp(op)
			return nil
		}
		switch journal.Stage {
		case JournalStageStagingBuilt, JournalStageValidated:
			if op.StagingPathKey != "" && op.PetID != "" {
				w.storage.RemoveStagingDir(op.StagingPathKey)
			}
			op.State = BuildOpStateFailedRetryable
			op.ErrorCode = "LEASE_EXPIRED_BEFORE_PUBLISH"
			op.ErrorMessage = "租约在文件发布前过期"
			w.leaseManager.ReleaseLease(op)
			w.updateOp(op)
			w.journalManager.MarkFailed(journal, "租约在文件发布前过期")

		case JournalStageFilesPublished:
			releaseData, err := w.repo.GetRelease(op.ReleaseID)
			if err == nil && releaseData != nil {
				op.State = BuildOpStateCompleted
				op.Stage = BuildOpStageDatabaseCommitted
				op.CompletedAt = formatRecoveryTimestamp(time.Now())
				w.leaseManager.ReleaseLease(op)
				w.updateOp(op)
				w.journalManager.UpdateStage(journal, JournalStageDatabaseCommitted,
					journal.ContentRootHash, "", journal.PublishedPath)
				w.journalManager.UpdateStage(journal, JournalStageCompleted,
					journal.ContentRootHash, "", journal.PublishedPath)
				log.Logger.Infof("Operation %s recovered to completed", op.ID)
			} else {
				if op.PetID != "" && op.ReleaseID != "" {
					w.storage.RemovePublishedDir(op.PetID, op.ReleaseID)
				}
				op.State = BuildOpStateFailedTerminal
				op.ErrorCode = "RELEASE_RECORD_MISSING"
				op.ErrorMessage = "Release 记录不存在"
				w.leaseManager.ReleaseLease(op)
				w.updateOp(op)
				w.journalManager.MarkFailed(journal, "Release 记录不存在")
			}

		case JournalStageDatabaseCommitted, JournalStageCompleted:
			op.State = BuildOpStateCompleted
			op.Stage = BuildOpStageDatabaseCommitted
			op.CompletedAt = formatRecoveryTimestamp(time.Now())
			w.leaseManager.ReleaseLease(op)
			w.updateOp(op)

		default:
			op.State = BuildOpStateFailedRetryable
			op.ErrorCode = "UNKNOWN_JOURNAL_STAGE"
			op.ErrorMessage = fmt.Sprintf("未知日志阶段: %s", journal.Stage)
			w.leaseManager.ReleaseLease(op)
			w.updateOp(op)
		}

	case BuildOpStateFailedRetryable:
		if op.RetryCount < 3 {
			op.RetryCount++
			op.State = BuildOpStateCreated
			op.ErrorCode = ""
			op.ErrorMessage = ""
			op.LeaseOwner = ""
			op.LeaseExpiresAt = ""
			w.updateOp(op)
			log.Logger.Infof("Operation %s queued for retry (attempt %d)", op.ID, op.RetryCount)
		} else {
			op.State = BuildOpStateFailedTerminal
			op.CompletedAt = formatRecoveryTimestamp(time.Now())
			w.updateOp(op)
			if op.PetID != "" && op.ReleaseID != "" {
				w.storage.RemovePublishedDir(op.PetID, op.ReleaseID)
			}
			if op.StagingPathKey != "" {
				w.storage.RemoveStagingDir(op.StagingPathKey)
			}
			if w.eventPublisher != nil {
				w.eventPublisher.PublishReleaseEvent(ReleaseEvent{
					EventType:  EventReleaseBuildFailed,
					UserID:     op.UserID,
					PetID:      op.PetID,
					ReleaseID:  op.ReleaseID,
					OccurredAt: formatRecoveryTimestamp(time.Now()),
				})
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
			continue
		}
		if op.State == BuildOpStateCompleted || op.State == BuildOpStateCancelled {
			w.journalManager.UpdateStage(journal, JournalStageCompleted,
				journal.ContentRootHash, "", journal.PublishedPath)
			continue
		}
		if op.State == BuildOpStateFailedTerminal {
			w.journalManager.MarkFailed(journal, fmt.Sprintf("关联操作已终态失败: %s", op.ErrorMessage))
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
	w.storage.RemoveWorkspaceDir(op.ID)
	w.storage.RemoveStagingDir(op.ReleaseID)
	w.markImportOperationFailed(op, "LEASE_EXPIRED_BEFORE_PREPARE", "租约在导入准备前过期")
	return nil
}

func (w *ReleaseRecoveryWorker) recoverImportPrepared(ctx context.Context, op *ReleaseBuildOperation, journal *ReleasePublishJournal) error {
	w.storage.RemoveWorkspaceDir(op.ID)
	w.storage.RemoveStagingDir(op.ReleaseID)
	if op.PetID != "" && op.ReleaseID != "" {
		w.storage.RemovePublishedDir(op.PetID, op.ReleaseID)
	}
	if releaseRecord, err := w.repo.GetRelease(op.ReleaseID); err == nil && releaseRecord != nil {
		releaseRecord.Lifecycle = string(ReleaseLifecycleFailed)
		releaseRecord.IntegrityStatus = string(ReleaseIntegrityUnknown)
		releaseRecord.UpdatedAt = formatRecoveryTimestamp(time.Now())
		w.repo.UpdateRelease(releaseRecord)
	}
	w.markImportOperationFailed(op, "LEASE_EXPIRED_PREPARED", "租约在导入准备后过期")
	return nil
}

func (w *ReleaseRecoveryWorker) recoverImportPublished(ctx context.Context, op *ReleaseBuildOperation, journal *ReleasePublishJournal) error {
	if w.isPublishedConsistent(op) {
		op.State = BuildOpStateCompleted
		op.Stage = ImportJournalStageFilesPublished
		op.CompletedAt = formatRecoveryTimestamp(time.Now())
		w.leaseManager.ReleaseLease(op)
		w.updateOp(op)
		log.Logger.Infof("Import operation %s recovered to completed (published consistent)", op.ID)
	} else {
		w.storage.RemoveStagingDir(op.ReleaseID)
		if op.PetID != "" && op.ReleaseID != "" {
			w.storage.RemovePublishedDir(op.PetID, op.ReleaseID)
		}
		if releaseRecord, err := w.repo.GetRelease(op.ReleaseID); err == nil && releaseRecord != nil {
			releaseRecord.Lifecycle = string(ReleaseLifecycleFailed)
			releaseRecord.IntegrityStatus = string(ReleaseIntegrityUnknown)
			releaseRecord.UpdatedAt = formatRecoveryTimestamp(time.Now())
			w.repo.UpdateRelease(releaseRecord)
		}
		w.markImportOperationFailed(op, "PUBLISH_INCOMPLETE", "发布未完成且数据不一致")
	}
	return nil
}

func (w *ReleaseRecoveryWorker) recoverImportFinalized(ctx context.Context, op *ReleaseBuildOperation, journal *ReleasePublishJournal) error {
	if w.isPublishedConsistent(op) {
		op.State = BuildOpStateCompleted
		op.Stage = ImportJournalStageSnapshotCommitted
		op.CompletedAt = formatRecoveryTimestamp(time.Now())
		w.leaseManager.ReleaseLease(op)
		w.updateOp(op)
		log.Logger.Infof("Import operation %s recovered to completed", op.ID)
	} else {
		if releaseRecord, err := w.repo.GetRelease(op.ReleaseID); err == nil && releaseRecord != nil {
			releaseRecord.Lifecycle = string(ReleaseLifecycleFailed)
			releaseRecord.IntegrityStatus = string(ReleaseIntegrityUnknown)
			releaseRecord.UpdatedAt = formatRecoveryTimestamp(time.Now())
			w.repo.UpdateRelease(releaseRecord)
		}
		w.markImportOperationFailed(op, "FINALIZE_INCOMPLETE", "定稿阶段失败")
	}
	return nil
}

func (w *ReleaseRecoveryWorker) verifyCompletedImport(ctx context.Context, op *ReleaseBuildOperation, journal *ReleasePublishJournal) error {
	if !w.isPublishedConsistent(op) {
		log.Logger.Warnf("Import operation %s marked completed but published data inconsistent", op.ID)
		return w.markImportManualReview(op, journal, "COMPLETED_BUT_INCONSISTENT")
	}
	op.State = BuildOpStateCompleted
	op.Stage = ImportJournalStageCompleted
	op.CompletedAt = formatRecoveryTimestamp(time.Now())
	w.leaseManager.ReleaseLease(op)
	w.updateOp(op)
	return nil
}

func (w *ReleaseRecoveryWorker) markImportManualReview(op *ReleaseBuildOperation, journal *ReleasePublishJournal, code string) error {
	if w.journalManager != nil && journal != nil {
		w.journalManager.UpdateStage(journal, ImportJournalStageManualReview, journal.ContentRootHash, "", "")
	}
	op.State = BuildOpStateFailedRetryable
	op.ErrorCode = code
	op.ErrorMessage = "导入进入人工审核"
	w.leaseManager.ReleaseLease(op)
	w.updateOp(op)
	return nil
}

func (w *ReleaseRecoveryWorker) isPublishedConsistent(op *ReleaseBuildOperation) bool {
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
	if releaseRecord.ContentRootHash == "" {
		return false
	}
	if err := w.verifyImportConsistency(op, releaseRecord, publishedDir); err != nil {
		log.Logger.Warnf("import consistency check failed for operation %s: %v", op.ID, err)
		return false
	}
	return true
}

func (w *ReleaseRecoveryWorker) verifyImportConsistency(op *ReleaseBuildOperation, releaseData *ReleaseData, publishedDir string) error {
	if releaseData == nil {
		return errors.New("release data is nil")
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
		hash, size, err := HashFile(fullPath)
		if err != nil {
			return fmt.Errorf("hash file %s: %w", item.Path, err)
		}
		if hash != item.SHA256 || size != item.Bytes {
			return fmt.Errorf("file mismatch: %s (expected %s/%d, got %s/%d)", item.Path, item.SHA256, item.Bytes, hash, size)
		}
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

	if releaseData.ArchiveStorageKey != "" {
		archivePath, archiveErr := w.storage.ArchivePath(releaseData.PetID, releaseData.ID)
		if archiveErr != nil {
			return fmt.Errorf("resolve archive path: %w", archiveErr)
		}
		hash, bytes, hashErr := HashArchiveFile(archivePath)
		if hashErr != nil {
			return fmt.Errorf("hash archive: %w", hashErr)
		}
		if !strings.EqualFold(hash, releaseData.ArchiveHash) {
			return errors.New("archive hash mismatch")
		}
		if bytes != releaseData.ArchiveBytes {
			return errors.New("archive size mismatch")
		}
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

func HashArchiveFile(archivePath string) (string, int64, error) {
	hash, size, err := HashFile(archivePath)
	if err != nil {
		return "", 0, err
	}
	return hash, size, nil
}

func (w *ReleaseRecoveryWorker) markImportOperationFailed(op *ReleaseBuildOperation, code string, msg string) {
	op.State = BuildOpStateFailedRetryable
	op.ErrorCode = code
	op.ErrorMessage = msg
	w.leaseManager.ReleaseLease(op)
	w.updateOp(op)
}

func (w *ReleaseRecoveryWorker) updateOp(op *ReleaseBuildOperation) {
	op.UpdatedAt = formatRecoveryTimestamp(time.Now())
	w.repo.UpdateBuildOperation(op)
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
		data, _ := json.Marshal(event)
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
