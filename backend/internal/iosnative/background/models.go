package background

import "time"

type BackgroundState string

const (
	BackgroundStateReady       BackgroundState = "ready"
	BackgroundStateBusy        BackgroundState = "busy"
	BackgroundStateDisabled    BackgroundState = "disabled"
	BackgroundStateMaintenance BackgroundState = "maintenance"
	BackgroundStateFrozen      BackgroundState = "frozen"
)

type BackgroundRefreshStatus string

const (
	BackgroundRefreshEnabled  BackgroundRefreshStatus = "enabled"
	BackgroundRefreshLimited  BackgroundRefreshStatus = "limited"
	BackgroundRefreshDisabled BackgroundRefreshStatus = "disabled"
	BackgroundRefreshRestricted BackgroundRefreshStatus = "restricted"
)

type RuntimePersistence string

const (
	RuntimePersistenceSupported    RuntimePersistence = "supported"
	RuntimePersistenceLimited      RuntimePersistence = "limited"
	RuntimePersistenceNotSupported RuntimePersistence = "not_supported"
)

type BackgroundSystemClass string

const (
	BackgroundClassRefresh    BackgroundSystemClass = "app_refresh"
	BackgroundClassProcessing BackgroundSystemClass = "processing"
	BackgroundClassContinued  BackgroundSystemClass = "continued_processing"
	BackgroundClassCleanup    BackgroundSystemClass = "foreground_cleanup"
)

type ContinuedTaskStrategy string

const (
	ContinuedStrategyQueueIfNeeded     ContinuedTaskStrategy = "queue_if_needed"
	ContinuedStrategyFailIfNotImmediate ContinuedTaskStrategy = "fail_if_not_immediate"
)

type TaskInitiator string

const (
	InitiatorUser              TaskInitiator = "user"
	InitiatorForegroundShortcut TaskInitiator = "foreground_shortcut"
	InitiatorExplicitAppIntent TaskInitiator = "explicit_app_intent_with_foreground_user_action"
	InitiatorScheduler         TaskInitiator = "scheduler"
	InitiatorProactive         TaskInitiator = "proactive"
	InitiatorBackgroundEvent   TaskInitiator = "background_event"
	InitiatorSilentPush        TaskInitiator = "silent_push"
)

type ContinuedResourceType string

const (
	ContinuedResourceGPU ContinuedResourceType = "gpu"
)

type BackgroundStatus struct {
	Supported             bool                    `json:"supported"`
	AppRefreshSupported   bool                    `json:"appRefreshSupported"`
	ProcessingSupported   bool                    `json:"processingSupported"`
	ContinuedSupported    bool                    `json:"continuedSupported"`
	PendingRequests       int                     `json:"pendingRequests"`
	BackgroundRefreshEnabled BackgroundRefreshStatus `json:"backgroundRefreshEnabled"`
	RuntimePersistence    RuntimePersistence      `json:"runtimePersistence"`
	State                 BackgroundState         `json:"state"`
}

type BackgroundTaskRegistration struct {
	SystemClass    BackgroundSystemClass `json:"systemClass"`
	Identifier     string                `json:"identifier"`
	Success        bool                  `json:"success"`
	RegisteredAt   string                `json:"registeredAt"`
	ErrorCode      string                `json:"errorCode,omitempty"`
	ErrorMessage   string                `json:"errorMessage,omitempty"`
}

type BackgroundRegistrationRequest struct {
	SystemClass BackgroundSystemClass `json:"systemClass"`
	Identifier  string                `json:"identifier"`
}

type BackgroundSubmissionRequest struct {
	SystemClass       BackgroundSystemClass    `json:"systemClass"`
	TaskRunID         string                   `json:"taskRunId,omitempty"`
	TaskDefinitionID  string                   `json:"taskDefinitionId,omitempty"`
	IdentifierClass   string                   `json:"identifierClass"`
	EarliestBeginAt   *time.Time               `json:"earliestBeginAt,omitempty"`
	Reason            string                   `json:"reason,omitempty"`
	Strategy          ContinuedTaskStrategy    `json:"strategy,omitempty"`
	Initiator         TaskInitiator            `json:"initiator,omitempty"`
	NetworkRequired   bool                     `json:"networkRequired"`
	ExternalPowerRequired bool                 `json:"externalPowerRequired"`
	GPURequired       bool                     `json:"gpuRequired"`
	Title             string                   `json:"title,omitempty"`
	Subtitle          string                   `json:"subtitle,omitempty"`
}

type BackgroundSubmissionResult struct {
	Submitted   bool   `json:"submitted"`
	RequestID   string `json:"requestId,omitempty"`
	SystemClass string `json:"systemClass"`
	Error       string `json:"error,omitempty"`
	ErrorCode   string `json:"errorCode,omitempty"`
}

type BackgroundCancelRequest struct {
	SystemClass BackgroundSystemClass `json:"systemClass"`
	RequestID   string                `json:"requestId,omitempty"`
}

type BackgroundCancelAllRequest struct {
	SystemClass BackgroundSystemClass `json:"systemClass"`
}

type BackgroundTaskProgress struct {
	TaskRunID       string  `json:"taskRunId"`
	IdentifierClass string  `json:"identifierClass"`
	TotalUnits      int64   `json:"totalUnits"`
	CompletedUnits  int64   `json:"completedUnits"`
	Phase           string  `json:"phase"`
}

type BackgroundTaskExpireRequest struct {
	SystemClass    BackgroundSystemClass `json:"systemClass"`
	TaskRunID      string                `json:"taskRunId"`
	IdentifierClass string               `json:"identifierClass"`
}

type BackgroundTaskCompleteRequest struct {
	SystemClass    BackgroundSystemClass `json:"systemClass"`
	TaskRunID      string                `json:"taskRunId"`
	IdentifierClass string               `json:"identifierClass"`
	Success        bool                  `json:"success"`
}

type BackgroundTaskReconcileRequest struct {
	TaskRunID    string `json:"taskRunId"`
	StagingFiles []string `json:"stagingFiles,omitempty"`
}

type BackgroundReadiness struct {
	Ready           bool   `json:"ready"`
	NativeHostReady bool   `json:"nativeHostReady"`
	BackendReady    bool   `json:"backendReady"`
	CanRunTask      bool   `json:"canRunTask"`
	Error           string `json:"error,omitempty"`
}

type BackgroundReadinessRequest struct {
	TaskDefinitionID string `json:"taskDefinitionId,omitempty"`
	TaskRunID        string `json:"taskRunId,omitempty"`
	TimeoutMs        int64  `json:"timeoutMs,omitempty"`
}

type BackgroundCheckpoint struct {
	TaskRunID      string         `json:"taskRunId"`
	Generation     int64          `json:"generation"`
	LastUnit       int64          `json:"lastUnit"`
	Phase          string         `json:"phase"`
	CheckpointData map[string]any `json:"checkpointData,omitempty"`
	UpdatedAt      string         `json:"updatedAt"`
}

type BackgroundCheckpointSetRequest struct {
	TaskRunID      string         `json:"taskRunId"`
	Generation     int64          `json:"generation"`
	LastUnit       int64          `json:"lastUnit"`
	Phase          string         `json:"phase"`
	CheckpointData map[string]any `json:"checkpointData,omitempty"`
}

type BackgroundTaskBinding struct {
	SystemClass     string `json:"systemClass"`
	TaskRunID       string `json:"taskRunId"`
	TaskDefinitionID string `json:"taskDefinitionId,omitempty"`
	Generation      int64  `json:"generation"`
	CorrelationID   string `json:"correlationId,omitempty"`
	CreatedAt       string `json:"createdAt"`
}

type BackgroundPendingRequest struct {
	IdentifierClass string `json:"identifierClass"`
	SystemClass     string `json:"systemClass"`
	HasTaskRun      bool   `json:"hasTaskRun"`
}
