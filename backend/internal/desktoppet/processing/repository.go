// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const desktopPetTimeFormat = "2006-01-02 15:04:05"

type Repository interface {
	DB() *gorm.DB

	CreateProcessingTask(tx *gorm.DB, task *ProcessingTask) error
	GetProcessingTask(id string) (*ProcessingTask, error)
	ListProcessingTasksByGenerationTask(generationTaskID string) ([]ProcessingTask, error)
	UpdateProcessingTaskStatus(tx *gorm.DB, taskID string, updates map[string]interface{}) error
	UpdateProcessingTaskStatusNoTx(taskID string, updates map[string]interface{}) error
	ClaimProcessingTask(taskID string, workerID string, executionID string, leaseExpiresAt string) (bool, error)
	UpdateProcessingHeartbeat(taskID string, now string) error
	RefreshProcessingLease(taskID string, leaseExpiresAt string, now string) error
	ListRecoverableProcessingTasks() ([]ProcessingTask, error)
	ListQueuedProcessingTasks() ([]ProcessingTask, error)
	SetProcessingCancelRequested(taskID string, now string) error
	GetProcessingTaskForUpdate(tx *gorm.DB, taskID string) (*ProcessingTask, error)

	CreateProcessingActions(tx *gorm.DB, actions []ProcessingAction) error
	EnsureProcessingActions(tx *gorm.DB, actions []ProcessingAction) error
	ListProcessingActions(processingTaskID string) ([]ProcessingAction, error)
	ListProcessingActionsOrdered(processingTaskID string) ([]ProcessingAction, error)
	GetProcessingActionByActionKey(processingTaskID, actionKey string) (*ProcessingAction, error)
	UpdateProcessingAction(tx *gorm.DB, actionID string, updates map[string]interface{}) error
	UpdateProcessingActionNoTx(actionID string, updates map[string]interface{}) error
	UpdateProcessingActionAttempt(tx *gorm.DB, actionID string, attemptNumber int) error
	SetActionExcluded(actionID string, excluded bool) error
	UpdateProcessingActionOwned(tx *gorm.DB, actionID, executionID string, updates map[string]interface{}) (bool, error)
	UpdateProcessingActionWithRowVersion(tx *gorm.DB, actionID string, expectedRowVersion int64, updates map[string]interface{}) (bool, error)
	BeginProcessingActionAttempt(tx *gorm.DB, actionID string, expectedRowVersion int64, executionID string, sourceGenerationAttempt int) (*ProcessingActionAttempt, error)
	ListProcessingActionAttempts(processingActionID string) ([]ProcessingActionAttempt, error)
	GetLatestProcessingActionAttempt(processingActionID string) (*ProcessingActionAttempt, error)
	CreateProcessingActionAttemptRecord(tx *gorm.DB, attempt *ProcessingActionAttempt) error

	CreateProcessedFrames(tx *gorm.DB, frames []ProcessedFrame) error
	ListProcessedFramesByAction(processingActionID string) ([]ProcessedFrame, error)
	ListProcessedFramesByAttempt(processingAttemptID string) ([]ProcessedFrame, error)
	ListCurrentProcessedFrames(processingActionID string) ([]ProcessedFrame, error)
	UpdateProcessedFrame(tx *gorm.DB, frameID string, updates map[string]interface{}) error
	UpdateProcessedFrameNoTx(frameID string, updates map[string]interface{}) error

	UpdateProcessingTaskOwned(taskID, executionID string, updates map[string]interface{}) (bool, error)
	RefreshProcessingLeaseOwned(taskID, executionID, leaseExpiresAt, now string) (bool, error)
	RecoverExpiredProcessingTask(taskID, expectedExecutionID, expectedLeaseExpiresAt, now string) (bool, error)

	CreatePackage(pkg *Package) error
	GetPackage(id string) (*Package, error)
	UpdatePackageStatus(id string, updates map[string]interface{}) error
	ListPackagesByUser(userID string, page, pageSize int) ([]Package, int64, error)
	ListPackagesByGenerationTask(generationTaskID string) ([]Package, error)
	GetPackageByProcessingTaskID(processingTaskID string) (*Package, error)

	GetProcessingTaskRowVersion(taskID string) (int64, error)
	ResetProcessingActionToPending(actionID string) error

	GetGenerationTask(generationTaskID string) (*desktoppet.GenerationTask, error)
	ListActionsByTaskID(taskID string) ([]desktoppet.GenerationTaskAction, error)
	ListFramesByAction(taskActionID string) ([]desktoppet.GenerationFrame, error)
	ListSucceededActions(taskID string) ([]desktoppet.GenerationTaskAction, error)

	CreateProcessingRevision(tx *gorm.DB, rev *ProcessingRevision) error
	CreateProcessingArtifacts(tx *gorm.DB, artifacts []ProcessingArtifactRecord) error
	CreateProcessingTransforms(tx *gorm.DB, transforms []ProcessingTransformRecord) error
	CreateFrameMeasurements(tx *gorm.DB, measurements []FrameMeasurementRecord) error
	ActivateRevision(tx *gorm.DB, processingActionID, revisionID string) error
	GetActiveRevision(processingActionID string) (*ProcessingRevision, error)
	GetRevision(revisionID string) (*ProcessingRevision, error)
	ListRevisionsByAction(processingActionID string) ([]ProcessingRevision, error)
	ListArtifactsByRevision(revisionID string) ([]ProcessingArtifactRecord, error)
}

type repository struct {
	db  *gorm.DB
	ctx *app.AppContext
}

func NewRepository(db *gorm.DB, ctx *app.AppContext) Repository {
	return &repository{db: db, ctx: ctx}
}

func (r *repository) DB() *gorm.DB { return r.db }

func (r *repository) CreateProcessingTask(tx *gorm.DB, task *ProcessingTask) error {
	return tx.Create(task).Error
}

func (r *repository) GetProcessingTask(id string) (*ProcessingTask, error) {
	var task ProcessingTask
	err := r.db.Where("id = ?", id).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *repository) ListProcessingTasksByGenerationTask(generationTaskID string) ([]ProcessingTask, error) {
	var tasks []ProcessingTask
	err := r.db.Where("generation_task_id = ?", generationTaskID).
		Order("created_at DESC").
		Find(&tasks).Error
	if tasks == nil {
		tasks = []ProcessingTask{}
	}
	return tasks, err
}

func (r *repository) UpdateProcessingTaskStatus(tx *gorm.DB, taskID string, updates map[string]interface{}) error {
	return tx.Model(&ProcessingTask{}).Where("id = ?", taskID).Updates(updates).Error
}

func (r *repository) UpdateProcessingTaskStatusNoTx(taskID string, updates map[string]interface{}) error {
	return r.db.Model(&ProcessingTask{}).Where("id = ?", taskID).Updates(updates).Error
}

func (r *repository) UpdateProcessingTaskOwned(taskID, executionID string, updates map[string]interface{}) (bool, error) {
	result := r.db.Model(&ProcessingTask{}).
		Where("id = ? AND execution_id = ? AND status = ?", taskID, executionID, "processing").
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *repository) ClaimProcessingTask(taskID string, workerID string, executionID string, leaseExpiresAt string) (bool, error) {
	now := time.Now().Format(desktopPetTimeFormat)
	updates := map[string]interface{}{
		"status":            "processing",
		"worker_id":         workerID,
		"execution_id":      executionID,
		"lease_expires_at":  leaseExpiresAt,
		"last_heartbeat_at": now,
		"current_stage":     "validating_sources",
	}
	result := r.db.Model(&ProcessingTask{}).
		Where("id = ? AND status = ?", taskID, "queued").
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *repository) UpdateProcessingHeartbeat(taskID string, now string) error {
	return r.db.Model(&ProcessingTask{}).
		Where("id = ?", taskID).
		Update("last_heartbeat_at", now).Error
}

func (r *repository) RefreshProcessingLease(taskID string, leaseExpiresAt string, now string) error {
	return r.db.Model(&ProcessingTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"lease_expires_at":  leaseExpiresAt,
			"last_heartbeat_at": now,
		}).Error
}

func (r *repository) RefreshProcessingLeaseOwned(taskID, executionID, leaseExpiresAt, now string) (bool, error) {
	result := r.db.Model(&ProcessingTask{}).
		Where("id = ? AND execution_id = ? AND status = ?", taskID, executionID, "processing").
		Updates(map[string]interface{}{
			"lease_expires_at":  leaseExpiresAt,
			"last_heartbeat_at": now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *repository) ListRecoverableProcessingTasks() ([]ProcessingTask, error) {
	now := time.Now().Format(desktopPetTimeFormat)
	var tasks []ProcessingTask
	err := r.db.Where("status = ? AND lease_expires_at < ?", "processing", now).
		Order("lease_expires_at ASC").
		Find(&tasks).Error
	if tasks == nil {
		tasks = []ProcessingTask{}
	}
	return tasks, err
}

func (r *repository) RecoverExpiredProcessingTask(taskID, expectedExecutionID, expectedLeaseExpiresAt, now string) (bool, error) {
	result := r.db.Model(&ProcessingTask{}).
		Where("id = ? AND execution_id = ? AND lease_expires_at = ? AND status = ?", taskID, expectedExecutionID, expectedLeaseExpiresAt, "processing").
		Updates(map[string]interface{}{
			"status":            "queued",
			"execution_id":      "",
			"worker_id":         "",
			"lease_expires_at":  "",
			"last_heartbeat_at": "",
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *repository) ListQueuedProcessingTasks() ([]ProcessingTask, error) {
	var tasks []ProcessingTask
	err := r.db.Where("status = ?", "queued").
		Order("created_at ASC").
		Find(&tasks).Error
	if tasks == nil {
		tasks = []ProcessingTask{}
	}
	return tasks, err
}

func (r *repository) SetProcessingCancelRequested(taskID string, now string) error {
	return r.db.Model(&ProcessingTask{}).
		Where("id = ?", taskID).
		Update("cancel_requested_at", now).Error
}

func (r *repository) GetProcessingTaskForUpdate(tx *gorm.DB, taskID string) (*ProcessingTask, error) {
	var task ProcessingTask
	err := tx.Where("id = ?", taskID).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *repository) CreateProcessingActions(tx *gorm.DB, actions []ProcessingAction) error {
	if len(actions) == 0 {
		return nil
	}
	return tx.Create(&actions).Error
}

func (r *repository) EnsureProcessingActions(tx *gorm.DB, actions []ProcessingAction) error {
	if len(actions) == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&actions).Error
}

func (r *repository) ListProcessingActions(processingTaskID string) ([]ProcessingAction, error) {
	var actions []ProcessingAction
	err := r.db.Where("processing_task_id = ?", processingTaskID).
		Find(&actions).Error
	if actions == nil {
		actions = []ProcessingAction{}
	}
	return actions, err
}

func (r *repository) ListProcessingActionsOrdered(processingTaskID string) ([]ProcessingAction, error) {
	var actions []ProcessingAction
	err := r.db.Where("processing_task_id = ?", processingTaskID).
		Order("created_at ASC").
		Find(&actions).Error
	if actions == nil {
		actions = []ProcessingAction{}
	}
	return actions, err
}

func (r *repository) GetProcessingActionByActionKey(processingTaskID, actionKey string) (*ProcessingAction, error) {
	var action ProcessingAction
	err := r.db.Where("processing_task_id = ? AND action_key = ?", processingTaskID, actionKey).First(&action).Error
	if err != nil {
		return nil, err
	}
	return &action, nil
}

func (r *repository) UpdateProcessingAction(tx *gorm.DB, actionID string, updates map[string]interface{}) error {
	return tx.Model(&ProcessingAction{}).Where("id = ?", actionID).Updates(updates).Error
}

func (r *repository) UpdateProcessingActionNoTx(actionID string, updates map[string]interface{}) error {
	return r.db.Model(&ProcessingAction{}).Where("id = ?", actionID).Updates(updates).Error
}

func (r *repository) UpdateProcessingActionAttempt(tx *gorm.DB, actionID string, attemptNumber int) error {
	return tx.Model(&ProcessingAction{}).
		Where("id = ?", actionID).
		Update("source_attempt_number", attemptNumber).Error
}

func (r *repository) SetActionExcluded(actionID string, excluded bool) error {
	excludedVal := 0
	if excluded {
		excludedVal = 1
	}
	return r.db.Model(&ProcessingAction{}).
		Where("id = ?", actionID).
		Update("excluded", excludedVal).Error
}

func (r *repository) UpdateProcessingActionOwned(tx *gorm.DB, actionID, executionID string, updates map[string]interface{}) (bool, error) {
	result := tx.Model(&ProcessingAction{}).
		Where("id = ? AND active_execution_id = ?", actionID, executionID).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *repository) UpdateProcessingActionWithRowVersion(tx *gorm.DB, actionID string, expectedRowVersion int64, updates map[string]interface{}) (bool, error) {
	updates["row_version"] = expectedRowVersion + 1
	result := tx.Model(&ProcessingAction{}).
		Where("id = ? AND row_version = ?", actionID, expectedRowVersion).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *repository) BeginProcessingActionAttempt(tx *gorm.DB, actionID string, expectedRowVersion int64, executionID string, sourceGenerationAttempt int) (*ProcessingActionAttempt, error) {
	var action ProcessingAction
	if err := tx.Where("id = ? AND row_version = ?", actionID, expectedRowVersion).First(&action).Error; err != nil {
		return nil, err
	}

	newAttemptNumber := action.ProcessingAttempt + 1
	now := time.Now().Format(desktopPetTimeFormat)

	result := tx.Model(&ProcessingAction{}).
		Where("id = ? AND row_version = ?", actionID, expectedRowVersion).
		Updates(map[string]interface{}{
			"processing_attempt":  newAttemptNumber,
			"row_version":         expectedRowVersion + 1,
			"active_execution_id": executionID,
			"error_code":          "",
			"error_message":       "",
			"updated_at":          now,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("processing action %s row version mismatch", actionID)
	}

	attemptID := fmt.Sprintf("pa_%d_%d", time.Now().UnixNano(), newAttemptNumber)
	attempt := &ProcessingActionAttempt{
		ID:                      attemptID,
		ProcessingActionID:      actionID,
		ProcessingTaskID:        action.ProcessingTaskID,
		ActionKey:               action.ActionKey,
		AttemptNumber:           newAttemptNumber,
		SourceGenerationAttempt: sourceGenerationAttempt,
		ExecutionID:             executionID,
		Status:                  "pending",
		Progress:                0,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := tx.Create(attempt).Error; err != nil {
		return nil, err
	}
	return attempt, nil
}

func (r *repository) ListProcessingActionAttempts(processingActionID string) ([]ProcessingActionAttempt, error) {
	var attempts []ProcessingActionAttempt
	err := r.db.Where("processing_action_id = ?", processingActionID).
		Order("attempt_number DESC").
		Find(&attempts).Error
	if attempts == nil {
		attempts = []ProcessingActionAttempt{}
	}
	return attempts, err
}

func (r *repository) GetLatestProcessingActionAttempt(processingActionID string) (*ProcessingActionAttempt, error) {
	var attempt ProcessingActionAttempt
	err := r.db.Where("processing_action_id = ?", processingActionID).
		Order("attempt_number DESC").
		First(&attempt).Error
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (r *repository) CreateProcessingActionAttemptRecord(tx *gorm.DB, attempt *ProcessingActionAttempt) error {
	return tx.Create(attempt).Error
}

func (r *repository) CreateProcessedFrames(tx *gorm.DB, frames []ProcessedFrame) error {
	if len(frames) == 0 {
		return nil
	}
	return tx.Create(&frames).Error
}

func (r *repository) ListProcessedFramesByAction(processingActionID string) ([]ProcessedFrame, error) {
	var frames []ProcessedFrame
	err := r.db.Where("processing_action_id = ?", processingActionID).
		Order("frame_index ASC").
		Find(&frames).Error
	if frames == nil {
		frames = []ProcessedFrame{}
	}
	return frames, err
}

func (r *repository) ListProcessedFramesByAttempt(processingAttemptID string) ([]ProcessedFrame, error) {
	var frames []ProcessedFrame
	err := r.db.Where("processing_attempt_id = ?", processingAttemptID).
		Order("frame_index ASC").
		Find(&frames).Error
	if frames == nil {
		frames = []ProcessedFrame{}
	}
	return frames, err
}

func (r *repository) ListCurrentProcessedFrames(processingActionID string) ([]ProcessedFrame, error) {
	var action ProcessingAction
	if err := r.db.Where("id = ?", processingActionID).First(&action).Error; err != nil {
		return []ProcessedFrame{}, err
	}
	if action.ProcessingAttempt <= 0 {
		return []ProcessedFrame{}, nil
	}
	var attempt ProcessingActionAttempt
	err := r.db.Where("processing_action_id = ? AND attempt_number = ?", processingActionID, action.ProcessingAttempt).
		First(&attempt).Error
	if err != nil {
		return []ProcessedFrame{}, nil
	}
	return r.ListProcessedFramesByAttempt(attempt.ID)
}

func (r *repository) UpdateProcessedFrame(tx *gorm.DB, frameID string, updates map[string]interface{}) error {
	return tx.Model(&ProcessedFrame{}).Where("id = ?", frameID).Updates(updates).Error
}

func (r *repository) UpdateProcessedFrameNoTx(frameID string, updates map[string]interface{}) error {
	return r.db.Model(&ProcessedFrame{}).Where("id = ?", frameID).Updates(updates).Error
}

func (r *repository) CreatePackage(pkg *Package) error {
	return r.db.Create(pkg).Error
}

func (r *repository) GetPackage(id string) (*Package, error) {
	var pkg Package
	err := r.db.Where("id = ?", id).First(&pkg).Error
	if err != nil {
		return nil, err
	}
	return &pkg, nil
}

func (r *repository) UpdatePackageStatus(id string, updates map[string]interface{}) error {
	return r.db.Model(&Package{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) ListPackagesByUser(userID string, page, pageSize int) ([]Package, int64, error) {
	var packages []Package
	var total int64
	q := r.db.Model(&Package{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	err := q.Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&packages).Error
	if packages == nil {
		packages = []Package{}
	}
	return packages, total, err
}

func (r *repository) ListPackagesByGenerationTask(generationTaskID string) ([]Package, error) {
	var packages []Package
	err := r.db.Where("generation_task_id = ?", generationTaskID).
		Order("version DESC").
		Find(&packages).Error
	if packages == nil {
		packages = []Package{}
	}
	return packages, err
}

func (r *repository) GetGenerationTask(generationTaskID string) (*desktoppet.GenerationTask, error) {
	var task desktoppet.GenerationTask
	err := r.db.Where("id = ?", generationTaskID).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *repository) ListActionsByTaskID(taskID string) ([]desktoppet.GenerationTaskAction, error) {
	var actions []desktoppet.GenerationTaskAction
	err := r.db.Where("task_id = ?", taskID).
		Order("sort_order ASC").
		Find(&actions).Error
	if actions == nil {
		actions = []desktoppet.GenerationTaskAction{}
	}
	return actions, err
}

func (r *repository) ListFramesByAction(taskActionID string) ([]desktoppet.GenerationFrame, error) {
	var frames []desktoppet.GenerationFrame
	err := r.db.Where("task_action_id = ?", taskActionID).
		Order("frame_index ASC").
		Find(&frames).Error
	if frames == nil {
		frames = []desktoppet.GenerationFrame{}
	}
	return frames, err
}

func (r *repository) ListSucceededActions(taskID string) ([]desktoppet.GenerationTaskAction, error) {
	var actions []desktoppet.GenerationTaskAction
	err := r.db.Where("task_id = ? AND status = ?", taskID, "succeeded").
		Order("sort_order ASC").
		Find(&actions).Error
	if actions == nil {
		actions = []desktoppet.GenerationTaskAction{}
	}
	return actions, err
}

func (r *repository) GetProcessingTaskRowVersion(taskID string) (int64, error) {
	var task ProcessingTask
	err := r.db.Select("row_version").Where("id = ?", taskID).First(&task).Error
	if err != nil {
		return 0, err
	}
	return task.RowVersion, nil
}

func (r *repository) ResetProcessingActionToPending(actionID string) error {
	now := time.Now().Format(desktopPetTimeFormat)
	return r.db.Model(&ProcessingAction{}).
		Where("id = ? AND status IN ?", actionID, []string{"processing", "queued"}).
		Updates(map[string]interface{}{
			"status":        "pending",
			"progress":      0,
			"started_at":    "",
			"completed_at":  "",
			"updated_at":    now,
			"row_version":   gorm.Expr("row_version + 1"),
		}).Error
}

func (r *repository) GetPackageByProcessingTaskID(processingTaskID string) (*Package, error) {
	var pkg Package
	err := r.db.Where("processing_task_id = ?", processingTaskID).
		Order("created_at DESC").
		First(&pkg).Error
	if err != nil {
		return nil, err
	}
	return &pkg, nil
}

func (r *repository) CreateProcessingRevision(tx *gorm.DB, rev *ProcessingRevision) error {
	return tx.Create(rev).Error
}

func (r *repository) CreateProcessingArtifacts(tx *gorm.DB, artifacts []ProcessingArtifactRecord) error {
	if len(artifacts) == 0 {
		return nil
	}
	return tx.CreateInBatches(artifacts, 100).Error
}

func (r *repository) CreateProcessingTransforms(tx *gorm.DB, transforms []ProcessingTransformRecord) error {
	if len(transforms) == 0 {
		return nil
	}
	return tx.CreateInBatches(transforms, 100).Error
}

func (r *repository) CreateFrameMeasurements(tx *gorm.DB, measurements []FrameMeasurementRecord) error {
	if len(measurements) == 0 {
		return nil
	}
	return tx.CreateInBatches(measurements, 100).Error
}

func (r *repository) ActivateRevision(tx *gorm.DB, processingActionID, revisionID string) error {
	now := time.Now().Format(desktopPetTimeFormat)
	if err := tx.Model(&ProcessingRevision{}).
		Where("processing_action_id = ? AND active = 1", processingActionID).
		Updates(map[string]interface{}{"active": 0, "updated_at": now}).Error; err != nil {
		return err
	}
	if err := tx.Model(&ProcessingRevision{}).
		Where("id = ?", revisionID).
		Updates(map[string]interface{}{"active": 1, "status": "active", "updated_at": now}).Error; err != nil {
		return err
	}
	return tx.Model(&ProcessingAction{}).
		Where("id = ?", processingActionID).
		Updates(map[string]interface{}{"active_revision_id": revisionID, "updated_at": now}).Error
}

func (r *repository) GetActiveRevision(processingActionID string) (*ProcessingRevision, error) {
	var rev ProcessingRevision
	err := r.db.Where("processing_action_id = ? AND active = 1", processingActionID).
		Order("revision_number DESC").
		First(&rev).Error
	if err != nil {
		return nil, err
	}
	return &rev, nil
}

func (r *repository) GetRevision(revisionID string) (*ProcessingRevision, error) {
	var rev ProcessingRevision
	err := r.db.Where("id = ?", revisionID).First(&rev).Error
	if err != nil {
		return nil, err
	}
	return &rev, nil
}

func (r *repository) ListRevisionsByAction(processingActionID string) ([]ProcessingRevision, error) {
	var revs []ProcessingRevision
	err := r.db.Where("processing_action_id = ?", processingActionID).
		Order("revision_number DESC").
		Find(&revs).Error
	if revs == nil {
		revs = []ProcessingRevision{}
	}
	return revs, err
}

func (r *repository) ListArtifactsByRevision(revisionID string) ([]ProcessingArtifactRecord, error) {
	var artifacts []ProcessingArtifactRecord
	err := r.db.Where("revision_id = ?", revisionID).
		Order("frame_index ASC, artifact_kind ASC").
		Find(&artifacts).Error
	if artifacts == nil {
		artifacts = []ProcessingArtifactRecord{}
	}
	return artifacts, err
}
