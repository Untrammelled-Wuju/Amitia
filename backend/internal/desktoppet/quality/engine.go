// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type Engine struct {
	measurementSource MeasurementSource
	detectorManager   *DetectorManager
	ruleEvaluator     RuleEvaluator
	scorer            Scorer
	verdictResolver   VerdictResolver
	profileRegistry   *ProfileRegistry
	dataDir           string
}

type EngineConfig struct {
	MeasurementSource MeasurementSource
	DataDir           string
	Detectors         []Detector
}

func NewEngine(cfg EngineConfig) *Engine {
	dm := NewDetectorManager()

	engine := &Engine{
		measurementSource: cfg.MeasurementSource,
		detectorManager:   dm,
		ruleEvaluator:     NewRuleEvaluator(),
		scorer:            NewScorer(),
		verdictResolver:   NewVerdictResolver(),
		profileRegistry:   NewProfileRegistry(),
		dataDir:           cfg.DataDir,
	}

	if len(cfg.Detectors) > 0 {
		dm.RegisterAll(cfg.Detectors)
	}

	return engine
}

func (e *Engine) ProfileRegistry() *ProfileRegistry {
	return e.profileRegistry
}

func (e *Engine) Evaluate(ctx context.Context, req EvaluateRequest) (*EvaluateResult, error) {
	startTime := time.Now()

	measurements, err := e.measurementSource.LoadActionMeasurements(ctx, req.ActionRevisionID)
	if err != nil {
		return nil, NewQualityError(ErrCodeRevisionNotFound, "failed to load measurements", err)
	}
	if measurements == nil {
		return nil, ErrRevisionNotFound
	}

	if req.ExpectedRevisionHash != "" && measurements.RevisionHash != "" {
		if req.ExpectedRevisionHash != measurements.RevisionHash {
			return nil, ErrInputHashMismatch
		}
	}

	profile := req.Profile
	if profile.Hash == "" {
		profile = e.profileRegistry.ComposeProfile(
			req.ActionKey,
			measurements.ActionSpecHash,
			measurements.LoopType,
			measurements.AnchorProfile,
			measurements.FrameCount,
			profile.BackgroundPolicy,
			profile.ArtStyle,
			profile.QualityMode,
		)
	}

	var allObservations []Observation
	detectorErrors := make([]Observation, 0)

	for _, det := range e.detectorManager.Ordered() {
		detKey := det.Key()
		if !profile.DetectorEnabled(detKey) {
			continue
		}

		input := DetectorInput{
			Measurements: measurements,
			Profile:      profile,
		}

		observations, detErr := det.Detect(ctx, input)
		if detErr != nil {
			if profile.CanDegrade(detKey) {
				detectorErrors = append(detectorErrors, Observation{
					DetectorKey:     detKey,
					DetectorVersion: det.Version(),
					MetricName:      RuleDetectorFailure,
					Value:           1.0,
					Confidence:      0.3,
				})
				continue
			}
			return nil, NewQualityError(ErrCodeDetectorFailed, "detector "+detKey+" failed", detErr)
		}
		allObservations = append(allObservations, observations...)
	}
	allObservations = append(allObservations, detectorErrors...)

	findings, err := e.ruleEvaluator.Evaluate(ctx, allObservations, profile)
	if err != nil {
		return nil, NewQualityError(ErrCodeScoreInvalid, "rule evaluation failed", err)
	}

	scores, overallScore, overallConfidence, err := e.scorer.Score(findings, allObservations, profile)
	if err != nil {
		return nil, NewQualityError(ErrCodeScoreInvalid, "scoring failed", err)
	}

	verdict := e.verdictResolver.Resolve(findings, scores, overallScore, overallConfidence, profile)

	result := &ActionQualityResult{
		EvaluationID:      uuid.NewString(),
		ActionRevisionID:  req.ActionRevisionID,
		ActionKey:         req.ActionKey,
		ExecutionStatus:   EvalSucceeded,
		Verdict:           &verdict,
		OverallScore:      &overallScore,
		OverallConfidence: overallConfidence,
		DimensionScores:   scores,
		FindingSummary:    computeFindingSummary(findings),
		ProfileHash:       profile.Hash,
		EngineVersion:     EngineVersion,
	}

	duration := time.Since(startTime)

	return &EvaluateResult{
		Result:       result,
		Observations: allObservations,
		Findings:     findings,
		Scores:       scores,
		Verdict:      verdict,
		Profile:      profile,
		DurationMS:   duration.Milliseconds(),
	}, nil
}

func computeFindingSummary(findings []QualityFinding) FindingSummary {
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
	return summary
}

type EngineExecutor struct {
	engine    *Engine
	repo      QualityRepository
	report    *ReportGenerator
	events    EventPublisher
	committer QualityCommitter
}

func NewEngineExecutor(engine *Engine, repo QualityRepository, reportGen *ReportGenerator, events EventPublisher) *EngineExecutor {
	return &EngineExecutor{
		engine: engine,
		repo:   repo,
		report: reportGen,
		events: events,
	}
}

func (ex *EngineExecutor) SetCommitter(committer QualityCommitter) {
	ex.committer = committer
}

func (ex *EngineExecutor) updateEvaluationOwned(ctx context.Context, eval *QualityEvaluation, executionID, operation string) error {
	updated, err := ex.repo.UpdateEvaluationOwned(ctx, eval, executionID)
	if err != nil {
		return NewQualityError(ErrCodeDatabaseCommitFailed, operation, err)
	}
	if !updated {
		return NewQualityError(ErrCodeExecutionOwnershipLost, operation, ErrExecutionOwnershipLost)
	}
	return nil
}

func (ex *EngineExecutor) ExecuteEvaluation(ctx context.Context, eval *QualityEvaluation, req EvaluateRequest) (*EvaluateResult, error) {
	if eval == nil || req.ExecutionID == "" {
		return nil, NewQualityError(ErrCodeExecutionOwnershipLost, "missing evaluation execution owner", ErrExecutionOwnershipLost)
	}

	eval.ExecutionID = req.ExecutionID
	eval.WorkerID = req.WorkerID
	eval.ExecutionStatus = EvalRunning
	eval.StartedAt = time.Now().UTC().Format(time.RFC3339)
	if err := ex.updateEvaluationOwned(ctx, eval, req.ExecutionID, "failed to mark evaluation running"); err != nil {
		return nil, err
	}

	_ = ex.events.PublishQualityEvent(ctx, QualityEvent{
		JobID:            req.ExecutionID,
		ProcessingTaskID: req.ProcessingTaskID,
		ActionKey:        req.ActionKey,
		EvaluationID:     eval.ID,
		Stage:            "evaluation_started",
		Status:           string(EvalRunning),
	})

	result, err := ex.engine.Evaluate(ctx, req)
	if err != nil {
		// Cancellation is a lifecycle/lease signal, not a terminal quality result.
		// Leave the row running until the owner releases or recovery requeues it.
		if ctx.Err() != nil {
			return nil, NewQualityError(ErrCodeCancelled, "evaluation cancelled", ctx.Err())
		}

		eval.ExecutionStatus = EvalFailed
		eval.ErrorCode = ErrorCode(err)
		eval.ErrorMessage = err.Error()
		eval.CompletedAt = time.Now().UTC().Format(time.RFC3339)
		if persistErr := ex.updateEvaluationOwned(ctx, eval, req.ExecutionID, "failed to persist failed evaluation"); persistErr != nil {
			return nil, errors.Join(err, persistErr)
		}
		_ = ex.events.PublishQualityEvent(ctx, QualityEvent{
			JobID:            req.ExecutionID,
			ProcessingTaskID: req.ProcessingTaskID,
			ActionKey:        req.ActionKey,
			EvaluationID:     eval.ID,
			Stage:            "evaluation_failed",
			Status:           string(EvalFailed),
			Message:          err.Error(),
		})
		return nil, err
	}

	eval.Verdict = string(result.Verdict)
	eval.OverallScore = result.Result.OverallScore
	eval.OverallConfidence = result.Result.OverallConfidence
	eval.ExecutionStatus = EvalSucceeded
	eval.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	eval.ErrorCode = ""
	eval.ErrorMessage = ""
	eval.ProfileHash = result.Profile.Hash
	if eval.ProfileID == "" {
		eval.ProfileID = result.Profile.ProfileID
	}
	if result.Profile.ProfileVersion > 0 {
		eval.ProfileVersion = strconv.Itoa(result.Profile.ProfileVersion)
	}
	if profileJSON, marshalErr := json.Marshal(result.Profile); marshalErr == nil {
		eval.ProfileSnapshotJSON = string(profileJSON)
	}

	reportPath, reportHash, reportErr := ex.report.GenerateReport(eval, result.Result, result.Findings, result.Scores, result.Observations)
	if reportErr != nil {
		eval.ErrorCode = ErrCodeReportWriteFailed
		eval.ErrorMessage = reportErr.Error()
	} else {
		eval.ReportPath = reportPath
		eval.ReportHash = reportHash
	}

	if ex.committer != nil {
		commitReq := CommitEvaluationRequest{
			Evaluation:          eval,
			ExecutionID:         req.ExecutionID,
			Findings:            result.Findings,
			Scores:              result.Scores,
			Verdict:             result.Verdict,
			OverallScore:        derefFloat64(result.Result.OverallScore),
			OverallConfidence:   result.Result.OverallConfidence,
			ReportPath:          reportPath,
			ReportHash:          reportHash,
			ProfileSnapshotJSON: eval.ProfileSnapshotJSON,
			ProfileHash:         eval.ProfileHash,
			ProcessingTaskID:    req.ProcessingTaskID,
			ActionKey:           req.ActionKey,
		}
		_, commitErr := ex.committer.CommitEvaluation(ctx, commitReq)
		if commitErr != nil {
			return nil, NewQualityError(ErrCodeDatabaseCommitFailed, "committer failed", commitErr)
		}
	} else {
		if err := ex.updateEvaluationOwned(ctx, eval, req.ExecutionID, "failed to update evaluation result"); err != nil {
			return nil, err
		}
		findingRecords := FindingsToRecords(result.Findings, eval.ID)
		if err := ex.repo.CreateFindings(ctx, findingRecords); err != nil {
			return nil, NewQualityError(ErrCodeDatabaseCommitFailed, "failed to persist findings", err)
		}
		scoreRecords := ScoresToRecords(result.Scores, eval.ID)
		if err := ex.repo.CreateDimensionScores(ctx, scoreRecords); err != nil {
			return nil, NewQualityError(ErrCodeDatabaseCommitFailed, "failed to persist dimension scores", err)
		}
		if err := ex.repo.SetActiveEvaluation(ctx, req.ProcessingTaskID, req.ActionKey, eval.ID); err != nil {
			return nil, NewQualityError(ErrCodeDatabaseCommitFailed, "failed to set active evaluation", err)
		}
	}

	// A configured committer persists evaluation_completed into the
	// transactional outbox. Publishing it here as well would duplicate the same
	// terminal event once the outbox dispatcher flushes. The fallback path has
	// no durable outbox, so it still publishes directly.
	if ex.committer == nil {
		_ = ex.events.PublishQualityEvent(ctx, QualityEvent{
			JobID:            req.ExecutionID,
			ProcessingTaskID: req.ProcessingTaskID,
			ActionKey:        req.ActionKey,
			EvaluationID:     eval.ID,
			Stage:            "evaluation_completed",
			Status:           string(EvalSucceeded),
			Progress:         100,
		})
	}

	return result, nil
}

func ScoresToRecords(scores []DimensionScore, evaluationID string) []QualityDimensionScoreRecord {
	records := make([]QualityDimensionScoreRecord, 0, len(scores))
	for _, s := range scores {
		records = append(records, QualityDimensionScoreRecord{
			EvaluationID:  evaluationID,
			DimensionKey:  s.Dimension,
			Applicability: string(s.Applicability),
			Score:         s.Score,
			Confidence:    s.Confidence,
			Weight:        s.Weight,
		})
	}
	return records
}

func derefFloat64(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
