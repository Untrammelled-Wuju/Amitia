package extension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	protocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

const (
	WorkflowMeshCatalog             = "workflow.catalog"
	WorkflowMeshGet                 = "workflow.get"
	WorkflowMeshUpsert              = "workflow.upsert"
	WorkflowMeshDelete              = "workflow.delete"
	WorkflowMeshSetEnabled          = "workflow.set_enabled"
	WorkflowMeshRun                 = "workflow.run"
	WorkflowMeshTriggerCapabilities = "workflow.trigger_capabilities"
	WorkflowMeshTriggerAppCatalog   = "workflow.trigger_app_catalog"
	WorkflowMeshTriggerWakeConfigs  = "workflow.trigger_wake_configs"
	WorkflowMeshCreateTriggerSecret = "workflow.trigger_secret.create"
)

type WorkflowDeviceRuntimeDispatcher interface {
	RegisterCancellable(handlerName string, handler agent.CancellableRuntimeInvokeHandler)
}

// RegisterDeviceWorkflowMeshHandlers installs only the narrow workflow control
// protocol on a Device Agent. It intentionally does not expose the full
// extension/business HTTP API over Device Mesh.
func RegisterDeviceWorkflowMeshHandlers(dispatcher WorkflowDeviceRuntimeDispatcher, runtime *Runtime) {
	if dispatcher == nil || runtime == nil {
		return
	}
	api := NewWorkflowAPIForLocation(runtime, workflow.WorkflowLocationLocal)
	for name, handler := range map[string]agent.CancellableRuntimeInvokeHandler{
		WorkflowMeshCatalog:             api.meshCatalog,
		WorkflowMeshGet:                 api.meshGet,
		WorkflowMeshUpsert:              api.meshUpsert,
		WorkflowMeshDelete:              api.meshDelete,
		WorkflowMeshSetEnabled:          api.meshSetEnabled,
		WorkflowMeshRun:                 api.meshRun,
		WorkflowMeshTriggerCapabilities: api.meshTriggerCapabilities,
		WorkflowMeshTriggerAppCatalog:   api.meshTriggerAppCatalog,
		WorkflowMeshTriggerWakeConfigs:  api.meshTriggerWakeConfigs,
		WorkflowMeshCreateTriggerSecret: api.meshCreateTriggerSecret,
	} {
		dispatcher.RegisterCancellable(name, handler)
	}
}

func meshResult(invoke protocol.RuntimeInvokePayload, value any) (*protocol.RuntimeResultPayload, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return &protocol.RuntimeResultPayload{
		InvocationID:         invoke.InvocationID,
		RuntimeSessionID:     invoke.RuntimeSessionID,
		ConnectionGeneration: invoke.ConnectionGeneration,
		DeviceID:             invoke.DeviceID,
		RuntimeID:            invoke.RuntimeID,
		Status:               "success",
		Result:               raw,
		CompletedAt:          time.Now().UTC(),
	}, nil
}

func meshWorkflowID(input json.RawMessage) (string, int64, error) {
	var request struct {
		WorkflowID       string `json:"workflowId"`
		ExpectedRevision int64  `json:"expectedRevision"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &request); err != nil {
			return "", 0, err
		}
	}
	request.WorkflowID = strings.TrimSpace(request.WorkflowID)
	if request.WorkflowID == "" {
		return "", 0, errors.New("workflowId is required")
	}
	return request.WorkflowID, request.ExpectedRevision, nil
}

func (api *WorkflowAPI) meshOwned(ctx context.Context, userID, workflowID string) (workflow.WorkflowDefinition, *workflow.WorkflowInstallation, error) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		return workflow.WorkflowDefinition{}, nil, err
	}
	def, ok := registry.Get(workflowID)
	if !ok || !workflowOwnedBy(def, userID) {
		return workflow.WorkflowDefinition{}, nil, errors.New("workflow not found")
	}
	inst, err := api.installationFor(ctx, def, userID)
	if err != nil {
		return workflow.WorkflowDefinition{}, nil, err
	}
	return applyInstallation(def, *inst), inst, nil
}

func (api *WorkflowAPI) workflowTriggerCapabilitiesSnapshot(ctx context.Context) []WorkflowTriggerCapabilityStatus {
	android := strings.TrimSpace(os.Getenv("ANDROID_ROOT")) != ""
	statuses := api.runtime.WorkflowTriggerCapabilityStatuses()
	defaults := []WorkflowTriggerCapabilityStatus{
		{ID: "workflow.trigger.android_intent.v1", Supported: android, Available: android, PermissionRequired: false, Reason: capabilityReason(android, "Android runtime unavailable")},
		{ID: "workflow.trigger.tasker.v1", Supported: android, Available: android, PermissionRequired: false, Reason: capabilityReason(android, "Android runtime unavailable")},
		{ID: "workflow.trigger.voice_wake.v1", Supported: android, Available: false, PermissionRequired: true, Permission: "android.permission.RECORD_AUDIO", Reason: capabilityReason(false, "Microphone permission status unavailable")},
		{ID: "workflow.trigger.voice_phrase.v1", Supported: true, Available: true, PermissionRequired: false},
		{ID: "workflow.trigger.app_foreground.v1", Supported: android, Available: false, PermissionRequired: true, Permission: "android.accessibilityservice.AccessibilityService", Reason: capabilityReason(false, "Accessibility service is not connected")},
	}
	items := make([]WorkflowTriggerCapabilityStatus, 0, len(defaults))
	for _, item := range defaults {
		if reported, ok := statuses[item.ID]; ok {
			item = reported
		}
		if item.ID == "workflow.trigger.voice_wake.v1" && item.Available {
			configs, err := api.runtime.workflowWakeConfigCatalog(ctx)
			if err != nil || len(configs) == 0 {
				item.Available = false
				if err != nil {
					item.Reason = "Wake-word backend status unavailable"
				} else {
					item.Reason = "No enabled wake-word recognizer configuration"
				}
			} else {
				status := api.runtime.workflowWakeStatus(ctx, false)
				if status.Required && !status.Ready {
					item.Available = false
					item.Reason = strings.TrimSpace(status.Reason)
					if item.Reason == "" {
						item.Reason = "Wake-word runtime is not ready"
					}
				}
			}
		}
		items = append(items, item)
	}
	return items
}

func (api *WorkflowAPI) meshTriggerCapabilities(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	return meshResult(invoke, map[string]any{"items": api.workflowTriggerCapabilitiesSnapshot(ctx)})
}

func (api *WorkflowAPI) meshTriggerAppCatalog(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	items, updatedAt := api.runtime.WorkflowTriggerAppCatalog()
	return meshResult(invoke, map[string]any{"items": items, "updatedAt": updatedAt})
}

func (api *WorkflowAPI) meshTriggerWakeConfigs(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	items, err := api.runtime.workflowWakeConfigCatalog(ctx)
	if err != nil {
		return nil, err
	}
	return meshResult(invoke, map[string]any{"items": items})
}

func (api *WorkflowAPI) meshCreateTriggerSecret(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	var request struct {
		Kind string `json:"kind"`
	}
	if len(invoke.Input) > 0 {
		if err := json.Unmarshal(invoke.Input, &request); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(request.Kind) != "tasker" {
		return nil, errors.New("unsupported workflow trigger secret kind")
	}
	value, err := api.newTaskerTriggerSecret(ctx, invoke.UserID.String())
	if err != nil {
		return nil, err
	}
	return meshResult(invoke, value)
}

func capabilityReason(available bool, reason string) string {
	if available {
		return ""
	}
	return reason
}

func (api *WorkflowAPI) meshCatalog(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		return nil, err
	}
	userID := strings.TrimSpace(invoke.UserID.String())
	installations, err := api.runtime.Kernel.Container().WorkflowInstallationRepo.List(ctx, userID, workflow.WorkflowLocationLocal, "")
	if err != nil {
		return nil, err
	}
	items := make([]workflowAPIResponse, 0, len(installations))
	for _, inst := range installations {
		def, ok := registry.Get(inst.WorkflowID)
		if !ok || !workflowOwnedBy(def, userID) {
			continue
		}
		items = append(items, workflowResponse(def, &inst))
	}
	return meshResult(invoke, map[string]any{"items": items, "total": len(items), "location": workflow.WorkflowLocationLocal})
}

func (api *WorkflowAPI) meshGet(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	workflowID, _, err := meshWorkflowID(invoke.Input)
	if err != nil {
		return nil, err
	}
	def, inst, err := api.meshOwned(ctx, invoke.UserID.String(), workflowID)
	if err != nil {
		return nil, err
	}
	return meshResult(invoke, workflowResponse(def, inst))
}

func (api *WorkflowAPI) meshUpsert(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	registry, _, err := api.kernelContainer()
	if err != nil {
		return nil, err
	}
	var request struct {
		Definition       workflow.WorkflowDefinition `json:"definition"`
		ExpectedRevision int64                       `json:"expectedRevision"`
	}
	wrappedErr := json.Unmarshal(invoke.Input, &request)
	if wrappedErr != nil || (strings.TrimSpace(request.Definition.ID) == "" && strings.TrimSpace(request.Definition.Name) == "" && len(request.Definition.Nodes) == 0) {
		// Also accept a bare WorkflowDefinition for old internal callers. JSON
		// unmarshalling ignores unknown wrapper fields, so detect an empty wrapped
		// definition instead of relying only on Unmarshal returning an error.
		var bare workflow.WorkflowDefinition
		if err := json.Unmarshal(invoke.Input, &bare); err != nil {
			if wrappedErr != nil {
				return nil, wrappedErr
			}
			return nil, err
		}
		request.Definition = bare
	}
	userID := strings.TrimSpace(invoke.UserID.String())
	incomingID := strings.TrimSpace(request.Definition.ID)
	if incomingID == "" {
		def, err := api.prepareValidatedUserWorkflow(request.Definition, userID, "")
		if err != nil {
			return nil, err
		}
		if err := api.validateExecutionTargets(def); err != nil {
			return nil, err
		}
		if _, exists := registry.Get(def.ID); exists {
			return nil, errors.New("workflow id already exists")
		}
		if err := api.registerNewUserWorkflow(ctx, registry, def, userID, "远程安装"); err != nil {
			return nil, err
		}
		inst, err := api.installationFor(ctx, def, userID)
		if err != nil {
			return nil, err
		}
		return meshResult(invoke, workflowResponse(def, inst))
	}

	unlock := api.lockWorkflowMutation(userID, incomingID)
	defer unlock()
	old, inst, err := api.meshOwned(ctx, userID, incomingID)
	if err != nil {
		return nil, err
	}
	expected := request.ExpectedRevision
	if expected <= 0 {
		expected = inst.Revision
	}
	if err := requireWorkflowRevision(expected, inst.Revision); err != nil {
		return nil, fmt.Errorf("WORKFLOW_REVISION_CONFLICT: %w", err)
	}
	def, err := api.prepareValidatedUserWorkflow(request.Definition, userID, old.ID)
	if err != nil {
		return nil, err
	}
	if err := api.validateExecutionTargets(def); err != nil {
		return nil, err
	}
	if old.DefinitionHash != def.DefinitionHash {
		if _, err := api.runtime.Kernel.Container().WorkflowDefRepo.SaveRevision(ctx, userID, old, "远程保存前自动快照"); err != nil {
			return nil, err
		}
	}
	if err := registry.Upsert(def); err != nil {
		return nil, err
	}
	rollback := func() {
		_ = api.syncTriggers(ctx, def, old, userID)
		_ = registry.Upsert(old)
	}
	if err := api.syncTriggers(ctx, old, def, userID); err != nil {
		rollback()
		return nil, err
	}
	updatedInst, err := api.updateInstallationCAS(ctx, def, userID, inst, expected)
	if err != nil {
		rollback()
		if errors.Is(err, sqlite.ErrWorkflowRevisionConflict) {
			return nil, fmt.Errorf("WORKFLOW_REVISION_CONFLICT: %w", err)
		}
		return nil, err
	}
	return meshResult(invoke, workflowResponse(def, updatedInst))
}

func (api *WorkflowAPI) meshDelete(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	workflowID, _, err := meshWorkflowID(invoke.Input)
	if err != nil {
		return nil, err
	}
	userID := invoke.UserID.String()
	unlock := api.lockWorkflowMutation(userID, workflowID)
	defer unlock()
	def, inst, err := api.meshOwned(ctx, userID, workflowID)
	if err != nil {
		return nil, err
	}
	registry, _, _ := api.kernelContainer()
	if err := api.syncTriggers(ctx, def, workflow.WorkflowDefinition{}, userID); err != nil {
		return nil, err
	}
	if err := registry.Unregister(def.ID); err != nil {
		_ = api.syncTriggers(ctx, workflow.WorkflowDefinition{}, def, userID)
		return nil, err
	}
	api.emitWorkflowInstallationEvent(ctx, "workflow.installation.deleted", inst)
	return meshResult(invoke, map[string]any{"deleted": true, "id": def.ID, "installation": inst})
}

func (api *WorkflowAPI) meshSetEnabled(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	var request struct {
		WorkflowID       string `json:"workflowId"`
		Enabled          bool   `json:"enabled"`
		ExpectedRevision int64  `json:"expectedRevision"`
	}
	if err := json.Unmarshal(invoke.Input, &request); err != nil {
		return nil, err
	}
	request.WorkflowID = strings.TrimSpace(request.WorkflowID)
	if request.WorkflowID == "" {
		return nil, errors.New("workflowId is required")
	}
	userID := invoke.UserID.String()
	unlock := api.lockWorkflowMutation(userID, request.WorkflowID)
	defer unlock()
	old, inst, err := api.meshOwned(ctx, userID, request.WorkflowID)
	if err != nil {
		return nil, err
	}
	expected := request.ExpectedRevision
	if expected <= 0 {
		expected = inst.Revision
	}
	if err := requireWorkflowRevision(expected, inst.Revision); err != nil {
		return nil, fmt.Errorf("WORKFLOW_REVISION_CONFLICT: %w", err)
	}
	registry, _, _ := api.kernelContainer()
	def := old
	def.Enabled = request.Enabled
	if err := registry.Upsert(def); err != nil {
		return nil, err
	}
	if err := api.syncTriggers(ctx, old, def, userID); err != nil {
		_ = registry.Upsert(old)
		return nil, err
	}
	updated, err := api.updateInstallationCAS(ctx, def, userID, inst, expected)
	if err != nil {
		_ = api.syncTriggers(ctx, def, old, userID)
		_ = registry.Upsert(old)
		return nil, err
	}
	eventType := "workflow.installation.disabled"
	if def.Enabled {
		eventType = "workflow.installation.enabled"
	}
	api.emitWorkflowInstallationEvent(ctx, eventType, updated)
	return meshResult(invoke, map[string]any{"id": def.ID, "enabled": def.Enabled, "installation": updated})
}

func (api *WorkflowAPI) meshRun(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	var request struct {
		WorkflowID string                    `json:"workflowId"`
		Input      json.RawMessage           `json:"input"`
		Context    workflow.ExecutionContext `json:"context"`
	}
	if err := json.Unmarshal(invoke.Input, &request); err != nil {
		return nil, err
	}
	request.WorkflowID = strings.TrimSpace(request.WorkflowID)
	if request.WorkflowID == "" {
		return nil, errors.New("workflowId is required")
	}
	def, inst, err := api.meshOwned(ctx, invoke.UserID.String(), request.WorkflowID)
	if err != nil {
		return nil, err
	}
	if !def.Enabled {
		return nil, errors.New("workflow is disabled")
	}
	if len(request.Input) == 0 {
		request.Input = json.RawMessage(`{}`)
	}
	_, executor, _ := api.kernelContainer()
	execution := request.Context
	if execution.UserID != "" && strings.TrimSpace(execution.UserID) != invoke.UserID.String() {
		return nil, errors.New("remote workflow context owner mismatch")
	}
	execution.UserID = invoke.UserID.String()
	execution.WorkflowID = def.ID
	execution.InstallationID = inst.InstallationID
	execution.DeviceID = invoke.DeviceID.String()
	if strings.TrimSpace(execution.InvocationID) == "" {
		execution.InvocationID = "wf-run-" + uuid.NewString()
	}
	if strings.TrimSpace(execution.RootID) == "" {
		execution.RootID = execution.InvocationID
	}
	if strings.TrimSpace(execution.OperationID) == "" {
		execution.OperationID = "wf-op-" + uuid.NewString()
	}
	if strings.TrimSpace(execution.TraceID) == "" {
		execution.TraceID = "trace-" + uuid.NewString()
	}
	result, err := executor.Execute(ctx, workflow.ExecuteRequest{
		WorkflowID: def.ID,
		Input:      request.Input,
		Context:    execution,
	})
	if err != nil {
		return nil, err
	}
	return meshResult(invoke, result)
}
