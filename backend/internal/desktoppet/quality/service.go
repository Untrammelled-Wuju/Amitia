// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type qualityService struct {
	repo              QualityRepository
	engine            *Engine
	executor          *EngineExecutor
	profileRegistry   *ProfileRegistry
	gateEvaluator     *GateEvaluator
	measurementSrc    MeasurementSource
	events            EventPublisher
	committer         QualityCommitter
	taskGateService   TaskQualityGateService
	gateInvalidator   QualityGateInvalidator
	reviewDecisionSvc QualityReviewDecisionService
	recoveryWorker    QualityRecoveryWorker
	outboxPublisher   QualityOutboxPublisher
	inputRepository   QualityInputRepository
	measurementEngine ImageMeasurementEngine
}

type ServiceConfig struct {
	DB                *gorm.DB
	DataDir           string
	MeasurementSrc    MeasurementSource
	EventPublisher    EventPublisher
	Detectors         []Detector
	Repo              QualityRepository
	Committer         QualityCommitter
	TaskGateService   TaskQualityGateService
	GateInvalidator   QualityGateInvalidator
	ReviewDecisionSvc QualityReviewDecisionService
	RecoveryWorker    QualityRecoveryWorker
	OutboxPublisher   QualityOutboxPublisher
	InputRepository   QualityInputRepository
	MeasurementEngine ImageMeasurementEngine
}

func NewQualityService(cfg ServiceConfig) (QualityService, error) {
	var repo QualityRepository
	if cfg.Repo != nil {
		repo = cfg.Repo
	} else {
		repo = NewRepository(cfg.DB)
	}
	engine := NewEngine(EngineConfig{
		MeasurementSource: cfg.MeasurementSrc,
		DataDir:           cfg.DataDir,
		Detectors:         cfg.Detectors,
	})
	reportGen := NewReportGenerator(cfg.DataDir)

	var events EventPublisher
	if cfg.EventPublisher != nil {
		events = cfg.EventPublisher
	} else {
		events = NewNoopEventPublisher()
	}

	executor := NewEngineExecutor(engine, repo, reportGen, events)
	if cfg.Committer != nil {
		executor.SetCommitter(cfg.Committer)
	}
	gateEvaluator := NewGateEvaluator(repo)

	return &qualityService{
		repo:              repo,
		engine:            engine,
		executor:          executor,
		profileRegistry:   engine.ProfileRegistry(),
		gateEvaluator:     gateEvaluator,
		measurementSrc:    cfg.MeasurementSrc,
		events:            events,
		committer:         cfg.Committer,
		taskGateService:   cfg.TaskGateService,
		gateInvalidator:   cfg.GateInvalidator,
		reviewDecisionSvc: cfg.ReviewDecisionSvc,
		recoveryWorker:    cfg.RecoveryWorker,
		outboxPublisher:   cfg.OutboxPublisher,
		inputRepository:   cfg.InputRepository,
		measurementEngine: cfg.MeasurementEngine,
	}, nil
}

func (s *qualityService) CreateEvaluation(ctx context.Context, req CreateEvaluationRequest) (*QualityEvaluation, error) {
	qualityMode := req.QualityMode
	if qualityMode == "" {
		qualityMode = QualityModeBalanced
	}

	eval := &QualityEvaluation{
		ID:                  uuid.NewString(),
		ProcessingTaskID:    req.ProcessingTaskID,
		ProcessingActionID:  req.ProcessingActionID,
		ActionRevisionID:    req.ActionRevisionID,
		ActionKey:           req.ActionKey,
		ExecutionStatus:     EvalPending,
		ProfileSnapshotJSON: "",
		ProfileHash:         "",
		EngineVersion:       EngineVersion,
		QualityMode:         qualityMode,
	}

	if err := s.repo.CreateEvaluation(ctx, eval); err != nil {
		return nil, NewQualityError(ErrCodeDatabaseCommitFailed, "failed to create evaluation", err)
	}

	return eval, nil
}

func (s *qualityService) ExecuteEvaluation(ctx context.Context, eval *QualityEvaluation, req EvaluateRequest) (*EvaluateResult, error) {
	return s.executor.ExecuteEvaluation(ctx, eval, req)
}

func (s *qualityService) GetEvaluation(ctx context.Context, evaluationID string) (*ActionQualityResult, error) {
	eval, err := s.repo.GetEvaluation(ctx, evaluationID)
	if err != nil {
		return nil, err
	}

	findings, err := s.repo.ListFindings(ctx, evaluationID)
	if err != nil {
		return nil, fmt.Errorf("failed to list findings: %w", err)
	}

	scores, err := s.repo.ListDimensionScores(ctx, evaluationID)
	if err != nil {
		return nil, fmt.Errorf("failed to list dimension scores: %w", err)
	}

	return s.buildActionQualityResult(eval, findings, scores), nil
}

func (s *qualityService) GetActiveActionQuality(ctx context.Context, processingTaskID, actionKey string) (*ActionQualityResult, error) {
	eval, err := s.repo.GetActiveEvaluation(ctx, processingTaskID, actionKey)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	findings, err := s.repo.ListFindings(ctx, eval.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list findings: %w", err)
	}

	scores, err := s.repo.ListDimensionScores(ctx, eval.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list dimension scores: %w", err)
	}

	return s.buildActionQualityResult(eval, findings, scores), nil
}

func (s *qualityService) Reevaluate(ctx context.Context, req ReevaluateRequest) (*QualityEvaluation, error) {
	oldEval, err := s.repo.GetEvaluation(ctx, req.EvaluationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get original evaluation: %w", err)
	}

	qualityMode := req.QualityMode
	if qualityMode == "" {
		qualityMode = oldEval.QualityMode
	}

	newEval := &QualityEvaluation{
		ID:                     uuid.NewString(),
		ProcessingTaskID:       oldEval.ProcessingTaskID,
		ProcessingActionID:     oldEval.ProcessingActionID,
		ActionRevisionID:       oldEval.ActionRevisionID,
		ActionKey:              oldEval.ActionKey,
		ExecutionStatus:        EvalPending,
		EngineVersion:          EngineVersion,
		SupersedesEvaluationID: oldEval.ID,
		QualityMode:            qualityMode,
	}

	if err := s.repo.CreateEvaluation(ctx, newEval); err != nil {
		return nil, NewQualityError(ErrCodeDatabaseCommitFailed, "failed to create reevaluation", err)
	}

	return newEval, nil
}

func (s *qualityService) GetTaskGate(ctx context.Context, processingTaskID string) (*QualityGateResult, error) {
	return s.gateEvaluator.GetGateStatus(ctx, processingTaskID)
}

func (s *qualityService) EvaluateTaskGate(ctx context.Context, processingTaskID string, actionVerdicts []ActionVerdictSummary, profile QualityProfileSnapshot) (*QualityGateResult, error) {
	return s.gateEvaluator.EvaluateTaskGate(ctx, processingTaskID, actionVerdicts, profile)
}

func (s *qualityService) ListProblemFrames(ctx context.Context, evaluationID string, page, pageSize int) ([]ProblemFrameSummary, int64, error) {
	findings, err := s.repo.ListFindings(ctx, evaluationID)
	if err != nil {
		return nil, 0, err
	}

	frameFindings := make(map[int][]QualityFinding)
	for _, r := range findings {
		f := recordToFinding(r)
		for _, idx := range f.FrameIndexes {
			frameFindings[idx] = append(frameFindings[idx], f)
		}
	}

	all := make([]ProblemFrameSummary, 0, len(frameFindings))
	for idx, fs := range frameFindings {
		maxSeverity := SeverityInfo
		for _, f := range fs {
			if severitySortOrder(f.Severity) < severitySortOrder(maxSeverity) {
				maxSeverity = f.Severity
			}
		}
		all = append(all, ProblemFrameSummary{
			FrameIndex: idx,
			Findings:   fs,
			Severity:   maxSeverity,
		})
	}

	total := int64(len(all))
	start := (page - 1) * pageSize
	if start >= len(all) {
		return []ProblemFrameSummary{}, total, nil
	}
	end := start + pageSize
	if end > len(all) {
		end = len(all)
	}

	return all[start:end], total, nil
}

func (s *qualityService) ListFindings(ctx context.Context, evaluationID string, severity string, dimension string, page, pageSize int) ([]QualityFinding, int64, error) {
	offset := (page - 1) * pageSize
	records, total, err := s.repo.ListFindingsPaged(ctx, evaluationID, severity, dimension, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}

	findings := make([]QualityFinding, 0, len(records))
	for _, r := range records {
		findings = append(findings, recordToFinding(r))
	}

	return findings, total, nil
}

func (s *qualityService) CheckEvaluationOwnership(ctx context.Context, evaluationID, userID string) error {
	eval, err := s.repo.GetEvaluation(ctx, evaluationID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return NewQualityError(ErrCodeQualityEvaluationNotFound, "评估不存在", err)
		}
		return err
	}
	if eval.UserID != userID {
		return NewQualityError(ErrCodeQualityNotOwned, "评估不属于当前用户", nil)
	}
	return nil
}

func (s *qualityService) CheckProcessingTaskOwnership(ctx context.Context, processingTaskID, userID string) error {
	evals, err := s.repo.ListEvaluationsByTask(ctx, processingTaskID)
	if err != nil {
		return NewQualityError(ErrCodeQualityEvaluationNotFound, "处理任务不存在", err)
	}
	if len(evals) == 0 {
		return NewQualityError(ErrCodeQualityEvaluationNotFound, "处理任务不存在", nil)
	}
	if evals[0].UserID != userID {
		return NewQualityError(ErrCodeQualityNotOwned, "处理任务不属于当前用户", nil)
	}
	return nil
}

func (s *qualityService) buildActionQualityResult(eval *QualityEvaluation, findingRecords []QualityFindingRecord, scoreRecords []QualityDimensionScoreRecord) *ActionQualityResult {
	findings := RecordsToFindings(findingRecords)
	scores := recordsToDimensionScores(scoreRecords)

	var verdict *ContentVerdict
	if eval.Verdict != "" {
		v := ContentVerdict(eval.Verdict)
		verdict = &v
	}

	summary := FindingSummary{Total: len(findings)}
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			summary.Critical++
		case SeverityError:
			summary.Error++
		case SeverityReview:
			summary.Review++
		case SeverityWarning:
			summary.Warning++
		case SeverityInfo:
			summary.Info++
		}
		if f.HardGate {
			summary.HardGates++
		}
	}

	return &ActionQualityResult{
		EvaluationID:      eval.ID,
		ActionRevisionID:  eval.ActionRevisionID,
		ActionKey:         eval.ActionKey,
		ExecutionStatus:   eval.ExecutionStatus,
		Verdict:           verdict,
		OverallScore:      eval.OverallScore,
		OverallConfidence: eval.OverallConfidence,
		DimensionScores:   scores,
		FindingSummary:    summary,
		ProfileHash:       eval.ProfileHash,
		EngineVersion:     eval.EngineVersion,
		ReportPath:        eval.ReportPath,
		ReportHash:        eval.ReportHash,
	}
}

func recordsToDimensionScores(records []QualityDimensionScoreRecord) []DimensionScore {
	scores := make([]DimensionScore, 0, len(records))
	for _, r := range records {
		var details map[string]interface{}
		if r.DetailsJSON != "" {
			json.Unmarshal([]byte(r.DetailsJSON), &details)
		}
		ds := DimensionScore{
			Dimension:     r.DimensionKey,
			Applicability: Applicability(r.Applicability),
			Score:         r.Score,
			Confidence:    r.Confidence,
			Weight:        r.Weight,
		}
		scores = append(scores, ds)
	}
	return scores
}

func recordToFinding(r QualityFindingRecord) QualityFinding {
	var frameIdx []int
	var framePairs []FramePairRef
	var regions []RegionRef
	json.Unmarshal([]byte(r.FrameIndexesJSON), &frameIdx)
	json.Unmarshal([]byte(r.FramePairsJSON), &framePairs)
	json.Unmarshal([]byte(r.RegionsJSON), &regions)

	return QualityFinding{
		ID:              r.ID,
		RuleCode:        r.RuleCode,
		RuleVersion:     r.RuleVersion,
		Dimension:       r.DimensionKey,
		Severity:        Severity(r.Severity),
		MessageKey:      r.MessageKey,
		Message:         r.Message,
		FrameIndexes:    frameIdx,
		FramePairs:      framePairs,
		Regions:         regions,
		MetricName:      r.MetricName,
		ObservedValue:   r.ObservedValue,
		ThresholdValue:  r.ThresholdValue,
		Comparison:      r.Comparison,
		Confidence:      r.Confidence,
		HardGate:        r.HardGate,
		SuggestedAction: r.SuggestedAction,
		EvidenceRef:     r.EvidenceRef,
	}
}

func (s *qualityService) ProfileRegistry() *ProfileRegistry {
	return s.profileRegistry
}

func (s *qualityService) Engine() *Engine {
	return s.engine
}

func (s *qualityService) Repo() QualityRepository {
	return s.repo
}

func (s *qualityService) Committer() QualityCommitter {
	return s.committer
}

func (s *qualityService) TaskGateService() TaskQualityGateService {
	return s.taskGateService
}

func (s *qualityService) GateInvalidator() QualityGateInvalidator {
	return s.gateInvalidator
}

func (s *qualityService) ReviewDecisionService() QualityReviewDecisionService {
	return s.reviewDecisionSvc
}

func (s *qualityService) RecoveryWorker() QualityRecoveryWorker {
	return s.recoveryWorker
}

func (s *qualityService) OutboxPublisher() QualityOutboxPublisher {
	return s.outboxPublisher
}

func (s *qualityService) InputRepository() QualityInputRepository {
	return s.inputRepository
}

func (s *qualityService) MeasurementEngine() ImageMeasurementEngine {
	return s.measurementEngine
}

func (s *qualityService) AcquireLease(ctx context.Context, evaluationID, executionID, workerID string, leaseDuration string) (bool, error) {
	return s.repo.AcquireLease(ctx, evaluationID, executionID, workerID, leaseDuration)
}

func (s *qualityService) RenewLease(ctx context.Context, evaluationID, executionID, leaseDuration string) (bool, error) {
	return s.repo.RenewLease(ctx, evaluationID, executionID, leaseDuration)
}

func (s *qualityService) ReleaseLease(ctx context.Context, evaluationID string) error {
	return s.repo.ReleaseLease(ctx, evaluationID)
}

func (s *qualityService) ListEvaluationsByTask(ctx context.Context, processingTaskID string) ([]*QualityEvaluation, error) {
	return s.repo.ListEvaluationsByTask(ctx, processingTaskID)
}

func (s *qualityService) ListEvaluationsByAction(ctx context.Context, processingActionID string) ([]*QualityEvaluation, error) {
	return s.repo.ListEvaluationsByAction(ctx, processingActionID)
}

func (s *qualityService) ListPendingEvaluations(ctx context.Context) ([]*QualityEvaluation, error) {
	return s.repo.ListPendingEvaluations(ctx)
}

func (s *qualityService) ListEvaluationsByStatus(ctx context.Context, status string) ([]*QualityEvaluation, error) {
	return s.repo.ListEvaluationsByStatus(ctx, status)
}

func (s *qualityService) GetEvaluationByInput(ctx context.Context, actionRevisionID, profileHash, engineVersion string) (*QualityEvaluation, error) {
	return s.repo.GetEvaluationByInput(ctx, actionRevisionID, profileHash, engineVersion)
}

func (s *qualityService) UpdateEvaluation(ctx context.Context, eval *QualityEvaluation) error {
	return s.repo.UpdateEvaluation(ctx, eval)
}

func (s *qualityService) ComposeProfile(actionKey, actionSpecHash, loopType, anchorProfile string, frameCount int, backgroundPolicy, artStyle, qualityMode string) QualityProfileSnapshot {
	return s.profileRegistry.ComposeProfile(actionKey, actionSpecHash, loopType, anchorProfile, frameCount, backgroundPolicy, artStyle, qualityMode)
}

func (s *qualityService) Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
