package build

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/release"
	"gorm.io/gorm"
)

type ReleaseOrchestrator struct {
	snapshotCreator *SnapshotCreator
	sequenceAlloc   *SequenceAllocator
	leaseManager    *LeaseManager
	journalManager  *PublishJournalManager
	repo            release.ReleaseRepository
	writer          PackageWriter
	eventPublisher  release.EventPublisher
}

func NewReleaseOrchestrator(
	snapshotCreator *SnapshotCreator,
	sequenceAlloc *SequenceAllocator,
	leaseManager *LeaseManager,
	journalManager *PublishJournalManager,
	repo release.ReleaseRepository,
	writer PackageWriter,
	eventPublisher release.EventPublisher,
) *ReleaseOrchestrator {
	return &ReleaseOrchestrator{
		snapshotCreator: snapshotCreator,
		sequenceAlloc:   sequenceAlloc,
		leaseManager:    leaseManager,
		journalManager:  journalManager,
		repo:            repo,
		writer:          writer,
		eventPublisher:  eventPublisher,
	}
}

type BuildReleaseRequest struct {
	UserID           string
	PetID            string
	ProcessingTaskID string
	CharacterID      string
	DefaultAction    string
	IncludedActions  []string
	IdempotencyKey   string
}

type BuildReleaseResult struct {
	Release          *release.ReleaseData
	BuildSnapshot    *release.ReleaseBuildSnapshot
	BuildOperation   *release.ReleaseBuildOperation
	ValidationResult *ValidationResult
}

func (o *ReleaseOrchestrator) Build(ctx context.Context, req *BuildReleaseRequest) (*BuildReleaseResult, error) {
	if req.IdempotencyKey != "" {
		existing, err := o.repo.GetBuildOperationByIdempotencyKey(req.UserID, req.IdempotencyKey)
		if err == nil && existing.State == release.BuildOpStateCompleted {
			return o.loadExistingResult(existing)
		}
	}

	now := formatTimestamp(time.Now())
	opID := uuid.NewString()
	op := &release.ReleaseBuildOperation{
		ID:             opID,
		UserID:         req.UserID,
		IdempotencyKey: req.IdempotencyKey,
		State:          release.BuildOpStateCreated,
		StartedAt:      now,
		UpdatedAt:      now,
	}
	if err := o.repo.CreateBuildOperation(op); err != nil {
		return nil, NewBuildError("OPERATION_CREATE_FAILED", "创建构建操作失败", err)
	}

	snapshotResult, err := o.snapshotCreator.Create(ctx, &CreateSnapshotRequest{
		UserID:           req.UserID,
		PetID:            req.PetID,
		ProcessingTaskID: req.ProcessingTaskID,
		CharacterID:      req.CharacterID,
		DefaultAction:    req.DefaultAction,
		IncludedActions:  req.IncludedActions,
	})
	if err != nil {
		o.failOperation(op, "snapshot_failed", err)
		return nil, err
	}

	op.SnapshotID = snapshotResult.Snapshot.ID
	op.PetID = snapshotResult.Identity.ID
	op.State = release.BuildOpStateSnapshotting
	op.Stage = release.BuildOpStageSnapshotCreated
	o.leaseManager.AcquireLease(op, opID)
	o.updateOperation(op)

	journal, err := o.journalManager.CreateJournal(opID, "", snapshotResult.Identity.ID)
	if err != nil {
		o.failOperation(op, "journal_create_failed", err)
		return nil, err
	}
	journal.PetID = snapshotResult.Identity.ID
	o.journalManager.UpdateStage(journal, release.JournalStageSnapshotCreated, "", "", "")

	leaseOwner := opID
	if !o.leaseManager.CanEnterPublish(op, leaseOwner) {
		o.failOperation(op, "lease_conflict", NewBuildError(ErrCodeReleaseOperationConflict, "另一个操作正在执行", nil))
		return nil, NewBuildError(ErrCodeReleaseOperationConflict, "另一个操作正在执行", nil)
	}

	releaseSeq, err := o.sequenceAlloc.AllocateSequence(ctx, snapshotResult.Identity.ID)
	if err != nil {
		o.failOperation(op, "sequence_failed", err)
		return nil, err
	}

	releaseID := uuid.NewString()
	version := generateVersion(releaseSeq)

	op.ReleaseID = releaseID
	op.State = release.BuildOpStateBuilding
	o.leaseManager.RenewLease(op)
	o.updateOperation(op)

	journal.ReleaseID = releaseID
	o.journalManager.UpdateStage(journal, release.JournalStageStagingBuilt, "", "", "")

	staged, err := o.writer.StagePackage(
		snapshotResult.Snapshot,
		snapshotResult.ActionSnapshots,
		snapshotResult.TaskInfo,
		snapshotResult.Identity,
		snapshotResult.PreviewArtifactID,
		snapshotResult.Snapshot.DefaultActionKey,
	)
	if err != nil {
		o.writer.RemoveStagingDir(staged.StagingDir)
		o.failOperation(op, "staging_failed", err)
		o.journalManager.MarkFailed(journal, err.Error())
		return nil, err
	}

	op.State = release.BuildOpStateValidating
	o.leaseManager.RenewLease(op)
	o.updateOperation(op)

	validation, err := o.writer.ValidatePackage(staged)
	if err != nil {
		o.writer.RemoveStagingDir(staged.StagingDir)
		o.failOperation(op, "validation_failed", err)
		o.journalManager.MarkFailed(journal, err.Error())
		return nil, err
	}

	if validation.Verdict == "invalid" {
		o.writer.RemoveStagingDir(staged.StagingDir)
		errMsg := fmt.Sprintf("包验证失败: %d 个错误", validation.ErrorCount)
		o.failOperation(op, "validation_failed", NewBuildError(ErrCodeReleaseValidationFailed, errMsg, nil))
		o.journalManager.MarkFailed(journal, errMsg)
		return nil, NewBuildError(ErrCodeReleaseValidationFailed, errMsg, nil)
	}

	if err := o.writer.WriteManifest(staged); err != nil {
		o.writer.RemoveStagingDir(staged.StagingDir)
		o.failOperation(op, "manifest_failed", err)
		o.journalManager.MarkFailed(journal, err.Error())
		return nil, err
	}

	o.journalManager.UpdateStage(journal, release.JournalStageValidated, staged.ContentRootHash, staged.StagingDir, "")

	op.State = release.BuildOpStatePublishing
	o.leaseManager.RenewLease(op)
	o.updateOperation(op)

	petID := snapshotResult.Identity.ID
	if err := o.writer.MoveStagingToPublished(petID, releaseID, staged.StagingDir); err != nil {
		o.writer.RemoveStagingDir(staged.StagingDir)
		o.failOperation(op, "publish_move_failed", err)
		o.journalManager.MarkFailed(journal, err.Error())
		return nil, err
	}

	publishedDir := o.writer.PublishedDir(petID, releaseID)
	if err := o.writer.BuildArchive(publishedDir, petID, releaseID); err != nil {
		o.writer.RemovePublishedDir(petID, releaseID)
		o.failOperation(op, "archive_failed", err)
		o.journalManager.MarkFailed(journal, err.Error())
		return nil, err
	}

	o.journalManager.UpdateStage(journal, release.JournalStageFilesPublished,
		staged.ContentRootHash, "", o.writer.PublishedStorageKey(petID, releaseID))

	op.PublishedPathKey = o.writer.PublishedStorageKey(petID, releaseID)
	op.StagingPathKey = releaseID

	releaseData := &release.ReleaseData{
		ID:                    releaseID,
		PetID:                 petID,
		OwnerUserID:           req.UserID,
		Version:               version,
		ReleaseSequence:       releaseSeq,
		SchemaVersion:         snapshotResult.Snapshot.PackageSchemaVersion,
		Lifecycle:             string(release.ReleaseLifecycleReady),
		ContentRootHash:       staged.ContentRootHash,
		ManifestHash:          staged.ManifestHash,
		StorageKey:            o.writer.PublishedStorageKey(petID, releaseID),
		ArchiveStorageKey:     o.writer.ArchiveStorageKey(petID, releaseID),
		TotalBytes:            staged.TotalBytes,
		FileCount:             staged.FileCount,
		ActionCount:           len(snapshotResult.ActionSnapshots),
		DefaultActionKey:      snapshotResult.Snapshot.DefaultActionKey,
		MinRuntimeVersion:     snapshotResult.Snapshot.RuntimeContractVersion,
		SourceType:            "generated",
		SourceProcessingTask:  req.ProcessingTaskID,
		SourceGenerationTask:  snapshotResult.TaskInfo.GenerationTaskID,
		ActiveRevisionSetHash: snapshotResult.Snapshot.ActiveRevisionSetHash,
		QualityGateID:         snapshotResult.Snapshot.QualityGateID,
		QualityGateHash:       snapshotResult.Snapshot.QualityGateHash,
		BuildSnapshotID:       snapshotResult.Snapshot.ID,
		IntegrityStatus:       string(release.ReleaseIntegrityVerified),
		CompatibilityStatus:   string(release.ReleaseCompatCompatible),
		ManifestJSON:          string(staged.ManifestData),
		PublishedAt:           formatTimestamp(time.Now()),
		CreatedAt:             formatTimestamp(time.Now()),
		UpdatedAt:             formatTimestamp(time.Now()),
	}

	err = o.repo.Transaction(func(tx *gorm.DB) error {
		if err := o.repo.CreateRelease(releaseData); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		o.writer.RemovePublishedDir(petID, releaseID)
		op.State = release.BuildOpStateFailedRetryable
		op.ErrorCode = "COMMIT_DB_FAILED"
		op.ErrorMessage = err.Error()
		o.updateOperation(op)
		o.journalManager.MarkFailed(journal, err.Error())
		return nil, err
	}

	o.journalManager.UpdateStage(journal, release.JournalStageDatabaseCommitted,
		staged.ContentRootHash, "", o.writer.PublishedStorageKey(petID, releaseID))
	o.journalManager.UpdateStage(journal, release.JournalStageCompleted,
		staged.ContentRootHash, "", o.writer.PublishedStorageKey(petID, releaseID))

	op.State = release.BuildOpStateCompleted
	op.Stage = release.BuildOpStageDatabaseCommitted
	op.CompletedAt = formatTimestamp(time.Now())
	o.leaseManager.ReleaseLease(op)
	o.updateOperation(op)

	if o.eventPublisher != nil {
		o.eventPublisher.PublishReleaseEvent(release.ReleaseEvent{
			EventType:             release.EventReleaseBuildCompleted,
			UserID:                req.UserID,
			PetID:                 petID,
			ReleaseID:             releaseID,
			ProcessingTaskID:      req.ProcessingTaskID,
			ActiveRevisionSetHash: snapshotResult.Snapshot.ActiveRevisionSetHash,
			QualityGateID:         snapshotResult.Snapshot.QualityGateID,
			ContentRootHash:       staged.ContentRootHash,
			OccurredAt:            formatTimestamp(time.Now()),
		})
	}

	resultJSON, _ := json.Marshal(map[string]interface{}{
		"releaseId":         releaseID,
		"version":           version,
		"contentHash":       staged.ContentRootHash,
		"validationVerdict": validation.Verdict,
	})
	op.ResultJSON = string(resultJSON)
	o.updateOperation(op)

	return &BuildReleaseResult{
		Release:          releaseData,
		BuildSnapshot:    snapshotResult.Snapshot,
		BuildOperation:   op,
		ValidationResult: validation,
	}, nil
}

func (o *ReleaseOrchestrator) loadExistingResult(op *release.ReleaseBuildOperation) (*BuildReleaseResult, error) {
	releaseData, err := o.repo.GetRelease(op.ReleaseID)
	if err != nil {
		return nil, err
	}
	snapshot, err := o.repo.GetBuildSnapshot(op.SnapshotID)
	if err != nil {
		return nil, err
	}
	return &BuildReleaseResult{
		Release:        releaseData,
		BuildSnapshot:  snapshot,
		BuildOperation: op,
	}, nil
}

func (o *ReleaseOrchestrator) failOperation(op *release.ReleaseBuildOperation, code string, err error) {
	op.State = classifyFailureState(code)
	op.ErrorCode = code
	if err != nil {
		op.ErrorMessage = err.Error()
	}
	op.UpdatedAt = formatTimestamp(time.Now())
	o.leaseManager.ReleaseLease(op)
	o.updateOperation(op)
}

func classifyFailureState(code string) string {
	switch code {
	case "validation_failed", "quality_gate_failed", "release_frame_set_incomplete",
		"release_frame_asset_missing", "release_frame_hash_mismatch",
		"release_default_action_invalid", "release_input_hash_mismatch",
		"legacy_package_write_disabled":
		return release.BuildOpStateFailedTerminal
	default:
		return release.BuildOpStateFailedRetryable
	}
}

func (o *ReleaseOrchestrator) updateOperation(op *release.ReleaseBuildOperation) {
	op.UpdatedAt = formatTimestamp(time.Now())
	o.repo.UpdateBuildOperation(op)
}

func (o *ReleaseOrchestrator) Cancel(operationID string) error {
	op, err := o.repo.GetBuildOperation(operationID)
	if err != nil {
		return err
	}
	if op.State == release.BuildOpStateCompleted || op.State == release.BuildOpStateCancelled {
		return NewBuildError(ErrCodeReleaseOperationConflict, "操作已完成或已取消", nil)
	}
	op.State = release.BuildOpStateCancelled
	op.CompletedAt = formatTimestamp(time.Now())
	o.leaseManager.ReleaseLease(op)
	o.updateOperation(op)

	if op.ReleaseID != "" && op.PetID != "" {
		o.writer.RemovePublishedDir(op.PetID, op.ReleaseID)
	}
	if op.StagingPathKey != "" {
		o.writer.RemoveStagingDir(op.StagingPathKey)
	}
	return nil
}

func (o *ReleaseOrchestrator) Archive(releaseID, userID string) error {
	releaseData, err := o.repo.GetRelease(releaseID)
	if err != nil {
		return err
	}
	if releaseData.OwnerUserID != userID {
		return NewBuildError(ErrCodeReleaseOwnershipDenied, "Release 不属于当前用户", nil)
	}
	if releaseData.Lifecycle != string(release.ReleaseLifecycleReady) {
		return NewBuildError(ErrCodeReleaseOperationConflict, "只有 ready 状态的 Release 可以归档", nil)
	}
	releaseData.Lifecycle = string(release.ReleaseLifecycleArchived)
	releaseData.UpdatedAt = formatTimestamp(time.Now())
	return o.repo.UpdateRelease(releaseData)
}

func (o *ReleaseOrchestrator) Revoke(releaseID, userID, reason string) error {
	releaseData, err := o.repo.GetRelease(releaseID)
	if err != nil {
		return err
	}
	if releaseData.OwnerUserID != userID {
		return NewBuildError(ErrCodeReleaseOwnershipDenied, "Release 不属于当前用户", nil)
	}
	releaseData.Lifecycle = string(release.ReleaseLifecycleRevoked)
	releaseData.UpdatedAt = formatTimestamp(time.Now())
	if err := o.repo.UpdateRelease(releaseData); err != nil {
		return err
	}
	if o.eventPublisher != nil {
		o.eventPublisher.PublishReleaseEvent(release.ReleaseEvent{
			EventType:  release.EventReleaseRevoked,
			UserID:     userID,
			PetID:      releaseData.PetID,
			ReleaseID:  releaseID,
			OccurredAt: formatTimestamp(time.Now()),
		})
	}
	return nil
}

func (o *ReleaseOrchestrator) Download(releaseID, userID string) (string, error) {
	releaseData, err := o.repo.GetRelease(releaseID)
	if err != nil {
		return "", err
	}
	if releaseData.OwnerUserID != userID {
		return "", NewBuildError(ErrCodeReleaseOwnershipDenied, "Release 不属于当前用户", nil)
	}
	if !release.IsInstallable(releaseData.Lifecycle, releaseData.IntegrityStatus, releaseData.CompatibilityStatus) {
		return "", NewBuildError(ErrCodeReleaseOperationConflict, "Release 不可下载", nil)
	}
	archivePath := o.writer.PublishedDir(releaseData.PetID, releaseID)
	return archivePath, nil
}
