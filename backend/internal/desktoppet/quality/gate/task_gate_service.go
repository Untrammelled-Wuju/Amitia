// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package gate

import (
	"context"

	"github.com/u-ai/backend/internal/desktoppet/quality"
	"gorm.io/gorm"
)

type TaskGateService struct {
	repo          quality.QualityRepository
	gateEvaluator *quality.GateEvaluator
}

func NewTaskGateService(repo quality.QualityRepository, gateEvaluator *quality.GateEvaluator) *TaskGateService {
	return &TaskGateService{
		repo:          repo,
		gateEvaluator: gateEvaluator,
	}
}

func (s *TaskGateService) Evaluate(ctx context.Context, req quality.EvaluateTaskGateRequest) (*quality.QualityGateResult, error) {
	actionVerdicts := make([]quality.ActionVerdictSummary, 0, len(req.RequiredActionKeys)+len(req.OptionalActionKeys))

	for _, actionKey := range req.RequiredActionKeys {
		eval, err := s.repo.GetActiveEvaluation(ctx, req.ProcessingTaskID, actionKey)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				actionVerdicts = append(actionVerdicts, quality.ActionVerdictSummary{
					ActionKey:       actionKey,
					Required:        true,
					Verdict:         quality.VerdictRejected,
					ExecutionStatus: quality.EvalFailed,
				})
				continue
			}
			return nil, err
		}
		actionVerdicts = append(actionVerdicts, s.buildVerdictSummary(ctx, eval, actionKey, true))
	}

	for _, actionKey := range req.OptionalActionKeys {
		eval, err := s.repo.GetActiveEvaluation(ctx, req.ProcessingTaskID, actionKey)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return nil, err
		}
		actionVerdicts = append(actionVerdicts, s.buildVerdictSummary(ctx, eval, actionKey, false))
	}

	profile := quality.QualityProfileSnapshot{}
	return s.gateEvaluator.EvaluateTaskGate(ctx, req.ProcessingTaskID, actionVerdicts, profile)
}

func (s *TaskGateService) buildVerdictSummary(ctx context.Context, eval *quality.QualityEvaluation, actionKey string, required bool) quality.ActionVerdictSummary {
	summary := quality.ActionVerdictSummary{
		ActionKey:        actionKey,
		Required:         required,
		Verdict:          quality.ContentVerdict(eval.Verdict),
		ExecutionStatus:  eval.ExecutionStatus,
		OverallScore:     eval.OverallScore,
		ActionRevisionID: eval.ActionRevisionID,
	}

	findings, err := s.repo.ListFindings(ctx, eval.ID)
	if err == nil {
		summary.FindingCount = len(findings)
		for _, f := range findings {
			if f.HardGate {
				summary.HardGateCount++
			}
		}
	}

	return summary
}

func (s *TaskGateService) GetValidGateForRelease(ctx context.Context, req quality.GetValidGateForReleaseRequest) (*quality.QualityGateResult, error) {
	record, err := s.repo.GetGateResult(ctx, req.ProcessingTaskID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	if record.ActiveRevisionSetHash != req.ActiveRevisionSetHash {
		return nil, nil
	}

	if record.InvalidatedAt != "" {
		return nil, nil
	}

	return s.gateEvaluator.GetGateStatus(ctx, req.ProcessingTaskID)
}

func (s *TaskGateService) GetGate(ctx context.Context, processingTaskID string) (*quality.QualityGateResult, error) {
	return s.gateEvaluator.GetGateStatus(ctx, processingTaskID)
}

type GateInvalidator struct {
	repo quality.QualityRepository
}

func NewGateInvalidator(repo quality.QualityRepository) *GateInvalidator {
	return &GateInvalidator{repo: repo}
}

func (g *GateInvalidator) InvalidateForActionRevision(ctx context.Context, processingTaskID, actionRevisionID string) error {
	return g.repo.InvalidateGateResult(ctx, processingTaskID)
}

func (g *GateInvalidator) InvalidateForTask(ctx context.Context, processingTaskID string) error {
	return g.repo.DeleteGateResult(ctx, processingTaskID)
}

func (g *GateInvalidator) InvalidateForRuleSetChange(ctx context.Context, processingTaskID, ruleSetVersion string) error {
	return g.repo.InvalidateGateResult(ctx, processingTaskID)
}

type ReviewDecisionService struct {
	repo quality.QualityRepository
}

func NewReviewDecisionService(repo quality.QualityRepository) *ReviewDecisionService {
	return &ReviewDecisionService{repo: repo}
}

func (s *ReviewDecisionService) CreateReviewDecision(ctx context.Context, req quality.ReviewDecisionRequest) (*quality.QualityReviewDecisionRecord, error) {
	record := &quality.QualityReviewDecisionRecord{
		EvaluationID:     req.EvaluationID,
		ActionRevisionID: req.ActionRevisionID,
		Decision:         req.Decision,
		Reason:           req.Reason,
		Reviewer:         req.Reviewer,
	}
	if err := s.repo.CreateReviewDecision(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *ReviewDecisionService) GetReviewDecision(ctx context.Context, evaluationID string) (*quality.QualityReviewDecisionRecord, error) {
	return s.repo.GetReviewDecision(ctx, evaluationID)
}
