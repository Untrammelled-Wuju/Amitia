// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

type ActionDefinition struct {
	ID                       int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ActionKey                string `gorm:"column:action_key;uniqueIndex" json:"actionKey"`
	Name                     string `gorm:"column:name" json:"name"`
	Description              string `gorm:"column:description" json:"description"`
	CategoryKey              string `gorm:"column:category_key" json:"categoryKey"`
	CategoryName             string `gorm:"column:category_name" json:"categoryName"`
	SupportsDefaultIdle      int    `gorm:"column:supports_default_idle" json:"supportsDefaultIdle"`
	Recommended              int    `gorm:"column:recommended" json:"recommended"`
	Enabled                  int    `gorm:"column:enabled" json:"enabled"`
	SortOrder                int    `gorm:"column:sort_order" json:"sortOrder"`
	DefinitionVersion        int    `gorm:"column:definition_version" json:"definitionVersion"`
	DefaultFrameCount        int    `gorm:"column:default_frame_count" json:"defaultFrameCount"`
	EstimatedGenerationCount int    `gorm:"column:estimated_generation_count" json:"estimatedGenerationCount"`
	SourceType               string `gorm:"column:source_type" json:"sourceType"`
	SchemaVersion            int    `gorm:"column:schema_version" json:"schemaVersion"`
	CatalogVersion           int    `gorm:"column:catalog_version" json:"catalogVersion"`
	DefaultFPS               int    `gorm:"column:default_fps" json:"defaultFps"`
	PlaybackMode             string `gorm:"column:playback_mode" json:"playbackMode"`
	ReturnPolicy             string `gorm:"column:return_policy" json:"returnPolicy"`
	ReturnActionKey          string `gorm:"column:return_action_key" json:"returnActionKey"`
	Interruptible            int    `gorm:"column:interruptible" json:"interruptible"`
	InterruptAfterMS         int    `gorm:"column:interrupt_after_ms" json:"interruptAfterMs"`
	MinimumPlayMS            int    `gorm:"column:minimum_play_ms" json:"minimumPlayMs"`
	MaximumPlayMS            int    `gorm:"column:maximum_play_ms" json:"maximumPlayMs"`
	Priority                 int    `gorm:"column:priority" json:"priority"`
	CooldownMS               int    `gorm:"column:cooldown_ms" json:"cooldownMs"`
	MutexGroup               string `gorm:"column:mutex_group" json:"mutexGroup"`
	QueuePolicy              string `gorm:"column:queue_policy" json:"queuePolicy"`
	DedupWindowMS            int    `gorm:"column:dedup_window_ms" json:"dedupWindowMs"`
	AnchorProfile            string `gorm:"column:anchor_profile" json:"anchorProfile"`
	SemanticTagsJSON         string `gorm:"column:semantic_tags_json" json:"semanticTagsJson"`
	GenerationSpecVersion    int    `gorm:"column:generation_spec_version" json:"generationSpecVersion"`
	SpecJSON                 string `gorm:"column:spec_json" json:"specJson"`
	SpecHash                 string `gorm:"column:spec_hash" json:"specHash"`
	CreatedAt                string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt                string `gorm:"column:updated_at" json:"updatedAt"`
}

func (ActionDefinition) TableName() string { return "desktop_pet_action_definitions" }

type GenerationTask struct {
	ID                       string `gorm:"column:id;primaryKey" json:"id"`
	UserID                   string `gorm:"column:user_id" json:"userId"`
	CharacterID              string `gorm:"column:character_id" json:"characterId"`
	ModelConfigID            int    `gorm:"column:model_config_id" json:"modelConfigId"`
	Name                     string `gorm:"column:name" json:"name"`
	SourceImagePath          string `gorm:"column:source_image_path" json:"sourceImagePath"`
	SourceImageOriginalName  string `gorm:"column:source_image_original_name" json:"sourceImageOriginalName"`
	SourceImageMimeType      string `gorm:"column:source_image_mime_type" json:"sourceImageMimeType"`
	SourceImageSize          int    `gorm:"column:source_image_size" json:"sourceImageSize"`
	SourceImageWidth         int    `gorm:"column:source_image_width" json:"sourceImageWidth"`
	SourceImageHeight        int    `gorm:"column:source_image_height" json:"sourceImageHeight"`
	SourceImageHash          string `gorm:"column:source_image_hash" json:"sourceImageHash"`
	Prompt                   string `gorm:"column:prompt" json:"prompt"`
	NegativePrompt           string `gorm:"column:negative_prompt" json:"negativePrompt"`
	OutputWidth              int    `gorm:"column:output_width" json:"outputWidth"`
	OutputHeight             int    `gorm:"column:output_height" json:"outputHeight"`
	Status                   string `gorm:"column:status" json:"status"`
	CurrentStage             string `gorm:"column:current_stage" json:"currentStage"`
	Progress                 int    `gorm:"column:progress" json:"progress"`
	SelectedActionCount      int    `gorm:"column:selected_action_count" json:"selectedActionCount"`
	EstimatedGenerationCount int    `gorm:"column:estimated_generation_count" json:"estimatedGenerationCount"`
	ErrorCode                string `gorm:"column:error_code" json:"errorCode"`
	ErrorMessage             string `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt                string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt                string `gorm:"column:updated_at" json:"updatedAt"`
	StartedAt                string `gorm:"column:started_at" json:"startedAt"`
	CompletedAt              string `gorm:"column:completed_at" json:"completedAt"`
	ExecutionID              string `gorm:"column:execution_id" json:"executionId"`
	WorkerID                 string `gorm:"column:worker_id" json:"workerId"`
	LeaseExpiresAt           string `gorm:"column:lease_expires_at" json:"leaseExpiresAt"`
	LastHeartbeatAt          string `gorm:"column:last_heartbeat_at" json:"lastHeartbeatAt"`
	AttemptCount             int    `gorm:"column:attempt_count" json:"attemptCount"`
	CancelRequestedAt        string `gorm:"column:cancel_requested_at" json:"cancelRequestedAt"`
	RowVersion               int64  `gorm:"column:row_version;default:0" json:"rowVersion"`
	StatusReason             string `gorm:"column:status_reason;default:''" json:"statusReason"`
	FailureStage             string `gorm:"column:failure_stage;default:''" json:"failureStage"`
	LastTransitionAt         string `gorm:"column:last_transition_at;default:''" json:"lastTransitionAt"`
	SubmittedAt              string `gorm:"column:submitted_at;default:''" json:"submittedAt"`
	CancellingAt             string `gorm:"column:cancelling_at;default:''" json:"cancellingAt"`
	CancelledAt              string `gorm:"column:cancelled_at;default:''" json:"cancelledAt"`
	ReferenceAssetID         string `gorm:"column:reference_asset_id;default:''" json:"referenceAssetId"`
	GenerationPlanVersion    int    `gorm:"column:generation_plan_version;default:0" json:"generationPlanVersion"`
	ProviderKeySnapshot      string `gorm:"column:provider_key_snapshot;default:''" json:"providerKeySnapshot"`
	ModelNameSnapshot        string `gorm:"column:model_name_snapshot;default:''" json:"modelNameSnapshot"`
	ConfigRevisionSnapshot   string `gorm:"column:config_revision_snapshot;default:''" json:"configRevisionSnapshot"`
	CapabilitySnapshotJSON   string `gorm:"column:capability_snapshot_json;default:'{}'" json:"capabilitySnapshotJson"`
	CapabilitySnapshotHash   string `gorm:"column:capability_snapshot_hash;default:''" json:"capabilitySnapshotHash"`
	CostEstimateJSON         string `gorm:"column:cost_estimate_json;default:'{}'" json:"costEstimateJson"`
	PlannedPrimaryRequestCount  int `gorm:"column:planned_primary_request_count;default:0" json:"plannedPrimaryRequestCount"`
	PlannedMaxProviderCallCount int `gorm:"column:planned_max_provider_call_count;default:0" json:"plannedMaxProviderCallCount"`
	ActualProviderCallCount  int    `gorm:"column:actual_provider_call_count;default:0" json:"actualProviderCallCount"`
	ActualOutputImageCount   int    `gorm:"column:actual_output_image_count;default:0" json:"actualOutputImageCount"`
}

func (GenerationTask) TableName() string { return "desktop_pet_generation_tasks" }

type GenerationTaskAction struct {
	ID                       string `gorm:"column:id;primaryKey" json:"id"`
	TaskID                   string `gorm:"column:task_id" json:"taskId"`
	ActionDefinitionID       int    `gorm:"column:action_definition_id" json:"actionDefinitionId"`
	ActionKey                string `gorm:"column:action_key" json:"actionKey"`
	ActionNameSnapshot       string `gorm:"column:action_name_snapshot" json:"actionNameSnapshot"`
	ActionDescriptionSnapshot string `gorm:"column:action_description_snapshot" json:"actionDescriptionSnapshot"`
	CategoryKeySnapshot      string `gorm:"column:category_key_snapshot" json:"categoryKeySnapshot"`
	CategoryNameSnapshot     string `gorm:"column:category_name_snapshot" json:"categoryNameSnapshot"`
	DefinitionVersion        int    `gorm:"column:definition_version" json:"definitionVersion"`
	SupportsDefaultIdle      int    `gorm:"column:supports_default_idle" json:"supportsDefaultIdle"`
	SortOrder                int    `gorm:"column:sort_order" json:"sortOrder"`
	FrameCount               int    `gorm:"column:frame_count" json:"frameCount"`
	EstimatedGenerationCount int    `gorm:"column:estimated_generation_count" json:"estimatedGenerationCount"`
	Status                   string `gorm:"column:status" json:"status"`
	Progress                 int    `gorm:"column:progress" json:"progress"`
	ErrorCode                string `gorm:"column:error_code" json:"errorCode"`
	ErrorMessage             string `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt                string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt                string `gorm:"column:updated_at" json:"updatedAt"`
	StartedAt                string `gorm:"column:started_at" json:"startedAt"`
	CompletedAt              string `gorm:"column:completed_at" json:"completedAt"`
	AttemptNumber            int    `gorm:"column:attempt_number" json:"attemptNumber"`
	GenerationSpecVersion    string `gorm:"column:generation_spec_version" json:"generationSpecVersion"`
	CurrentAttempt           int    `gorm:"column:current_attempt" json:"currentAttempt"`
	RowVersion               int64  `gorm:"column:row_version;default:0" json:"rowVersion"`
	CurrentStage             string `gorm:"column:current_stage;default:'created'" json:"currentStage"`
	StatusReason             string `gorm:"column:status_reason;default:''" json:"statusReason"`
	FailureStage             string `gorm:"column:failure_stage;default:''" json:"failureStage"`
	LastTransitionAt         string `gorm:"column:last_transition_at;default:''" json:"lastTransitionAt"`
	ExecutionID              string `gorm:"column:execution_id;default:''" json:"executionId"`
	WorkerID                 string `gorm:"column:worker_id;default:''" json:"workerId"`
	ActionSpecSchemaVersion  int    `gorm:"column:action_spec_schema_version" json:"actionSpecSchemaVersion"`
	ActionSpecVersion        int    `gorm:"column:action_spec_version" json:"actionSpecVersion"`
	ActionSpecJSON           string `gorm:"column:action_spec_json" json:"actionSpecJson"`
	ActionSpecHash           string `gorm:"column:action_spec_hash" json:"actionSpecHash"`
	PlaybackModeSnapshot     string `gorm:"column:playback_mode_snapshot" json:"playbackModeSnapshot"`
	DefaultFPSSnapshot       int    `gorm:"column:default_fps_snapshot" json:"defaultFpsSnapshot"`
	ReturnPolicySnapshot     string `gorm:"column:return_policy_snapshot" json:"returnPolicySnapshot"`
	ReturnActionKeySnapshot  string `gorm:"column:return_action_key_snapshot" json:"returnActionKeySnapshot"`
	InterruptibleSnapshot    int    `gorm:"column:interruptible_snapshot" json:"interruptibleSnapshot"`
	PrioritySnapshot         int    `gorm:"column:priority_snapshot" json:"prioritySnapshot"`
	CooldownMSSnapshot       int    `gorm:"column:cooldown_ms_snapshot" json:"cooldownMsSnapshot"`
	MutexGroupSnapshot       string `gorm:"column:mutex_group_snapshot" json:"mutexGroupSnapshot"`
	AnchorProfileSnapshot    string `gorm:"column:anchor_profile_snapshot" json:"anchorProfileSnapshot"`
	GenerationMode           string `gorm:"column:generation_mode;default:'legacy_frame'" json:"generationMode"`
	GenerationPlanJSON       string `gorm:"column:generation_plan_json;default:'{}'" json:"generationPlanJson"`
	GenerationPlanHash       string `gorm:"column:generation_plan_hash;default:''" json:"generationPlanHash"`
	PromptTemplateVersion    string `gorm:"column:prompt_template_version;default:''" json:"promptTemplateVersion"`
	ActiveAttemptID          string `gorm:"column:active_attempt_id;default:''" json:"activeAttemptId"`
	ActiveAttemptNumber      int    `gorm:"column:active_attempt_number;default:0" json:"activeAttemptNumber"`
	PlannedSegmentCount      int    `gorm:"column:planned_segment_count;default:0" json:"plannedSegmentCount"`
	PlannedPrimaryRequestCount  int `gorm:"column:planned_primary_request_count;default:0" json:"plannedPrimaryRequestCount"`
	PlannedMaxProviderCallCount int `gorm:"column:planned_max_provider_call_count;default:0" json:"plannedMaxProviderCallCount"`
}

func (GenerationTaskAction) TableName() string { return "desktop_pet_generation_task_actions" }

type GenerationFrame struct {
	ID                     string `gorm:"column:id;primaryKey" json:"id"`
	TaskID                 string `gorm:"column:task_id" json:"taskId"`
	TaskActionID           string `gorm:"column:task_action_id" json:"taskActionId"`
	ExecutionID            string `gorm:"column:execution_id" json:"executionId"`
	FrameIndex             int    `gorm:"column:frame_index" json:"frameIndex"`
	FramePhase             string `gorm:"column:frame_phase" json:"framePhase"`
	Status                 string `gorm:"column:status" json:"status"`
	AttemptNumber          int    `gorm:"column:attempt_number" json:"attemptNumber"`
	GenerationAttempt      int    `gorm:"column:generation_attempt" json:"generationAttempt"`
	ProviderAttempt        int    `gorm:"column:provider_attempt" json:"providerAttempt"`
	PromptSnapshot         string `gorm:"column:prompt_snapshot" json:"promptSnapshot"`
	NegativePromptSnapshot string `gorm:"column:negative_prompt_snapshot" json:"negativePromptSnapshot"`
	Provider               string `gorm:"column:provider" json:"provider"`
	Model                  string `gorm:"column:model" json:"model"`
	ProviderRequestID      string `gorm:"column:provider_request_id" json:"providerRequestId"`
	ProviderOperationID    string `gorm:"column:provider_operation_id" json:"providerOperationId"`
	SourceImagePath        string `gorm:"column:source_image_path" json:"sourceImagePath"`
	PreviousFramePath      string `gorm:"column:previous_frame_path" json:"previousFramePath"`
	ResultImagePath        string `gorm:"column:result_image_path" json:"resultImagePath"`
	ResultMimeType         string `gorm:"column:result_mime_type" json:"resultMimeType"`
	ResultWidth            int    `gorm:"column:result_width" json:"resultWidth"`
	ResultHeight           int    `gorm:"column:result_height" json:"resultHeight"`
	ResultSize             int    `gorm:"column:result_size" json:"resultSize"`
	ResultHash             string `gorm:"column:result_hash" json:"resultHash"`
	ProviderSeed           string `gorm:"column:provider_seed" json:"providerSeed"`
	ErrorCode              string `gorm:"column:error_code" json:"errorCode"`
	ErrorMessage           string `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt              string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt              string `gorm:"column:updated_at" json:"updatedAt"`
	StartedAt              string `gorm:"column:started_at" json:"startedAt"`
	CompletedAt            string `gorm:"column:completed_at" json:"completedAt"`
	RowVersion             int64  `gorm:"column:row_version;default:0" json:"rowVersion"`
	CurrentStage           string `gorm:"column:current_stage;default:'created'" json:"currentStage"`
	StatusReason           string `gorm:"column:status_reason;default:''" json:"statusReason"`
	FailureStage           string `gorm:"column:failure_stage;default:''" json:"failureStage"`
	LastTransitionAt       string `gorm:"column:last_transition_at;default:''" json:"lastTransitionAt"`
}

func (GenerationFrame) TableName() string { return "desktop_pet_generation_frames" }

type GenerationCallLog struct {
	ID                 string `gorm:"column:id;primaryKey" json:"id"`
	TaskID             string `gorm:"column:task_id" json:"taskId"`
	TaskActionID       string `gorm:"column:task_action_id" json:"taskActionId"`
	FrameID            string `gorm:"column:frame_id" json:"frameId"`
	ExecutionID        string `gorm:"column:execution_id" json:"executionId"`
	Provider           string `gorm:"column:provider" json:"provider"`
	Model              string `gorm:"column:model" json:"model"`
	ProviderRequestID  string `gorm:"column:provider_request_id" json:"providerRequestId"`
	RequestStartedAt   string `gorm:"column:request_started_at" json:"requestStartedAt"`
	RequestCompletedAt string `gorm:"column:request_completed_at" json:"requestCompletedAt"`
	RequestStatus      string `gorm:"column:request_status" json:"requestStatus"`
	AttemptNumber      int    `gorm:"column:attempt_number" json:"attemptNumber"`
	Usage              string `gorm:"column:usage" json:"usage"`
	ErrorCode          string `gorm:"column:error_code" json:"errorCode"`
	ErrorMessage       string `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt          string `gorm:"column:created_at" json:"createdAt"`
	AttemptID          string `gorm:"column:attempt_id;default:''" json:"attemptId"`
	ArtifactID         string `gorm:"column:artifact_id;default:''" json:"artifactId"`
	CallType           string `gorm:"column:call_type;default:'primary'" json:"callType"`
	CallAttemptIndex   int    `gorm:"column:call_attempt_index;default:0" json:"callAttemptIndex"`
	IdempotencyKeyHash string `gorm:"column:idempotency_key_hash;default:''" json:"idempotencyKeyHash"`
	RequestHash        string `gorm:"column:request_hash;default:''" json:"requestHash"`
	SubmissionState    string `gorm:"column:submission_state;default:''" json:"submissionState"`
	RetryClass         string `gorm:"column:retry_class;default:''" json:"retryClass"`
	HTTPStatus         int    `gorm:"column:http_status;default:0" json:"httpStatus"`
	UsageJSON          string `gorm:"column:usage_json;default:'{}'" json:"usageJson"`
	EstimatedCostJSON  string `gorm:"column:estimated_cost_json;default:'{}'" json:"estimatedCostJson"`
	ActualCostJSON     string `gorm:"column:actual_cost_json;default:'{}'" json:"actualCostJson"`
	ResponseReceiptJSON string `gorm:"column:response_receipt_json;default:'{}'" json:"responseReceiptJson"`
}

func (GenerationCallLog) TableName() string { return "desktop_pet_generation_call_logs" }

type ActionItemResponse struct {
	ID                       int      `json:"id"`
	Key                      string   `json:"key"`
	Name                     string   `json:"name"`
	Description              string   `json:"description"`
	SupportsDefaultIdle      bool     `json:"supportsDefaultIdle"`
	Recommended              bool     `json:"recommended"`
	DefaultFrameCount        int      `json:"defaultFrameCount"`
	EstimatedGenerationCount int      `json:"estimatedGenerationCount"`
	DefinitionVersion        int      `json:"definitionVersion"`
	PlaybackMode             string   `json:"playbackMode"`
	DefaultFPS               int      `json:"defaultFps"`
	ReturnPolicy             string   `json:"returnPolicy"`
	ReturnActionKey          string   `json:"returnActionKey"`
	Interruptible            bool     `json:"interruptible"`
	Priority                 int      `json:"priority"`
	CooldownMs               int      `json:"cooldownMs"`
	MutexGroup               string   `json:"mutexGroup"`
	QueuePolicy              string   `json:"queuePolicy"`
	AnchorProfile            string   `json:"anchorProfile"`
	Tags                     []string `json:"tags"`
}

type ActionCategoryResponse struct {
	Key       string              `json:"key"`
	Name      string              `json:"name"`
	SortOrder int                 `json:"sortOrder"`
	Actions   []ActionItemResponse `json:"actions"`
}

type PresetResponse struct {
	Key        string   `json:"key"`
	Name       string   `json:"name"`
	Description string  `json:"description"`
	ActionKeys []string `json:"actionKeys"`
}

type ActionDefinitionsResponse struct {
	Categories []ActionCategoryResponse `json:"categories"`
	Presets    []PresetResponse         `json:"presets"`
}

type TaskSummaryResponse struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	CharacterID              string `json:"characterId"`
	ModelConfigID            int    `json:"modelConfigId"`
	Status                   string `json:"status"`
	CurrentStage             string `json:"currentStage"`
	Progress                 int    `json:"progress"`
	SelectedActionCount      int    `json:"selectedActionCount"`
	EstimatedGenerationCount int    `json:"estimatedGenerationCount"`
	CreatedAt                string `json:"createdAt"`
	GenerationPlanVersion    int    `json:"generationPlanVersion"`
	ProviderKey              string `json:"providerKey"`
	RowVersion               int64  `json:"rowVersion"`
	StatusReason             string `json:"statusReason"`
	FailureStage             string `json:"failureStage"`
	LastTransitionAt         string `json:"lastTransitionAt"`
	SubmittedAt              string `json:"submittedAt"`
	CancellingAt             string `json:"cancellingAt"`
	CancelledAt              string `json:"cancelledAt"`
}

type TaskActionResponse struct {
	ID                       string `json:"id"`
	ActionKey                string `json:"actionKey"`
	ActionName               string `json:"actionName"`
	ActionDescription        string `json:"actionDescription"`
	CategoryKey              string `json:"categoryKey"`
	CategoryName             string `json:"categoryName"`
	DefinitionVersion        int    `json:"definitionVersion"`
	SupportsDefaultIdle      bool   `json:"supportsDefaultIdle"`
	SortOrder                int    `json:"sortOrder"`
	FrameCount               int    `json:"frameCount"`
	EstimatedGenerationCount int    `json:"estimatedGenerationCount"`
	Status                   string `json:"status"`
	Progress                 int    `json:"progress"`
	ErrorCode                string `json:"errorCode"`
	ErrorMessage             string `json:"errorMessage"`
	AttemptNumber            int    `json:"attemptNumber"`
	StartedAt                string `json:"startedAt"`
	CompletedAt              string `json:"completedAt"`
	FrameSucceeded           int    `json:"frameSucceeded"`
	FrameFailed              int    `json:"frameFailed"`
	FrameTotal               int    `json:"frameTotal"`
	PlaybackModeSnapshot     string `json:"playbackModeSnapshot"`
	DefaultFPSSnapshot       int    `json:"defaultFpsSnapshot"`
	ReturnPolicySnapshot     string `json:"returnPolicySnapshot"`
	ReturnActionKeySnapshot  string `json:"returnActionKeySnapshot"`
	InterruptibleSnapshot    bool   `json:"interruptibleSnapshot"`
	PrioritySnapshot         int    `json:"prioritySnapshot"`
	CooldownMsSnapshot       int    `json:"cooldownMsSnapshot"`
	MutexGroupSnapshot       string `json:"mutexGroupSnapshot"`
	AnchorProfileSnapshot    string `json:"anchorProfileSnapshot"`
	ActionSpecHash           string `json:"actionSpecHash"`
	GenerationMode           string `json:"generationMode"`
	ActiveAttemptID          string `json:"activeAttemptId"`
	ActiveAttemptNumber      int    `json:"activeAttemptNumber"`
	PlannedSegmentCount      int    `json:"plannedSegmentCount"`
	RowVersion               int64  `json:"rowVersion"`
	CurrentStage             string `json:"currentStage"`
	StatusReason             string `json:"statusReason"`
	FailureStage             string `json:"failureStage"`
	LastTransitionAt         string `json:"lastTransitionAt"`
}

type TaskDetailResponse struct {
	ID                       string              `json:"id"`
	Name                     string              `json:"name"`
	CharacterID              string              `json:"characterId"`
	CharacterName            string              `json:"characterName"`
	ModelConfigID            int                 `json:"modelConfigId"`
	ModelName                string              `json:"modelName"`
	Status                   string              `json:"status"`
	CurrentStage             string              `json:"currentStage"`
	Progress                 int                 `json:"progress"`
	SelectedActionCount      int                 `json:"selectedActionCount"`
	EstimatedGenerationCount int                 `json:"estimatedGenerationCount"`
	ReferenceImageUrl        string              `json:"referenceImageUrl"`
	ErrorMessage             string              `json:"errorMessage"`
	CreatedAt                string              `json:"createdAt"`
	UpdatedAt                string              `json:"updatedAt"`
	StartedAt                string              `json:"startedAt"`
	CompletedAt              string              `json:"completedAt"`
	Actions                  []TaskActionResponse `json:"actions"`
	SucceededActionCount     int                 `json:"succeededActionCount"`
	FailedActionCount        int                 `json:"failedActionCount"`
	CurrentAction            string              `json:"currentAction"`
	DurationSeconds          int64               `json:"durationSeconds"`
	GenerationPlanVersion    int                 `json:"generationPlanVersion"`
	ProviderKey              string              `json:"providerKey"`
	ModelNameSnapshot        string              `json:"modelNameSnapshot"`
	CostEstimateJSON         string              `json:"costEstimateJson"`
	PlannedPrimaryRequestCount  int             `json:"plannedPrimaryRequestCount"`
	PlannedMaxProviderCallCount int             `json:"plannedMaxProviderCallCount"`
	ActualProviderCallCount  int                 `json:"actualProviderCallCount"`
	RowVersion               int64               `json:"rowVersion"`
	StatusReason             string              `json:"statusReason"`
	FailureStage             string              `json:"failureStage"`
	LastTransitionAt         string              `json:"lastTransitionAt"`
	SubmittedAt              string              `json:"submittedAt"`
	CancellingAt             string              `json:"cancellingAt"`
	CancelledAt              string              `json:"cancelledAt"`
}

type TaskListItemResponse struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	CharacterID              string `json:"characterId"`
	CharacterName            string `json:"characterName"`
	ModelConfigID            int    `json:"modelConfigId"`
	ModelName                string `json:"modelName"`
	Status                   string `json:"status"`
	CurrentStage             string `json:"currentStage"`
	Progress                 int    `json:"progress"`
	SelectedActionCount      int    `json:"selectedActionCount"`
	EstimatedGenerationCount int    `json:"estimatedGenerationCount"`
	CreatedAt                string `json:"createdAt"`
	StatusReason             string `json:"statusReason"`
	FailureStage             string `json:"failureStage"`
	LastTransitionAt         string `json:"lastTransitionAt"`
}

type TaskListResponse struct {
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
	Items    []TaskListItemResponse `json:"items"`
}
