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
	BackgroundRefreshEnabled    BackgroundRefreshStatus = "enabled"
	BackgroundRefreshLimited    BackgroundRefreshStatus = "limited"
	BackgroundRefreshDisabled   BackgroundRefreshStatus = "disabled"
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
)

type BackgroundStatus struct {
	Supported                bool                    `json:"supported"`
	AppRefreshSupported      bool                    `json:"appRefreshSupported"`
	ProcessingSupported      bool                    `json:"processingSupported"`
	PendingRequests          int                     `json:"pendingRequests"`
	BackgroundRefreshEnabled BackgroundRefreshStatus `json:"backgroundRefreshEnabled"`
	RuntimePersistence       RuntimePersistence      `json:"runtimePersistence"`
	State                    BackgroundState         `json:"state"`
}

type BackgroundTaskRegistration struct {
	SystemClass  BackgroundSystemClass `json:"systemClass"`
	Identifier   string                `json:"identifier"`
	Success      bool                  `json:"success"`
	RegisteredAt string                `json:"registeredAt"`
	ErrorCode    string                `json:"errorCode,omitempty"`
	ErrorMessage string                `json:"errorMessage,omitempty"`
}

type BackgroundRegistrationRequest struct {
	SystemClass BackgroundSystemClass `json:"systemClass"`
	Identifier  string                `json:"identifier"`
}

type BackgroundSubmissionRequest struct {
	TaskRunID             string                `json:"taskRunId"`
	SystemClass           BackgroundSystemClass `json:"systemClass"`
	Identifier            string                `json:"identifier"`
	EarliestBeginAt       *time.Time            `json:"earliestBeginAt,omitempty"`
	NetworkRequired       bool                  `json:"networkRequired"`
	ExternalPowerRequired bool                  `json:"externalPowerRequired"`
	GPURequired           bool                  `json:"gpuRequired"`
	Reason                string                `json:"reason,omitempty"`
}

type BackgroundSubmissionResult struct {
	Submitted   bool   `json:"submitted"`
	TaskRunID   string `json:"taskRunId"`
	SystemClass string `json:"systemClass"`
	Error       string `json:"error,omitempty"`
	ErrorCode   string `json:"errorCode,omitempty"`
}

type BackgroundCancelRequest struct {
	TaskRunID string `json:"taskRunId"`
}

type BackgroundCancelAllRequest struct {
	SystemClass BackgroundSystemClass `json:"systemClass"`
}

type BackgroundTaskProgress struct {
	TaskRunID      string `json:"taskRunId"`
	TotalUnits     int64  `json:"totalUnits"`
	CompletedUnits int64  `json:"completedUnits"`
	Phase          string `json:"phase"`
}

type BackgroundTaskExpireRequest struct {
	TaskRunID string `json:"taskRunId"`
}

type BackgroundTaskCompleteRequest struct {
	TaskRunID string `json:"taskRunId"`
	Success   bool   `json:"success"`
}

type BackgroundTaskReconcileRequest struct {
	TaskRunID    string   `json:"taskRunId"`
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
	TaskRunID string `json:"taskRunId"`
	TimeoutMs int64  `json:"timeoutMs,omitempty"`
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
	SystemClass   string `json:"systemClass"`
	TaskRunID     string `json:"taskRunId"`
	Generation    int64  `json:"generation"`
	CreatedAt     string `json:"createdAt"`
}

type BackgroundPendingRequest struct {
	TaskRunID   string `json:"taskRunId"`
	SystemClass string `json:"systemClass"`
}
