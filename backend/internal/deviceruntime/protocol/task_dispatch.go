package protocol

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type TargetTaskDefinitionPin struct {
	DeviceID              string `json:"deviceId"`
	TaskID                string `json:"taskId"`
	ExtensionID           string `json:"extensionId"`
	ModuleID              string `json:"moduleId"`
	InstalledGeneration   int64  `json:"installedGeneration"`
	DefinitionFingerprint string `json:"definitionFingerprint"`
	PortableFingerprint   string `json:"portableFingerprint"`
	EntryHash             string `json:"entryHash"`
}

type TaskDispatchPayload struct {
	TaskGeneration       int64                            `json:"taskGeneration,omitempty"`
	ProgressBase         int64                            `json:"progressBase,omitempty"`
	RootTaskMetadata     json.RawMessage                  `json:"rootTaskMetadata,omitempty"`
	ResumeCheckpoint     json.RawMessage                  `json:"resumeCheckpoint,omitempty"`
	AuthorityCallID      string                           `json:"authorityCallId,omitempty"`
	OwnedExecutionScope  json.RawMessage                  `json:"ownedExecutionScope,omitempty"`
	TargetDefinitionPin  json.RawMessage                  `json:"targetDefinitionPin,omitempty"`
	TaskRunID            string                           `json:"taskRunId"`
	TaskDefinitionID     string                           `json:"taskDefinitionId"`
	AttemptID            string                           `json:"attemptId"`
	LeaseID              string                           `json:"leaseId"`
	Input                json.RawMessage                  `json:"input,omitempty"`
	DeadlineAt           *time.Time                       `json:"deadlineAt,omitempty"`
	MaxAttempts          int                              `json:"maxAttempts"`
	Placement            string                           `json:"placement"`
	DeviceID             runtimeidentity.DeviceID         `json:"deviceId"`
	RuntimeID            runtimeidentity.RuntimeID        `json:"runtimeId"`
	RuntimeSessionID     runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                            `json:"connectionGeneration"`
	SentAt               time.Time                        `json:"sentAt"`
}

type TaskOwnerRPCRequest struct {
	Scope                coordination.ExecutionScope `json:"executionScope"`
	AuthorityCallID      string                      `json:"authorityCallId"`
	TaskGeneration       int64                       `json:"taskGeneration"`
	AttemptID            string                      `json:"attemptId"`
	LeaseID              string                      `json:"leaseId"`
	SessionID            string                      `json:"sessionId"`
	ConnectionGeneration int64                       `json:"connectionGeneration"`
	RequestID            string                      `json:"requestId"`
	Method               string                      `json:"method"`
	Params               json.RawMessage             `json:"params"`
	NativeParamsBytes    []byte                      `json:"nativeParamsBytes,omitempty"`
}

type TaskCancelPayload struct {
	TaskRunID            string                           `json:"taskRunId"`
	AttemptID            string                           `json:"attemptId"`
	LeaseID              string                           `json:"leaseId"`
	Reason               string                           `json:"reason"`
	RuntimeSessionID     runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                            `json:"connectionGeneration"`
	SentAt               time.Time                        `json:"sentAt"`
}

type TaskClaimPayload struct {
	TaskRunID            string                           `json:"taskRunId"`
	AttemptID            string                           `json:"attemptId"`
	LeaseID              string                           `json:"leaseId"`
	WorkerID             string                           `json:"workerId"`
	LeaseDurationMs      int64                            `json:"leaseDurationMs"`
	RuntimeSessionID     runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                            `json:"connectionGeneration"`
	DeviceID             runtimeidentity.DeviceID         `json:"deviceId"`
	RuntimeID            runtimeidentity.RuntimeID        `json:"runtimeId"`
	ClaimedAt            time.Time                        `json:"claimedAt"`
}

type TaskLeaseAckPayload struct {
	TaskRunID            string                           `json:"taskRunId"`
	AttemptID            string                           `json:"attemptId"`
	LeaseID              string                           `json:"leaseId"`
	Sequence             int64                            `json:"sequence"`
	Accepted             bool                             `json:"accepted"`
	LeaseDurationMs      int64                            `json:"leaseDurationMs"`
	RuntimeSessionID     runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                            `json:"connectionGeneration"`
}

type TaskCompletePayload struct {
	PausedCheckpointVersion int64                            `json:"pausedCheckpointVersion,omitempty"`
	OutcomeUnknown          bool                             `json:"outcomeUnknown,omitempty"`
	ResultArtifactID        string                           `json:"resultArtifactId,omitempty"`
	TaskRunID               string                           `json:"taskRunId"`
	AttemptID               string                           `json:"attemptId"`
	LeaseID                 string                           `json:"leaseId"`
	Success                 bool                             `json:"success"`
	Result                  json.RawMessage                  `json:"result,omitempty"`
	Error                   string                           `json:"error,omitempty"`
	RuntimeSessionID        runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration    int64                            `json:"connectionGeneration"`
	DeviceID                runtimeidentity.DeviceID         `json:"deviceId"`
	RuntimeID               runtimeidentity.RuntimeID        `json:"runtimeId"`
	CompletedAt             time.Time                        `json:"completedAt"`
}

type OwnedTaskExecutionOutcome struct {
	PausedCheckpointVersion int64           `json:"pausedCheckpointVersion,omitempty"`
	Result                  json.RawMessage `json:"result,omitempty"`
	ResultArtifactID        string          `json:"resultArtifactId,omitempty"`
}

type TaskProgressPayload struct {
	TaskRunID            string                           `json:"taskRunId"`
	AttemptID            string                           `json:"attemptId"`
	LeaseID              string                           `json:"leaseId"`
	Sequence             int64                            `json:"sequence"`
	Current              *float64                         `json:"current,omitempty"`
	Total                *float64                         `json:"total,omitempty"`
	Percentage           *float64                         `json:"percentage,omitempty"`
	Stage                string                           `json:"stage,omitempty"`
	Message              string                           `json:"message,omitempty"`
	RuntimeSessionID     runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                            `json:"connectionGeneration"`
	DeviceID             runtimeidentity.DeviceID         `json:"deviceId"`
	RuntimeID            runtimeidentity.RuntimeID        `json:"runtimeId"`
	ReportedAt           time.Time                        `json:"reportedAt"`
}

type TaskCheckpointPayload struct {
	TaskRunID            string                           `json:"taskRunId"`
	AttemptID            string                           `json:"attemptId"`
	LeaseID              string                           `json:"leaseId"`
	CheckpointID         string                           `json:"checkpointId"`
	Version              int64                            `json:"version"`
	Payload              json.RawMessage                  `json:"payload"`
	PayloadHash          string                           `json:"payloadHash"`
	RuntimeSessionID     runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                            `json:"connectionGeneration"`
	DeviceID             runtimeidentity.DeviceID         `json:"deviceId"`
	RuntimeID            runtimeidentity.RuntimeID        `json:"runtimeId"`
	CheckpointAt         time.Time                        `json:"checkpointAt"`
}

type TaskHeartbeatPayload struct {
	TaskRunID            string                           `json:"taskRunId"`
	AttemptID            string                           `json:"attemptId"`
	LeaseID              string                           `json:"leaseId"`
	Sequence             int64                            `json:"sequence"`
	RuntimeSessionID     runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                            `json:"connectionGeneration"`
	DeviceID             runtimeidentity.DeviceID         `json:"deviceId"`
	RuntimeID            runtimeidentity.RuntimeID        `json:"runtimeId"`
	ReportedAt           time.Time                        `json:"reportedAt"`
}

type TaskPausePayload struct {
	TaskRunID            string                           `json:"taskRunId"`
	AttemptID            string                           `json:"attemptId"`
	LeaseID              string                           `json:"leaseId"`
	Reason               string                           `json:"reason"`
	RuntimeSessionID     runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                            `json:"connectionGeneration"`
	SentAt               time.Time                        `json:"sentAt"`
}

type TaskResumePayload struct {
	TaskRunID            string                           `json:"taskRunId"`
	AttemptID            string                           `json:"attemptId"`
	LeaseID              string                           `json:"leaseId"`
	CheckpointID         string                           `json:"checkpointId,omitempty"`
	RuntimeSessionID     runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                            `json:"connectionGeneration"`
	SentAt               time.Time                        `json:"sentAt"`
}
