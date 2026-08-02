// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

type EvaluationInputSnapshot struct {
	ID                   string `json:"id"`
	UserID               string `json:"userId"`
	CharacterID          string `json:"characterId"`
	ActionStreamID       string `json:"actionStreamId"`
	ActionRevisionID     string `json:"actionRevisionId"`
	ActionContentHash    string `json:"actionContentHash"`
	FrameSetHash         string `json:"frameSetHash"`
	BindingRevision      int64  `json:"bindingRevision"`
	ProcessingRevisionID string `json:"processingRevisionId"`
	ActionKey            string `json:"actionKey"`
	ActionConfigHash     string `json:"actionConfigHash"`
	ActionSpecHash       string `json:"actionSpecHash"`
	PlaybackMode         string `json:"playbackMode"`
	FPS                  int    `json:"fps"`
	ExpectedFrameCount   int    `json:"expectedFrameCount"`
	FrameInputsJSON      string `json:"frameInputsJson"`
	SnapshotHash         string `json:"snapshotHash"`
	CreatedAt            string `json:"createdAt"`
}

func (EvaluationInputSnapshot) TableName() string {
	return "desktop_pet_quality_input_snapshots"
}

type MeasurementSet struct {
	ID                     string `json:"id"`
	ActionRevisionID       string `json:"actionRevisionId"`
	ActionContentHash      string `json:"actionContentHash"`
	FrameSetHash           string `json:"frameSetHash"`
	MeasurementVersion     string `json:"measurementVersion"`
	MeasurementProfileHash string `json:"measurementProfileHash"`
	FrameCount             int    `json:"frameCount"`
	CanvasWidth            int    `json:"canvasWidth"`
	CanvasHeight           int    `json:"canvasHeight"`
	MeasurementSetHash     string `json:"measurementSetHash"`
	Status                 string `json:"status"`
	CreatedAt              string `json:"createdAt"`
}

func (MeasurementSet) TableName() string {
	return "desktop_pet_quality_measurement_sets"
}

type FrameMeasurementRecord struct {
	ID               string  `json:"id"`
	MeasurementSetID string  `json:"measurementSetId"`
	FrameArtifactID  string  `json:"frameArtifactId"`
	FrameIndex       int     `json:"frameIndex"`
	FileHash         string  `json:"fileHash"`
	PixelHash        string  `json:"pixelHash"`
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	MimeType         string  `json:"mimeTypa"`
	FileBytes        int64   `json:"fileBytes"`
	HasAlphaChannel  bool    `json:"hasAlphaChannel"`
	AlphaCoverage    float64 `json:"alphaCoverage"`
	FullyTransparent float64 `json:"fullyTransparentRatio"`
	SemiTransparent  float64 `json:"semiTransparentRatio"`
	Opaque           float64 `json:"opaqueRatio"`
	Decodable        bool    `json:"decodable"`
	CreatedAt        string  `json:"createdAt"`
}

func (FrameMeasurementRecord) TableName() string {
	return "desktop_pet_quality_frame_measurements"
}

type SequenceMeasurementRecord struct {
	ID                 string  `json:"id"`
	MeasurementSetID   string  `json:"measurementSetId"`
	FrameIndex         int     `json:"frameIndex"`
	SubjectAreaRatio   float64 `json:"subjectAreaRatio"`
	ConnectedCompCount int     `json:"connectedComponentCount"`
	LargestCompRatio   float64 `json:"largestComponentRatio"`
	BorderFGCoverage   float64 `json:"borderForegroundCoverage"`
	CentroidX          float64 `json:"centroidX"`
	CentroidY          float64 `json:"centroidY"`
	CreatedAt          string  `json:"createdAt"`
}

func (SequenceMeasurementRecord) TableName() string {
	return "desktop_pet_quality_sequence_measurements"
}

type QualityReportArtifact struct {
	ID            string `json:"id"`
	EvaluationID  string `json:"evaluationId"`
	StorageKey    string `json:"storageKey"`
	ContentHash   string `json:"contentHash"`
	ByteSize      int64  `json:"byteSize"`
	SchemaVersion string `json:"schemaVersion"`
	Status        string `json:"status"`
	CreatedAt     string `json:"createdAt"`
}

func (QualityReportArtifact) TableName() string {
	return "desktop_pet_quality_report_artifacts"
}

type ActiveQualityGateBinding struct {
	ID                    string `json:"id"`
	ProcessingTaskID      string `json:"processingTaskId"`
	GateProfileHash       string `json:"gateProfileHash"`
	ActiveGateID          string `json:"activeGateId"`
	ActiveRevisionSetHash string `json:"activeRevisionSetHash"`
	EvaluationSetHash     string `json:"evaluationSetHash"`
	BindingRevision       int64  `json:"bindingRevision"`
	CreatedAt             string `json:"createdAt"`
	UpdatedAt             string `json:"updatedAt"`
}

func (ActiveQualityGateBinding) TableName() string {
	return "desktop_pet_active_quality_gate_bindings"
}

type QualityGateSnapshot struct {
	ID                    string `json:"id"`
	UserID                string `json:"userId"`
	CharacterID           string `json:"characterId"`
	ProcessingTaskID      string `json:"processingTaskId"`
	ActiveRevisionSetHash string `json:"activeRevisionSetHash"`
	EvaluationSetHash     string `json:"evaluationSetHash"`
	GateProfileID         string `json:"gateProfileId"`
	GateProfileVersion    string `json:"gateProfileVersion"`
	RuleSetVersion        string `json:"ruleSetVersion"`
	RuleSetContentHash    string `json:"ruleSetContentHash"`
	GateStatus            string `json:"gateStatus"`
	RequiredActionKeys    string `json:"requiredActionKeysJson"`
	IncludedActionKeys    string `json:"includedActionKeysJson"`
	ExcludedActionKeys    string `json:"excludedActionKeysJson"`
	ActionVerdicts        string `json:"actionVerdictsJson"`
	RequiredActionCount   int    `json:"requiredActionCount"`
	AcceptedActionCount   int    `json:"acceptedActionCount"`
	WarningActionCount    int    `json:"warningActionCount"`
	ReviewActionCount     int    `json:"reviewActionCount"`
	RejectedActionCount   int    `json:"rejectedActionCount"`
	FailedEvaluationCount int    `json:"failedEvaluationCount"`
	SnapshotHash          string `json:"snapshotHash"`
	GateHash              string `json:"gateHash"`
	InvalidatedAt         string `json:"invalidatedAt"`
	InvalidationReason    string `json:"invalidationReason"`
	CreatedAt             string `json:"createdAt"`
}

func (QualityGateSnapshot) TableName() string {
	return "desktop_pet_quality_gate_snapshots"
}

type QualityOutboxEventV2 struct {
	ID           string `json:"id"`
	EventID      string `json:"eventId"`
	EventType    string `json:"eventType"`
	AggregateID  string `json:"aggregateId"`
	AggregateSeq int    `json:"aggregateSequence"`
	PayloadJSON  string `json:"payloadJson"`
	PayloadHash  string `json:"payloadHash"`
	Status       string `json:"status"`
	AttemptCount int    `json:"attemptCount"`
	AvailableAt  string `json:"availableAt"`
	LastError    string `json:"lastError"`
	CreatedAt    string `json:"createdAt"`
	PublishedAt  string `json:"publishedAt"`
}

func (QualityOutboxEventV2) TableName() string {
	return "desktop_pet_quality_outbox_events_v2"
}

type QualityCommitJournalV2 struct {
	ID                string `json:"id"`
	CommitHash        string `json:"commitHash"`
	EvaluationID      string `json:"evaluationId"`
	ActionRevisionID  string `json:"actionRevisionId"`
	ActionContentHash string `json:"actionContentHash"`
	Status            string `json:"status"`
	StepsJSON         string `json:"stepsJson"`
	ReportStagingKey  string `json:"reportStagingKey"`
	ReportFinalKey    string `json:"reportFinalKey"`
	ReportHash        string `json:"reportHash"`
	ResultHash        string `json:"resultHash"`
	LastError         string `json:"lastError"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
	CompletedAt       string `json:"completedAt"`
}

func (QualityCommitJournalV2) TableName() string {
	return "desktop_pet_quality_commit_journals_v2"
}

type QualityEvaluationRequestInbox struct {
	ID                string `json:"id"`
	EventID           string `json:"eventId"`
	ActionRevisionID  string `json:"actionRevisionId"`
	ActionContentHash string `json:"actionContentHash"`
	ProfileID         string `json:"profileId"`
	ProfileVersion    string `json:"profileVersion"`
	RuleSetVersion    string `json:"ruleSetVersion"`
	IdempotencyKey    string `json:"idempotencyKey"`
	PayloadHash       string `json:"payloadHash"`
	Status            string `json:"status"`
	AttemptCount      int    `json:"attemptCount"`
	LeaseOwner        string `json:"leaseOwner"`
	LeaseExpiresAt    string `json:"leaseExpiresAt"`
	LastError         string `json:"lastError"`
	ReceivedAt        string `json:"receivedAt"`
	ProcessedAt       string `json:"processedAt"`
	CreatedAt         string `json:"createdAt"`
}

func (QualityEvaluationRequestInbox) TableName() string {
	return "desktop_pet_quality_evaluation_request_inbox"
}

type ActiveQualityBindingHistory struct {
	ID               string `json:"id"`
	ActionRevisionID string `json:"actionRevisionId"`
	ProfileHash      string `json:"profileHash"`
	BindingRevision  int64  `json:"bindingRevision"`
	PreviousEvalID   string `json:"previousEvaluationId"`
	NewEvalID        string `json:"newEvaluationId"`
	Reason           string `json:"reason"`
	Actor            string `json:"actor"`
	OccurredAt       string `json:"occurredAt"`
}

func (ActiveQualityBindingHistory) TableName() string {
	return "desktop_pet_active_quality_binding_history"
}

type QualityGateRebuildRequest struct {
	ID               string `json:"id"`
	ProcessingTaskID string `json:"processingTaskId"`
	SourceEventType  string `json:"sourceEventType"`
	SourceEventID    string `json:"sourceEventId"`
	Reason           string `json:"reason"`
	Status           string `json:"status"`
	CreatedAt        string `json:"createdAt"`
}

func (QualityGateRebuildRequest) TableName() string {
	return "desktop_pet_quality_gate_rebuild_requests"
}
