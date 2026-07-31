// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

type ProcessingTask struct {
	ID                         string  `gorm:"column:id;primaryKey" json:"id"`
	GenerationTaskID           string  `gorm:"column:generation_task_id" json:"generationTaskId"`
	ProcessingVersion          int     `gorm:"column:processing_version" json:"processingVersion"`
	Status                     string  `gorm:"column:status" json:"status"`
	CurrentStage               string  `gorm:"column:current_stage" json:"currentStage"`
	Progress                   int     `gorm:"column:progress" json:"progress"`
	OutputWidth                int     `gorm:"column:output_width" json:"outputWidth"`
	OutputHeight               int     `gorm:"column:output_height" json:"outputHeight"`
	TargetCharacterHeightRatio float64 `gorm:"column:target_character_height_ratio" json:"targetCharacterHeightRatio"`
	AnchorMode                 string  `gorm:"column:anchor_mode" json:"anchorMode"`
	BackgroundMode             string  `gorm:"column:background_mode" json:"backgroundMode"`
	OutputFormat               string  `gorm:"column:output_format" json:"outputFormat"`
	DefaultFPS                 int     `gorm:"column:default_fps" json:"defaultFps"`
	ExecutionID                string  `gorm:"column:execution_id" json:"executionId"`
	WorkerID                   string  `gorm:"column:worker_id" json:"workerId"`
	LeaseExpiresAt             string  `gorm:"column:lease_expires_at" json:"leaseExpiresAt"`
	LastHeartbeatAt            string  `gorm:"column:last_heartbeat_at" json:"lastHeartbeatAt"`
	CancelRequestedAt          string  `gorm:"column:cancel_requested_at" json:"cancelRequestedAt"`
	ErrorCode                  string  `gorm:"column:error_code" json:"errorCode"`
	ErrorMessage               string  `gorm:"column:error_message" json:"errorMessage"`
	StartedAt                  string  `gorm:"column:started_at" json:"startedAt"`
	CompletedAt                string  `gorm:"column:completed_at" json:"completedAt"`
	CreatedAt                  string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt                  string  `gorm:"column:updated_at" json:"updatedAt"`
	RowVersion                 int64   `gorm:"column:row_version;default:0" json:"rowVersion"`
	StatusReason               string  `gorm:"column:status_reason;default:''" json:"statusReason"`
	FailureStage               string  `gorm:"column:failure_stage;default:''" json:"failureStage"`
	LastTransitionAt           string  `gorm:"column:last_transition_at;default:''" json:"lastTransitionAt"`
	SubmittedAt                string  `gorm:"column:submitted_at;default:''" json:"submittedAt"`
	CancellingAt               string  `gorm:"column:cancelling_at;default:''" json:"cancellingAt"`
	CancelledAt                string  `gorm:"column:cancelled_at;default:''" json:"cancelledAt"`
	ConfigSnapshot             string  `gorm:"column:config_snapshot;default:'{}'" json:"configSnapshot"`
	ConfigHash                 string  `gorm:"column:config_hash;default:''" json:"configHash"`
	PipelineVersion            string  `gorm:"column:pipeline_version;default:''" json:"pipelineVersion"`
	ActiveRevisionCount        int     `gorm:"column:active_revision_count;default:0" json:"activeRevisionCount"`
	PublishState               string  `gorm:"column:publish_state;default:''" json:"publishState"`
}

func (ProcessingTask) TableName() string { return "desktop_pet_processing_tasks" }

type ProcessingAction struct {
	ID                     string  `gorm:"column:id;primaryKey" json:"id"`
	ProcessingTaskID       string  `gorm:"column:processing_task_id" json:"processingTaskId"`
	GenerationTaskActionID string  `gorm:"column:generation_task_action_id" json:"generationTaskActionId"`
	ActionKey              string  `gorm:"column:action_key" json:"actionKey"`
	ActionNameSnapshot     string  `gorm:"column:action_name_snapshot" json:"actionNameSnapshot"`
	SourceAttemptNumber    int     `gorm:"column:source_attempt_number" json:"sourceAttemptNumber"`
	Status                 string  `gorm:"column:status" json:"status"`
	Progress               int     `gorm:"column:progress" json:"progress"`
	SourceFrameCount       int     `gorm:"column:source_frame_count" json:"sourceFrameCount"`
	ProcessedFrameCount    int     `gorm:"column:processed_frame_count" json:"processedFrameCount"`
	LoopType               string  `gorm:"column:loop_type" json:"loopType"`
	FPS                    int     `gorm:"column:fps" json:"fps"`
	FrameDurationMS        int     `gorm:"column:frame_duration_ms" json:"frameDurationMs"`
	AnchorType             string  `gorm:"column:anchor_type" json:"anchorType"`
	AnchorX                float64 `gorm:"column:anchor_x" json:"anchorX"`
	AnchorY                float64 `gorm:"column:anchor_y" json:"anchorY"`
	BoundingBox            string  `gorm:"column:bounding_box" json:"boundingBox"`
	Excluded               int     `gorm:"column:excluded" json:"excluded"`
	ProcessingAttempt      int     `gorm:"column:processing_attempt" json:"processingAttempt"`
	LastSuccessfulAttempt  int     `gorm:"column:last_successful_attempt" json:"lastSuccessfulAttempt"`
	ActiveExecutionID      string  `gorm:"column:active_execution_id" json:"activeExecutionId"`
	RowVersion             int64   `gorm:"column:row_version;default:0" json:"rowVersion"`
	ErrorCode              string  `gorm:"column:error_code" json:"errorCode"`
	ErrorMessage           string  `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt              string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt              string  `gorm:"column:updated_at" json:"updatedAt"`
	StartedAt              string  `gorm:"column:started_at" json:"startedAt"`
	CompletedAt            string  `gorm:"column:completed_at" json:"completedAt"`
	CurrentStage           string  `gorm:"column:current_stage;default:'created'" json:"currentStage"`
	StatusReason           string  `gorm:"column:status_reason;default:''" json:"statusReason"`
	FailureStage           string  `gorm:"column:failure_stage;default:''" json:"failureStage"`
	LastTransitionAt       string  `gorm:"column:last_transition_at;default:''" json:"lastTransitionAt"`
	ExecutionID            string  `gorm:"column:execution_id;default:''" json:"executionId"`
	WorkerID               string  `gorm:"column:worker_id;default:''" json:"workerId"`
	AttemptNumber          int     `gorm:"column:attempt_number;default:1" json:"attemptNumber"`
	ActionSpecSchemaVersion int    `gorm:"column:action_spec_schema_version" json:"actionSpecSchemaVersion"`
	ActionSpecVersion      int     `gorm:"column:action_spec_version" json:"actionSpecVersion"`
	ActionSpecHash         string  `gorm:"column:action_spec_hash" json:"actionSpecHash"`
	ReturnPolicy           string  `gorm:"column:return_policy" json:"returnPolicy"`
	ReturnActionKey        string  `gorm:"column:return_action_key" json:"returnActionKey"`
	Interruptible          int     `gorm:"column:interruptible" json:"interruptible"`
	InterruptAfterMS       int     `gorm:"column:interrupt_after_ms" json:"interruptAfterMs"`
	MinimumPlayMS          int     `gorm:"column:minimum_play_ms" json:"minimumPlayMs"`
	MaximumPlayMS          int     `gorm:"column:maximum_play_ms" json:"maximumPlayMs"`
	Priority               int     `gorm:"column:priority" json:"priority"`
	CooldownMS             int     `gorm:"column:cooldown_ms" json:"cooldownMs"`
	MutexGroup             string  `gorm:"column:mutex_group" json:"mutexGroup"`
	QueuePolicy            string  `gorm:"column:queue_policy" json:"queuePolicy"`
	DedupWindowMS          int     `gorm:"column:dedup_window_ms" json:"dedupWindowMs"`
	AnchorProfile          string  `gorm:"column:anchor_profile" json:"anchorProfile"`
	PlaybackMode           string  `gorm:"column:playback_mode" json:"playbackMode"`
	ActiveRevisionID       string  `gorm:"column:active_revision_id;default:''" json:"activeRevisionId"`
	NextRevisionNumber     int     `gorm:"column:next_revision_number;default:1" json:"nextRevisionNumber"`
	SourceAttemptID        string  `gorm:"column:source_attempt_id;default:''" json:"sourceAttemptId"`
	SourceCandidateIndex   int     `gorm:"column:source_candidate_index;default:0" json:"sourceCandidateIndex"`
	ProcessingProfileSnap  string  `gorm:"column:processing_profile_snapshot;default:'{}'" json:"processingProfileSnapshot"`
	PendingRetryRequestID  string  `gorm:"column:pending_retry_request_id;default:''" json:"pendingRetryRequestId"`
	ProcessingWarnings     string  `gorm:"column:processing_warnings;default:''" json:"processingWarnings"`
	WarningCount           int     `gorm:"column:warning_count;default:0" json:"warningCount"`
	ActionSpecSnapshot     string  `gorm:"column:action_spec_snapshot;default:'{}'" json:"actionSpecSnapshot"`
}

func (ProcessingAction) TableName() string { return "desktop_pet_processing_actions" }

type ProcessingActionAttempt struct {
	ID                     string `gorm:"column:id;primaryKey" json:"id"`
	ProcessingActionID     string `gorm:"column:processing_action_id" json:"processingActionId"`
	ProcessingTaskID       string `gorm:"column:processing_task_id" json:"processingTaskId"`
	ActionKey              string `gorm:"column:action_key" json:"actionKey"`
	AttemptNumber          int    `gorm:"column:attempt_number" json:"attemptNumber"`
	SourceGenerationAttempt int   `gorm:"column:source_generation_attempt" json:"sourceGenerationAttempt"`
	ExecutionID            string `gorm:"column:execution_id" json:"executionId"`
	Status                 string `gorm:"column:status" json:"status"`
	Progress               int    `gorm:"column:progress" json:"progress"`
	ErrorCode              string `gorm:"column:error_code" json:"errorCode"`
	ErrorMessage           string `gorm:"column:error_message" json:"errorMessage"`
	StartedAt              string `gorm:"column:started_at" json:"startedAt"`
	CompletedAt            string `gorm:"column:completed_at" json:"completedAt"`
	CreatedAt              string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt              string `gorm:"column:updated_at" json:"updatedAt"`
	LeaseOwner             string `gorm:"column:lease_owner;default:''" json:"leaseOwner"`
	LeaseExpiresAt         string `gorm:"column:lease_expires_at;default:''" json:"leaseExpiresAt"`
	HeartbeatAt            string `gorm:"column:heartbeat_at;default:''" json:"heartbeatAt"`
	CommitID               string `gorm:"column:commit_id;default:''" json:"commitId"`
}

func (ProcessingActionAttempt) TableName() string { return "desktop_pet_processing_action_attempts" }

type ProcessedFrame struct {
	ID                     string  `gorm:"column:id;primaryKey" json:"id"`
	ProcessingActionID     string  `gorm:"column:processing_action_id" json:"processingActionId"`
	SourceFrameID          string  `gorm:"column:source_frame_id" json:"sourceFrameId"`
	FrameIndex             int     `gorm:"column:frame_index" json:"frameIndex"`
	Status                 string  `gorm:"column:status" json:"status"`
	SourcePath             string  `gorm:"column:source_path" json:"sourcePath"`
	ProcessedPath          string  `gorm:"column:processed_path" json:"processedPath"`
	Width                  int     `gorm:"column:width" json:"width"`
	Height                 int     `gorm:"column:height" json:"height"`
	ContentHash            string  `gorm:"column:content_hash" json:"contentHash"`
	SubjectBox             string  `gorm:"column:subject_box" json:"subjectBox"`
	AnchorX                float64 `gorm:"column:anchor_x" json:"anchorX"`
	AnchorY                float64 `gorm:"column:anchor_y" json:"anchorY"`
	AlphaCoverage          float64 `gorm:"column:alpha_coverage" json:"alphaCoverage"`
	QualityFlags           string  `gorm:"column:quality_flags" json:"qualityFlags"`
	ProcessingAttemptID    string  `gorm:"column:processing_attempt_id" json:"processingAttemptId"`
	ProcessingAttemptNumber int    `gorm:"column:processing_attempt_number" json:"processingAttemptNumber"`
	ExecutionID            string  `gorm:"column:execution_id;default:''" json:"executionId"`
	ErrorCode              string  `gorm:"column:error_code" json:"errorCode"`
	ErrorMessage           string  `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt              string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt              string  `gorm:"column:updated_at" json:"updatedAt"`
	RowVersion             int64   `gorm:"column:row_version;default:0" json:"rowVersion"`
	CurrentStage           string  `gorm:"column:current_stage;default:'created'" json:"currentStage"`
	StatusReason           string  `gorm:"column:status_reason;default:''" json:"statusReason"`
	FailureStage           string  `gorm:"column:failure_stage;default:''" json:"failureStage"`
	LastTransitionAt       string  `gorm:"column:last_transition_at;default:''" json:"lastTransitionAt"`
	StartedAt              string  `gorm:"column:started_at;default:''" json:"startedAt"`
	CompletedAt            string  `gorm:"column:completed_at;default:''" json:"completedAt"`
	RevisionID             string  `gorm:"column:revision_id;default:''" json:"revisionId"`
	MaskPath               string  `gorm:"column:mask_path;default:''" json:"maskPath"`
	TransformChainID       string  `gorm:"column:transform_chain_id;default:''" json:"transformChainId"`
	MeasurementID          string  `gorm:"column:measurement_id;default:''" json:"measurementId"`
	SourceArtifactID       string  `gorm:"column:source_artifact_id;default:''" json:"sourceArtifactId"`
	SourceCellIndex        *int    `gorm:"column:source_cell_index" json:"sourceCellIndex,omitempty"`
}

func (ProcessedFrame) TableName() string { return "desktop_pet_processed_frames" }

type Package struct {
	ID               string `gorm:"column:id;primaryKey" json:"id"`
	UserID           string `gorm:"column:user_id" json:"userId"`
	CharacterID      string `gorm:"column:character_id" json:"characterId"`
	GenerationTaskID string `gorm:"column:generation_task_id" json:"generationTaskId"`
	ProcessingTaskID string `gorm:"column:processing_task_id" json:"processingTaskId"`
	Name             string `gorm:"column:name" json:"name"`
	Version          int    `gorm:"column:version" json:"version"`
	Status           string `gorm:"column:status" json:"status"`
	DefaultActionKey string `gorm:"column:default_action_key" json:"defaultActionKey"`
	CanvasWidth      int    `gorm:"column:canvas_width" json:"canvasWidth"`
	CanvasHeight     int    `gorm:"column:canvas_height" json:"canvasHeight"`
	PackagePath      string `gorm:"column:package_path" json:"packagePath"`
	ManifestPath     string `gorm:"column:manifest_path" json:"manifestPath"`
	PreviewPath      string `gorm:"column:preview_path" json:"previewPath"`
	ActionCount      int    `gorm:"column:action_count" json:"actionCount"`
	PackageHash      string `gorm:"column:package_hash" json:"packageHash"`
	IncludedActions  string `gorm:"column:included_actions" json:"includedActions"`
	CreatedAt        string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        string `gorm:"column:updated_at" json:"updatedAt"`
	RowVersion       int64  `gorm:"column:row_version;default:0" json:"rowVersion"`
	CurrentStage     string `gorm:"column:current_stage;default:'created'" json:"currentStage"`
	StatusReason     string `gorm:"column:status_reason;default:''" json:"statusReason"`
	FailureStage     string `gorm:"column:failure_stage;default:''" json:"failureStage"`
	LastTransitionAt string `gorm:"column:last_transition_at;default:''" json:"lastTransitionAt"`
	ErrorCode        string `gorm:"column:error_code;default:''" json:"errorCode"`
	ErrorMessage     string `gorm:"column:error_message;default:''" json:"errorMessage"`
}

func (Package) TableName() string { return "desktop_pet_packages" }
