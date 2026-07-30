package editing

type ActionRevision struct {
	ID                   string `gorm:"column:id;primaryKey" json:"id"`
	ProcessingTaskID     string `gorm:"column:processing_task_id" json:"processingTaskId"`
	ProcessingActionID   string `gorm:"column:processing_action_id" json:"processingActionId"`
	GenerationTaskID     string `gorm:"column:generation_task_id" json:"generationTaskId"`
	ActionKey            string `gorm:"column:action_key" json:"actionKey"`
	ParentRevisionID     string `gorm:"column:parent_revision_id" json:"parentRevisionId"`
	RootRevisionID       string `gorm:"column:root_revision_id" json:"rootRevisionId"`
	RevisionNumber       int    `gorm:"column:revision_number" json:"revisionNumber"`
	RevisionType         string `gorm:"column:revision_type" json:"revisionType"`
	Status               string `gorm:"column:status" json:"status"`
	ManifestPath         string `gorm:"column:manifest_path" json:"manifestPath"`
	ManifestHash         string `gorm:"column:manifest_hash" json:"manifestHash"`
	FrameCount           int    `gorm:"column:frame_count" json:"frameCount"`
	DurationMS           int    `gorm:"column:duration_ms" json:"durationMs"`
	DefaultFPS           int    `gorm:"column:default_fps" json:"defaultFps"`
	LoopType             string `gorm:"column:loop_type" json:"loopType"`
	ReturnAction         string `gorm:"column:return_action" json:"returnAction"`
	Interruptible        int    `gorm:"column:interruptible" json:"interruptible"`
	PriorityOverride     *int   `gorm:"column:priority_override" json:"priorityOverride"`
	CooldownMSOverride   *int   `gorm:"column:cooldown_ms_override" json:"cooldownMsOverride"`
	QualityEvaluationID  string `gorm:"column:quality_evaluation_id" json:"qualityEvaluationId"`
	QualityVerdict       string `gorm:"column:quality_verdict" json:"qualityVerdict"`
	CreatedByUserID      string `gorm:"column:created_by_user_id" json:"createdByUserId"`
	CreatedFromSessionID string `gorm:"column:created_from_session_id" json:"createdFromSessionId"`
	ChangeSummary        string `gorm:"column:change_summary" json:"changeSummary"`
	SourceSummaryJSON    string `gorm:"column:source_summary_json" json:"sourceSummaryJson"`
	CreatedAt            string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt            string `gorm:"column:updated_at" json:"updatedAt"`
	ReadyAt              string `gorm:"column:ready_at" json:"readyAt"`
}

func (ActionRevision) TableName() string { return "desktop_pet_action_revisions" }

type ActiveRevisionBinding struct {
	ProcessingTaskID string `gorm:"column:processing_task_id;primaryKey" json:"processingTaskId"`
	ActionKey        string `gorm:"column:action_key;primaryKey" json:"actionKey"`
	RevisionID       string `gorm:"column:revision_id" json:"revisionId"`
	BindingVersion   int64  `gorm:"column:binding_version" json:"bindingVersion"`
	ActivatedBy      string `gorm:"column:activated_by" json:"activatedBy"`
	Reason           string `gorm:"column:reason" json:"reason"`
	CreatedAt        string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        string `gorm:"column:updated_at" json:"updatedAt"`
}

func (ActiveRevisionBinding) TableName() string { return "desktop_pet_action_active_revisions" }

type FrameAsset struct {
	ID           string `gorm:"column:id;primaryKey" json:"id"`
	ContentHash  string `gorm:"column:content_hash" json:"contentHash"`
	StoragePath  string `gorm:"column:storage_path" json:"storagePath"`
	MimeType     string `gorm:"column:mime_type" json:"mimeType"`
	Width        int    `gorm:"column:width" json:"width"`
	Height       int    `gorm:"column:height" json:"height"`
	ByteSize     int64  `gorm:"column:byte_size" json:"byteSize"`
	AlphaMode    string `gorm:"column:alpha_mode" json:"alphaMode"`
	ColorSpace   string `gorm:"column:color_space" json:"colorSpace"`
	SourceType   string `gorm:"column:source_type" json:"sourceType"`
	SourceRefID  string `gorm:"column:source_ref_id" json:"sourceRefId"`
	OriginalHash string `gorm:"column:original_hash" json:"originalHash"`
	CreatedBy    string `gorm:"column:created_by" json:"createdBy"`
	Status       string `gorm:"column:status" json:"status"`
	CreatedAt    string `gorm:"column:created_at" json:"createdAt"`
}

func (FrameAsset) TableName() string { return "desktop_pet_frame_assets" }

type ActionRevisionFrame struct {
	ID               string  `gorm:"column:id;primaryKey" json:"id"`
	RevisionID       string  `gorm:"column:revision_id" json:"revisionId"`
	FrameID          string  `gorm:"column:frame_id" json:"frameId"`
	AssetID          string  `gorm:"column:asset_id" json:"assetId"`
	LogicalIndex     int     `gorm:"column:logical_index" json:"logicalIndex"`
	DurationMS       int     `gorm:"column:duration_ms" json:"durationMs"`
	SourceFrameID    string  `gorm:"column:source_frame_id" json:"sourceFrameId"`
	SourceRevisionID string  `gorm:"column:source_revision_id" json:"sourceRevisionId"`
	SourceAttemptID  string  `gorm:"column:source_attempt_id" json:"sourceAttemptId"`
	AnchorX          float64 `gorm:"column:anchor_x" json:"anchorX"`
	AnchorY          float64 `gorm:"column:anchor_y" json:"anchorY"`
	AnchorSpace      string  `gorm:"column:anchor_space" json:"anchorSpace"`
	OffsetX          float64 `gorm:"column:offset_x" json:"offsetX"`
	OffsetY          float64 `gorm:"column:offset_y" json:"offsetY"`
	MaskAssetID      string  `gorm:"column:mask_asset_id" json:"maskAssetId"`
	TransformJSON    string  `gorm:"column:transform_json" json:"transformJson"`
	MetadataJSON     string  `gorm:"column:metadata_json" json:"metadataJson"`
	CopiedFromFrameID string `gorm:"column:copied_from_frame_id" json:"copiedFromFrameId"`
	CreatedAt        string  `gorm:"column:created_at" json:"createdAt"`
}

func (ActionRevisionFrame) TableName() string { return "desktop_pet_action_revision_frames" }

type EditSession struct {
	ID                  string `gorm:"column:id;primaryKey" json:"id"`
	UserID              string `gorm:"column:user_id" json:"userId"`
	ProcessingTaskID    string `gorm:"column:processing_task_id" json:"processingTaskId"`
	ActionKey           string `gorm:"column:action_key" json:"actionKey"`
	BaseRevisionID      string `gorm:"column:base_revision_id" json:"baseRevisionId"`
	SessionVersion      int64  `gorm:"column:session_version" json:"sessionVersion"`
	Status              string `gorm:"column:status" json:"status"`
	Cursor              int    `gorm:"column:cursor" json:"cursor"`
	LastOperationSeq    int    `gorm:"column:last_operation_seq" json:"lastOperationSeq"`
	CheckpointID        string `gorm:"column:checkpoint_id" json:"checkpointId"`
	ClientInstanceID    string `gorm:"column:client_instance_id" json:"clientInstanceId"`
	ExpiresAt           string `gorm:"column:expires_at" json:"expiresAt"`
	CreatedAt           string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt           string `gorm:"column:updated_at" json:"updatedAt"`
	CommittedRevisionID string `gorm:"column:committed_revision_id" json:"committedRevisionId"`
}

func (EditSession) TableName() string { return "desktop_pet_edit_sessions" }

type EditOperation struct {
	ID             string `gorm:"column:id;primaryKey" json:"id"`
	SessionID      string `gorm:"column:session_id" json:"sessionId"`
	Sequence       int    `gorm:"column:sequence" json:"sequence"`
	OperationType  string `gorm:"column:operation_type" json:"operationType"`
	PayloadJSON    string `gorm:"column:payload_json" json:"payloadJson"`
	InverseJSON    string `gorm:"column:inverse_json" json:"inverseJson"`
	IdempotencyKey string `gorm:"column:idempotency_key" json:"idempotencyKey"`
	BaseVersion    int64  `gorm:"column:base_version" json:"baseVersion"`
	ResultVersion  int64  `gorm:"column:result_version" json:"resultVersion"`
	Status         string `gorm:"column:status" json:"status"`
	CreatedBy      string `gorm:"column:created_by" json:"createdBy"`
	CreatedAt      string `gorm:"column:created_at" json:"createdAt"`
}

func (EditOperation) TableName() string { return "desktop_pet_edit_operations" }

type EditCheckpoint struct {
	ID              string `gorm:"column:id;primaryKey" json:"id"`
	SessionID       string `gorm:"column:session_id" json:"sessionId"`
	Sequence        int    `gorm:"column:sequence" json:"sequence"`
	ManifestJSON    string `gorm:"column:manifest_json" json:"manifestJson"`
	ManifestHash    string `gorm:"column:manifest_hash" json:"manifestHash"`
	FrameCount      int    `gorm:"column:frame_count" json:"frameCount"`
	CreatedAt       string `gorm:"column:created_at" json:"createdAt"`
}

func (EditCheckpoint) TableName() string { return "desktop_pet_edit_checkpoints" }

type RegenerationJob struct {
	ID                    string `gorm:"column:id;primaryKey" json:"id"`
	SessionID             string `gorm:"column:session_id" json:"sessionId"`
	ProcessingTaskID      string `gorm:"column:processing_task_id" json:"processingTaskId"`
	ActionKey             string `gorm:"column:action_key" json:"actionKey"`
	TargetFrameID         string `gorm:"column:target_frame_id" json:"targetFrameId"`
	JobType               string `gorm:"column:job_type" json:"jobType"`
	Status                string `gorm:"column:status" json:"status"`
	IdempotencyKey        string `gorm:"column:idempotency_key" json:"idempotencyKey"`
	ProviderAttemptID     string `gorm:"column:provider_attempt_id" json:"providerAttemptId"`
	RequestSnapshotJSON   string `gorm:"column:request_snapshot_json" json:"requestSnapshotJson"`
	CostEstimateJSON      string `gorm:"column:cost_estimate_json" json:"costEstimateJson"`
	CostActualJSON        string `gorm:"column:cost_actual_json" json:"costActualJson"`
	ErrorCode             string `gorm:"column:error_code" json:"errorCode"`
	ErrorMessage          string `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt             string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt             string `gorm:"column:updated_at" json:"updatedAt"`
}

func (RegenerationJob) TableName() string { return "desktop_pet_regeneration_jobs" }

type EditCandidate struct {
	ID              string `gorm:"column:id;primaryKey" json:"id"`
	SessionID       string `gorm:"column:session_id" json:"sessionId"`
	JobID           string `gorm:"column:job_id" json:"jobId"`
	TargetFrameID   string `gorm:"column:target_frame_id" json:"targetFrameId"`
	CandidateType   string `gorm:"column:candidate_type" json:"candidateType"`
	AssetID         string `gorm:"column:asset_id" json:"assetId"`
	CandidateRevisionID string `gorm:"column:candidate_revision_id" json:"candidateRevisionId"`
	Status          string `gorm:"column:status" json:"status"`
	MetadataJSON    string `gorm:"column:metadata_json" json:"metadataJson"`
	DecidedBy       string `gorm:"column:decided_by" json:"decidedBy"`
	DecidedAt       string `gorm:"column:decided_at" json:"decidedAt"`
	CreatedAt       string `gorm:"column:created_at" json:"createdAt"`
}

func (EditCandidate) TableName() string { return "desktop_pet_edit_candidates" }

type MaskPatch struct {
	ID              string `gorm:"column:id;primaryKey" json:"id"`
	SessionID       string `gorm:"column:session_id" json:"sessionId"`
	FrameID         string `gorm:"column:frame_id" json:"frameId"`
	SourceAssetHash string `gorm:"column:source_asset_hash" json:"sourceAssetHash"`
	ResultAssetID   string `gorm:"column:result_asset_id" json:"resultAssetId"`
	PatchType       string `gorm:"column:patch_type" json:"patchType"`
	BrushDataPath   string `gorm:"column:brush_data_path" json:"brushDataPath"`
	BrushSize       int    `gorm:"column:brush_size" json:"brushSize"`
	BrushHardness   float64 `gorm:"column:brush_hardness" json:"brushHardness"`
	BrushOpacity    float64 `gorm:"column:brush_opacity" json:"brushOpacity"`
	CoordinateSpace string `gorm:"column:coordinate_space" json:"coordinateSpace"`
	CanvasWidth     int    `gorm:"column:canvas_width" json:"canvasWidth"`
	CanvasHeight    int    `gorm:"column:canvas_height" json:"canvasHeight"`
	AlgorithmVersion string `gorm:"column:algorithm_version" json:"algorithmVersion"`
	OperationSeq    int    `gorm:"column:operation_seq" json:"operationSeq"`
	CreatedAt       string `gorm:"column:created_at" json:"createdAt"`
}

func (MaskPatch) TableName() string { return "desktop_pet_mask_patches" }

type PublishJournal struct {
	ID              string `gorm:"column:id;primaryKey" json:"id"`
	RevisionID      string `gorm:"column:revision_id" json:"revisionId"`
	SessionID       string `gorm:"column:session_id" json:"sessionId"`
	Action          string `gorm:"column:action" json:"action"`
	Status          string `gorm:"column:status" json:"status"`
	TmpDirPath      string `gorm:"column:tmp_dir_path" json:"tmpDirPath"`
	FinalDirPath    string `gorm:"column:final_dir_path" json:"finalDirPath"`
	ManifestPath    string `gorm:"column:manifest_path" json:"manifestPath"`
	ErrorMessage    string `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt       string `gorm:"column:created_at" json:"createdAt"`
	CompletedAt     string `gorm:"column:completed_at" json:"completedAt"`
}

func (PublishJournal) TableName() string { return "desktop_pet_publish_journal" }

type EditIdempotencyRecord struct {
	ID            string `gorm:"column:id;primaryKey" json:"id"`
	UserID        string `gorm:"column:user_id" json:"userId"`
	SessionID     string `gorm:"column:session_id" json:"sessionId"`
	IdempotencyKey string `gorm:"column:idempotency_key" json:"idempotencyKey"`
	Endpoint      string `gorm:"column:endpoint" json:"endpoint"`
	ResultJSON    string `gorm:"column:result_json" json:"resultJson"`
	Status        string `gorm:"column:status" json:"status"`
	CreatedAt     string `gorm:"column:created_at" json:"createdAt"`
}

func (EditIdempotencyRecord) TableName() string { return "desktop_pet_edit_idempotency" }
