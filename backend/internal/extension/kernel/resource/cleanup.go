package resource

import "time"

type CleanupJob struct {
	JobID      string         `json:"job_id"`
	ResourceID string         `json:"resource_id"`
	JobType    string         `json:"job_type"`
	Priority   int            `json:"priority"`
	Status     string         `json:"status"`
	CreatedAt  time.Time      `json:"created_at"`
	StartedAt  *time.Time     `json:"started_at,omitempty"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
	Error      string         `json:"error,omitempty"`
	Retries    int            `json:"retries"`
	MaxRetries int            `json:"max_retries"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

const (
	CleanupJobStatusPending   = "pending"
	CleanupJobStatusRunning   = "running"
	CleanupJobStatusCompleted = "completed"
	CleanupJobStatusFailed    = "failed"
	CleanupJobStatusCancelled = "cancelled"
)

const (
	CleanupJobTypeStopProcess     = "stop_process"
	CleanupJobTypeCloseConnection = "close_connection"
	CleanupJobTypeDeleteFile      = "delete_file"
	CleanupJobTypeRemoveDir       = "remove_directory"
	CleanupJobTypeClearCache      = "clear_cache"
	CleanupJobTypeDeleteSecret    = "delete_secret"
	CleanupJobTypeUnregisterTool  = "unregister_tool"
	CleanupJobTypeRemoveUI        = "remove_ui"
	CleanupJobTypeCancelSchedule  = "cancel_schedule"
	CleanupJobTypeUnsubscribe     = "unsubscribe"
)
