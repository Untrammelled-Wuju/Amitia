// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"context"
	"image"
	"time"
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
	CheckEvaluationOwnership(ctx context.Context, evaluationID, userID string) error
	CheckProcessingTaskOwnership(ctx context.Context, processingTaskID, userID string) error
}

type QualityRepository interface {
	CreateEvaluation(ctx context.Context, ev *QualityEvaluation) error
	UpdateEvaluation(ctx context.Context, ev *QualityEvaluation) error
	UpdateEvaluationOwned(ctx context.Context, ev *QualityEvaluation, executionID string) (bool, error)
	GetEvaluation(ctx context.Context, evaluationID string) (*QualityEvaluation, error)
	GetEvaluationByInput(ctx context.Context, actionRevisionID, profileHash, engineVersion string) (*QualityEvaluation, error)
	GetActiveEvaluation(ctx context.Context, processingTaskID, actionKey string) (*QualityEvaluation, error)
	ListEvaluationsByTask(ctx context.Context, processingTaskID string) ([]*QualityEvaluation, error)
	ListEvaluationsByAction(ctx context.Context, processingActionID string) ([]*QualityEvaluation, error)
	ListPendingEvaluations(ctx context.Context) ([]*QualityEvaluation, error)
	ListEvaluationsByStatus(ctx context.Context, status string) ([]*QualityEvaluation, error)
	AcquireLease(ctx context.Context, evaluationID, executionID, workerID string, leaseDuration string) (bool, error)
	RenewLease(ctx context.Context, evaluationID, executionID string, leaseDuration string) (bool, error)
	ReleaseLease(ctx context.Context, evaluationID, executionID string) (bool, error)
	RecoverExpiredEvaluation(ctx context.Context, evaluationID, executionID string, now time.Time) (bool, error)
	SetActiveEvaluation(ctx context.Context, processingTaskID, actionKey, evaluationID string) error
	CreateFindings(ctx context.Context, findings []QualityFindingRecord) error
	ListFindings(ctx context.Context, evaluationID string) ([]QualityFindingRecord, error)
	ListFindingsPaged(ctx context.Context, evaluationID string, severity string, dimension string, offset, limit int) ([]QualityFindingRecord, int64, error)
	CreateDimensionScores(ctx context.Context, scores []QualityDimensionScoreRecord) error
	ListDimensionScores(ctx context.Context, evaluationID string) ([]QualityDimensionScoreRecord, error)
	UpsertGateResult(ctx context.Context, gate *QualityGateResultRecord) error
	GetGateResult(ctx context.Context, processingTaskID string) (*QualityGateResultRecord, error)
	DeleteGateResult(ctx context.Context, processingTaskID string) error
	InvalidateGateResult(ctx context.Context, processingTaskID string) error
	SupersedeEvaluation(ctx context.Context, evaluationID string) error
	UpdateEvaluationStatus(ctx context.Context, evaluationID string, status EvaluationExecutionStatus, errorCode, errorMessage string) error
	GetEvaluationByActionRevision(ctx context.Context, actionRevisionID string) (*QualityEvaluation, error)

	CreateActiveQualityBinding(ctx context.Context, binding *ActiveQualityEvaluationBindingRecord) error
	GetActiveQualityBinding(ctx context.Context, actionRevisionID, profileID string) (*ActiveQualityEvaluationBindingRecord, error)
	UnbindActiveQualityEvaluation(ctx context.Context, actionRevisionID, profileID string) error

	CreateCommitJournal(ctx context.Context, journal *QualityCommitJournalRecord) error

	CreateReviewDecision(ctx context.Context, decision *QualityReviewDecisionRecord) error
	GetReviewDecision(ctx context.Context, evaluationID string) (*QualityReviewDecisionRecord, error)

	GetMeasurementCache(ctx context.Context, frameArtifactID, contentHash string) (*QualityMeasurementCacheRecord, error)
	CreateMeasurementCache(ctx context.Context, cache *QualityMeasurementCacheRecord) error

	CreateOutboxEvent(ctx context.Context, event *QualityOutboxEventRecord) error
	ListPendingOutboxEvents(ctx context.Context, limit int) ([]QualityOutboxEventRecord, error)
	MarkOutboxEventPublished(ctx context.Context, eventID string) error
}

type QualityInputRepository interface {
	LoadActionRevisionInput(ctx context.Context, userID string, actionRevisionID string) (*QualityActionInput, error)
}

type QualityActionInput struct {
	UserID               string
	CharacterID          string
	ProcessingTaskID     string
	ProcessingActionID   string
	ActionKey            string
	ActionRevisionID     string
	ActionContentHash    string
	ProcessingRevisionID string
	PlaybackMode         string
	FPS                  int
	ExpectedFrameCount   int
	Frames               []QualityFrameInput
	InputSource          string
	InputSnapshotID      string
	InputSnapshot        *EvaluationInputSnapshot
}

type QualityFrameInput struct {
	FrameRevisionID string
	FrameArtifactID string
	FrameIndex      int
	AbsolutePath    string
	RelativePath    string
	ContentHash     string
	PixelHash       string
	MimeType        string
	Width           int
	Height          int
	SubjectBox      Rect
	Anchor          Point
	CoordinateSpace string
	AlphaCoverage   float64
	TransformChain  []Transform
	Measurements    map[string]float64
}

type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Transform struct {
	Type        string  `json:"type"`
	ScaleX      float64 `json:"scaleX"`
	ScaleY      float64 `json:"scaleY"`
	OffsetX     float64 `json:"offsetX"`
	OffsetY     float64 `json:"offsetY"`
	Rotation    float64 `json:"rotation"`
	SourceSpace string  `json:"sourceSpace"`
	TargetSpace string  `json:"targetSpace"`
}

type ImageMeasurementEngine interface {
	MeasureFrame(ctx context.Context, framePath string, contentHash string, frameArtifactID string) (*FrameMeasurementResult, error)
}

type FrameMeasurementResult struct {
	Width                 int
	Height                int
	HasAlphaChannel       bool
	AlphaCoverage         float64
	FullyTransparentRatio float64
	SemiTransparentRatio  float64
	OpaqueRatio           float64
	Decodable             bool
	MimeType              string
	PixelHash             string
	FileSize              int64
	FileHash              string
}

type QualityCommitter interface {
	CommitEvaluation(ctx context.Context, req CommitEvaluationRequest) (*CommitEvaluationResult, error)
}

type ActiveQualityEvaluationBindingService interface {
	BindActiveEvaluation(ctx context.Context, actionRevisionID, profileID, evaluationID string) error
	GetActiveBinding(ctx context.Context, actionRevisionID, profileID string) (*ActiveQualityEvaluationBindingRecord, error)
	UnbindActiveEvaluation(ctx context.Context, actionRevisionID, profileID string) error
}

type ActionRevisionQualityWriteback interface {
	WritebackQualitySnapshot(ctx context.Context, req QualityWritebackRequest) error
}

type QualityWritebackRequest struct {
	ActionRevisionID  string
	ContentHash       string
	EvaluationID      string
	ProfileID         string
	RuleSetVersion    string
	Verdict           string
	Score             *float64
	SourceContentHash string
}

type TaskQualityGateService interface {
	Evaluate(ctx context.Context, req EvaluateTaskGateRequest) (*QualityGateResult, error)
	GetValidGateForRelease(ctx context.Context, req GetValidGateForReleaseRequest) (*QualityGateResult, error)
	GetGate(ctx context.Context, processingTaskID string) (*QualityGateResult, error)
}

type QualityGateInvalidator interface {
	InvalidateForActionRevision(ctx context.Context, processingTaskID, actionRevisionID string) error
	InvalidateForTask(ctx context.Context, processingTaskID string) error
	InvalidateForRuleSetChange(ctx context.Context, processingTaskID, ruleSetVersion string) error
}

type QualityRecoveryWorker interface {
	Start(ctx context.Context)
	Stop()
	RecoverStuckEvaluations(ctx context.Context) (int, error)
}

type QualityOutboxPublisher interface {
	PublishEvent(ctx context.Context, event QualityOutboxEvent) error
	Flush(ctx context.Context) error
}

type QualityReviewDecisionService interface {
	CreateReviewDecision(ctx context.Context, req ReviewDecisionRequest) (*QualityReviewDecisionRecord, error)
	GetReviewDecision(ctx context.Context, evaluationID string) (*QualityReviewDecisionRecord, error)
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
