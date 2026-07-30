// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

type EvaluationExecutionStatus string

const (
	EvalPending  EvaluationExecutionStatus = "pending"
	EvalRunning  EvaluationExecutionStatus = "running"
	EvalSucceeded EvaluationExecutionStatus = "succeeded"
	EvalFailed   EvaluationExecutionStatus = "evaluation_failed"
	EvalCancelled EvaluationExecutionStatus = "cancelled"
)

type ContentVerdict string

const (
	VerdictAccepted           ContentVerdict = "accepted"
	VerdictAcceptedWithWarning ContentVerdict = "accepted_with_warning"
	VerdictNeedsReview        ContentVerdict = "needs_review"
	VerdictRejected           ContentVerdict = "rejected"
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
	GatePassed            GateStatus = "passed"
	GatePassedWithWarnings GateStatus = "passed_with_warnings"
	GateReviewRequired    GateStatus = "review_required"
	GateBlocked           GateStatus = "blocked"
	GatePartialCandidate  GateStatus = "partial_candidate"
)

type Applicability string

const (
	ApplicabilityApplicable    Applicability = "applicable"
	ApplicabilityNotApplicable Applicability = "not_applicable"
	ApplicabilityMissing       Applicability = "missing_measurement"
)

const (
	DimensionIntegrity           = "integrity"
	DimensionSubjectIntegrity    = "subject_integrity"
	DimensionBackgroundCleanliness = "background_cleanliness"
	DimensionAnchorStability     = "anchor_stability"
	DimensionIdentityConsistency = "identity_consistency"
	DimensionMotionContinuity    = "motion_continuity"
	DimensionLoopContinuity      = "loop_continuity"
	DimensionVisualConsistency   = "visual_consistency"
	DimensionEvaluationConfidence = "evaluation_confidence"
)

const (
	RuleFileMissing              = "FILE_MISSING"
	RuleFileUndecodable          = "FILE_UNDECODABLE"
	RuleFileHashMismatch         = "FILE_HASH_MISMATCH"
	RuleFrameCountMismatch       = "FRAME_COUNT_MISMATCH"
	RuleFrameIndexGap            = "FRAME_INDEX_GAP"
	RuleFrameDimensionMismatch   = "FRAME_DIMENSION_MISMATCH"
	RuleAlphaAllTransparent      = "ALPHA_ALL_TRANSPARENT"
	RuleAlphaPolicyViolation     = "ALPHA_POLICY_VIOLATION"
	RuleSubjectEmpty             = "SUBJECT_EMPTY"
	RuleSubjectTooSmall          = "SUBJECT_TOO_SMALL"
	RuleSubjectTooLarge          = "SUBJECT_TOO_LARGE"
	RuleSubjectFragmented        = "SUBJECT_FRAGMENTED"
	RuleSubjectClipped           = "SUBJECT_CLIPPED"
	RuleUnexpectedEdgeContact    = "UNEXPECTED_EDGE_CONTACT"
	RuleBackgroundResidueComponent = "BACKGROUND_RESIDUE_COMPONENT"
	RuleAlphaHalo                = "ALPHA_HALO"
	RuleAnchorJitter             = "ANCHOR_JITTER"
	RuleScaleJitter              = "SCALE_JITTER"
	RuleIdentityDrift            = "IDENTITY_DRIFT"
	RuleMotionJump               = "MOTION_JUMP"
	RuleMotionDirectionReversal  = "MOTION_DIRECTION_REVERSAL"
	RuleExactDuplicateFrame      = "EXACT_DUPLICATE_FRAME"
	RulePerceptualDuplicateFrame = "PERCEPTUAL_DUPLICATE_FRAME"
	RuleFrozenSequence           = "FROZEN_SEQUENCE"
	RuleLoopDiscontinuity        = "LOOP_DISCONTINUITY"
	RuleLoopVelocityDiscontinuity = "LOOP_VELOCITY_DISCONTINUITY"
	RuleColorFlicker             = "COLOR_FLICKER"
	RuleLowEvaluationConfidence  = "LOW_EVALUATION_CONFIDENCE"
	RuleMissingMeasurement       = "MISSING_MEASUREMENT"
	RuleDetectorFailure          = "DETECTOR_FAILURE"
	RuleLegacyFlagImported       = "LEGACY_FLAG_IMPORTED"
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
	ID             string        `json:"id"`
	RuleCode       string        `json:"ruleCode"`
	RuleVersion    int           `json:"ruleVersion"`
	Dimension      string        `json:"dimension"`
	Severity       Severity      `json:"severity"`
	MessageKey     string        `json:"messageKey"`
	Message        string        `json:"message"`
	FrameIndexes   []int         `json:"frameIndexes"`
	FramePairs     []FramePairRef `json:"framePairs"`
	Regions        []RegionRef   `json:"regions"`
	MetricName     string        `json:"metricName"`
	ObservedValue  *float64      `json:"observedValue,omitempty"`
	ThresholdValue *float64      `json:"thresholdValue,omitempty"`
	Comparison     string        `json:"comparison,omitempty"`
	Confidence     float64       `json:"confidence"`
	HardGate       bool          `json:"hardGate"`
	SuggestedAction string       `json:"suggestedAction"`
	EvidenceRef    string        `json:"evidenceRef"`
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
	Total      int `json:"total"`
	Critical   int `json:"critical"`
	Error      int `json:"error"`
	Review     int `json:"review"`
	Warning    int `json:"warning"`
	Info       int `json:"info"`
	HardGates  int `json:"hardGates"`
}

type ActionQualityResult struct {
	EvaluationID       string                 `json:"evaluationId"`
	ActionRevisionID   string                 `json:"actionRevisionId"`
	ActionKey          string                 `json:"actionKey"`
	ExecutionStatus    EvaluationExecutionStatus `json:"executionStatus"`
	Verdict            *ContentVerdict        `json:"verdict,omitempty"`
	OverallScore       *float64               `json:"overallScore,omitempty"`
	OverallConfidence  float64                `json:"overallConfidence"`
	DimensionScores    []DimensionScore       `json:"dimensionScores"`
	FindingSummary     FindingSummary         `json:"findingSummary"`
	ProfileHash        string                 `json:"profileHash"`
	EngineVersion      string                 `json:"engineVersion"`
	ReportPath         string                 `json:"reportPath"`
	ReportHash         string                 `json:"reportHash"`
}

type QualityEvaluation struct {
	ID                  string                 `json:"id"`
	ProcessingTaskID    string                 `json:"processingTaskId"`
	ProcessingActionID  string                 `json:"processingActionId"`
	ActionRevisionID    string                 `json:"actionRevisionId"`
	MeasurementSetID    string                 `json:"measurementSetId"`
	ActionKey           string                 `json:"actionKey"`
	ExecutionStatus     EvaluationExecutionStatus `json:"executionStatus"`
	Verdict             string                 `json:"verdict"`
	OverallScore        *float64               `json:"overallScore,omitempty"`
	OverallConfidence   float64                `json:"overallConfidence"`
	ProfileSnapshotJSON string                 `json:"profileSnapshotJson"`
	ProfileHash         string                 `json:"profileHash"`
	EngineVersion       string                 `json:"engineVersion"`
	QualityMode         string                 `json:"qualityMode"`
	ReportPath          string                 `json:"reportPath"`
	ReportHash          string                 `json:"reportHash"`
	SupersedesEvaluationID string              `json:"supersedesEvaluationId"`
	ExecutionID         string                 `json:"executionId"`
	WorkerID            string                 `json:"workerId"`
	LeaseExpiresAt      string                 `json:"leaseExpiresAt"`
	ErrorCode           string                 `json:"errorCode"`
	ErrorMessage         string                 `json:"errorMessage"`
	IsActive             bool                   `json:"isActive"`
	StartedAt           string                 `json:"startedAt"`
	CompletedAt         string                 `json:"completedAt"`
	CreatedAt           string                 `json:"createdAt"`
	UpdatedAt           string                 `json:"updatedAt"`
}

func (QualityEvaluation) TableName() string { return "desktop_pet_quality_evaluations" }

type QualityFindingRecord struct {
	ID             string   `json:"id"`
	EvaluationID   string   `json:"evaluationId"`
	RuleCode       string   `json:"ruleCode"`
	RuleVersion    int      `json:"ruleVersion"`
	DimensionKey   string   `json:"dimensionKey"`
	Severity       string   `json:"severity"`
	HardGate       bool     `json:"hardGate"`
	FrameIndexesJSON string `json:"frameIndexesJson"`
	FramePairsJSON   string `json:"framePairsJson"`
	RegionsJSON      string `json:"regionsJson"`
	MetricName       string `json:"metricName"`
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
	ID            string        `json:"id"`
	EvaluationID  string        `json:"evaluationId"`
	DimensionKey  string        `json:"dimensionKey"`
	Applicability string        `json:"applicability"`
	Score         *float64      `json:"score,omitempty"`
	Confidence    float64       `json:"confidence"`
	Weight        float64       `json:"weight"`
	DetailsJSON   string        `json:"detailsJson"`
	CreatedAt     string        `json:"createdAt"`
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
	CreatedAt             string `json:"createdAt"`
	UpdatedAt             string `json:"updatedAt"`
}

func (QualityGateResultRecord) TableName() string { return "desktop_pet_quality_gate_results" }

type Observation struct {
	DetectorKey  string  `json:"detectorKey"`
	DetectorVersion string `json:"detectorVersion"`
	FrameIndex   int     `json:"frameIndex"`
	FramePairFrom int    `json:"framePairFrom,omitempty"`
	FramePairTo   int    `json:"framePairTo,omitempty"`
	MetricName   string  `json:"metricName"`
	Value        float64 `json:"value"`
	Confidence   float64 `json:"confidence"`
	Details      map[string]float64 `json:"details,omitempty"`
}

type CoordinateSpace struct {
	ID         string `json:"id"`
	CanvasWidth  int    `json:"canvasWidth"`
	CanvasHeight int    `json:"canvasHeight"`
}

type ActionMeasurementSet struct {
	ActionRevisionID string                 `json:"actionRevisionId"`
	ActionKey        string                 `json:"actionKey"`
	CanvasWidth      int                    `json:"canvasWidth"`
	CanvasHeight     int                    `json:"canvasHeight"`
	FrameCount       int                    `json:"frameCount"`
	FrameMeasurements []FrameMeasurement    `json:"frameMeasurements"`
	LoopType         string                 `json:"loopType"`
	PlaybackMode     string                 `json:"playbackMode"`
	AnchorProfile    string                 `json:"anchorProfile"`
	ActionSpecHash   string                 `json:"actionSpecHash"`
	RevisionHash     string                 `json:"revisionHash"`
}

type FrameMeasurement struct {
	FrameIndex       int     `json:"frameIndex"`
	FilePath         string  `json:"filePath"`
	PixelHash        string  `json:"pixelHash"`
	FileHash         string  `json:"fileHash"`
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	ForegroundCoverage float64 `json:"foregroundCoverage"`
	SemiTransparentCoverage float64 `json:"semiTransparentCoverage"`
	OpaqueCoverage   float64 `json:"opaqueCoverage"`
	TransparentCoverage float64 `json:"transparentCoverage"`
	BorderForegroundCoverage float64 `json:"borderForegroundCoverage"`
	MaskArea         float64 `json:"maskArea"`
	SubjectBoxX      float64 `json:"subjectBoxX"`
	SubjectBoxY      float64 `json:"subjectBoxY"`
	SubjectBoxWidth  float64 `json:"subjectBoxWidth"`
	SubjectBoxHeight float64 `json:"subjectBoxHeight"`
	CentroidX        float64 `json:"centroidX"`
	CentroidY        float64 `json:"centroidY"`
	AnchorX          float64 `json:"anchorX"`
	AnchorY          float64 `json:"anchorY"`
	HasAlpha         bool    `json:"hasAlpha"`
	Decodable        bool    `json:"decodable"`
	FileExists       bool    `json:"fileExists"`
	FileSize         int64   `json:"fileSize"`
}

type DecodedFrame struct {
	Index  int
	Image  interface{}
	Width  int
	Height int
}

type ProblemFrameSummary struct {
	FrameIndex     int      `json:"frameIndex"`
	Findings       []QualityFinding `json:"findings"`
	ThumbnailURL   string   `json:"thumbnailUrl"`
	Severity       Severity `json:"severity"`
}

type EvaluateRequest struct {
	ActionRevisionID    string                  `json:"actionRevisionId"`
	ProcessingTaskID    string                  `json:"processingTaskId"`
	ProcessingActionID  string                  `json:"processingActionId"`
	ActionKey           string                  `json:"actionKey"`
	Profile             QualityProfileSnapshot  `json:"profile"`
	ExpectedRevisionHash string                 `json:"expectedRevisionHash"`
	ExecutionID         string                  `json:"executionId"`
	WorkerID            string                  `json:"workerId"`
}

type CreateEvaluationRequest struct {
	ProcessingTaskID   string `json:"processingTaskId"`
	ProcessingActionID string `json:"processingActionId"`
	ActionRevisionID   string `json:"actionRevisionId"`
	ActionKey          string `json:"actionKey"`
	QualityMode        string `json:"qualityMode"`
}

type ReevaluateRequest struct {
	EvaluationID  string `json:"evaluationId"`
	QualityMode   string `json:"qualityMode"`
}

type QualityGateResult struct {
	ProcessingTaskID      string                 `json:"processingTaskId"`
	GateStatus            GateStatus             `json:"gateStatus"`
	RequiredActionCount   int                    `json:"requiredActionCount"`
	AcceptedActionCount   int                    `json:"acceptedActionCount"`
	WarningActionCount    int                    `json:"warningActionCount"`
	ReviewActionCount     int                    `json:"reviewActionCount"`
	RejectedActionCount   int                    `json:"rejectedActionCount"`
	FailedEvaluationCount int                    `json:"failedEvaluationCount"`
	ActionVerdicts        []ActionVerdictSummary `json:"actionVerdicts"`
}

type ActionVerdictSummary struct {
	ActionKey       string          `json:"actionKey"`
	ActionName      string          `json:"actionName"`
	Required        bool            `json:"required"`
	Verdict         ContentVerdict  `json:"verdict"`
	ExecutionStatus EvaluationExecutionStatus `json:"executionStatus"`
	OverallScore    *float64        `json:"overallScore,omitempty"`
	FindingCount    int             `json:"findingCount"`
	HardGateCount   int             `json:"hardGateCount"`
}

type DetectorInput struct {
	Measurements *ActionMeasurementSet
	Profile      QualityProfileSnapshot
}

type EvaluateResult struct {
	Result      *ActionQualityResult
	Observations []Observation
	Findings    []QualityFinding
	Scores      []DimensionScore
	Verdict     ContentVerdict
	DurationMS  int64
}
