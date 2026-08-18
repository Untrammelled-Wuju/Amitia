package protocol

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type RuntimeInvokePayload struct {
	InvocationID       string                           `json:"invocationId"`
	RuntimeType        string                           `json:"runtimeType"`
	Handler            string                           `json:"handler"`
	Input              json.RawMessage                  `json:"input,omitempty"`
	ProviderID         string                           `json:"providerId,omitempty"`
	DeviceID           runtimeidentity.DeviceID         `json:"deviceId"`
	RuntimeID          runtimeidentity.RuntimeID        `json:"runtimeId"`
	RuntimeSessionID   runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                          `json:"connectionGeneration"`
	DeadlineMs         int64                            `json:"deadlineMs,omitempty"`
	SentAt             time.Time                        `json:"sentAt"`
}

type RuntimeResultPayload struct {
	InvocationID       string                           `json:"invocationId"`
	RuntimeSessionID   runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                          `json:"connectionGeneration"`
	DeviceID           runtimeidentity.DeviceID         `json:"deviceId"`
	RuntimeID          runtimeidentity.RuntimeID        `json:"runtimeId"`
	Status             string                           `json:"status"`
	Result             json.RawMessage                  `json:"result,omitempty"`
	CompletedAt        time.Time                        `json:"completedAt"`
}

type RuntimeErrorPayload struct {
	InvocationID       string                           `json:"invocationId"`
	RuntimeSessionID   runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                          `json:"connectionGeneration"`
	DeviceID           runtimeidentity.DeviceID         `json:"deviceId"`
	RuntimeID          runtimeidentity.RuntimeID        `json:"runtimeId"`
	ErrorCode          string                           `json:"errorCode"`
	Message            string                           `json:"message"`
	Retryable          bool                             `json:"retryable"`
	FailedAt           time.Time                        `json:"failedAt"`
}

type RuntimeCancelPayload struct {
	InvocationID       string                           `json:"invocationId"`
	Reason             string                           `json:"reason"`
	RuntimeSessionID   runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration int64                          `json:"connectionGeneration"`
	DeviceID           runtimeidentity.DeviceID         `json:"deviceId"`
	RuntimeID          runtimeidentity.RuntimeID        `json:"runtimeId"`
	SentAt             time.Time                        `json:"sentAt"`
}
