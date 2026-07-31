package generation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/generation/activebinding"
	"gorm.io/gorm"
)

type OutboxProcessor struct {
	db             *gorm.DB
	repo           OutboxRepository
	bindingService *activebinding.BindingService
	attemptRepo    AttemptRepository
	artifactRepo   ArtifactRepository
	batchSize      int
}

func NewOutboxProcessor(db *gorm.DB, repo OutboxRepository, bindingService *activebinding.BindingService, attemptRepo AttemptRepository, artifactRepo ArtifactRepository) *OutboxProcessor {
	return &OutboxProcessor{
		db:             db,
		repo:           repo,
		bindingService: bindingService,
		attemptRepo:    attemptRepo,
		artifactRepo:   artifactRepo,
		batchSize:      50,
	}
}

func (p *OutboxProcessor) WithBatchSize(size int) *OutboxProcessor {
	if size > 0 {
		p.batchSize = size
	}
	return p
}

func (p *OutboxProcessor) Process(ctx context.Context) error {
	entries, err := p.repo.ListPending(p.batchSize)
	if err != nil {
		return fmt.Errorf("list pending outbox entries: %w", err)
	}
	for i := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		entry := &entries[i]
		_ = p.processEntry(entry)
	}
	return nil
}

func (p *OutboxProcessor) processEntry(entry *GenerationOutboxEntry) error {
	acquired, err := p.repo.MarkProcessing(entry.ID)
	if err != nil {
		return fmt.Errorf("mark processing %s: %w", entry.ID, err)
	}
	if !acquired {
		return nil
	}
	if err := p.dispatch(entry); err != nil {
		retryErr := p.repo.IncrementRetry(entry.ID, err.Error())
		if retryErr != nil {
			return fmt.Errorf("dispatch failed: %v; increment retry: %w", err, retryErr)
		}
		return fmt.Errorf("dispatch failed: %w", err)
	}
	if err := p.repo.MarkCompleted(entry.ID); err != nil {
		return fmt.Errorf("mark completed %s: %w", entry.ID, err)
	}
	return nil
}

func (p *OutboxProcessor) dispatch(entry *GenerationOutboxEntry) error {
	switch OutboxEventType(entry.EventType) {
	case OutboxEventAttemptSucceeded:
		return p.handleAttemptSucceeded(entry)
	case OutboxEventAttemptFailed:
		return p.handleAttemptFailed(entry)
	case OutboxEventArtifactPersisted:
		return p.handleArtifactPersisted(entry)
	case OutboxEventActionCompleted:
		return p.handleActionCompleted(entry)
	case OutboxEventTaskCompleted:
		return p.handleTaskCompleted(entry)
	default:
		return fmt.Errorf("unknown outbox event type: %s", entry.EventType)
	}
}

type outboxAttemptSucceededPayload struct {
	TaskActionID      string `json:"taskActionId"`
	AttemptID         string `json:"attemptId"`
	PrimaryArtifactID string `json:"primaryArtifactId"`
	ArtifactHash      string `json:"artifactHash"`
}

func (p *OutboxProcessor) handleAttemptSucceeded(entry *GenerationOutboxEntry) error {
	if p.bindingService == nil {
		return fmt.Errorf("binding service is not configured")
	}
	var payload outboxAttemptSucceededPayload
	if err := json.Unmarshal([]byte(entry.PayloadJSON), &payload); err != nil {
		return fmt.Errorf("unmarshal attempt_succeeded payload: %w", err)
	}
	taskActionID := payload.TaskActionID
	if taskActionID == "" {
		taskActionID = entry.TaskActionID
	}
	attemptID := payload.AttemptID
	if attemptID == "" {
		attemptID = entry.AttemptID
	}
	if taskActionID == "" || attemptID == "" || payload.PrimaryArtifactID == "" {
		return fmt.Errorf("attempt_succeeded payload missing required fields: taskActionId=%s attemptId=%s primaryArtifactId=%s",
			taskActionID, attemptID, payload.PrimaryArtifactID)
	}
	tx := p.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("begin transaction: %w", tx.Error)
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()
	req := activebinding.BindRequest{
		TaskActionID:      taskActionID,
		AttemptID:         attemptID,
		PrimaryArtifactID: payload.PrimaryArtifactID,
		ArtifactHash:      payload.ArtifactHash,
		Reason:            "outbox_attempt_succeeded",
	}
	if err := p.bindingService.BindActiveArtifact(tx, req); err != nil {
		return fmt.Errorf("bind active artifact: %w", err)
	}
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	committed = true
	return nil
}

type outboxAttemptFailedPayload struct {
	AttemptID    string `json:"attemptId"`
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

func (p *OutboxProcessor) handleAttemptFailed(entry *GenerationOutboxEntry) error {
	var payload outboxAttemptFailedPayload
	if err := json.Unmarshal([]byte(entry.PayloadJSON), &payload); err != nil {
		return fmt.Errorf("unmarshal attempt_failed payload: %w", err)
	}
	attemptID := payload.AttemptID
	if attemptID == "" {
		attemptID = entry.AttemptID
	}
	if attemptID == "" {
		return fmt.Errorf("attempt_failed payload missing attemptId")
	}
	now := nowRFC3339()
	updates := map[string]interface{}{
		"status":        string(AttemptStatusFailed),
		"error_code":    payload.ErrorCode,
		"error_message": payload.ErrorMessage,
		"completed_at":  now,
		"updated_at":    now,
	}
	if err := p.attemptRepo.UpdateAttemptStatus(attemptID, updates); err != nil {
		return fmt.Errorf("update attempt failed status: %w", err)
	}
	return nil
}

type outboxArtifactPersistedPayload struct {
	ArtifactID string `json:"artifactId"`
}

func (p *OutboxProcessor) handleArtifactPersisted(entry *GenerationOutboxEntry) error {
	var payload outboxArtifactPersistedPayload
	if err := json.Unmarshal([]byte(entry.PayloadJSON), &payload); err != nil {
		return fmt.Errorf("unmarshal artifact_persisted payload: %w", err)
	}
	if payload.ArtifactID == "" {
		return fmt.Errorf("artifact_persisted payload missing artifactId")
	}
	if err := p.artifactRepo.UpdateArtifact(payload.ArtifactID, map[string]interface{}{
		"status":     string(ArtifactStatusPersisted),
		"updated_at": nowRFC3339(),
	}); err != nil {
		return fmt.Errorf("update artifact persisted status: %w", err)
	}
	return nil
}

type outboxActionCompletedPayload struct {
	TaskActionID string `json:"taskActionId"`
}

func (p *OutboxProcessor) handleActionCompleted(entry *GenerationOutboxEntry) error {
	var payload outboxActionCompletedPayload
	if err := json.Unmarshal([]byte(entry.PayloadJSON), &payload); err != nil {
		return fmt.Errorf("unmarshal action_completed payload: %w", err)
	}
	taskActionID := payload.TaskActionID
	if taskActionID == "" {
		taskActionID = entry.TaskActionID
	}
	if taskActionID == "" {
		return fmt.Errorf("action_completed payload missing taskActionId")
	}
	now := nowRFC3339()
	result := p.db.Table("desktop_pet_generation_task_actions").
		Where("id = ?", taskActionID).
		Updates(map[string]interface{}{
			"status":       "succeeded",
			"progress":     100,
			"completed_at": now,
			"updated_at":   now,
		})
	if result.Error != nil {
		return fmt.Errorf("update task action completed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("task action not found: %s", taskActionID)
	}
	return nil
}

type outboxTaskCompletedPayload struct {
	TaskID string `json:"taskId"`
}

func (p *OutboxProcessor) handleTaskCompleted(entry *GenerationOutboxEntry) error {
	var payload outboxTaskCompletedPayload
	if err := json.Unmarshal([]byte(entry.PayloadJSON), &payload); err != nil {
		return fmt.Errorf("unmarshal task_completed payload: %w", err)
	}
	taskID := payload.TaskID
	if taskID == "" {
		taskID = entry.TaskID
	}
	if taskID == "" {
		return fmt.Errorf("task_completed payload missing taskId")
	}
	now := nowRFC3339()
	result := p.db.Table("desktop_pet_generation_tasks").
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":        "succeeded",
			"current_stage": "completed",
			"completed_at":  now,
			"updated_at":    now,
		})
	if result.Error != nil {
		return fmt.Errorf("update task completed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}
	return nil
}
