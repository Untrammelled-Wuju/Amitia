package workflow

import (
	"errors"
	"strings"
	"time"
)

type WorkflowLocation string

const (
	WorkflowLocationLocal WorkflowLocation = "local"
	WorkflowLocationCloud WorkflowLocation = "cloud"
)

func (l WorkflowLocation) Valid() bool {
	switch l {
	case WorkflowLocationLocal, WorkflowLocationCloud:
		return true
	default:
		return false
	}
}

// WorkflowInstallation is the authoritative deployment state for a user
// workflow. Definition fields with equivalent names remain readable during the
// compatibility window, but new code should read/write installation state.
type WorkflowInstallation struct {
	InstallationID  string                      `json:"installationId"`
	WorkflowID      string                      `json:"workflowId"`
	OwnerUserID     string                      `json:"ownerUserId,omitempty"`
	Location        WorkflowLocation            `json:"location"`
	HostDeviceID    string                      `json:"hostDeviceId,omitempty"`
	Enabled         bool                        `json:"enabled"`
	Triggers        []WorkflowTriggerDefinition `json:"triggers,omitempty"`
	CallableByAgent bool                        `json:"callableByAgent"`
	AgentTool       WorkflowAgentToolConfig     `json:"agentTool,omitempty"`
	Revision        int64                       `json:"revision"`
	CreatedAt       time.Time                   `json:"createdAt"`
	UpdatedAt       time.Time                   `json:"updatedAt"`
}

func (i WorkflowInstallation) Validate() error {
	if strings.TrimSpace(i.InstallationID) == "" {
		return errors.New("workflow installation id is required")
	}
	if strings.TrimSpace(i.WorkflowID) == "" {
		return errors.New("workflow installation workflow id is required")
	}
	if strings.TrimSpace(i.OwnerUserID) == "" {
		return errors.New("workflow installation owner is required")
	}
	if !i.Location.Valid() {
		return errors.New("workflow installation location must be local or cloud")
	}
	return nil
}

type WorkflowExecutionPlacement string

const (
	WorkflowExecutionAuto   WorkflowExecutionPlacement = "auto"
	WorkflowExecutionLocal  WorkflowExecutionPlacement = "local"
	WorkflowExecutionCloud  WorkflowExecutionPlacement = "cloud"
	WorkflowExecutionDevice WorkflowExecutionPlacement = "device"
)

type WorkflowOfflinePolicy string

const (
	WorkflowOfflineFail WorkflowOfflinePolicy = "fail"
	WorkflowOfflineWait WorkflowOfflinePolicy = "wait"
)

// WorkflowExecutionTarget describes where a single node should execute. Empty
// values preserve legacy behavior and are normalized by the execution router.
type WorkflowExecutionTarget struct {
	Placement          WorkflowExecutionPlacement `json:"placement,omitempty"`
	DeviceID           string                     `json:"deviceId,omitempty"`
	RuntimeID          string                     `json:"runtimeId,omitempty"`
	ProviderID         string                     `json:"providerId,omitempty"`
	ProviderInstanceID string                     `json:"providerInstanceId,omitempty"`
	OfflinePolicy      WorkflowOfflinePolicy      `json:"offlinePolicy,omitempty"`
}

func (t WorkflowExecutionTarget) Normalized(defaultPlacement WorkflowExecutionPlacement) WorkflowExecutionTarget {
	result := t
	if result.Placement == "" {
		result.Placement = defaultPlacement
	}
	if result.OfflinePolicy == "" {
		result.OfflinePolicy = WorkflowOfflineFail
	}
	return result
}

func (t WorkflowExecutionTarget) Validate() error {
	if t.Placement == "" {
		return nil
	}
	switch t.Placement {
	case WorkflowExecutionAuto, WorkflowExecutionLocal, WorkflowExecutionCloud:
	case WorkflowExecutionDevice:
		if strings.TrimSpace(t.DeviceID) == "" {
			return errors.New("device execution target requires deviceId")
		}
	default:
		return errors.New("workflow execution target placement must be auto, local, cloud, or device")
	}
	if t.OfflinePolicy != "" && t.OfflinePolicy != WorkflowOfflineFail && t.OfflinePolicy != WorkflowOfflineWait {
		return errors.New("workflow execution target offlinePolicy must be fail or wait")
	}
	return nil
}
