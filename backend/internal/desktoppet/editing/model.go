package editing

type ActionRevision struct {
	ID                   string `gorm:"column:id;primaryKey" json:"id"`
	UserID               string `gorm:"column:user_id;default:''" json:"userId"`
	CharacterID          string `gorm:"column:character_id;default:''" json:"characterId"`
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
	QualityEvaluationID      string  `gorm:"column:quality_evaluation_id" json:"qualityEvaluationId"`
	QualityVerdict           string  `gorm:"column:quality_verdict" json:"qualityVerdict"`
	QualityOverallScore      *float64 `gorm:"column:quality_overall_score" json:"qualityOverallScore"`
	QualityProfileID         string  `gorm:"column:quality_profile_id;default:''" json:"qualityProfileId"`
	QualityRulesetVersion    string  `gorm:"column:quality_ruleset_version;default:''" json:"qualityRulesetVersion"`
	QualitySourceContentHash string  `gorm:"column:quality_source_content_hash;default:''" json:"qualitySourceContentHash"`
	QualityEvaluatedAt       string  `gorm:"column:quality_evaluated_at;default:''" json:"qualityEvaluatedAt"`
	CreatedByUserID      string `gorm:"column:created_by_user_id" json:"createdByUserId"`
	CreatedFromSessionID string `gorm:"column:created_from_session_id" json:"createdFromSessionId"`
	ChangeSummary        string `gorm:"column:change_summary" json:"changeSummary"`
	SourceSummaryJSON    string `gorm:"column:source_summary_json" json:"sourceSummaryJson"`
	CreatedAt            string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt            string `gorm:"column:updated_at" json:"updatedAt"`
	ReadyAt              string `gorm:"column:ready_at" json:"readyAt"`

	SourceType                 string `gorm:"column:source_type;default:''" json:"sourceType"`
	SourceProcessingRevisionID string `gorm:"column:source_processing_revision_id;default:''" json:"sourceProcessingRevisionId"`
	ContentHash                string `gorm:"column:content_hash;default:''" json:"contentHash"`
	ContentHashVersion         string `gorm:"column:content_hash_version;default:''" json:"contentHashVersion"`
	ActionConfigHash           string `gorm:"column:action_config_hash;default:''" json:"actionConfigHash"`
	FrameSetHash               string `gorm:"column:frame_set_hash;default:''" json:"frameSetHash"`
	Origin                     string `gorm:"column:origin;default:''" json:"origin"`
	PlaybackMode               string `gorm:"column:playback_mode;default:''" json:"playbackMode"`
	AnchorJSON                 string `gorm:"column:anchor_json;default:''" json:"anchorJson"`
	ArchivedAt                 string `gorm:"column:archived_at;default:''" json:"archivedAt"`
	ArchivedReason             string `gorm:"column:archived_reason;default:''" json:"archivedReason"`
}

func (ActionRevision) TableName() string { return "desktop_pet_action_revisions" }

type ActiveRevisionBinding struct {
	ProcessingTaskID     string `gorm:"column:processing_task_id;primaryKey" json:"processingTaskId"`
	ActionKey            string `gorm:"column:action_key;primaryKey" json:"actionKey"`
	RevisionID           string `gorm:"column:revision_id" json:"revisionId"`
	BindingVersion       int64  `gorm:"column:binding_version" json:"bindingVersion"`
	ActivatedBy          string `gorm:"column:activated_by" json:"activatedBy"`
	Reason               string `gorm:"column:reason" json:"reason"`
	CreatedAt            string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt            string `gorm:"column:updated_at" json:"updatedAt"`
	UserID               string `gorm:"column:user_id;default:''" json:"userId"`
	CharacterID          string `gorm:"column:character_id;default:''" json:"characterId"`
	ActiveActionRevisionID string `gorm:"column:active_action_revision_id;default:''" json:"activeActionRevisionId"`
	BindingRevision      int64  `gorm:"column:binding_revision;default:0" json:"bindingRevision"`
	BoundReason          string `gorm:"column:bound_reason;default:''" json:"boundReason"`
	BoundBy              string `gorm:"column:bound_by;default:''" json:"boundBy"`
	BoundAt              string `gorm:"column:bound_at;default:''" json:"boundAt"`
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
	ID                     string `gorm:"column:id;primaryKey" json:"id"`
	UserID                 string `gorm:"column:user_id" json:"userId"`
	ProcessingTaskID       string `gorm:"column:processing_task_id" json:"processingTaskId"`
	ActionKey              string `gorm:"column:action_key" json:"actionKey"`
	BaseRevisionID         string `gorm:"column:base_revision_id" json:"baseRevisionId"`
	BaseActionContentHash  string `gorm:"column:base_action_content_hash" json:"baseActionContentHash"`
	BaseBindingRevision    int64  `gorm:"column:base_binding_revision" json:"baseBindingRevision"`
	SessionVersion         int64  `gorm:"column:session_version" json:"sessionVersion"`
	Status                 string `gorm:"column:status" json:"status"`
	Cursor                 int    `gorm:"column:cursor" json:"cursor"`
	LastOperationSeq       int    `gorm:"column:last_operation_seq" json:"lastOperationSeq"`
	CheckpointID           string `gorm:"column:checkpoint_id" json:"checkpointId"`
	ClientInstanceID       string `gorm:"column:client_instance_id" json:"clientInstanceId"`
	ExpiresAt              string `gorm:"column:expires_at" json:"expiresAt"`
	CreatedAt              string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt              string `gorm:"column:updated_at" json:"updatedAt"`
	CommittedRevisionID    string `gorm:"column:committed_revision_id" json:"committedRevisionId"`
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
	Mode                  string `gorm:"column:mode" json:"mode"`
	Status                string `gorm:"column:status" json:"status"`
	IdempotencyKey        string `gorm:"column:idempotency_key" json:"idempotencyKey"`
	ProviderAttemptID     string `gorm:"column:provider_attempt_id" json:"providerAttemptId"`
	ActiveAttemptID       string `gorm:"column:active_attempt_id" json:"activeAttemptId"`
	ProviderReceiptID     string `gorm:"column:provider_receipt_id" json:"providerReceiptId"`
	RequestHash           string `gorm:"column:request_hash" json:"requestHash"`
	ArtifactID            string `gorm:"column:artifact_id" json:"artifactId"`
	ProcessingRevisionID  string `gorm:"column:processing_revision_id" json:"processingRevisionId"`
	CandidateRevisionID   string `gorm:"column:candidate_revision_id" json:"candidateRevisionId"`
	QualityEvaluationID   string `gorm:"column:quality_evaluation_id" json:"qualityEvaluationId"`
	RequestSnapshotJSON   string `gorm:"column:request_snapshot_json" json:"requestSnapshotJson"`
	CostEstimateJSON      string `gorm:"column:cost_estimate_json" json:"costEstimateJson"`
	CostActualJSON        string `gorm:"column:cost_actual_json" json:"costActualJson"`
	ErrorCode             string `gorm:"column:error_code" json:"errorCode"`
	ErrorMessage          string `gorm:"column:error_message" json:"errorMessage"`
	LeaseOwner            string `gorm:"column:lease_owner" json:"leaseOwner"`
	LeaseExpiresAt        string `gorm:"column:lease_expires_at" json:"leaseExpiresAt"`
	HeartbeatAt           string `gorm:"column:heartbeat_at" json:"heartbeatAt"`
	BaseActionRevisionID  string `gorm:"column:base_action_revision_id" json:"baseActionRevisionId"`
	BaseContentHash       string `gorm:"column:base_content_hash" json:"baseContentHash"`
	BaseBindingRevision   int64  `gorm:"column:base_binding_revision" json:"baseBindingRevision"`
	RejectReason          string `gorm:"column:reject_reason" json:"rejectReason"`
	RejectedBy            string `gorm:"column:rejected_by" json:"rejectedBy"`
	RejectedAt            string `gorm:"column:rejected_at" json:"rejectedAt"`
	CreatedAt             string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt             string `gorm:"column:updated_at" json:"updatedAt"`
}

func (RegenerationJob) TableName() string { return "desktop_pet_regeneration_jobs" }

type EditCandidate struct {
	ID                      string `gorm:"column:id;primaryKey" json:"id"`
	SessionID               string `gorm:"column:session_id" json:"sessionId"`
	JobID                   string `gorm:"column:job_id" json:"jobId"`
	TargetFrameID           string `gorm:"column:target_frame_id" json:"targetFrameId"`
	CandidateType           string `gorm:"column:candidate_type" json:"candidateType"`
	AssetID                 string `gorm:"column:asset_id" json:"assetId"`
	CandidateRevisionID     string `gorm:"column:candidate_revision_id" json:"candidateRevisionId"`
	Status                  string `gorm:"column:status" json:"status"`
	MetadataJSON            string `gorm:"column:metadata_json" json:"metadataJson"`
	DecidedBy               string `gorm:"column:decided_by" json:"decidedBy"`
	DecidedAt               string `gorm:"column:decided_at" json:"decidedAt"`
	SourceType              string `gorm:"column:source_type" json:"sourceType"`
	ParentRevisionID        string `gorm:"column:parent_action_revision_id" json:"parentActionRevisionId"`
	BaseBindingRevision     int64  `gorm:"column:base_binding_revision" json:"baseBindingRevision"`
	QualityStatus           string `gorm:"column:quality_status" json:"qualityStatus"`
	QualityEvaluationID     string `gorm:"column:quality_evaluation_id" json:"qualityEvaluationId"`
	ContentHash             string `gorm:"column:content_hash" json:"contentHash"`
	FrameSetHash            string `gorm:"column:frame_set_hash" json:"frameSetHash"`
	ActionConfigHash        string `gorm:"column:action_config_hash" json:"actionConfigHash"`
	AcceptedAt              string `gorm:"column:accepted_at" json:"acceptedAt"`
	RejectedAt              string `gorm:"column:rejected_at" json:"rejectedAt"`
	RejectReason            string `gorm:"column:reject_reason" json:"rejectReason"`
	CreatedAt               string `gorm:"column:created_at" json:"createdAt"`
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

type ActionStream struct {
	ID                 string `gorm:"column:id;primaryKey" json:"id"`
	UserID             string `gorm:"column:user_id" json:"userId"`
	CharacterID        string `gorm:"column:character_id" json:"characterId"`
	ActionKey          string `gorm:"column:action_key" json:"actionKey"`
	NextRevisionNumber int64  `gorm:"column:next_revision_number" json:"nextRevisionNumber"`
	CreatedAt          string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt          string `gorm:"column:updated_at" json:"updatedAt"`
}

func (ActionStream) TableName() string { return "desktop_pet_action_streams" }

type RevisionBridgeJournal struct {
	ID                   string `gorm:"column:id;primaryKey" json:"id"`
	ProcessingRevisionID string `gorm:"column:processing_revision_id" json:"processingRevisionId"`
	ProcessingActionID   string `gorm:"column:processing_action_id" json:"processingActionId"`
	ActionRevisionID     string `gorm:"column:action_revision_id" json:"actionRevisionId"`
	TargetActionKey      string `gorm:"column:target_action_key" json:"targetActionKey"`
	Status               string `gorm:"column:status" json:"status"`
	LastError            string `gorm:"column:last_error" json:"lastError"`
	RetryCount           int    `gorm:"column:retry_count" json:"retryCount"`
	CreatedAt            string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt            string `gorm:"column:updated_at" json:"updatedAt"`
}

func (RevisionBridgeJournal) TableName() string { return "desktop_pet_revision_bridge_journals" }

type ActiveActionRevisionBinding struct {
	ID                    string `gorm:"column:id;primaryKey" json:"id"`
	UserID                string `gorm:"column:user_id" json:"userId"`
	CharacterID           string `gorm:"column:character_id" json:"characterId"`
	ActionKey             string `gorm:"column:action_key" json:"actionKey"`
	ActiveActionRevisionID string `gorm:"column:active_action_revision_id" json:"activeActionRevisionId"`
	BindingRevision       int64  `gorm:"column:binding_revision" json:"bindingRevision"`
	BoundReason           string `gorm:"column:bound_reason" json:"boundReason"`
	BoundBy               string `gorm:"column:bound_by" json:"boundBy"`
	BoundAt               string `gorm:"column:bound_at" json:"boundAt"`
	CreatedAt             string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt             string `gorm:"column:updated_at" json:"updatedAt"`
}

func (ActiveActionRevisionBinding) TableName() string { return "desktop_pet_active_action_revision_bindings" }

type RegenerationJournal struct {
	ID                    string `gorm:"column:id;primaryKey" json:"id"`
	JobID                 string `gorm:"column:job_id" json:"jobId"`
	State                 string `gorm:"column:state" json:"state"`
	AttemptID             string `gorm:"column:attempt_id" json:"attemptId"`
	ProviderReceiptID     string `gorm:"column:provider_receipt_id" json:"providerReceiptId"`
	ArtifactID            string `gorm:"column:artifact_id" json:"artifactId"`
	ProcessingRevisionID  string `gorm:"column:processing_revision_id" json:"processingRevisionId"`
	CandidateRevisionID   string `gorm:"column:candidate_revision_id" json:"candidateRevisionId"`
	QualityEvaluationID   string `gorm:"column:quality_evaluation_id" json:"qualityEvaluationId"`
	ErrorMessage          string `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt             string `gorm:"column:created_at" json:"createdAt"`
}

func (RegenerationJournal) TableName() string { return "desktop_pet_regeneration_journals" }

type CandidateRevisionMetadata struct {
	ID                    string `gorm:"column:id;primaryKey" json:"id"`
	CandidateRevisionID   string `gorm:"column:candidate_revision_id" json:"candidateRevisionId"`
	SourceType            string `gorm:"column:source_type" json:"sourceType"`
	ParentRevisionID      string `gorm:"column:parent_action_revision_id" json:"parentActionRevisionId"`
	BaseBindingRevision   int64  `gorm:"column:base_binding_revision" json:"baseBindingRevision"`
	RegenerationJobID     string `gorm:"column:regeneration_job_id" json:"regenerationJobId"`
	ContentHash           string `gorm:"column:content_hash" json:"contentHash"`
	FrameSetHash          string `gorm:"column:frame_set_hash" json:"frameSetHash"`
	ActionConfigHash      string `gorm:"column:action_config_hash" json:"actionConfigHash"`
	Status                string `gorm:"column:status" json:"status"`
	CreatedAt             string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt             string `gorm:"column:updated_at" json:"updatedAt"`
}

func (CandidateRevisionMetadata) TableName() string { return "desktop_pet_candidate_revision_metadata" }

type EditAuditLog struct {
	ID                       string `gorm:"column:id;primaryKey" json:"id"`
	EventType                string `gorm:"column:event_type" json:"eventType"`
	UserID                   string `gorm:"column:user_id" json:"userId"`
	CharacterID              string `gorm:"column:character_id" json:"characterId"`
	ActionKey                string `gorm:"column:action_key" json:"actionKey"`
	EditSessionID            string `gorm:"column:edit_session_id" json:"editSessionId"`
	JobID                    string `gorm:"column:job_id" json:"jobId"`
	BaseRevisionID           string `gorm:"column:base_revision_id" json:"baseRevisionId"`
	CandidateRevisionID      string `gorm:"column:candidate_revision_id" json:"candidateRevisionId"`
	PreviousActiveRevisionID string `gorm:"column:previous_active_revision_id" json:"previousActiveRevisionId"`
	NewActiveRevisionID      string `gorm:"column:new_active_revision_id" json:"newActiveRevisionId"`
	Reason                   string `gorm:"column:reason" json:"reason"`
	OccurredAt               string `gorm:"column:occurred_at" json:"occurredAt"`
}

func (EditAuditLog) TableName() string { return "desktop_pet_edit_audit_logs" }
