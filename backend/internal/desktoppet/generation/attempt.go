package generation

type AttemptStatus string

const (
	AttemptStatusPending            AttemptStatus = "pending"
	AttemptStatusPreparingReference AttemptStatus = "preparing_reference"
	AttemptStatusBuildingPrompt     AttemptStatus = "building_prompt"
	AttemptStatusWaitingRateLimit   AttemptStatus = "waiting_rate_limit"
	AttemptStatusSubmitting         AttemptStatus = "submitting"
	AttemptStatusPolling            AttemptStatus = "polling"
	AttemptStatusResultReceived     AttemptStatus = "result_received"
	AttemptStatusPersisting         AttemptStatus = "persisting"
	AttemptStatusSucceeded          AttemptStatus = "succeeded"
	AttemptStatusFailed             AttemptStatus = "failed"
	AttemptStatusUnknownSubmission  AttemptStatus = "unknown_submission"
)

type ActionGenerationAttempt struct {
	ID                     string `gorm:"column:id;primaryKey;type:text" json:"id"`
	TaskID                 string `gorm:"column:task_id;type:text" json:"taskId"`
	TaskActionID           string `gorm:"column:task_action_id;type:text" json:"taskActionId"`
	AttemptNumber          int    `gorm:"column:attempt_number;type:integer" json:"attemptNumber"`
	ParentAttemptID        string `gorm:"column:parent_attempt_id;type:text" json:"parentAttemptId"`
	Mode                   string `gorm:"column:mode;type:text" json:"mode"`
	Reason                 string `gorm:"column:reason;type:text" json:"reason"`
	Status                 string `gorm:"column:status;type:text" json:"status"`
	Provider               string `gorm:"column:provider;type:text" json:"provider"`
	Model                  string `gorm:"column:model;type:text" json:"model"`
	ConfigID               int    `gorm:"column:config_id;type:integer" json:"configId"`
	ConfigRevision         string `gorm:"column:config_revision;type:text" json:"configRevision"`
	CapabilityHash         string `gorm:"column:capability_hash;type:text" json:"capabilityHash"`
	ReferenceAssetID       string `gorm:"column:reference_asset_id;type:text" json:"referenceAssetId"`
	PlanJSON               string `gorm:"column:plan_json;type:text" json:"planJson"`
	PromptDocumentJSON     string `gorm:"column:prompt_document_json;type:text" json:"promptDocumentJson"`
	PromptSnapshot         string `gorm:"column:prompt_snapshot;type:text" json:"promptSnapshot"`
	PromptHash             string `gorm:"column:prompt_hash;type:text" json:"promptHash"`
	NegativePromptSnapshot string `gorm:"column:negative_prompt_snapshot;type:text" json:"negativePromptSnapshot"`
	SeedPolicy             string `gorm:"column:seed_policy;type:text" json:"seedPolicy"`
	SeedValue              *int64 `gorm:"column:seed_value;type:integer" json:"seedValue"`
	OutputCount            int    `gorm:"column:output_count;type:integer" json:"outputCount"`
	ProviderRequestID      string `gorm:"column:provider_request_id;type:text" json:"providerRequestId"`
	ProviderOperationID    string `gorm:"column:provider_operation_id;type:text" json:"providerOperationId"`
	ExecutionID            string `gorm:"column:execution_id;type:text" json:"executionId"`
	WorkerID               string `gorm:"column:worker_id;type:text" json:"workerId"`
	Lease                  string `gorm:"column:lease;type:text" json:"lease"`
	SubmittedAt            string `gorm:"column:submitted_at;type:text" json:"submittedAt"`
	CompletedAt            string `gorm:"column:completed_at;type:text" json:"completedAt"`
	ErrorCode              string `gorm:"column:error_code;type:text" json:"errorCode"`
	ErrorMessage           string `gorm:"column:error_message;type:text" json:"errorMessage"`
	CreatedAt              string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt              string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (ActionGenerationAttempt) TableName() string {
	return "desktop_pet_action_generation_attempts"
}

func (a *ActionGenerationAttempt) IsActive() bool {
	switch AttemptStatus(a.Status) {
	case AttemptStatusPending,
		AttemptStatusPreparingReference,
		AttemptStatusBuildingPrompt,
		AttemptStatusWaitingRateLimit,
		AttemptStatusSubmitting,
		AttemptStatusPolling,
		AttemptStatusResultReceived,
		AttemptStatusPersisting,
		AttemptStatusUnknownSubmission:
		return true
	default:
		return false
	}
}

func (a *ActionGenerationAttempt) IsTerminal() bool {
	return AttemptStatus(a.Status) == AttemptStatusSucceeded || AttemptStatus(a.Status) == AttemptStatusFailed
}

func (a *ActionGenerationAttempt) HasSubmitted() bool {
	switch AttemptStatus(a.Status) {
	case AttemptStatusSubmitting,
		AttemptStatusPolling,
		AttemptStatusResultReceived,
		AttemptStatusPersisting,
		AttemptStatusSucceeded,
		AttemptStatusUnknownSubmission:
		return true
	default:
		return false
	}
}

func (a *ActionGenerationAttempt) HasResultReceived() bool {
	switch AttemptStatus(a.Status) {
	case AttemptStatusResultReceived,
		AttemptStatusPersisting,
		AttemptStatusSucceeded:
		return true
	default:
		return false
	}
}

func NewAttempt() *ActionGenerationAttempt {
	return &ActionGenerationAttempt{
		ID:        generateUUID(),
		Status:    string(AttemptStatusPending),
		CreatedAt: nowRFC3339(),
		UpdatedAt: nowRFC3339(),
	}
}
