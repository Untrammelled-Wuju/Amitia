// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package writeback

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/quality"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

type EditingRepoInterface interface {
	GetActionRevision(id string) (actionRevisionID, contentHash string, err error)
	ListActiveBindings(processingTaskID string) ([]ActiveBindingInfo, error)
}

type ActiveBindingInfo struct {
	ActionKey  string
	RevisionID string
}

type ActiveBindingService struct {
	repo quality.QualityRepository
}

func NewActiveBindingService(repo quality.QualityRepository) *ActiveBindingService {
	return &ActiveBindingService{repo: repo}
}

func (s *ActiveBindingService) BindActiveEvaluation(ctx context.Context, actionRevisionID, profileID, evaluationID string) error {
	var bindingRevision int64
	existing, err := s.repo.GetActiveQualityBinding(ctx, actionRevisionID, profileID)
	if err != nil {
		return err
	}
	if existing != nil {
		bindingRevision = existing.BindingRevision
	}
	now := time.Now().UTC().Format(time.RFC3339)
	record := &quality.ActiveQualityEvaluationBindingRecord{
		ActionRevisionID:   actionRevisionID,
		ProfileID:          profileID,
		ActiveEvaluationID: evaluationID,
		BindingRevision:    bindingRevision + 1,
		BoundAt:            now,
	}
	return s.repo.CreateActiveQualityBinding(ctx, record)
}

func (s *ActiveBindingService) GetActiveBinding(ctx context.Context, actionRevisionID, profileID string) (*quality.ActiveQualityEvaluationBindingRecord, error) {
	return s.repo.GetActiveQualityBinding(ctx, actionRevisionID, profileID)
}

func (s *ActiveBindingService) UnbindActiveEvaluation(ctx context.Context, actionRevisionID, profileID string) error {
	return s.repo.UnbindActiveQualityEvaluation(ctx, actionRevisionID, profileID)
}

type QualityWritebackService struct {
	db *gorm.DB
}

func NewQualityWritebackService(db *gorm.DB) *QualityWritebackService {
	return &QualityWritebackService{db: db}
}

func (s *QualityWritebackService) WritebackQualitySnapshot(ctx context.Context, req quality.QualityWritebackRequest) error {
	now := time.Now().UTC().Format(time.RFC3339)
	updates := map[string]interface{}{
		"quality_evaluation_id":       req.EvaluationID,
		"quality_profile_id":          req.ProfileID,
		"quality_ruleset_version":     req.RuleSetVersion,
		"quality_verdict":             req.Verdict,
		"quality_overall_score":       req.Score,
		"quality_source_content_hash": req.SourceContentHash,
		"quality_evaluated_at":        now,
		"updated_at":                  now,
	}
	query := s.db.WithContext(ctx).Table("desktop_pet_action_revisions").Where("id = ?", req.ActionRevisionID)
	if req.ContentHash != "" {
		query = query.Where("content_hash = ?", req.ContentHash)
	}
	result := query.Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return quality.ErrWritebackStaleRevision
	}
	return nil
}

type Committer struct {
	db            *gorm.DB
	repo          quality.QualityRepository
	writeback     *QualityWritebackService
	activeBinding *ActiveBindingService
}

func NewCommitter(db *gorm.DB, repo quality.QualityRepository, writeback *QualityWritebackService, activeBinding *ActiveBindingService) *Committer {
	return &Committer{
		db:            db,
		repo:          repo,
		writeback:     writeback,
		activeBinding: activeBinding,
	}
}

func (c *Committer) CommitEvaluation(ctx context.Context, req quality.CommitEvaluationRequest) (*quality.CommitEvaluationResult, error) {
	ev := req.Evaluation
	if ev == nil {
		return nil, quality.NewQualityError(quality.ErrCodeDatabaseCommitFailed, "evaluation is nil", nil)
	}
	evaluationID := ev.ID
	now := time.Now().UTC().Format(time.RFC3339)

	ev.Verdict = string(req.Verdict)
	ev.OverallScore = &req.OverallScore
	ev.OverallConfidence = req.OverallConfidence
	ev.ReportPath = req.ReportPath
	ev.ReportHash = req.ReportHash
	ev.ProfileSnapshotJSON = req.ProfileSnapshotJSON
	ev.ProfileHash = req.ProfileHash
	ev.ExecutionStatus = quality.EvalSucceeded
	ev.CompletedAt = now

	var writebackApplied bool
	var activeBindingSet bool

	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := quality.NewRepository(tx)

		if err := txRepo.UpdateEvaluation(ctx, ev); err != nil {
			return quality.NewQualityError(quality.ErrCodeDatabaseCommitFailed, "failed to update evaluation", err)
		}

		findingRecords := quality.FindingsToRecords(req.Findings, evaluationID)
		if err := txRepo.CreateFindings(ctx, findingRecords); err != nil {
			return quality.NewQualityError(quality.ErrCodeDatabaseCommitFailed, "failed to create findings", err)
		}

		scoreRecords := quality.ScoresToRecords(req.Scores, evaluationID)
		if err := txRepo.CreateDimensionScores(ctx, scoreRecords); err != nil {
			return quality.NewQualityError(quality.ErrCodeDatabaseCommitFailed, "failed to create dimension scores", err)
		}

		if req.ProcessingTaskID != "" && req.ActionKey != "" {
			if err := txRepo.SetActiveEvaluation(ctx, req.ProcessingTaskID, req.ActionKey, evaluationID); err != nil {
				return quality.NewQualityError(quality.ErrCodeDatabaseCommitFailed, "failed to set active evaluation", err)
			}
		}

		writebackReq := quality.QualityWritebackRequest{
			ActionRevisionID:  ev.ActionRevisionID,
			ContentHash:       ev.ActionContentHash,
			EvaluationID:      evaluationID,
			ProfileID:         ev.ProfileID,
			RuleSetVersion:    ev.RuleSetVersion,
			Verdict:           string(req.Verdict),
			Score:             &req.OverallScore,
			SourceContentHash: ev.ActionContentHash,
		}
		if err := c.writeback.WritebackQualitySnapshot(ctx, writebackReq); err != nil {
			log.Logger.Warnf("committer: writeback failed for evaluation %s: %v", evaluationID, err)
		} else {
			writebackApplied = true
		}

		if ev.ActionRevisionID != "" && ev.ProfileID != "" {
			if err := c.activeBinding.BindActiveEvaluation(ctx, ev.ActionRevisionID, ev.ProfileID, evaluationID); err != nil {
				return quality.NewQualityError(quality.ErrCodeDatabaseCommitFailed, "failed to bind active evaluation", err)
			}
			activeBindingSet = true
		}

		stepsCompleted := "evaluation_updated,findings_persisted,scores_persisted"
		if writebackApplied {
			stepsCompleted += ",writeback_applied"
		}
		if activeBindingSet {
			stepsCompleted += ",active_binding_set"
		}
		journal := &quality.QualityCommitJournalRecord{
			EvaluationID:   evaluationID,
			CommitHash:     uuid.NewString(),
			Status:         string(quality.EvalSucceeded),
			StepsCompleted: stepsCompleted,
			CreatedAt:      now,
			CompletedAt:    now,
		}
		return txRepo.CreateCommitJournal(ctx, journal)
	})

	if err != nil {
		return nil, err
	}

	outboxEvent := quality.QualityOutboxEvent{
		EventType:        quality.OutboxEventEvaluationCompleted,
		UserID:           ev.UserID,
		CharacterID:      ev.CharacterID,
		ProcessingTaskID: ev.ProcessingTaskID,
		ActionKey:        ev.ActionKey,
		ActionRevisionID: ev.ActionRevisionID,
		EvaluationID:     evaluationID,
		Status:           string(quality.EvalSucceeded),
		Verdict:          string(req.Verdict),
		OccurredAt:       now,
	}
	payloadJSON, _ := json.Marshal(outboxEvent)
	outboxRecord := &quality.QualityOutboxEventRecord{
		EventType:   quality.OutboxEventEvaluationCompleted,
		PayloadJSON: string(payloadJSON),
	}
	if err := c.repo.CreateOutboxEvent(ctx, outboxRecord); err != nil {
		log.Logger.Errorf("committer: failed to create outbox event for %s: %v", evaluationID, err)
	}

	return &quality.CommitEvaluationResult{
		EvaluationID:     evaluationID,
		WritebackApplied: writebackApplied,
		ActiveBindingSet: activeBindingSet,
	}, nil
}
