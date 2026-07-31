// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

type EvaluationExecutionStatus string

const (
	EvalCreated    EvaluationExecutionStatus = "created"
	EvalQueued     EvaluationExecutionStatus = "queued"
	EvalPending    EvaluationExecutionStatus = "pending"
	EvalRunning    EvaluationExecutionStatus = "running"
	EvalCommitting EvaluationExecutionStatus = "committing"
	EvalSucceeded  EvaluationExecutionStatus = "succeeded"
	EvalFailed     EvaluationExecutionStatus = "evaluation_failed"
	EvalCancelled  EvaluationExecutionStatus = "cancelled"
	EvalSuperseded EvaluationExecutionStatus = "superseded"
)

func IsTerminalStatus(s EvaluationExecutionStatus) bool {
	switch s {
	case EvalSucceeded, EvalFailed, EvalCancelled, EvalSuperseded:
		return true
	}
	return false
}

func IsRetryableErrorCode(code string) bool {
	switch code {
	case ErrCodeDetectorFailed, ErrCodeReportWriteFailed, ErrCodeDatabaseCommitFailed:
		return true
	}
	return false
}

func IsMaterialErrorCode(code string) bool {
	switch code {
	case ErrCodeFrameDecodeFailed, ErrCodeContentHashMismatch, ErrCodeMeasurementMissing:
		return true
	}
	return false
}

type ContentVerdict string

const (
	VerdictAccepted            ContentVerdict = "accepted"
	VerdictAcceptedWithWarning ContentVerdict = "accepted_with_warning"
	VerdictNeedsReview         ContentVerdict = "needs_review"
	VerdictRejected            ContentVerdict = "rejected"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityReview   Severity = "review"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

type GateStatus string

const (
	GatePending           GateStatus = "pending"
	GatePassed            GateStatus = "passed"
	GatePassedWithWarning GateStatus = "passed_with_warning"
	GateReviewRequired    GateStatus = "review_required"
	GateFailed            GateStatus = "failed"
	GateError             GateStatus = "error"
	GateStale             GateStatus = "stale"

	GatePassedWithWarnings GateStatus = "passed_with_warnings"
	GateBlocked            GateStatus = "blocked"
	GatePartialCandidate   GateStatus = "partial_candidate"
)

func NormalizeGateStatus(s GateStatus) GateStatus {
	switch s {
	case GatePassedWithWarnings:
		return GatePassedWithWarning
	case GateBlocked:
		return GateFailed
	case GatePartialCandidate:
		return GateFailed
	}
	return s
}

type Applicability string

const (
	ApplicabilityApplicable    Applicability = "applicable"
	ApplicabilityNotApplicable Applicability = "not_applicable"
	ApplicabilityMissing       Applicability = "missing_measurement"
)

const (
	DimensionIntegrity            = "integrity"
	DimensionSubjectIntegrity     = "subject_integrity"
	DimensionBackgroundCleanliness = "background_cleanliness"
	DimensionAnchorStability      = "anchor_stability"
	DimensionIdentityConsistency  = "identity_consistency"
	DimensionMotionContinuity     = "motion_continuity"
	DimensionLoopContinuity       = "loop_continuity"
	DimensionVisualConsistency    = "visual_consistency"
	DimensionEvaluationConfidence = "evaluation_confidence"
)

const (
	RuleFileMissing               = "FILE_MISSING"
	RuleFileUndecodable           = "FILE_UNDECODABLE"
	RuleFileHashMismatch          = "FILE_HASH_MISMATCH"
	RuleFrameCountMismatch        = "FRAME_COUNT_MISMATCH"
	RuleFrameIndexGap             = "FRAME_INDEX_GAP"
	RuleFrameDimensionMismatch    = "FRAME_DIMENSION_MISMATCH"
	RuleAlphaAllTransparent       = "ALPHA_ALL_TRANSPARENT"
	RuleAlphaPolicyViolation      = "ALPHA_POLICY_VIOLATION"
	RuleSubjectEmpty              = "SUBJECT_EMPTY"
	RuleSubjectTooSmall           = "SUBJECT_TOO_SMALL"
	RuleSubjectTooLarge           = "SUBJECT_TOO_LARGE"
	RuleSubjectFragmented         = "SUBJECT_FRAGMENTED"
	RuleSubjectClipped            = "SUBJECT_CLIPPED"
	RuleUnexpectedEdgeContact     = "UNEXPECTED_EDGE_CONTACT"
	RuleBackgroundResidueComponent = "BACKGROUND_RESIDUE_COMPONENT"
	RuleAlphaHalo                 = "ALPHA_HALO"
	RuleAnchorJitter              = "ANCHOR_JITTER"
	RuleScaleJitter               = "SCALE_JITTER"
	RuleIdentityDrift             = "IDENTITY_DRIFT"
	RuleMotionJump                = "MOTION_JUMP"
	RuleMotionDirectionReversal   = "MOTION_DIRECTION_REVERSAL"
	RuleExactDuplicateFrame       = "EXACT_DUPLICATE_FRAME"
	RulePerceptualDuplicateFrame  = "PERCEPTUAL_DUPLICATE_FRAME"
	RuleFrozenSequence            = "FROZEN_SEQUENCE"
	RuleLoopDiscontinuity         = "LOOP_DISCONTINUITY"
	RuleLoopVelocityDiscontinuity = "LOOP_VELOCITY_DISCONTINUITY"
	RuleColorFlicker              = "COLOR_FLICKER"
	RuleLowEvaluationConfidence   = "LOW_EVALUATION_CONFIDENCE"
	RuleMissingMeasurement        = "MISSING_MEASUREMENT"
	RuleDetectorFailure           = "DETECTOR_FAILURE"
	RuleLegacyFlagImported        = "LEGACY_FLAG_IMPORTED"
	RuleMeasurementMissing        = "MEASUREMENT_MISSING_REQUIRED"
	RuleOptionalMeasurementMissing = "OPTIONAL_MEASUREMENT_MISSING"
	RuleCoordinateSpaceMismatch   = "COORDINATE_SPACE_MISMATCH"
)

type FramePairRef struct {
	From          int  `json:"from"`
	To            int  `json:"to"`
	CrossLoopHead bool `json:"crossLoopHead"`
}

type RegionRef struct {
	FrameIndex      int     `json:"frameIndex"`
	X               float64 `json:"x"`
	Y               float64 `json:"y"`
	Width           float64 `json:"width"`
	Height          float64 `json:"height"`
	CoordinateSpace string  `json:"coordinateSpace"`
}

type QualityFinding struct {
	ID              string         `json:"id"`
	RuleCode        string         `json:"ruleCode"`
	RuleVersion     int            `json:"ruleVersion"`
	Dimension       string         `json:"dimension"`
	Severity        Severity       `json:"severity"`
	MessageKey      string         `json:"messageKey"`
	Message         string         `json:"message"`
	FrameIndexes    []int          `json:"frameIndexes"`
	FramePairs      []FramePairRef `json:"framePairs"`
	Regions         []RegionRef    `json:"regions"`
	MetricName      string         `json:"metricName"`
	ObservedValue   *float64       `json:"observedValue,omitempty"`
	ThresholdValue  *float64       `json:"thresholdValue,omitempty"`
	Comparison      string         `json:"comparison,omitempty"`
	Confidence      float64        `json:"confidence"`
	HardGate        bool           `json:"hardGate"`
	SuggestedAction string         `json:"suggestedAction"`
	EvidenceRef     string         `json:"evidenceRef"`
}

type DimensionScore struct {
	Dimension     string        `json:"dimension"`
	Applicability Applicability `json:"applicability"`
	Score         *float64      `json:"score,omitempty"`
	Confidence    float64       `json:"confidence"`
	Weight        float64       `json:"weight"`
	FindingIDs    []string      `json:"findingIds"`
}

type FindingSummary struct {
	Total     int `json:"total"`
	Critical  int `json:"critical"`
	Error     int `json:"error"`
	Review    int `json:"review"`
	Warning   int `json:"warning"`
	Info      int `json:"info"`
	HardGates int `json:"hardGates"`
}

type ActionQualityResult struct {
	EvaluationID        string                    `json:"evaluationId"`
	ActionRevisionID    string                    `json:"actionRevisionId"`
	ActionKey           string                    `json:"actionKey"`
	ExecutionStatus     EvaluationExecutionStatus `json:"executionStatus"`
	Verdict             *ContentVerdict           `json:"verdict,omitempty"`
	OverallScore        *float64                  `json:"overallScore,omitempty"`
	OverallConfidence   float64                   `json:"overallConfidence"`
	DimensionScores     []DimensionScore          `json:"dimensionScores"`
	FindingSummary      FindingSummary            `json:"findingSummary"`
	ProfileHash         string                    `json:"profileHash"`
	EngineVersion       string                    `json:"engineVersion"`
	ReportPath          string                    `json:"reportPath"`
	ReportHash          string                    `json:"reportHash"`
}

type QualityEvaluation struct {
	ID                     string                    `json:"id"`
	UserID                 string                    `json:"userId"`
	CharacterID            string                    `json:"characterId"`
	ProcessingTaskID       string                    `json:"processingTaskId"`
	ProcessingActionID     string                    `json:"processingActionId"`
	ActionRevisionID       string                    `json:"actionRevisionId"`
	ActionContentHash      string                    `json:"actionContentHash"`
	ProcessingRevisionID   string                    `json:"processingRevisionId"`
	MeasurementSetID       string                    `json:"measurementSetId"`
	ActionKey              string                    `json:"actionKey"`
	ExecutionStatus        EvaluationExecutionStatus `json:"executionStatus"`
	Verdict                string                    `json:"verdict"`
	OverallScore           *float64                  `json:"overallScore,omitempty"`
	OverallConfidence      float64                   `json:"overallConfidence"`
	ProfileSnapshotJSON    string                    `json:"profileSnapshotJson"`
	ProfileHash            string                    `json:"profileHash"`
	ProfileID              string                    `json:"profileId"`
	ProfileVersion         string                    `json:"profileVersion"`
	RuleSetVersion         string                    `json:"ruleSetVersion"`
	RulesetContentHash     string                    `json:"rulesetContentHash"`
	MeasurementVersion     string                    `json:"measurementVersion"`
	EngineVersion          string                    `json:"engineVersion"`
	QualityMode            string                    `json:"qualityMode"`
	ReportPath             string                    `json:"reportPath"`
	ReportHash             string                    `json:"reportHash"`
	SupersedesEvaluationID string                    `json:"supersedesEvaluationId"`
	ExecutionID            string                    `json:"executionId"`
	WorkerID               string                    `json:"workerId"`
	LeaseOwner             string                    `json:"leaseOwner"`
	LeaseExpiresAt         string                    `json:"leaseExpiresAt"`
	HeartbeatAt            string                    `json:"heartbeatAt"`
	AttemptCount           int                       `json:"attemptCount"`
	IdempotencyKey         string                    `json:"idempotencyKey"`
	ErrorCode              string                    `json:"errorCode"`
	ErrorMessage           string                    `json:"errorMessage"`
	IsActive               bool                      `json:"isActive"`
	StartedAt              string                    `json:"startedAt"`
	CompletedAt            string                    `json:"completedAt"`
	CreatedAt              string                    `json:"createdAt"`
	UpdatedAt              string                    `json:"updatedAt"`
}

func (QualityEvaluation) TableName() string { return "desktop_pet_quality_evaluations" }

type QualityFindingRecord struct {
	ID               string   `json:"id"`
	EvaluationID     string   `json:"evaluationId"`
	RuleCode         string   `json:"ruleCode"`
	RuleVersion      int      `json:"ruleVersion"`
	DimensionKey     string   `json:"dimensionKey"`
	Severity         string   `json:"severity"`
	HardGate         bool     `json:"hardGate"`
	FrameIndexesJSON string   `json:"frameIndexesJson"`
	FramePairsJSON   string   `json:"framePairsJson"`
	RegionsJSON      string   `json:"regionsJson"`
	MetricName       string   `json:"metricName"`
	ObservedValue    *float64 `json:"observedValue,omitempty"`
	ThresholdValue   *float64 `json:"thresholdValue,omitempty"`
	Comparison       string   `json:"comparison"`
	Confidence       float64  `json:"confidence"`
	MessageKey       string   `json:"messageKey"`
	Message          string   `json:"message"`
	SuggestedAction  string   `json:"suggestedAction"`
	EvidenceRef      string   `json:"evidenceRef"`
	SortKey          string   `json:"sortKey"`
	CreatedAt        string   `json:"createdAt"`
}

func (QualityFindingRecord) TableName() string { return "desktop_pet_quality_findings" }

type QualityDimensionScoreRecord struct {
	ID            string   `json:"id"`
	EvaluationID  string   `json:"evaluationId"`
	DimensionKey  string   `json:"dimensionKey"`
	Applicability string   `json:"applicability"`
	Score         *float64 `json:"score,omitempty"`
	Confidence    float64  `json:"confidence"`
	Weight        float64  `json:"weight"`
	DetailsJSON   string   `json:"detailsJson"`
	CreatedAt     string   `json:"createdAt"`
}

func (QualityDimensionScoreRecord) TableName() string { return "desktop_pet_quality_dimension_scores" }

type QualityGateResultRecord struct {
	ID                    string `json:"id"`
	ProcessingTaskID      string `json:"processingTaskId"`
	GateStatus            string `json:"gateStatus"`
	RequiredActionCount   int    `json:"requiredActionCount"`
	AcceptedActionCount   int    `json:"acceptedActionCount"`
	WarningActionCount    int    `json:"warningActionCount"`
	ReviewActionCount     int    `json:"reviewActionCount"`
	RejectedActionCount   int    `json:"rejectedActionCount"`
	FailedEvaluationCount int    `json:"failedEvaluationCount"`
	SnapshotJSON          string `json:"snapshotJson"`
	SnapshotHash          string `json:"snapshotHash"`
	ActiveRevisionSetHash string `json:"activeRevisionSetHash"`
	EvaluationSetHash     string `json:"evaluationSetHash"`
	RuleSetVersion        string `json:"ruleSetVersion"`
	ProfileID             string `json:"profileId"`
	InvalidatedAt         string `json:"invalidatedAt"`
	CreatedAt             string `json:"createdAt"`
	UpdatedAt             string `json:"updatedAt"`
}

func (QualityGateResultRecord) TableName() string { return "desktop_pet_quality_gate_results" }

type ActiveQualityEvaluationBindingRecord struct {
	ID                string `json:"id"`
	ActionRevisionID  string `json:"actionRevisionId"`
	ProfileID         string `json:"profileId"`
	ActiveEvaluationID string `json:"activeEvaluationId"`
	BindingRevision   int64  `json:"bindingRevision"`
	BoundAt           string `json:"boundAt"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

func (ActiveQualityEvaluationBindingRecord) TableName() string {
	return "desktop_pet_active_quality_evaluation_bindings"
}

type QualityCommitJournalRecord struct {
	ID             string `json:"id"`
	EvaluationID   string `json:"evaluationId"`
	CommitHash     string `json:"commitHash"`
	Status         string `json:"status"`
	StepsCompleted string `json:"stepsCompleted"`
	ErrorMessage   string `json:"errorMessage"`
	CreatedAt      string `json:"createdAt"`
	CompletedAt    string `json:"completedAt"`
}

func (QualityCommitJournalRecord) TableName() string { return "desktop_pet_quality_commit_journals" }

type QualityReviewDecisionRecord struct {
	ID               string `json:"id"`
	EvaluationID     string `json:"evaluationId"`
	ActionRevisionID string `json:"actionRevisionId"`
	Decision         string `json:"decision"`
	Reason           string `json:"reason"`
	Reviewer         string `json:"reviewer"`
	ReviewedAt       string `json:"reviewedAt"`
	CreatedAt        string `json:"createdAt"`
}

func (QualityReviewDecisionRecord) TableName() string { return "desktop_pet_quality_review_decisions" }

type QualityMeasurementCacheRecord struct {
	ID                  string  `json:"id"`
	FrameArtifactID     string  `json:"frameArtifactId"`
	ContentHash         string  `json:"contentHash"`
	MeasurementVersion  string  `json:"measurementVersion"`
	Width               int     `json:"width"`
	Height              int     `json:"height"`
	HasAlphaChannel     bool    `json:"hasAlphaChannel"`
	AlphaCoverage       float64 `json:"alphaCoverage"`
	FullyTransparentRatio float64 `json:"fullyTransparentRatio"`
	SemiTransparentRatio  float64 `json:"semiTransparentRatio"`
	OpaqueRatio           float64 `json:"opaqueRatio"`
	Decodable           bool    `json:"decodable"`
	MimeType            string  `json:"mimeType"`
	PixelHash           string  `json:"pixelHash"`
	MeasurementsJSON    string  `json:"measurementsJson"`
	CreatedAt           string  `json:"createdAt"`
}

func (QualityMeasurementCacheRecord) TableName() string { return "desktop_pet_quality_measurement_cache" }

type QualityOutboxEventRecord struct {
	ID            string `json:"id"`
	EventType     string `json:"eventType"`
	PayloadJSON   string `json:"payloadJson"`
	Status        string `json:"status"`
	CreatedAt     string `json:"createdAt"`
	PublishedAt   string `json:"publishedAt"`
}

func (QualityOutboxEventRecord) TableName() string { return "desktop_pet_quality_outbox_events" }

type Observation struct {
	DetectorKey     string             `json:"detectorKey"`
	DetectorVersion string             `json:"detectorVersion"`
	FrameIndex      int                `json:"frameIndex"`
	FramePairFrom   int                `json:"framePairFrom,omitempty"`
	FramePairTo     int                `json:"framePairTo,omitempty"`
	MetricName      string             `json:"metricName"`
	Value           float64            `json:"value"`
	Confidence      float64            `json:"confidence"`
	Details         map[string]float64 `json:"details,omitempty"`
}

type CoordinateSpace struct {
	ID           string `json:"id"`
	CanvasWidth  int    `json:"canvasWidth"`
	CanvasHeight int    `json:"canvasHeight"`
}

type ActionMeasurementSet struct {
	ActionRevisionID   string            `json:"actionRevisionId"`
	ActionKey          string            `json:"actionKey"`
	CanvasWidth        int               `json:"canvasWidth"`
	CanvasHeight       int               `json:"canvasHeight"`
	FrameCount         int               `json:"frameCount"`
	FrameMeasurements  []FrameMeasurement `json:"frameMeasurements"`
	LoopType           string            `json:"loopType"`
	PlaybackMode       string            `json:"playbackMode"`
	AnchorProfile      string            `json:"anchorProfile"`
	ActionSpecHash     string            `json:"actionSpecHash"`
	RevisionHash       string            `json:"revisionHash"`
}

type FrameMeasurement struct {
	FrameIndex               int     `json:"frameIndex"`
	FilePath                 string  `json:"filePath"`
	PixelHash                string  `json:"pixelHash"`
	FileHash                 string  `json:"fileHash"`
	Width                    int     `json:"width"`
	Height                   int     `json:"height"`
	ForegroundCoverage       float64 `json:"foregroundCoverage"`
	SemiTransparentCoverage  float64 `json:"semiTransparentCoverage"`
	OpaqueCoverage           float64 `json:"opaqueCoverage"`
	TransparentCoverage      float64 `json:"transparentCoverage"`
	BorderForegroundCoverage float64 `json:"borderForegroundCoverage"`
	MaskArea                 float64 `json:"maskArea"`
	SubjectBoxX              float64 `json:"subjectBoxX"`
	SubjectBoxY              float64 `json:"subjectBoxY"`
	SubjectBoxWidth          float64 `json:"subjectBoxWidth"`
	SubjectBoxHeight         float64 `json:"subjectBoxHeight"`
	CentroidX                float64 `json:"centroidX"`
	CentroidY                float64 `json:"centroidY"`
	AnchorX                  float64 `json:"anchorX"`
	AnchorY                  float64 `json:"anchorY"`
	HasAlpha                 bool    `json:"hasAlpha"`
	HasAlphaChannel          bool    `json:"hasAlphaChannel"`
	AlphaCoverage            float64 `json:"alphaCoverage"`
	FullyTransparentRatio    float64 `json:"fullyTransparentRatio"`
	SemiTransparentRatio     float64 `json:"semiTransparentRatio"`
	OpaqueRatio              float64 `json:"opaqueRatio"`
	Decodable                bool    `json:"decodable"`
	FileExists               bool    `json:"fileExists"`
	FileSize                 int64   `json:"fileSize"`
	MimeType                 string  `json:"mimeType"`
	CoordinateSpace          string  `json:"coordinateSpace"`
}

type DecodedFrame struct {
	Index  int
	Image  interface{}
	Width  int
	Height int
}

type ProblemFrameSummary struct {
	FrameIndex   int             `json:"frameIndex"`
	Findings     []QualityFinding `json:"findings"`
	ThumbnailURL string          `json:"thumbnailUrl"`
	Severity     Severity        `json:"severity"`
}

type EvaluateRequest struct {
	ActionRevisionID     string                 `json:"actionRevisionId"`
	ProcessingTaskID     string                 `json:"processingTaskId"`
	ProcessingActionID   string                 `json:"processingActionId"`
	ActionKey            string                 `json:"actionKey"`
	Profile              QualityProfileSnapshot `json:"profile"`
	ExpectedRevisionHash string                 `json:"expectedRevisionHash"`
	ExecutionID          string                 `json:"executionId"`
	WorkerID             string                 `json:"workerId"`
	ActionContentHash    string                 `json:"actionContentHash"`
}

type CreateEvaluationRequest struct {
	UserID               string `json:"userId"`
	CharacterID          string `json:"characterId"`
	ProcessingTaskID     string `json:"processingTaskId"`
	ProcessingActionID   string `json:"processingActionId"`
	ActionRevisionID     string `json:"actionRevisionId"`
	ActionContentHash    string `json:"actionContentHash"`
	ProcessingRevisionID string `json:"processingRevisionId"`
	ActionKey            string `json:"actionKey"`
	QualityMode          string `json:"qualityMode"`
	ProfileID            string `json:"profileId"`
	RuleSetVersion       string `json:"ruleSetVersion"`
	IdempotencyKey       string `json:"idempotencyKey"`
}

type ReevaluateRequest struct {
	EvaluationID string `json:"evaluationId"`
	QualityMode  string `json:"qualityMode"`
}

type QualityGateResult struct {
	ProcessingTaskID       string                 `json:"processingTaskId"`
	GateStatus             GateStatus             `json:"gateStatus"`
	RequiredActionCount    int                    `json:"requiredActionCount"`
	AcceptedActionCount    int                    `json:"acceptedActionCount"`
	WarningActionCount     int                    `json:"warningActionCount"`
	ReviewActionCount      int                    `json:"reviewActionCount"`
	RejectedActionCount    int                    `json:"rejectedActionCount"`
	FailedEvaluationCount  int                    `json:"failedEvaluationCount"`
	ActionVerdicts         []ActionVerdictSummary `json:"actionVerdicts"`
	ActiveRevisionSetHash  string                 `json:"activeRevisionSetHash"`
	EvaluationSetHash      string                 `json:"evaluationSetHash"`
	RuleSetVersion         string                 `json:"ruleSetVersion"`
	ProfileID              string                 `json:"profileId"`
}

type ActionVerdictSummary struct {
	ActionKey       string                    `json:"actionKey"`
	ActionName      string                    `json:"actionName"`
	Required        bool                      `json:"required"`
	Verdict         ContentVerdict            `json:"verdict"`
	ExecutionStatus EvaluationExecutionStatus `json:"executionStatus"`
	OverallScore    *float64                  `json:"overallScore,omitempty"`
	FindingCount    int                       `json:"findingCount"`
	HardGateCount   int                       `json:"hardGateCount"`
	ActionRevisionID string                   `json:"actionRevisionId"`
}

type DetectorInput struct {
	Measurements *ActionMeasurementSet
	Profile      QualityProfileSnapshot
}

type EvaluateResult struct {
	Result       *ActionQualityResult
	Observations []Observation
	Findings     []QualityFinding
	Scores       []DimensionScore
	Verdict      ContentVerdict
	DurationMS   int64
}

type CommitEvaluationRequest struct {
	Evaluation           *QualityEvaluation
	Findings             []QualityFinding
	Scores               []DimensionScore
	Verdict              ContentVerdict
	OverallScore         float64
	OverallConfidence    float64
	ReportPath           string
	ReportHash           string
	ProfileSnapshotJSON  string
	ProfileHash          string
	ProcessingTaskID     string
	ActionKey            string
}

type CommitEvaluationResult struct {
	EvaluationID       string
	GateResult         *QualityGateResult
	WritebackApplied   bool
	ActiveBindingSet   bool
}

type EvaluateTaskGateRequest struct {
	UserID                string
	CharacterID           string
	ProcessingTaskID      string
	ActiveRevisionSetHash string
	RequiredActionKeys    []string
	OptionalActionKeys    []string
	ProfileID             string
	RuleSetVersion        string
}

type ReviewDecisionRequest struct {
	EvaluationID     string
	ActionRevisionID string
	Decision         string
	Reason           string
	Reviewer         string
}

type GetValidGateForReleaseRequest struct {
	UserID                string
	ProcessingTaskID      string
	ActiveRevisionSetHash string
}

type QualityOutboxEvent struct {
	EventType        string `json:"eventType"`
	UserID           string `json:"userId"`
	CharacterID      string `json:"characterId"`
	ProcessingTaskID string `json:"processingTaskId"`
	ActionKey        string `json:"actionKey"`
	ActionRevisionID string `json:"actionRevisionId"`
	EvaluationID     string `json:"evaluationId"`
	GateID           string `json:"gateId"`
	Status           string `json:"status"`
	Verdict          string `json:"verdict"`
	OccurredAt       string `json:"occurredAt"`
}

const (
	OutboxEventEvaluationCreated   = "desktop_pet.quality_evaluation.created"
	OutboxEventEvaluationStarted   = "desktop_pet.quality_evaluation.started"
	OutboxEventEvaluationCompleted = "desktop_pet.quality_evaluation.completed"
	OutboxEventEvaluationFailed    = "desktop_pet.quality_evaluation.failed"
	OutboxEventGateUpdated         = "desktop_pet.quality_gate.updated"
	OutboxEventGateStale           = "desktop_pet.quality_gate.stale"
)

const (
	ReviewDecisionApproveWithReason  = "approve_with_reason"
	ReviewDecisionReject             = "reject"
	ReviewDecisionRequestRegeneration = "request_regeneration"
)

const (
	InputSourceNewBridge     = "quality_input_bridge"
	InputSourceLegacyProjection = "legacy_projection"
)
