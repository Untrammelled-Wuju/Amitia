package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type workflowDeviceRemoteRunner struct {
	control WorkflowDeviceControlPlane
}

func (r workflowDeviceRemoteRunner) RunRemoteWorkflow(ctx context.Context, request workflow.RemoteWorkflowRequest) (json.RawMessage, error) {
	if r.control == nil {
		return nil, fmt.Errorf("workflow remote control plane unavailable")
	}
	deviceID := strings.TrimSpace(request.Target.DeviceID)
	if deviceID == "" {
		return nil, fmt.Errorf("remote nested workflow requires deviceId")
	}
	payload, err := json.Marshal(map[string]any{
		"workflowId": request.WorkflowID,
		"input":      request.Input,
		"context":    request.Context,
	})
	if err != nil {
		return nil, err
	}
	resultRaw, err := r.control.Invoke(ctx, request.Context.UserID, deviceID, WorkflowMeshRun, payload)
	if err != nil {
		lower := strings.ToLower(err.Error())
		if request.Target.OfflinePolicy == workflow.WorkflowOfflineWait &&
			(strings.Contains(lower, "offline") || strings.Contains(lower, "not connected") || strings.Contains(lower, "runtime_not_ready") || strings.Contains(lower, "runtime_offline")) {
			return nil, &workflow.WorkflowDeviceUnavailableError{DeviceID: deviceID, Cause: err}
		}
		return nil, err
	}
	var result workflow.ExecuteResult
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		return nil, fmt.Errorf("decode remote workflow result: %w", err)
	}
	if !result.Success {
		if result.Status == workflow.RunStatusWaitingDevice {
			return nil, &workflow.WorkflowDeviceUnavailableError{DeviceID: deviceID, Cause: fmt.Errorf("remote workflow is waiting for device")}
		}
		return nil, fmt.Errorf("remote workflow %s failed: %s", request.WorkflowID, result.Error)
	}
	if len(result.Output) == 0 {
		return json.RawMessage(`{}`), nil
	}
	return result.Output, nil
}
