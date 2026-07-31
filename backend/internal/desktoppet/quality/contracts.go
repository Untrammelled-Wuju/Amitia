// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"context"
	"image"
)

type MeasurementSource interface {
	LoadActionMeasurements(ctx context.Context, actionRevisionID string) (*ActionMeasurementSet, error)
	OpenFrame(ctx context.Context, actionRevisionID string, frameIndex int) (image.Image, error)
}

type Detector interface {
	Key() string
	Version() string
	Detect(ctx context.Context, input DetectorInput) ([]Observation, error)
}

type RuleEvaluator interface {
	Evaluate(ctx context.Context, observations []Observation, profile QualityProfileSnapshot) ([]QualityFinding, error)
}

type Scorer interface {
	Score(findings []QualityFinding, observations []Observation, profile QualityProfileSnapshot) ([]DimensionScore, float64, float64, error)
}

type VerdictResolver interface {
	Resolve(findings []QualityFinding, scores []DimensionScore, overallScore float64, overallConfidence float64, profile QualityProfileSnapshot) ContentVerdict
}

type QualityEngine interface {
	Evaluate(ctx context.Context, req EvaluateRequest) (*EvaluateResult, error)
}

type QualityService interface {
	CreateEvaluation(ctx context.Context, req CreateEvaluationRequest) (*QualityEvaluation, error)
	GetEvaluation(ctx context.Context, evaluationID string) (*ActionQualityResult, error)
	GetActiveActionQuality(ctx context.Context, processingTaskID, actionKey string) (*ActionQualityResult, error)
	Reevaluate(ctx context.Context, req ReevaluateRequest) (*QualityEvaluation, error)
	GetTaskGate(ctx context.Context, processingTaskID string) (*QualityGateResult, error)
	ListProblemFrames(ctx context.Context, evaluationID string, page, pageSize int) ([]ProblemFrameSummary, int64, error)
	ListFindings(ctx context.Context, evaluationID string, severity string, dimension string, page, pageSize int) ([]QualityFinding, int64, error)
}

type QualityRepository interface {
	CreateEvaluation(ctx context.Context, ev *QualityEvaluation) error
	UpdateEvaluation(ctx context.Context, ev *QualityEvaluation) error
	GetEvaluation(ctx context.Context, evaluationID string) (*QualityEvaluation, error)
	GetEvaluationByInput(ctx context.Context, actionRevisionID, profileHash, engineVersion string) (*QualityEvaluation, error)
	GetActiveEvaluation(ctx context.Context, processingTaskID, actionKey string) (*QualityEvaluation, error)
	ListEvaluationsByTask(ctx context.Context, processingTaskID string) ([]*QualityEvaluation, error)
	ListEvaluationsByAction(ctx context.Context, processingActionID string) ([]*QualityEvaluation, error)
	ListPendingEvaluations(ctx context.Context) ([]*QualityEvaluation, error)
	ListEvaluationsByStatus(ctx context.Context, status string) ([]*QualityEvaluation, error)
	AcquireLease(ctx context.Context, evaluationID, executionID, workerID string, leaseDuration string) (bool, error)
	RenewLease(ctx context.Context, evaluationID, executionID string, leaseDuration string) (bool, error)
	ReleaseLease(ctx context.Context, evaluationID string) error
	SetActiveEvaluation(ctx context.Context, processingTaskID, actionKey, evaluationID string) error
	CreateFindings(ctx context.Context, findings []QualityFindingRecord) error
	ListFindings(ctx context.Context, evaluationID string) ([]QualityFindingRecord, error)
	ListFindingsPaged(ctx context.Context, evaluationID string, severity string, dimension string, offset, limit int) ([]QualityFindingRecord, int64, error)
	CreateDimensionScores(ctx context.Context, scores []QualityDimensionScoreRecord) error
	ListDimensionScores(ctx context.Context, evaluationID string) ([]QualityDimensionScoreRecord, error)
	UpsertGateResult(ctx context.Context, gate *QualityGateResultRecord) error
	GetGateResult(ctx context.Context, processingTaskID string) (*QualityGateResultRecord, error)
	DeleteGateResult(ctx context.Context, processingTaskID string) error
}

type QualityEvent struct {
	JobID            string `json:"jobId"`
	ProcessingTaskID string `json:"processingTaskId"`
	ActionKey        string `json:"actionKey"`
	EvaluationID     string `json:"evaluationId"`
	Stage            string `json:"stage"`
	Progress         int    `json:"progress"`
	Status           string `json:"status"`
	Message          string `json:"message"`
	Sequence         int64  `json:"sequence"`
}

type EventPublisher interface {
	PublishQualityEvent(ctx context.Context, event QualityEvent) error
}
