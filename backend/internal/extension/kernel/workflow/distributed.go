package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// WorkflowCallFrame is transported across Core/Device boundaries so nested
// workflow cycle detection is not limited to one process. InstallationID and
// DeviceID are retained when known; WorkflowID is always required.
type WorkflowCallFrame struct {
	InstallationID string `json:"installationId,omitempty"`
	WorkflowID     string `json:"workflowId"`
	DeviceID       string `json:"deviceId,omitempty"`
}

func (f WorkflowCallFrame) key() string {
	installationID := strings.TrimSpace(f.InstallationID)
	workflowID := strings.TrimSpace(f.WorkflowID)
	deviceID := strings.TrimSpace(f.DeviceID)
	if installationID != "" {
		return "installation:" + installationID
	}
	return "workflow:" + workflowID + "@device:" + deviceID
}

func AppendWorkflowCallFrame(stack []WorkflowCallFrame, frame WorkflowCallFrame) ([]WorkflowCallFrame, error) {
	if strings.TrimSpace(frame.WorkflowID) == "" {
		return nil, fmt.Errorf("workflow call frame workflowId is required")
	}
	key := frame.key()
	for _, existing := range stack {
		if existing.key() == key {
			path := make([]string, 0, len(stack)+1)
			for _, item := range stack {
				path = append(path, item.WorkflowID)
			}
			path = append(path, frame.WorkflowID)
			return nil, fmt.Errorf("workflow: distributed nested workflow cycle detected: %s", strings.Join(path, " -> "))
		}
	}
	out := append([]WorkflowCallFrame(nil), stack...)
	return append(out, frame), nil
}

type RemoteWorkflowRequest struct {
	WorkflowID string                  `json:"workflowId"`
	Input      json.RawMessage         `json:"input"`
	Target     WorkflowExecutionTarget `json:"target"`
	Context    ExecutionContext        `json:"context"`
}

type RemoteWorkflowRunner interface {
	RunRemoteWorkflow(ctx context.Context, request RemoteWorkflowRequest) (json.RawMessage, error)
}

// WorkflowDeviceUnavailableError is intentionally distinct from retryable
// step failures. offlinePolicy=wait is converted into durable waiting_device
// state by WorkflowExecutor instead of consuming retry attempts.
type WorkflowDeviceUnavailableError struct {
	DeviceID string
	Cause    error
}

func (e *WorkflowDeviceUnavailableError) Error() string {
	if e == nil {
		return "workflow target device unavailable"
	}
	if e.Cause != nil {
		if strings.TrimSpace(e.DeviceID) != "" {
			return fmt.Sprintf("workflow target device %s unavailable: %v", e.DeviceID, e.Cause)
		}
		return fmt.Sprintf("workflow target device unavailable: %v", e.Cause)
	}
	if strings.TrimSpace(e.DeviceID) != "" {
		return fmt.Sprintf("workflow target device %s unavailable", e.DeviceID)
	}
	return "workflow target device unavailable"
}

func (e *WorkflowDeviceUnavailableError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
