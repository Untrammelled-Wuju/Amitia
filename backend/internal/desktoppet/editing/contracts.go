package editing

type RevisionManifest struct {
	SchemaVersion    int              `json:"schemaVersion"`
	RevisionID       string           `json:"revisionId"`
	ParentRevisionID string           `json:"parentRevisionId"`
	ProcessingTaskID string           `json:"processingTaskId"`
	ActionKey        string           `json:"actionKey"`
	Playback         ManifestPlayback `json:"playback"`
	Frames           []ManifestFrame  `json:"frames"`
	Quality          ManifestQuality  `json:"quality"`
	CreatedAt        string           `json:"createdAt"`
}

type ManifestPlayback struct {
	LoopType      string `json:"loopType"`
	DefaultFPS    int    `json:"defaultFps"`
	ReturnAction  string `json:"returnAction"`
	Interruptible bool   `json:"interruptible"`
}

type ManifestFrame struct {
	FrameID      string          `json:"frameId"`
	LogicalIndex int             `json:"logicalIndex"`
	AssetID      string          `json:"assetId"`
	ContentHash  string          `json:"contentHash"`
	DurationMS   int             `json:"durationMs"`
	Anchor       ManifestAnchor  `json:"anchor"`
	Lineage      ManifestLineage `json:"lineage"`
}

type ManifestAnchor struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Space string  `json:"space"`
}

type ManifestLineage struct {
	Type             string `json:"type"`
	SourceRevisionID string `json:"sourceRevisionId,omitempty"`
	SourceAttemptID  string `json:"sourceAttemptId,omitempty"`
	SourceFrameID    string `json:"sourceFrameId,omitempty"`
}

type ManifestQuality struct {
	EvaluationID string `json:"evaluationId"`
	Verdict      string `json:"verdict"`
}

type OperationPayload struct {
	Type          string `json:"type"`
	SchemaVersion int    `json:"schemaVersion"`
	Payload       any    `json:"payload"`
}

type FrameReorderPayload struct {
	FrameID       string `json:"frameId"`
	BeforeFrameID string `json:"beforeFrameId,omitempty"`
	AfterFrameID  string `json:"afterFrameId,omitempty"`
}

type FrameDeletePayload struct {
	FrameID string `json:"frameId"`
}

type FrameRestorePayload struct {
	FrameID       string `json:"frameId"`
	BeforeFrameID string `json:"beforeFrameId,omitempty"`
	AfterFrameID  string `json:"afterFrameId,omitempty"`
}

type FrameDuplicatePayload struct {
	FrameID      string `json:"frameId"`
	AfterFrameID string `json:"afterFrameID,omitempty"`
}

type FrameInsertAssetPayload struct {
	AssetID       string `json:"assetId"`
	BeforeFrameID string `json:"beforeFrameId,omitempty"`
	AfterFrameID  string `json:"afterFrameID,omitempty"`
	DurationMS    int    `json:"durationMs,omitempty"`
}

type FrameReplaceAssetPayload struct {
	FrameID    string `json:"frameId"`
	AssetID    string `json:"assetId"`
	KeepAnchor bool   `json:"keepAnchor"`
}

type FrameSetDurationPayload struct {
	FrameID    string `json:"frameId"`
	DurationMS int    `json:"durationMs"`
}

type FrameBatchSetDurationPayload struct {
	FrameIDs   []string `json:"frameIds"`
	DurationMS int      `json:"durationMs"`
}

type ActionSetDefaultFPSPayload struct {
	DefaultFPS  int  `json:"defaultFps"`
	Recalculate bool `json:"recalculate"`
}

type ActionSetLoopTypePayload struct {
	LoopType string `json:"loopType"`
}

type ActionSetReturnActionPayload struct {
	ReturnAction string `json:"returnAction"`
}

type ActionSetInterruptiblePayload struct {
	Interruptible bool `json:"interruptible"`
}

type ActionSetPriorityOverridePayload struct {
	Priority *int `json:"priority"`
}

type ActionSetCooldownOverridePayload struct {
	CooldownMS *int `json:"cooldownMs"`
}

type AnchorSetFramePayload struct {
	FrameID string  `json:"frameId"`
	AnchorX float64 `json:"anchorX"`
	AnchorY float64 `json:"anchorY"`
	Space   string  `json:"space"`
}

type AnchorBatchOffsetPayload struct {
	FrameIDs []string `json:"frameIds"`
	DeltaX   float64  `json:"deltaX"`
	DeltaY   float64  `json:"deltaY"`
}

type AnchorResetPayload struct {
	FrameIDs []string `json:"frameIds,omitempty"`
}

type BackgroundApplyPatchPayload struct {
	FrameID         string  `json:"frameId"`
	PatchType       string  `json:"patchType"`
	BrushData       []byte  `json:"-"`
	BrushDataBase64 string  `json:"brushDataBase64"`
	BrushSize       int     `json:"brushSize"`
	BrushHardness   float64 `json:"brushHardness"`
	BrushOpacity    float64 `json:"brushOpacity"`
	CanvasWidth     int     `json:"canvasWidth"`
	CanvasHeight    int     `json:"canvasHeight"`
}

type BackgroundResetPatchPayload struct {
	FrameID string `json:"frameId"`
}

type CandidateAcceptPayload struct {
	CandidateID string `json:"candidateId"`
	FrameID     string `json:"frameId"`
}

type CandidateRejectPayload struct {
	CandidateID string `json:"candidateId"`
}

type CreateSessionRequest struct {
	BaseRevisionID   string `json:"baseRevisionId"`
	ClientInstanceID string `json:"clientInstanceId"`
	IdempotencyKey   string `json:"idempotencyKey"`
}

type CreateSessionResponse struct {
	SessionID      string `json:"sessionId"`
	SessionVersion int64  `json:"sessionVersion"`
	BaseRevisionID string `json:"baseRevisionId"`
}

type ApplyOperationRequest struct {
	BaseSessionVersion int64            `json:"baseSessionVersion"`
	IdempotencyKey     string           `json:"idempotencyKey"`
	Operation          OperationPayload `json:"operation"`
}

type ApplyOperationResponse struct {
	SessionVersion int64  `json:"sessionVersion"`
	Sequence       int    `json:"sequence"`
	Status         string `json:"status"`
}

type SessionConflictDetail struct {
	CurrentVersion          int64              `json:"currentVersion"`
	ExpectedVersion         int64              `json:"expectedVersion"`
	OperationsSinceExpected []OperationSummary `json:"operationsSinceExpected"`
}

type OperationSummary struct {
	Sequence      int    `json:"sequence"`
	OperationType string `json:"operationType"`
	Status        string `json:"status"`
}

type CommitSessionRequest struct {
	ExpectedSessionVersion int64  `json:"expectedSessionVersion"`
	ChangeSummary          string `json:"changeSummary"`
	ActivationPolicy       string `json:"activationPolicy"`
	IdempotencyKey         string `json:"idempotencyKey"`
}

type CommitSessionResponse struct {
	RevisionID   string `json:"revisionId"`
	QualityJobID string `json:"qualityJobId"`
	Status       string `json:"status"`
}

type CreateRegenerationJobRequest struct {
	TargetFrameID         string `json:"targetFrameId"`
	JobType               string `json:"jobType"`
	IdempotencyKey        string `json:"idempotencyKey"`
	CostConfirmationToken string `json:"costConfirmationToken"`
	FixIntent             string `json:"fixIntent,omitempty"`
	UseAdjacentFrames     bool   `json:"useAdjacentFrames"`
}

type CreateRegenerationJobResponse struct {
	JobID        string `json:"jobId"`
	Status       string `json:"status"`
	CostEstimate any    `json:"costEstimate,omitempty"`
}

type AcceptCandidateRequest struct {
	IdempotencyKey string `json:"idempotencyKey"`
}

type RejectCandidateRequest struct {
	IdempotencyKey string `json:"idempotencyKey"`
}

type ActivateRevisionRequest struct {
	RevisionID             string `json:"revisionId"`
	ExpectedBindingVersion int64  `json:"expectedBindingVersion"`
	Reason                 string `json:"reason"`
	IdempotencyKey         string `json:"idempotencyKey"`
}

type RevisionSummary struct {
	ID               string `json:"id"`
	RevisionNumber   int    `json:"revisionNumber"`
	RevisionType     string `json:"revisionType"`
	Status           string `json:"status"`
	FrameCount       int    `json:"frameCount"`
	DurationMS       int    `json:"durationMs"`
	DefaultFPS       int    `json:"defaultFps"`
	LoopType         string `json:"loopType"`
	QualityVerdict   string `json:"qualityVerdict"`
	ChangeSummary    string `json:"changeSummary"`
	ParentRevisionID string `json:"parentRevisionId"`
	IsActive         bool   `json:"isActive"`
	CreatedAt        string `json:"createdAt"`
}

type RevisionDetail struct {
	Revision ActionRevision        `json:"revision"`
	Frames   []ActionRevisionFrame `json:"frames"`
	Assets   []FrameAsset          `json:"assets"`
	Manifest *RevisionManifest     `json:"manifest,omitempty"`
}

type ActionStreamSummary struct {
	ID                   string `json:"id"`
	UserID               string `json:"userId"`
	CharacterID          string `json:"characterId"`
	ActionKey            string `json:"actionKey"`
	RootProcessingTaskID string `json:"rootProcessingTaskId"`
	StreamKey            string `json:"streamKey"`
	NextRevisionNumber   int64  `json:"nextRevisionNumber"`
	ActiveRevisionID     string `json:"activeRevisionId"`
	BindingRevision      int64  `json:"bindingRevision"`
	RevisionCount        int    `json:"revisionCount"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
}

type FrameTimelineItem struct {
	FrameID         string  `json:"frameId"`
	LogicalIndex    int     `json:"logicalIndex"`
	AssetID         string  `json:"assetId"`
	ContentHash     string  `json:"contentHash"`
	DurationMS      int     `json:"durationMs"`
	SourceType      string  `json:"sourceType"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	AnchorX         float64 `json:"anchorX"`
	AnchorY         float64 `json:"anchorY"`
	HasQualityIssue bool    `json:"hasQualityIssue"`
}

type ActionEditSummary struct {
	ActionKey         string              `json:"actionKey"`
	ActiveRevisionID  string              `json:"activeRevisionId"`
	ActiveRevisionNum int                 `json:"activeRevisionNum"`
	BindingVersion    int64               `json:"bindingVersion"`
	FrameCount        int                 `json:"frameCount"`
	DurationMS        int                 `json:"durationMs"`
	QualityVerdict    string              `json:"qualityVerdict"`
	HasOpenSession    bool                `json:"hasOpenSession"`
	RevisionCount     int                 `json:"revisionCount"`
	Timeline          []FrameTimelineItem `json:"timeline,omitempty"`
}

type UploadCandidateResponse struct {
	CandidateID string `json:"candidateId"`
	AssetID     string `json:"assetId"`
	Status      string `json:"status"`
}

type SessionEvent struct {
	EventType string `json:"eventType"`
	SessionID string `json:"sessionId"`
	Data      any    `json:"data,omitempty"`
	Timestamp string `json:"timestamp"`
}

type JobProgressDetail struct {
	JobID       string `json:"jobId"`
	Status      string `json:"status"`
	Progress    int    `json:"progress"`
	CandidateID string `json:"candidateId,omitempty"`
}
