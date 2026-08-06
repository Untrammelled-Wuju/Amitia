package release

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

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
		if !isImportStage(op.Stage) {
			continue
		}
		if err := w.recoverImportOperation(ctx, op); err != nil {
			log.Logger.Warnf("Failed to recover import operation %s: %v", op.ID, err)
		}
	}
	return nil
}

func isImportStage(stage string) bool {
	switch stage {
	case BuildOpStageWorkspaceCreated, BuildOpStageStagingBuilt, BuildOpStageStagingMoved:
		return true
	default:
		return false
	}
}

func (w *ReleaseRecoveryWorker) recoverImportOperation(ctx context.Context, op *ReleaseBuildOperation) error {
	log.Logger.Infof("Recovering stale import operation %s (state=%s, stage=%s)", op.ID, op.State, op.Stage)

	w.storage.RemoveWorkspaceDir(op.ID)

	switch op.Stage {
	case BuildOpStageWorkspaceCreated:
		w.storage.RemoveStagingDir(op.ReleaseID)
		w.markImportOperationFailed(op, "LEASE_EXPIRED_WORKSPACE", "租约在工作区创建阶段过期")

	case BuildOpStageStagingBuilt, BuildOpStageStagingMoved:
		if w.isPublishedConsistent(op) {
			op.State = BuildOpStateCompleted
			op.Stage = BuildOpStageDatabaseCommitted
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

	default:
		w.markImportOperationFailed(op, "UNKNOWN_IMPORT_STAGE", fmt.Sprintf("未知导入阶段: %s", op.Stage))
	}

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
	if releaseRecord.Lifecycle == string(ReleaseLifecycleReady) && releaseRecord.ContentRootHash != "" {
		return true
	}
	return false
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
