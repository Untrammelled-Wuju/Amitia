package editing

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type Repository interface {
	CreateActionRevision(rev *ActionRevision) error
	GetActionRevision(id string) (*ActionRevision, error)
	ListActionRevisions(processingTaskID, actionKey string) ([]ActionRevision, error)
	UpdateActionRevisionStatus(id, status string) error
	UpdateActionRevisionQuality(id, evaluationID, verdict string) error
	UpdateActionRevisionManifest(id, path, hash string, frameCount, durationMS int) error
	GetNextRevisionNumber(processingTaskID, actionKey string) (int, error)

	CreateRevisionFrames(frames []ActionRevisionFrame) error
	ListRevisionFrames(revisionID string) ([]ActionRevisionFrame, error)
	DeleteRevisionFrames(revisionID string) error

	GetActiveRevisionBinding(processingTaskID, actionKey string) (*ActiveRevisionBinding, error)
	UpsertActiveRevisionBinding(binding *ActiveRevisionBinding) error
	ListActiveBindings(processingTaskID string) ([]ActiveRevisionBinding, error)

	GetActionStreamByID(streamID string) (*ActionStream, error)
	GetActionStreamByKey(userID, characterID, actionKey string) (*ActionStream, error)
	GetActiveActionRevisionBindingByStream(streamID string) (*ActiveActionRevisionBinding, error)
	GetActiveActionRevisionBindingByTask(processingTaskID, actionKey string) (*ActiveActionRevisionBinding, error)
	ListActionRevisionsByStream(streamID string) ([]ActionRevision, error)
	ListAllActionStreams(userID string) ([]ActionStream, error)

	CreateFrameAsset(asset *FrameAsset) error
	GetFrameAsset(id string) (*FrameAsset, error)
	GetFrameAssetByHash(hash, mimeType string) (*FrameAsset, error)
	UpdateFrameAssetStatus(id, status string) error

	CreateEditSession(session *EditSession) error
	GetEditSession(id string) (*EditSession, error)
	ListOpenSessions(userID string) ([]EditSession, error)
	ListSessionsByTaskAction(processingTaskID, actionKey string) ([]EditSession, error)
	UpdateSessionVersion(id string, expectedVersion int64) (int64, error)
	UpdateSessionStatus(id, status string) error
	UpdateSessionCursor(id string, cursor, lastOpSeq int) error
	UpdateSessionCheckpoint(id, checkpointID string) error
	UpdateSessionCommitted(id, revisionID string) error
	UpdateSessionExpiry(id string, expiresAt string) error
	ListExpiredSessions() ([]EditSession, error)

	CreateOperation(op *EditOperation) error
	GetOperation(sessionID string, sequence int) (*EditOperation, error)
	ListOperations(sessionID string) ([]EditOperation, error)
	ListOperationsSince(sessionID string, sinceSeq int) ([]EditOperation, error)
	UpdateOperationStatus(id, status string) error
	GetOperationByIdempotencyKey(sessionID, key string) (*EditOperation, error)

	CreateCheckpoint(cp *EditCheckpoint) error
	ListCheckpoints(sessionID string) ([]EditCheckpoint, error)
	DeleteOldCheckpoints(sessionID string, keep int) error

	CreateRegenerationJob(job *RegenerationJob) error
	GetRegenerationJob(id string) (*RegenerationJob, error)
	ListJobsBySession(sessionID string) ([]RegenerationJob, error)
	ListPendingJobs() ([]RegenerationJob, error)
	UpdateJobStatus(id, status string) error
	UpdateJobResult(id, providerAttemptID string, costActual any) error
	UpdateJobError(id, errorCode, errorMessage string) error
	UpdateJobHeartbeat(id string) error
	ListStaleJobs(leaseDuration time.Duration) ([]RegenerationJob, error)
	GetJobByIdempotencyKey(sessionID, key string) (*RegenerationJob, error)

	CreateCandidate(candidate *EditCandidate) error
	GetCandidate(id string) (*EditCandidate, error)
	ListCandidatesBySession(sessionID string) ([]EditCandidate, error)
	ListPendingCandidates(sessionID string) ([]EditCandidate, error)
	UpdateCandidateStatus(id, status, decidedBy string) error

	CreateMaskPatch(patch *MaskPatch) error
	ListMaskPatchesBySession(sessionID string) ([]MaskPatch, error)

	CreatePublishJournal(journal *PublishJournal) error
	GetPublishJournal(id string) (*PublishJournal, error)
	ListPendingJournals() ([]PublishJournal, error)
	UpdateJournalStatus(id, status, errorMessage string) error

	CreateIdempotencyRecord(record *EditIdempotencyRecord) error
	GetIdempotencyRecord(userID, sessionID, key string) (*EditIdempotencyRecord, error)
	UpdateIdempotencyRecord(id, status string, resultJSON string) error

	CreateRegenerationJournal(journal *RegenerationJournal) error
	ListJournalsByJob(jobID string) ([]RegenerationJournal, error)

	CreateCandidateRevisionMetadata(meta *CandidateRevisionMetadata) error
	GetCandidateRevisionMetadata(candidateRevisionID string) (*CandidateRevisionMetadata, error)
	UpdateCandidateRevisionMetadataStatus(candidateRevisionID, status string) error

	CreateAuditLog(logEntry *EditAuditLog) error

	CASUpdateActiveBinding(processingTaskID, actionKey string, expectedBindingVersion int64, newRevisionID, activatedBy, reason string) (bool, error)
	UpdateJobFields(id string, fields map[string]any) error
	UpdateCandidateFields(id string, fields map[string]any) error
	ListJobsForRecovery(leaseDuration time.Duration) ([]RegenerationJob, error)
	ListExpiredCandidates(retentionDays int) ([]EditCandidate, error)
	GetCandidateByRevisionID(revisionID string) (*EditCandidate, error)

	DB() *gorm.DB
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) DB() *gorm.DB { return r.db }

func (r *repository) CreateActionRevision(rev *ActionRevision) error {
	return r.db.Create(rev).Error
}

func (r *repository) GetActionRevision(id string) (*ActionRevision, error) {
	var rev ActionRevision
	err := r.db.Where("id = ?", id).First(&rev).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRevisionNotFound
		}
		return nil, err
	}
	return &rev, nil
}

func (r *repository) ListActionRevisions(processingTaskID, actionKey string) ([]ActionRevision, error) {
	var revs []ActionRevision
	err := r.db.Where("processing_task_id = ? AND action_key = ?", processingTaskID, actionKey).
		Order("revision_number ASC").Find(&revs).Error
	return revs, err
}

func (r *repository) UpdateActionRevisionStatus(id, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	updates := map[string]any{"status": status, "updated_at": now}
	if status == RevisionStatusReady || status == RevisionStatusQualityPending {
		updates["ready_at"] = now
	}
	return r.db.Model(&ActionRevision{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) UpdateActionRevisionQuality(id, evaluationID, verdict string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	status := RevisionStatusQualityReady
	if verdict == "rejected" || verdict == "needs_review" {
		status = RevisionStatusQualityReady
	}
	return r.db.Model(&ActionRevision{}).Where("id = ?", id).Updates(map[string]any{
		"quality_evaluation_id": evaluationID,
		"quality_verdict":       verdict,
		"status":                status,
		"updated_at":            now,
	}).Error
}

func (r *repository) WritebackQualitySnapshotCAS(id, contentHash, evaluationID, profileID, ruleSetVersion, verdict string, score *float64, sourceContentHash string) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	updates := map[string]any{
		"quality_evaluation_id":        evaluationID,
		"quality_profile_id":           profileID,
		"quality_ruleset_version":      ruleSetVersion,
		"quality_verdict":              verdict,
		"quality_overall_score":        score,
		"quality_source_content_hash":  sourceContentHash,
		"quality_evaluated_at":         now,
		"updated_at":                   now,
	}
	query := r.db.Model(&ActionRevision{}).Where("id = ?", id)
	if contentHash != "" {
		query = query.Where("content_hash = ?", contentHash)
	}
	result := query.Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *repository) GetActionRevisionContentHash(id string) (string, error) {
	var rev ActionRevision
	err := r.db.Select("content_hash").Where("id = ?", id).First(&rev).Error
	if err != nil {
		return "", err
	}
	return rev.ContentHash, nil
}

func (r *repository) UpdateActionRevisionManifest(id, path, hash string, frameCount, durationMS int) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&ActionRevision{}).Where("id = ?", id).Updates(map[string]any{
		"manifest_path": path,
		"manifest_hash": hash,
		"frame_count":   frameCount,
		"duration_ms":   durationMS,
		"updated_at":    now,
	}).Error
}

func (r *repository) GetNextRevisionNumber(processingTaskID, actionKey string) (int, error) {
	var maxNum int
	err := r.db.Model(&ActionRevision{}).
		Where("processing_task_id = ? AND action_key = ?", processingTaskID, actionKey).
		Select("COALESCE(MAX(revision_number), 0)").Scan(&maxNum).Error
	if err != nil {
		return 0, err
	}
	return maxNum + 1, nil
}

func (r *repository) CreateRevisionFrames(frames []ActionRevisionFrame) error {
	if len(frames) == 0 {
		return nil
	}
	return r.db.CreateInBatches(frames, 100).Error
}

func (r *repository) ListRevisionFrames(revisionID string) ([]ActionRevisionFrame, error) {
	var frames []ActionRevisionFrame
	err := r.db.Where("revision_id = ?", revisionID).Order("logical_index ASC").Find(&frames).Error
	return frames, err
}

func (r *repository) DeleteRevisionFrames(revisionID string) error {
	return r.db.Where("revision_id = ?", revisionID).Delete(&ActionRevisionFrame{}).Error
}

func (r *repository) GetActiveRevisionBinding(processingTaskID, actionKey string) (*ActiveRevisionBinding, error) {
	var binding ActiveRevisionBinding
	err := r.db.Where("processing_task_id = ? AND action_key = ?", processingTaskID, actionKey).First(&binding).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &binding, nil
}

func (r *repository) UpsertActiveRevisionBinding(binding *ActiveRevisionBinding) error {
	now := time.Now().UTC().Format(time.RFC3339)
	binding.UpdatedAt = now
	if binding.CreatedAt == "" {
		binding.CreatedAt = now
	}
	result := r.db.Where("processing_task_id = ? AND action_key = ?",
		binding.ProcessingTaskID, binding.ActionKey).
		Assign(binding).FirstOrCreate(binding)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return r.db.Save(binding).Error
	}
	return nil
}

func (r *repository) ListActiveBindings(processingTaskID string) ([]ActiveRevisionBinding, error) {
	var bindings []ActiveRevisionBinding
	err := r.db.Where("processing_task_id = ?", processingTaskID).Find(&bindings).Error
	return bindings, err
}

func (r *repository) CreateFrameAsset(asset *FrameAsset) error {
	return r.db.Create(asset).Error
}

func (r *repository) GetFrameAsset(id string) (*FrameAsset, error) {
	var asset FrameAsset
	err := r.db.Where("id = ?", id).First(&asset).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAssetNotFound
		}
		return nil, err
	}
	return &asset, nil
}

func (r *repository) GetFrameAssetByHash(hash, mimeType string) (*FrameAsset, error) {
	var asset FrameAsset
	err := r.db.Where("content_hash = ? AND mime_type = ?", hash, mimeType).First(&asset).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &asset, nil
}

func (r *repository) UpdateFrameAssetStatus(id, status string) error {
	return r.db.Model(&FrameAsset{}).Where("id = ?", id).Update("status", status).Error
}

func (r *repository) CreateEditSession(session *EditSession) error {
	return r.db.Create(session).Error
}

func (r *repository) GetEditSession(id string) (*EditSession, error) {
	var session EditSession
	err := r.db.Where("id = ?", id).First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return &session, nil
}

func (r *repository) ListOpenSessions(userID string) ([]EditSession, error) {
	var sessions []EditSession
	err := r.db.Where("user_id = ? AND status = ?", userID, SessionStatusOpen).Find(&sessions).Error
	return sessions, err
}

func (r *repository) ListSessionsByTaskAction(processingTaskID, actionKey string) ([]EditSession, error) {
	var sessions []EditSession
	err := r.db.Where("processing_task_id = ? AND action_key = ? AND status = ?",
		processingTaskID, actionKey, SessionStatusOpen).Find(&sessions).Error
	return sessions, err
}

func (r *repository) UpdateSessionVersion(id string, expectedVersion int64) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result := r.db.Model(&EditSession{}).
		Where("id = ? AND session_version = ?", id, expectedVersion).
		Updates(map[string]any{
			"session_version": expectedVersion + 1,
			"updated_at":      now,
		})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		var session EditSession
		err := r.db.Where("id = ?", id).First(&session).Error
		if err != nil {
			return 0, err
		}
		return session.SessionVersion, ErrSessionNotFound
	}
	return expectedVersion + 1, nil
}

func (r *repository) UpdateSessionStatus(id, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&EditSession{}).Where("id = ?", id).Updates(map[string]any{
		"status":     status,
		"updated_at": now,
	}).Error
}

func (r *repository) UpdateSessionCursor(id string, cursor, lastOpSeq int) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&EditSession{}).Where("id = ?", id).Updates(map[string]any{
		"cursor":             cursor,
		"last_operation_seq": lastOpSeq,
		"updated_at":         now,
	}).Error
}

func (r *repository) UpdateSessionCheckpoint(id, checkpointID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&EditSession{}).Where("id = ?", id).Updates(map[string]any{
		"checkpoint_id": checkpointID,
		"updated_at":    now,
	}).Error
}

func (r *repository) UpdateSessionCommitted(id, revisionID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&EditSession{}).Where("id = ?", id).Updates(map[string]any{
		"status":                SessionStatusCommitted,
		"committed_revision_id": revisionID,
		"updated_at":            now,
	}).Error
}

func (r *repository) UpdateSessionExpiry(id string, expiresAt string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&EditSession{}).Where("id = ?", id).Updates(map[string]any{
		"expires_at": expiresAt,
		"updated_at": now,
	}).Error
}

func (r *repository) ListExpiredSessions() ([]EditSession, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var sessions []EditSession
	err := r.db.Where("status = ? AND expires_at != '' AND expires_at < ?", SessionStatusOpen, now).Find(&sessions).Error
	return sessions, err
}

func (r *repository) CreateOperation(op *EditOperation) error {
	return r.db.Create(op).Error
}

func (r *repository) GetOperation(sessionID string, sequence int) (*EditOperation, error) {
	var op EditOperation
	err := r.db.Where("session_id = ? AND sequence = ?", sessionID, sequence).First(&op).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &op, nil
}

func (r *repository) ListOperations(sessionID string) ([]EditOperation, error) {
	var ops []EditOperation
	err := r.db.Where("session_id = ?", sessionID).Order("sequence ASC").Find(&ops).Error
	return ops, err
}

func (r *repository) ListOperationsSince(sessionID string, sinceSeq int) ([]EditOperation, error) {
	var ops []EditOperation
	err := r.db.Where("session_id = ? AND sequence > ?", sessionID, sinceSeq).Order("sequence ASC").Find(&ops).Error
	return ops, err
}

func (r *repository) UpdateOperationStatus(id, status string) error {
	return r.db.Model(&EditOperation{}).Where("id = ?", id).Update("status", status).Error
}

func (r *repository) GetOperationByIdempotencyKey(sessionID, key string) (*EditOperation, error) {
	var op EditOperation
	err := r.db.Where("session_id = ? AND idempotency_key = ?", sessionID, key).First(&op).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &op, nil
}

func (r *repository) CreateCheckpoint(cp *EditCheckpoint) error {
	return r.db.Create(cp).Error
}

func (r *repository) ListCheckpoints(sessionID string) ([]EditCheckpoint, error) {
	var cps []EditCheckpoint
	err := r.db.Where("session_id = ?", sessionID).Order("sequence DESC").Find(&cps).Error
	return cps, err
}

func (r *repository) DeleteOldCheckpoints(sessionID string, keep int) error {
	var cps []EditCheckpoint
	err := r.db.Where("session_id = ?", sessionID).Order("sequence DESC").Limit(keep).Find(&cps).Error
	if err != nil || len(cps) < keep {
		return err
	}
	keepIDs := make([]string, len(cps))
	for i, cp := range cps {
		keepIDs[i] = cp.ID
	}
	return r.db.Where("session_id = ? AND id NOT IN ?", sessionID, keepIDs).Delete(&EditCheckpoint{}).Error
}

func (r *repository) CreateRegenerationJob(job *RegenerationJob) error {
	return r.db.Create(job).Error
}

func (r *repository) GetRegenerationJob(id string) (*RegenerationJob, error) {
	var job RegenerationJob
	err := r.db.Where("id = ?", id).First(&job).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

func (r *repository) ListJobsBySession(sessionID string) ([]RegenerationJob, error) {
	var jobs []RegenerationJob
	err := r.db.Where("session_id = ?", sessionID).Order("created_at DESC").Find(&jobs).Error
	return jobs, err
}

func (r *repository) ListPendingJobs() ([]RegenerationJob, error) {
	var jobs []RegenerationJob
	err := r.db.Where("status IN ?", []string{
		JobStatusCreated, JobStatusQueued,
		JobStatusPreparing, JobStatusSubmitting, JobStatusPolling,
		JobStatusArtifactReady, JobStatusProcessing,
		JobStatusCandidateCommitting, JobStatusQualityPending, JobStatusQualityRunning,
		JobStatusRunning, JobStatusProviderSucceeded, JobStatusMaterializing,
	}).Find(&jobs).Error
	return jobs, err
}

func (r *repository) UpdateJobStatus(id, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&RegenerationJob{}).Where("id = ?", id).Updates(map[string]any{
		"status":     status,
		"updated_at": now,
	}).Error
}

func (r *repository) UpdateJobResult(id, providerAttemptID string, costActual any) error {
	now := time.Now().UTC().Format(time.RFC3339)
	costJSON := "{}"
	if costActual != nil {
		if b, err := json.Marshal(costActual); err == nil {
			costJSON = string(b)
		}
	}
	return r.db.Model(&RegenerationJob{}).Where("id = ?", id).Updates(map[string]any{
		"provider_attempt_id": providerAttemptID,
		"cost_actual_json":    costJSON,
		"status":              JobStatusCompleted,
		"updated_at":          now,
	}).Error
}

func (r *repository) UpdateJobError(id, errorCode, errorMessage string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&RegenerationJob{}).Where("id = ?", id).Updates(map[string]any{
		"error_code":    errorCode,
		"error_message": errorMessage,
		"status":        JobStatusFailed,
		"updated_at":    now,
	}).Error
}

func (r *repository) GetJobByIdempotencyKey(sessionID, key string) (*RegenerationJob, error) {
	var job RegenerationJob
	err := r.db.Where("session_id = ? AND idempotency_key = ?", sessionID, key).First(&job).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

func (r *repository) UpdateJobHeartbeat(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&RegenerationJob{}).Where("id = ?", id).Update("updated_at", now).Error
}

func (r *repository) ListStaleJobs(leaseDuration time.Duration) ([]RegenerationJob, error) {
	cutoff := time.Now().UTC().Add(-leaseDuration).Format(time.RFC3339)
	var jobs []RegenerationJob
	err := r.db.Where("status IN ? AND updated_at < ?", []string{JobStatusRunning, JobStatusProviderSucceeded, JobStatusMaterializing}, cutoff).Find(&jobs).Error
	return jobs, err
}

func (r *repository) CreateCandidate(candidate *EditCandidate) error {
	return r.db.Create(candidate).Error
}

func (r *repository) GetCandidate(id string) (*EditCandidate, error) {
	var candidate EditCandidate
	err := r.db.Where("id = ?", id).First(&candidate).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCandidateNotFound
		}
		return nil, err
	}
	return &candidate, nil
}

func (r *repository) ListCandidatesBySession(sessionID string) ([]EditCandidate, error) {
	var candidates []EditCandidate
	err := r.db.Where("session_id = ?", sessionID).Order("created_at DESC").Find(&candidates).Error
	return candidates, err
}

func (r *repository) ListPendingCandidates(sessionID string) ([]EditCandidate, error) {
	var candidates []EditCandidate
	err := r.db.Where("session_id = ? AND status = ?", sessionID, CandidateStatusPending).Find(&candidates).Error
	return candidates, err
}

func (r *repository) UpdateCandidateStatus(id, status, decidedBy string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&EditCandidate{}).Where("id = ?", id).Updates(map[string]any{
		"status":     status,
		"decided_by": decidedBy,
		"decided_at": now,
	}).Error
}

func (r *repository) CreateMaskPatch(patch *MaskPatch) error {
	return r.db.Create(patch).Error
}

func (r *repository) ListMaskPatchesBySession(sessionID string) ([]MaskPatch, error) {
	var patches []MaskPatch
	err := r.db.Where("session_id = ?", sessionID).Order("created_at DESC").Find(&patches).Error
	return patches, err
}

func (r *repository) CreatePublishJournal(journal *PublishJournal) error {
	return r.db.Create(journal).Error
}

func (r *repository) GetPublishJournal(id string) (*PublishJournal, error) {
	var journal PublishJournal
	err := r.db.Where("id = ?", id).First(&journal).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &journal, nil
}

func (r *repository) ListPendingJournals() ([]PublishJournal, error) {
	var journals []PublishJournal
	err := r.db.Where("status = ?", JournalStatusPending).Find(&journals).Error
	return journals, err
}

func (r *repository) UpdateJournalStatus(id, status, errorMessage string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	updates := map[string]any{"status": status}
	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}
	if status == JournalStatusCompleted || status == JournalStatusFailed {
		updates["completed_at"] = now
	}
	return r.db.Model(&PublishJournal{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) CreateIdempotencyRecord(record *EditIdempotencyRecord) error {
	return r.db.Create(record).Error
}

func (r *repository) GetIdempotencyRecord(userID, sessionID, key string) (*EditIdempotencyRecord, error) {
	var record EditIdempotencyRecord
	err := r.db.Where("user_id = ? AND session_id = ? AND idempotency_key = ?", userID, sessionID, key).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *repository) UpdateIdempotencyRecord(id, status string, resultJSON string) error {
	return r.db.Model(&EditIdempotencyRecord{}).Where("id = ?", id).Updates(map[string]any{
		"status":      status,
		"result_json": resultJSON,
	}).Error
}

func (r *repository) CreateRegenerationJournal(journal *RegenerationJournal) error {
	return r.db.Create(journal).Error
}

func (r *repository) ListJournalsByJob(jobID string) ([]RegenerationJournal, error) {
	var journals []RegenerationJournal
	err := r.db.Where("job_id = ?", jobID).Order("created_at ASC").Find(&journals).Error
	return journals, err
}

func (r *repository) CreateCandidateRevisionMetadata(meta *CandidateRevisionMetadata) error {
	return r.db.Create(meta).Error
}

func (r *repository) GetCandidateRevisionMetadata(candidateRevisionID string) (*CandidateRevisionMetadata, error) {
	var meta CandidateRevisionMetadata
	err := r.db.Where("candidate_revision_id = ?", candidateRevisionID).First(&meta).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &meta, nil
}

func (r *repository) UpdateCandidateRevisionMetadataStatus(candidateRevisionID, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&CandidateRevisionMetadata{}).
		Where("candidate_revision_id = ?", candidateRevisionID).
		Updates(map[string]any{"status": status, "updated_at": now}).Error
}

func (r *repository) CreateAuditLog(logEntry *EditAuditLog) error {
	return r.db.Create(logEntry).Error
}

func (r *repository) CASUpdateActiveBinding(processingTaskID, actionKey string, expectedBindingVersion int64, newRevisionID, activatedBy, reason string) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result := r.db.Model(&ActiveRevisionBinding{}).
		Where("processing_task_id = ? AND action_key = ? AND binding_version = ?",
			processingTaskID, actionKey, expectedBindingVersion).
		Updates(map[string]any{
			"revision_id":     newRevisionID,
			"binding_version": expectedBindingVersion + 1,
			"activated_by":    activatedBy,
			"reason":          reason,
			"updated_at":      now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *repository) UpdateJobFields(id string, fields map[string]any) error {
	now := time.Now().UTC().Format(time.RFC3339)
	fields["updated_at"] = now
	return r.db.Model(&RegenerationJob{}).Where("id = ?", id).Updates(fields).Error
}

func (r *repository) UpdateCandidateFields(id string, fields map[string]any) error {
	return r.db.Model(&EditCandidate{}).Where("id = ?", id).Updates(fields).Error
}

func (r *repository) ListJobsForRecovery(leaseDuration time.Duration) ([]RegenerationJob, error) {
	cutoff := time.Now().UTC().Add(-leaseDuration).Format(time.RFC3339)
	var jobs []RegenerationJob
	err := r.db.Where("status IN ? AND (lease_expires_at != '' AND lease_expires_at < ? OR heartbeat_at != '' AND heartbeat_at < ?)",
		[]string{JobStatusPreparing, JobStatusSubmitting, JobStatusPolling, JobStatusArtifactReady, JobStatusProcessing, JobStatusCandidateCommitting, JobStatusQualityPending, JobStatusQualityRunning},
		cutoff, cutoff).Find(&jobs).Error
	return jobs, err
}

func (r *repository) ListExpiredCandidates(retentionDays int) ([]EditCandidate, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
	var candidates []EditCandidate
	err := r.db.Where("status IN ? AND created_at < ?",
		[]string{CandidateStatusRejected, CandidateStatusExpired, CandidateStatusArchived}, cutoff).Find(&candidates).Error
	return candidates, err
}

func (r *repository) GetCandidateByRevisionID(revisionID string) (*EditCandidate, error) {
	var candidate EditCandidate
	err := r.db.Where("candidate_revision_id = ?", revisionID).First(&candidate).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &candidate, nil
}

func (r *repository) GetActionStreamByID(streamID string) (*ActionStream, error) {
	var stream ActionStream
	err := r.db.Where("id = ?", streamID).First(&stream).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &stream, nil
}

func (r *repository) GetActionStreamByKey(userID, characterID, actionKey string) (*ActionStream, error) {
	streamKey := fmt.Sprintf("%s:%s:%s", userID, characterID, actionKey)
	var stream ActionStream
	err := r.db.Where("stream_key = ?", streamKey).First(&stream).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &stream, nil
}

func (r *repository) GetActiveActionRevisionBindingByStream(streamID string) (*ActiveActionRevisionBinding, error) {
	var binding ActiveActionRevisionBinding
	err := r.db.Where("action_stream_id = ?", streamID).First(&binding).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &binding, nil
}

func (r *repository) GetActiveActionRevisionBindingByTask(processingTaskID, actionKey string) (*ActiveActionRevisionBinding, error) {
	var binding ActiveActionRevisionBinding
	err := r.db.Raw(`
		SELECT b.* FROM desktop_pet_active_action_revision_bindings b
		WHERE b.action_key = ? AND b.action_stream_id IN (
			SELECT s.id FROM desktop_pet_action_streams s
			WHERE s.root_processing_task_id = ?
		)
		LIMIT 1
	`, actionKey, processingTaskID).Scan(&binding).Error
	if err != nil {
		return nil, err
	}
	if binding.ID == "" {
		return nil, nil
	}
	return &binding, nil
}

func (r *repository) ListActionRevisionsByStream(streamID string) ([]ActionRevision, error) {
	var revs []ActionRevision
	err := r.db.Where("action_stream_id = ?", streamID).
		Order("revision_number ASC").Find(&revs).Error
	return revs, err
}

func (r *repository) ListAllActionStreams(userID string) ([]ActionStream, error) {
	var streams []ActionStream
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").Find(&streams).Error
	return streams, err
}
