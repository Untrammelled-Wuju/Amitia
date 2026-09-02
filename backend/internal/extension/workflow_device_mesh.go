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
	WorkflowMeshCatalog              = "workflow.catalog"
	WorkflowMeshToolCatalog          = "workflow.tool_catalog"
	WorkflowMeshGet                  = "workflow.get"
	WorkflowMeshUpsert               = "workflow.upsert"
	WorkflowMeshDelete               = "workflow.delete"
	WorkflowMeshSetEnabled           = "workflow.set_enabled"
	WorkflowMeshRun                  = "workflow.run"
	WorkflowMeshRunStart             = "workflow.run.start"
	WorkflowMeshRunList              = "workflow.run.list"
	WorkflowMeshRunGet               = "workflow.run.get"
	WorkflowMeshRunSteps             = "workflow.run.steps"
	WorkflowMeshRunAttempts          = "workflow.run.attempts"
	WorkflowMeshRunCheckpoints       = "workflow.run.checkpoints"
	WorkflowMeshRunLogs              = "workflow.run.logs"
	WorkflowMeshRunPause             = "workflow.run.pause"
	WorkflowMeshRunResume            = "workflow.run.resume"
	WorkflowMeshRunConfirm           = "workflow.run.confirm"
	WorkflowMeshRunCancel            = "workflow.run.cancel"
	WorkflowMeshRunRecover           = "workflow.run.recover"
	WorkflowMeshRunRerun             = "workflow.run.rerun"
	WorkflowMeshSyncOutbox           = "workflow.sync.outbox"
	WorkflowMeshSyncAck              = "workflow.sync.ack"
	WorkflowMeshSyncState            = "workflow.sync.state"
	WorkflowMeshTriggerCapabilities  = "workflow.trigger_capabilities"
	WorkflowMeshTriggerAppCatalog    = "workflow.trigger_app_catalog"
	WorkflowMeshTriggerWakeConfigs   = "workflow.trigger_wake_configs"
	WorkflowMeshCreateWakeConfig     = "workflow.trigger_wake_config.create"
	WorkflowMeshCreateTriggerSecret  = "workflow.trigger_secret.create"
	WorkflowMeshAndroidRuntimeHealth = "workflow.android_runtime_health"
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
		WorkflowMeshCatalog:              api.meshCatalog,
		WorkflowMeshToolCatalog:          api.meshToolCatalog,
		WorkflowMeshGet:                  api.meshGet,
		WorkflowMeshUpsert:               api.meshUpsert,
		WorkflowMeshDelete:               api.meshDelete,
		WorkflowMeshSetEnabled:           api.meshSetEnabled,
		WorkflowMeshRun:                  api.meshRun,
		WorkflowMeshRunStart:             api.meshRunStart,
		WorkflowMeshRunList:              api.meshRunList,
		WorkflowMeshRunGet:               api.meshRunGet,
		WorkflowMeshRunSteps:             api.meshRunSteps,
		WorkflowMeshRunAttempts:          api.meshRunAttempts,
		WorkflowMeshRunCheckpoints:       api.meshRunCheckpoints,
		WorkflowMeshRunLogs:              api.meshRunLogs,
		WorkflowMeshRunPause:             api.meshRunPause,
		WorkflowMeshRunResume:            api.meshRunResume,
		WorkflowMeshRunConfirm:           api.meshRunConfirm,
		WorkflowMeshRunCancel:            api.meshRunCancel,
		WorkflowMeshRunRecover:           api.meshRunRecover,
		WorkflowMeshRunRerun:             api.meshRunRerun,
		WorkflowMeshSyncOutbox:           api.meshSyncOutbox,
		WorkflowMeshSyncAck:              api.meshSyncAck,
		WorkflowMeshSyncState:            api.meshSyncState,
		WorkflowMeshTriggerCapabilities:  api.meshTriggerCapabilities,
		WorkflowMeshTriggerAppCatalog:    api.meshTriggerAppCatalog,
		WorkflowMeshTriggerWakeConfigs:   api.meshTriggerWakeConfigs,
		WorkflowMeshCreateWakeConfig:     api.meshCreateWakeConfig,
		WorkflowMeshCreateTriggerSecret:  api.meshCreateTriggerSecret,
		WorkflowMeshAndroidRuntimeHealth: api.meshAndroidRuntimeHealth,
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
		{ID: "workflow.trigger.notification.v1", Supported: android, Available: false, PermissionRequired: true, Permission: "android.permission.BIND_NOTIFICATION_LISTENER_SERVICE", Reason: capabilityReason(false, "Notification access status unavailable")},
		{ID: "workflow.trigger.system_event.v1", Supported: android, Available: android, PermissionRequired: false, Reason: capabilityReason(android, "Android runtime unavailable")},
		{ID: "workflow.trigger.network.v1", Supported: android, Available: android, PermissionRequired: false, Permission: "android.permission.ACCESS_NETWORK_STATE", Reason: capabilityReason(android, "Android runtime unavailable")},
		{ID: "workflow.trigger.bluetooth.v1", Supported: android, Available: false, PermissionRequired: true, Permission: "android.permission.BLUETOOTH_CONNECT,android.permission.BLUETOOTH_SCAN", Reason: capabilityReason(false, "Bluetooth permission status unavailable")},
		{ID: "workflow.trigger.location.v1", Supported: android, Available: false, PermissionRequired: true, Permission: "android.permission.ACCESS_FINE_LOCATION,android.permission.ACCESS_BACKGROUND_LOCATION", Reason: capabilityReason(false, "Location permission status unavailable")},
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

func (api *WorkflowAPI) meshToolCatalog(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	items, err := api.workflowToolCatalogSnapshot(ctx, invoke.UserID.String())
	if err != nil {
		return nil, err
	}
	return meshResult(invoke, map[string]any{"items": items})
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

func (api *WorkflowAPI) meshCreateWakeConfig(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	var request workflowWakeConfigCreateRequest
	if len(invoke.Input) > 0 {
		if err := json.Unmarshal(invoke.Input, &request); err != nil {
			return nil, err
		}
	}
	item, err := api.runtime.createWorkflowWakeConfig(ctx, request)
	if err != nil {
		return nil, err
	}
	return meshResult(invoke, item)
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

func (api *WorkflowAPI) meshRunRerun(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	runID, err := meshRunID(invoke.Input)
	if err != nil {
		return nil, err
	}
	previous, executor, err := api.meshOwnedRun(ctx, invoke.UserID.String(), runID)
	if err != nil {
		return nil, err
	}
	if !previous.Status.IsTerminal() {
		return nil, errors.New("workflow run must be terminal before rerun")
	}
	def, inst, err := api.meshOwned(ctx, strings.TrimSpace(invoke.UserID.String()), previous.WorkflowID)
	if err != nil {
		return nil, err
	}
	if !def.Enabled {
		return nil, errors.New("workflow is disabled")
	}
	input := previous.Input
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	executionID := "wf-run-" + uuid.NewString()
	opts := workflow.ExecutionOptionsForRerun(previous)
	execution := workflow.ExecutionContext{
		UserID: strings.TrimSpace(invoke.UserID.String()), WorkflowID: def.ID, InstallationID: inst.InstallationID,
		RootID: executionID, InvocationID: executionID, OperationID: "wf-op-" + uuid.NewString(),
		TraceID: "trace-" + uuid.NewString(), IdempotencyKey: executionID,
	}
	req := workflow.ExecuteRequest{WorkflowID: def.ID, Input: input, Context: execution, Options: opts}
	materializeInitialState := opts.Mode == workflow.ExecutionModeDryRun || (opts.Mode == workflow.ExecutionModeControlled && len(opts.MissingControlledApprovals(def.Nodes)) > 0)
	if materializeInitialState {
		result, execErr := executor.Execute(ctx, req)
		if execErr != nil {
			return nil, execErr
		}
		return meshResult(invoke, map[string]any{
			"accepted": true, "executionId": result.ExecutionID, "workflowId": result.WorkflowID,
			"status": result.Status, "executionMode": result.ExecutionMode,
			"requiredConfirmations": result.RequiredConfirmations, "sourceExecutionId": previous.ExecutionID,
		})
	}
	runCtx := context.WithoutCancel(ctx)
	go func() { _, _ = executor.Execute(runCtx, req) }()
	return meshResult(invoke, map[string]any{
		"accepted": true, "executionId": executionID, "workflowId": def.ID,
		"status": workflow.RunStatusRunning, "executionMode": opts.Mode, "sourceExecutionId": previous.ExecutionID,
	})
}

func (api *WorkflowAPI) meshSyncOutbox(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	if api == nil || api.runtime == nil || api.runtime.Kernel == nil || api.runtime.Kernel.Container() == nil || api.runtime.Kernel.Container().WorkflowDefRepo == nil {
		return nil, errors.New("workflow sync repository unavailable")
	}
	var request struct {
		Limit int `json:"limit"`
	}
	_ = json.Unmarshal(invoke.Input, &request)
	items, err := api.runtime.Kernel.Container().WorkflowDefRepo.ListWorkflowSyncOutbox(ctx, invoke.UserID.String(), request.Limit)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.EventID)
	}
	if err := api.runtime.Kernel.Container().WorkflowDefRepo.MarkWorkflowSyncSent(ctx, ids); err != nil {
		return nil, err
	}
	return meshResult(invoke, map[string]any{"items": items})
}

func (api *WorkflowAPI) meshSyncAck(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	if api == nil || api.runtime == nil || api.runtime.Kernel == nil || api.runtime.Kernel.Container() == nil || api.runtime.Kernel.Container().WorkflowDefRepo == nil {
		return nil, errors.New("workflow sync repository unavailable")
	}
	var request struct {
		EventIDs []string `json:"eventIds"`
	}
	if err := json.Unmarshal(invoke.Input, &request); err != nil {
		return nil, err
	}
	if err := api.runtime.Kernel.Container().WorkflowDefRepo.AckWorkflowSyncEvents(ctx, invoke.UserID.String(), request.EventIDs); err != nil {
		return nil, err
	}
	return meshResult(invoke, map[string]any{"acked": len(request.EventIDs)})
}

func (api *WorkflowAPI) meshSyncState(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	if api == nil || api.runtime == nil || api.runtime.Kernel == nil || api.runtime.Kernel.Container() == nil || api.runtime.Kernel.Container().WorkflowDefRepo == nil {
		return nil, errors.New("workflow sync repository unavailable")
	}
	items, err := api.runtime.Kernel.Container().WorkflowDefRepo.ListWorkflowSyncStates(ctx, invoke.UserID.String())
	if err != nil {
		return nil, err
	}
	return meshResult(invoke, map[string]any{"items": items})
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
		SyncRevision     int64                       `json:"syncRevision"`
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

	if _, exists := registry.Get(incomingID); !exists {
		def, err := api.prepareValidatedUserWorkflow(request.Definition, userID, incomingID)
		if err != nil {
			return nil, err
		}
		if err := api.validateExecutionTargets(def); err != nil {
			return nil, err
		}
		mutationCtx := ctx
		if request.SyncRevision > 0 {
			mutationCtx = sqlite.SuppressWorkflowSync(ctx)
		}
		if err := api.registerNewUserWorkflow(mutationCtx, registry, def, userID, "远程同步安装"); err != nil {
			return nil, err
		}
		if request.SyncRevision > 0 {
			if err := api.runtime.Kernel.Container().WorkflowDefRepo.SetWorkflowSyncState(ctx, userID, def.ID, request.SyncRevision, workflow.ComputeDefinitionHash(def), false); err != nil {
				return nil, err
			}
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
	mutationCtx := ctx
	if request.SyncRevision > 0 {
		mutationCtx = sqlite.SuppressWorkflowSync(ctx)
	}
	if err := registry.UpsertContext(mutationCtx, def); err != nil {
		return nil, err
	}
	rollback := func() {
		_ = api.syncTriggers(ctx, def, old, userID)
		_ = registry.UpsertContext(mutationCtx, old)
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
	if request.SyncRevision > 0 {
		if err := api.runtime.Kernel.Container().WorkflowDefRepo.SetWorkflowSyncState(ctx, userID, def.ID, request.SyncRevision, workflow.ComputeDefinitionHash(def), false); err != nil {
			return nil, err
		}
	}
	return meshResult(invoke, workflowResponse(def, updatedInst))
}

func (api *WorkflowAPI) meshDelete(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	var request struct {
		WorkflowID   string `json:"workflowId"`
		SyncRevision int64  `json:"syncRevision"`
	}
	if err := json.Unmarshal(invoke.Input, &request); err != nil {
		return nil, err
	}
	workflowID := strings.TrimSpace(request.WorkflowID)
	if workflowID == "" {
		return nil, errors.New("workflowId is required")
	}
	userID := invoke.UserID.String()
	unlock := api.lockWorkflowMutation(userID, workflowID)
	defer unlock()
	def, inst, err := api.meshOwned(ctx, userID, workflowID)
	if err != nil {
		if request.SyncRevision > 0 && strings.Contains(strings.ToLower(err.Error()), "not found") {
			if stateErr := api.runtime.Kernel.Container().WorkflowDefRepo.SetWorkflowSyncState(ctx, userID, workflowID, request.SyncRevision, "", true); stateErr != nil {
				return nil, stateErr
			}
			return meshResult(invoke, map[string]any{"deleted": true, "id": workflowID, "alreadyMissing": true})
		}
		return nil, err
	}
	registry, _, _ := api.kernelContainer()
	if err := api.syncTriggers(ctx, def, workflow.WorkflowDefinition{}, userID); err != nil {
		return nil, err
	}
	mutationCtx := ctx
	if request.SyncRevision > 0 {
		mutationCtx = sqlite.SuppressWorkflowSync(ctx)
	}
	if err := registry.UnregisterContext(mutationCtx, def.ID); err != nil {
		_ = api.syncTriggers(ctx, workflow.WorkflowDefinition{}, def, userID)
		return nil, err
	}
	if request.SyncRevision > 0 {
		if err := api.runtime.Kernel.Container().WorkflowDefRepo.SetWorkflowSyncState(ctx, userID, def.ID, request.SyncRevision, workflow.ComputeDefinitionHash(def), true); err != nil {
			return nil, err
		}
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
		Options    workflow.ExecutionOptions `json:"options"`
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
		Options:    request.Options,
	})
	if err != nil {
		return nil, err
	}
	return meshResult(invoke, result)
}

type workflowMeshRunRequest struct {
	WorkflowID string                    `json:"workflowId"`
	Input      json.RawMessage           `json:"input"`
	Context    workflow.ExecutionContext `json:"context"`
	Options    workflow.ExecutionOptions `json:"options"`
}

func (api *WorkflowAPI) prepareMeshRun(ctx context.Context, invoke protocol.RuntimeInvokePayload) (workflow.WorkflowDefinition, *workflow.WorkflowInstallation, workflowMeshRunRequest, workflow.ExecutionContext, *workflow.WorkflowExecutor, error) {
	var request workflowMeshRunRequest
	if err := json.Unmarshal(invoke.Input, &request); err != nil {
		return workflow.WorkflowDefinition{}, nil, request, workflow.ExecutionContext{}, nil, err
	}
	request.WorkflowID = strings.TrimSpace(request.WorkflowID)
	if request.WorkflowID == "" {
		return workflow.WorkflowDefinition{}, nil, request, workflow.ExecutionContext{}, nil, errors.New("workflowId is required")
	}
	def, inst, err := api.meshOwned(ctx, invoke.UserID.String(), request.WorkflowID)
	if err != nil {
		return workflow.WorkflowDefinition{}, nil, request, workflow.ExecutionContext{}, nil, err
	}
	if !def.Enabled {
		return workflow.WorkflowDefinition{}, nil, request, workflow.ExecutionContext{}, nil, errors.New("workflow is disabled")
	}
	if err := workflow.ValidateDAG(def.Nodes); err != nil {
		return workflow.WorkflowDefinition{}, nil, request, workflow.ExecutionContext{}, nil, err
	}
	if len(request.Input) == 0 {
		request.Input = json.RawMessage(`{}`)
	}
	_, executor, err := api.kernelContainer()
	if err != nil {
		return workflow.WorkflowDefinition{}, nil, request, workflow.ExecutionContext{}, nil, err
	}
	execution := request.Context
	userID := strings.TrimSpace(invoke.UserID.String())
	if execution.UserID != "" && strings.TrimSpace(execution.UserID) != userID {
		return workflow.WorkflowDefinition{}, nil, request, workflow.ExecutionContext{}, nil, errors.New("remote workflow context owner mismatch")
	}
	execution.UserID = userID
	execution.WorkflowID = def.ID
	execution.InstallationID = inst.InstallationID
	execution.DeviceID = strings.TrimSpace(invoke.DeviceID.String())
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
	if strings.TrimSpace(execution.IdempotencyKey) == "" {
		execution.IdempotencyKey = execution.InvocationID
	}
	return def, inst, request, execution, executor, nil
}

// meshRunStart acknowledges a durable run as soon as its execution id is
// known. Execution continues on the owning device even if the cloud caller
// disconnects, while the control-plane endpoints below can observe/control it.
func (api *WorkflowAPI) meshRunStart(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	def, _, request, execution, executor, err := api.prepareMeshRun(ctx, invoke)
	if err != nil {
		return nil, err
	}
	runCtx := context.WithoutCancel(ctx)
	opts, normalizeErr := request.Options.Normalize()
	if normalizeErr != nil {
		return nil, normalizeErr
	}
	if opts.IsDryRun() || (opts.Mode == workflow.ExecutionModeControlled && len(opts.MissingControlledApprovals(def.Nodes)) > 0) {
		result, runErr := executor.Execute(runCtx, workflow.ExecuteRequest{WorkflowID: def.ID, Input: request.Input, Context: execution, Options: opts})
		if runErr != nil {
			return nil, runErr
		}
		return meshResult(invoke, result)
	}
	go func() {
		_, _ = executor.Execute(runCtx, workflow.ExecuteRequest{WorkflowID: def.ID, Input: request.Input, Context: execution, Options: opts})
	}()
	return meshResult(invoke, map[string]any{
		"accepted":      true,
		"executionId":   execution.InvocationID,
		"workflowId":    def.ID,
		"deviceId":      execution.DeviceID,
		"status":        workflow.RunStatusRunning,
		"executionMode": opts.Mode,
		"traceId":       execution.TraceID,
	})
}

func meshRunID(input json.RawMessage) (string, error) {
	var request struct {
		RunID string `json:"runId"`
	}
	if err := json.Unmarshal(input, &request); err != nil {
		return "", err
	}
	request.RunID = strings.TrimSpace(request.RunID)
	if request.RunID == "" {
		return "", errors.New("runId is required")
	}
	return request.RunID, nil
}

func (api *WorkflowAPI) meshOwnedRun(ctx context.Context, userID, runID string) (*workflow.WorkflowRun, *workflow.WorkflowExecutor, error) {
	_, executor, err := api.kernelContainer()
	if err != nil {
		return nil, nil, err
	}
	kc := api.runtime.Kernel.Container()
	run, err := kc.WorkflowExecRepo.Get(ctx, runID)
	if err != nil || run == nil {
		return nil, nil, errors.New("workflow run not found")
	}
	requestedUserID := strings.TrimSpace(userID)
	if owner := strings.TrimSpace(run.Context.UserID); owner != "" {
		if owner != requestedUserID {
			return nil, nil, errors.New("workflow run not found")
		}
		return run, executor, nil
	}
	// Legacy runs may predate persisted Context.UserID. Prefer their immutable
	// definition snapshot before falling back to the mutable registry.
	var def workflow.WorkflowDefinition
	owned := false
	if len(run.Context.DefinitionSnapshot) > 0 && json.Unmarshal(run.Context.DefinitionSnapshot, &def) == nil {
		owned = workflowOwnedBy(def, requestedUserID)
	}
	if !owned {
		if current, ok := kc.WorkflowRegistry.Get(run.WorkflowID); ok {
			owned = workflowOwnedBy(current, requestedUserID)
		}
	}
	if !owned {
		return nil, nil, errors.New("workflow run not found")
	}
	return run, executor, nil
}

func (api *WorkflowAPI) meshRunEnvelope(ctx context.Context, userID, runID string) (map[string]any, error) {
	run, executor, err := api.meshOwnedRun(ctx, userID, runID)
	if err != nil {
		return nil, err
	}
	kc := api.runtime.Kernel.Container()
	steps, err := kc.WorkflowExecRepo.ListStepRuns(ctx, run.ExecutionID)
	if err != nil {
		return nil, err
	}
	attempts, err := kc.WorkflowExecRepo.ListStepAttempts(ctx, run.ExecutionID)
	if err != nil {
		return nil, err
	}
	compensations, err := kc.WorkflowExecRepo.ListCompensations(ctx, run.ExecutionID)
	if err != nil {
		return nil, err
	}
	checkpoints := []workflow.Checkpoint{}
	if store := executor.CheckpointStore(); store != nil {
		checkpoints, err = store.List(ctx, run.ExecutionID)
		if err != nil {
			return nil, err
		}
	}
	def, ok := kc.WorkflowRegistry.Get(run.WorkflowID)
	if !ok && len(run.Context.DefinitionSnapshot) > 0 {
		_ = json.Unmarshal(run.Context.DefinitionSnapshot, &def)
	}
	return map[string]any{
		"run":                   run,
		"stepRuns":              steps,
		"attempts":              attempts,
		"trace":                 workflow.BuildDistributedTrace(run, steps, attempts),
		"checkpoints":           checkpoints,
		"compensations":         compensations,
		"workflow":              def,
		"requiredConfirmations": workflow.MissingControlledApprovalsForRun(run),
		"executionOwner": map[string]any{
			"kind":     "device",
			"deviceId": strings.TrimSpace(run.Context.DeviceID),
		},
	}, nil
}

func (api *WorkflowAPI) meshRunList(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	var request struct {
		WorkflowID string `json:"workflowId"`
		Limit      int    `json:"limit"`
		Offset     int    `json:"offset"`
	}
	if err := json.Unmarshal(invoke.Input, &request); err != nil {
		return nil, err
	}
	request.WorkflowID = strings.TrimSpace(request.WorkflowID)
	if request.WorkflowID == "" {
		return nil, errors.New("workflowId is required")
	}
	if _, _, err := api.meshOwned(ctx, invoke.UserID.String(), request.WorkflowID); err != nil {
		return nil, err
	}
	if request.Limit <= 0 || request.Limit > 200 {
		request.Limit = 50
	}
	if request.Offset < 0 {
		request.Offset = 0
	}
	items, total, err := api.runtime.Kernel.Container().WorkflowExecRepo.ListRuns(ctx, request.WorkflowID, "", request.Limit, request.Offset)
	if err != nil {
		return nil, err
	}
	return meshResult(invoke, map[string]any{"items": items, "total": total})
}

func (api *WorkflowAPI) meshRunGet(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	runID, err := meshRunID(invoke.Input)
	if err != nil {
		return nil, err
	}
	envelope, err := api.meshRunEnvelope(ctx, invoke.UserID.String(), runID)
	if err != nil {
		return nil, err
	}
	return meshResult(invoke, envelope)
}

func (api *WorkflowAPI) meshRunSteps(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	runID, err := meshRunID(invoke.Input)
	if err != nil {
		return nil, err
	}
	run, _, err := api.meshOwnedRun(ctx, invoke.UserID.String(), runID)
	if err != nil {
		return nil, err
	}
	items, err := api.runtime.Kernel.Container().WorkflowExecRepo.ListStepRuns(ctx, run.ExecutionID)
	if err != nil {
		return nil, err
	}
	return meshResult(invoke, map[string]any{"items": items})
}

func (api *WorkflowAPI) meshRunAttempts(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	runID, err := meshRunID(invoke.Input)
	if err != nil {
		return nil, err
	}
	run, _, err := api.meshOwnedRun(ctx, invoke.UserID.String(), runID)
	if err != nil {
		return nil, err
	}
	items, err := api.runtime.Kernel.Container().WorkflowExecRepo.ListStepAttempts(ctx, run.ExecutionID)
	if err != nil {
		return nil, err
	}
	return meshResult(invoke, map[string]any{"items": items})
}

func (api *WorkflowAPI) meshRunCheckpoints(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	runID, err := meshRunID(invoke.Input)
	if err != nil {
		return nil, err
	}
	run, executor, err := api.meshOwnedRun(ctx, invoke.UserID.String(), runID)
	if err != nil {
		return nil, err
	}
	items := []workflow.Checkpoint{}
	if store := executor.CheckpointStore(); store != nil {
		items, err = store.List(ctx, run.ExecutionID)
		if err != nil {
			return nil, err
		}
	}
	return meshResult(invoke, map[string]any{"items": items})
}

func (api *WorkflowAPI) meshRunLogs(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	runID, err := meshRunID(invoke.Input)
	if err != nil {
		return nil, err
	}
	run, _, err := api.meshOwnedRun(ctx, invoke.UserID.String(), runID)
	if err != nil {
		return nil, err
	}
	kc := api.runtime.Kernel.Container()
	steps, err := kc.WorkflowExecRepo.ListStepRuns(ctx, run.ExecutionID)
	if err != nil {
		return nil, err
	}
	attempts, err := kc.WorkflowExecRepo.ListStepAttempts(ctx, run.ExecutionID)
	if err != nil {
		return nil, err
	}
	compensations, err := kc.WorkflowExecRepo.ListCompensations(ctx, run.ExecutionID)
	if err != nil {
		return nil, err
	}
	return meshResult(invoke, map[string]any{"items": workflow.BuildWorkflowRunLogs(run, steps, attempts, compensations)})
}

func (api *WorkflowAPI) meshRunPause(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	var request struct {
		RunID  string `json:"runId"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(invoke.Input, &request); err != nil {
		return nil, err
	}
	request.RunID = strings.TrimSpace(request.RunID)
	if request.RunID == "" {
		return nil, errors.New("runId is required")
	}
	_, executor, err := api.meshOwnedRun(ctx, invoke.UserID.String(), request.RunID)
	if err != nil {
		return nil, err
	}
	run, err := executor.Pause(ctx, request.RunID, request.Reason)
	if err != nil {
		return nil, err
	}
	return meshResult(invoke, run)
}

func (api *WorkflowAPI) meshRunResume(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	runID, err := meshRunID(invoke.Input)
	if err != nil {
		return nil, err
	}
	_, executor, err := api.meshOwnedRun(ctx, invoke.UserID.String(), runID)
	if err != nil {
		return nil, err
	}
	run, err := executor.Resume(ctx, runID)
	if err != nil {
		return nil, err
	}
	return meshResult(invoke, run)
}

func (api *WorkflowAPI) meshRunConfirm(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	var request struct {
		RunID   string   `json:"runId"`
		NodeIDs []string `json:"nodeIds"`
	}
	if err := json.Unmarshal(invoke.Input, &request); err != nil {
		return nil, err
	}
	request.RunID = strings.TrimSpace(request.RunID)
	if request.RunID == "" {
		return nil, errors.New("runId is required")
	}
	_, executor, err := api.meshOwnedRun(ctx, invoke.UserID.String(), request.RunID)
	if err != nil {
		return nil, err
	}
	run, missing, err := executor.ConfirmControlledRun(ctx, request.RunID, request.NodeIDs)
	if err != nil {
		return nil, err
	}
	return meshResult(invoke, map[string]any{"accepted": len(missing) == 0, "run": run, "missingConfirmations": missing})
}

func (api *WorkflowAPI) meshRunCancel(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	runID, err := meshRunID(invoke.Input)
	if err != nil {
		return nil, err
	}
	_, executor, err := api.meshOwnedRun(ctx, invoke.UserID.String(), runID)
	if err != nil {
		return nil, err
	}
	cancelled, err := executor.CancelRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	return meshResult(invoke, map[string]any{"cancelled": cancelled, "executionId": runID})
}

func (api *WorkflowAPI) meshRunRecover(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
	runID, err := meshRunID(invoke.Input)
	if err != nil {
		return nil, err
	}
	run, executor, err := api.meshOwnedRun(ctx, invoke.UserID.String(), runID)
	if err != nil {
		return nil, err
	}
	if run.Status != workflow.RunStatusFailed && run.Status != workflow.RunStatusCancelled {
		return nil, errors.New("only failed or cancelled runs can recover from checkpoints")
	}
	if strings.TrimSpace(run.Context.DefinitionHash) == "" {
		return nil, errors.New("run predates safe checkpoint recovery")
	}
	if len(run.Context.DefinitionSnapshot) > 0 {
		var snapshot workflow.WorkflowDefinition
		if err := json.Unmarshal(run.Context.DefinitionSnapshot, &snapshot); err != nil {
			return nil, errors.New("run definition snapshot is invalid; checkpoint recovery is unsafe")
		}
		if snapshot.ID != run.WorkflowID || workflow.ComputeDefinitionHash(snapshot) != run.Context.DefinitionHash {
			return nil, errors.New("run definition snapshot integrity check failed; checkpoint recovery is unsafe")
		}
	} else {
		def, exists := api.runtime.Kernel.Container().WorkflowRegistry.Get(run.WorkflowID)
		if !exists || !def.Enabled || workflow.ComputeDefinitionHash(def) != run.Context.DefinitionHash {
			return nil, errors.New("workflow definition changed or run predates safe checkpoint recovery")
		}
	}
	store := executor.CheckpointStore()
	if store == nil {
		return nil, errors.New("checkpoint store unavailable")
	}
	checkpoints, err := store.List(ctx, run.ExecutionID)
	if err != nil {
		return nil, err
	}
	if len(checkpoints) == 0 {
		return nil, errors.New("this run has no checkpoint to recover from")
	}
	execution := run.Context
	execution.UserID = strings.TrimSpace(invoke.UserID.String())
	execution.InvocationID = run.ExecutionID
	execution.Recovery = true
	execution.Generation = run.Generation + 1
	execution.OperationID = "wf-recover-" + uuid.NewString()
	execution.TraceID = "trace-" + uuid.NewString()
	runCtx := context.WithoutCancel(ctx)
	go func() {
		_, _ = executor.Execute(runCtx, workflow.ExecuteRequest{WorkflowID: run.WorkflowID, Input: run.Input, Context: execution})
	}()
	return meshResult(invoke, map[string]any{
		"accepted":        true,
		"executionId":     run.ExecutionID,
		"workflowId":      run.WorkflowID,
		"status":          workflow.RunStatusRunning,
		"generation":      execution.Generation,
		"checkpointCount": len(checkpoints),
	})
}
