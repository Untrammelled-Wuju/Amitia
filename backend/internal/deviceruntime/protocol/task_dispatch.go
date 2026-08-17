package protocol

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type TaskDispatchPayload struct {
	TaskRunID          string                           `json:"taskRunId"`
	TaskDefinitionID   string                           `json:"taskDefinitionId"`
	AttemptID          string                           `json:"attemptId"`
	LeaseID            string                           `json:"leaseId"`
	Input              json.RawMessage                  `json:"input,omitempty"`
	DeadlineAt         *time.Time                       `json:"deadlineAt,omitempty"`
	MaxAttempts        int                              `json:"maxAttempts"`
	Placement          string                           `json:"placement"`
	DeviceID           runtimeidentity.DeviceID         `json:"deviceId"`
	RuntimeID          runtimeidentity.RuntimeID        `json:"runtimeId"`
	RuntimeSessionID   runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                          `json:"connectionGeneration"`
	SentAt             time.Time                        `json:"sentAt"`
}

type TaskCancelPayload struct {
	TaskRunID          string                           `json:"taskRunId"`
	AttemptID          string                           `json:"attemptId"`
	LeaseID            string                           `json:"leaseId"`
	Reason             string                           `json:"reason"`
	RuntimeSessionID   runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                          `json:"connectionGeneration"`
	SentAt             time.Time                        `json:"sentAt"`
}

type TaskClaimPayload struct {
	TaskRunID          string                           `json:"taskRunId"`
	AttemptID          string                           `json:"attemptId"`
	LeaseID            string                           `json:"leaseId"`
	WorkerID           string                           `json:"workerId"`
	LeaseDurationMs    int64                            `json:"leaseDurationMs"`
	RuntimeSessionID   runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                          `json:"connectionGeneration"`
	DeviceID           runtimeidentity.DeviceID         `json:"deviceId"`
	RuntimeID          runtimeidentity.RuntimeID        `json:"runtimeId"`
	ClaimedAt          time.Time                        `json:"claimedAt"`
}

type TaskCompletePayload struct {
	TaskRunID          string                           `json:"taskRunId"`
	AttemptID          string                           `json:"attemptId"`
	LeaseID            string                           `json:"leaseId"`
	Success            bool                             `json:"success"`
	Result             json.RawMessage                  `json:"result,omitempty"`
	Error              string                           `json:"error,omitempty"`
	RuntimeSessionID   runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                          `json:"connectionGeneration"`
	DeviceID           runtimeidentity.DeviceID         `json:"deviceId"`
	RuntimeID          runtimeidentity.RuntimeID        `json:"runtimeId"`
	CompletedAt        time.Time                        `json:"completedAt"`
}

type TaskProgressPayload struct {
	TaskRunID          string                           `json:"taskRunId"`
	AttemptID          string                           `json:"attemptId"`
	LeaseID            string                           `json:"leaseId"`
	Sequence           int64                            `json:"sequence"`
	Current            *float64                         `json:"current,omitempty"`
	Total              *float64                         `json:"total,omitempty"`
	Percentage         *float64                         `json:"percentage,omitempty"`
	Stage              string                           `json:"stage,omitempty"`
	Message            string                           `json:"message,omitempty"`
	RuntimeSessionID   runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                          `json:"connectionGeneration"`
	DeviceID           runtimeidentity.DeviceID         `json:"deviceId"`
	RuntimeID          runtimeidentity.RuntimeID        `json:"runtimeId"`
	ReportedAt         time.Time                        `json:"reportedAt"`
}

type TaskCheckpointPayload struct {
	TaskRunID          string                           `json:"taskRunId"`
	AttemptID          string                           `json:"attemptId"`
	LeaseID            string                           `json:"leaseId"`
	CheckpointID       string                           `json:"checkpointId"`
	Version            int64                            `json:"version"`
	Payload            json.RawMessage                  `json:"payload"`
	PayloadHash        string                           `json:"payloadHash"`
	RuntimeSessionID   runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                          `json:"connectionGeneration"`
	DeviceID           runtimeidentity.DeviceID         `json:"deviceId"`
	RuntimeID          runtimeidentity.RuntimeID        `json:"runtimeId"`
	CheckpointAt       time.Time                        `json:"checkpointAt"`
}
