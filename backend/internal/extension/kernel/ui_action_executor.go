package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type UIActionExecContext struct {
	SessionID            string
	ContributionID       string
	ExtensionID          string
	ModuleID             string
	Generation           int64
	ScopeSnapshotID      string
	PermissionSnapshotID string
	CharacterID          string
	ConversationID       string
	TraceID              string
}

type UIActionExecutor struct {
	hostAPIGateway      *host_api.DefaultGateway
	workflowExecutor    *workflow.WorkflowExecutor
	runStore            workflow.RunStore
	hostCommandRegistry *HostCommandRegistry
	operationRepo       sqlite.OperationRepository
}

func NewUIActionExecutor(gateway *host_api.DefaultGateway, wfExecutor *workflow.WorkflowExecutor, runStore workflow.RunStore, hostCmdRegistry *HostCommandRegistry, opRepo sqlite.OperationRepository) *UIActionExecutor {
	return &UIActionExecutor{
		hostAPIGateway:      gateway,
		workflowExecutor:    wfExecutor,
		runStore:            runStore,
		hostCommandRegistry: hostCmdRegistry,
		operationRepo:       opRepo,
	}
}

func (e *UIActionExecutor) Execute(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage) (json.RawMessage, error) {
	identity := runtime_supervisor.RuntimeIdentity{
		InstanceID:  execCtx.SessionID,
		ExtensionID: domain.ExtensionID(execCtx.ExtensionID),
		ModuleID:    domain.ModuleID(execCtx.ModuleID),
		Generation:  execCtx.Generation,
	}

	switch action.Target.Type {
	case ui_contribution.ActionTargetTool:
		return e.executeTool(ctx, execCtx, action, input, identity)
	case ui_contribution.ActionTargetWorkflow:
		return e.executeWorkflow(ctx, execCtx, action, input, identity)
	case ui_contribution.ActionTargetNavigation:
		return e.executeNavigation(ctx, execCtx, action, input, identity)
	case ui_contribution.ActionTargetDialog:
		return e.executeDialog(ctx, execCtx, action, input, identity)
	case ui_contribution.ActionTargetCopy:
		return e.executeClipboardWrite(ctx, execCtx, action, input, identity)
	case ui_contribution.ActionTargetHostCommand:
		return e.executeHostCommand(ctx, execCtx, action, input, identity)
	case ui_contribution.ActionTargetOpenResource:
		return e.executeOpenResource(ctx, execCtx, action, input, identity)
	default:
		return nil, fmt.Errorf("unsupported action target type: %s", action.Target.Type)
	}
}

func (e *UIActionExecutor) executeTool(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage, identity runtime_supervisor.RuntimeIdentity) (json.RawMessage, error) {
	toolID := action.Target.ToolID
	if toolID == "" {
		toolID = action.ActionID
	}
	toolInput, _ := json.Marshal(map[string]any{
		"toolId": toolID,
		"input":  json.RawMessage(input),
	})
	callReq := host_api.CallRequest{
		CallID:               fmt.Sprintf("ui-action-tool-%s-%s", execCtx.SessionID, uuid.NewString()),
		RuntimeIdentity:      identity,
		Method:               host_api.MethodToolExecute,
		Version:              1,
		Input:                toolInput,
		ScopeSnapshotID:      execCtx.ScopeSnapshotID,
		PermissionSnapshotID: execCtx.PermissionSnapshotID,
	}
	result := e.hostAPIGateway.Call(ctx, callReq)
	if result.Error != nil {
		return nil, fmt.Errorf("action %s failed: %s", action.ActionID, result.Error.Message)
	}
	return result.Output, nil
}

func (e *UIActionExecutor) executeWorkflow(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage, identity runtime_supervisor.RuntimeIdentity) (json.RawMessage, error) {
	workflowID := action.Target.WorkflowID
	if workflowID == "" {
		return nil, fmt.Errorf("workflow action %s requires workflow_id", action.ActionID)
	}

	wfAction := action.Target.WorkflowAction
	if wfAction == "" {
		wfAction = ui_contribution.WorkflowActionRun
	}
	if !wfAction.Valid() {
		return nil, fmt.Errorf("unsupported workflow action: %s", wfAction)
	}

	switch wfAction {
	case ui_contribution.WorkflowActionRun:
		return e.executeWorkflowRun(ctx, execCtx, workflowID, input, action)
	case ui_contribution.WorkflowActionCancel:
		return e.executeWorkflowCancel(ctx, input, action)
	case ui_contribution.WorkflowActionStatus:
		return e.executeWorkflowStatus(ctx, input, action)
	default:
		return nil, fmt.Errorf("unsupported workflow action: %s", wfAction)
	}
}

func (e *UIActionExecutor) executeWorkflowRun(ctx context.Context, execCtx UIActionExecContext, workflowID string, input json.RawMessage, action *ui_contribution.UIActionDefinition) (json.RawMessage, error) {
	if e.workflowExecutor == nil {
		return nil, fmt.Errorf("workflow executor unavailable for action %s", action.ActionID)
	}

	operationID := fmt.Sprintf("ui-wf-%s", uuid.NewString())
	invocationID := fmt.Sprintf("ui-wf-%s-%s", execCtx.SessionID, uuid.NewString())
	traceID := execCtx.TraceID
	if traceID == "" {
		traceID = fmt.Sprintf("ui-wf-trace-%s", uuid.NewString())
	}

	execContext := workflow.ExecutionContext{
		ExtensionID:      execCtx.ExtensionID,
		ModuleID:         execCtx.ModuleID,
		Generation:       execCtx.Generation,
		OperationID:      operationID,
		InvocationID:     invocationID,
		ScopeSnapshotID:  execCtx.ScopeSnapshotID,
		PermissionSnapID: execCtx.PermissionSnapshotID,
		CharacterID:      execCtx.CharacterID,
		ConversationID:   execCtx.ConversationID,
		TraceID:          traceID,
	}

	req := workflow.ExecuteRequest{
		WorkflowID: workflowID,
		Input:      input,
		Context:    execContext,
	}

	result, err := e.workflowExecutor.Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("workflow %s execution failed: %w", workflowID, err)
	}

	output := result.Output
	if len(output) == 0 {
		output = json.RawMessage(`{}`)
	}

	return json.Marshal(map[string]any{
		"workflowRunID": result.ExecutionID,
		"status":        string(result.Status),
		"operationID":   operationID,
		"accepted":      result.Accepted,
		"success":       result.Success,
		"output":        json.RawMessage(output),
	})
}

func (e *UIActionExecutor) executeWorkflowCancel(ctx context.Context, input json.RawMessage, action *ui_contribution.UIActionDefinition) (json.RawMessage, error) {
	if e.workflowExecutor == nil {
		return nil, fmt.Errorf("workflow executor unavailable for action %s", action.ActionID)
	}

	var p struct {
		ExecutionID string `json:"executionId"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &p); err != nil {
			return nil, fmt.Errorf("workflow cancel input invalid: %w", err)
		}
	}
	if p.ExecutionID == "" {
		return nil, fmt.Errorf("workflow cancel requires executionId")
	}

	cancelled := e.workflowExecutor.Cancel(p.ExecutionID)
	return json.Marshal(map[string]any{
		"executionID": p.ExecutionID,
		"cancelled":   cancelled,
	})
}

func (e *UIActionExecutor) executeWorkflowStatus(ctx context.Context, input json.RawMessage, action *ui_contribution.UIActionDefinition) (json.RawMessage, error) {
	if e.runStore == nil {
		return nil, fmt.Errorf("workflow run store unavailable for action %s", action.ActionID)
	}

	var p struct {
		ExecutionID string `json:"executionId"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &p); err != nil {
			return nil, fmt.Errorf("workflow status input invalid: %w", err)
		}
	}
	if p.ExecutionID == "" {
		return nil, fmt.Errorf("workflow status requires executionId")
	}

	run, err := e.runStore.Get(ctx, p.ExecutionID)
	if err != nil {
		return nil, fmt.Errorf("workflow status query failed: %w", err)
	}
	if run == nil {
		return nil, fmt.Errorf("workflow run %s not found", p.ExecutionID)
	}

	var finishedAt any
	if run.FinishedAt != nil {
		finishedAt = run.FinishedAt
	}

	return json.Marshal(map[string]any{
		"workflowRunID": run.ExecutionID,
		"workflowID":    run.WorkflowID,
		"status":        string(run.Status),
		"startedAt":     run.StartedAt,
		"finishedAt":    finishedAt,
		"error":         run.Error,
	})
}

func (e *UIActionExecutor) executeNavigation(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage, identity runtime_supervisor.RuntimeIdentity) (json.RawMessage, error) {
	target := action.Target.RouteID
	if target == "" {
		target = action.Target.Resource
	}
	if target == "" {
		target = action.ActionID
	}
	navInput, _ := json.Marshal(map[string]any{
		"target": target,
	})
	callReq := host_api.CallRequest{
		CallID:               fmt.Sprintf("ui-action-nav-%s-%s", execCtx.SessionID, uuid.NewString()),
		RuntimeIdentity:      identity,
		Method:               host_api.MethodUINavigate,
		Version:              1,
		Input:                navInput,
		ScopeSnapshotID:      execCtx.ScopeSnapshotID,
		PermissionSnapshotID: execCtx.PermissionSnapshotID,
	}
	result := e.hostAPIGateway.Call(ctx, callReq)
	if result.Error != nil {
		return nil, fmt.Errorf("navigation action %s failed: %s", action.ActionID, result.Error.Message)
	}
	return result.Output, nil
}

func (e *UIActionExecutor) executeDialog(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage, identity runtime_supervisor.RuntimeIdentity) (json.RawMessage, error) {
	dialogID := action.Target.DialogID
	if dialogID == "" {
		dialogID = action.ActionID
	}

	var in struct {
		Message string   `json:"message"`
		Buttons []string `json:"buttons"`
	}
	if len(input) > 0 {
		_ = json.Unmarshal(input, &in)
	}
	if in.Message == "" {
		in.Message = action.Title.Resolve("")
	}

	dialogInput, _ := json.Marshal(map[string]any{
		"dialogId": dialogID,
		"message":  in.Message,
		"buttons":  in.Buttons,
	})
	callReq := host_api.CallRequest{
		CallID:               fmt.Sprintf("ui-action-dialog-%s-%s", execCtx.SessionID, uuid.NewString()),
		RuntimeIdentity:      identity,
		Method:               host_api.MethodUIDialog,
		Version:              1,
		Input:                dialogInput,
		ScopeSnapshotID:      execCtx.ScopeSnapshotID,
		PermissionSnapshotID: execCtx.PermissionSnapshotID,
	}
	result := e.hostAPIGateway.Call(ctx, callReq)
	if result.Error != nil {
		return nil, fmt.Errorf("dialog action %s failed: %s", action.ActionID, result.Error.Message)
	}
	return result.Output, nil
}

func (e *UIActionExecutor) executeClipboardWrite(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage, identity runtime_supervisor.RuntimeIdentity) (json.RawMessage, error) {
	var p struct {
		Text string `json:"text"`
	}
	if len(input) > 0 {
		_ = json.Unmarshal(input, &p)
	}
	if p.Text == "" {
		p.Text = action.Title.Resolve("")
	}
	clipInput, _ := json.Marshal(map[string]any{
		"text": p.Text,
	})
	callReq := host_api.CallRequest{
		CallID:               fmt.Sprintf("ui-action-clip-%s-%s", execCtx.SessionID, uuid.NewString()),
		RuntimeIdentity:      identity,
		Method:               host_api.MethodClipboardWrite,
		Version:              1,
		Input:                clipInput,
		ScopeSnapshotID:      execCtx.ScopeSnapshotID,
		PermissionSnapshotID: execCtx.PermissionSnapshotID,
	}
	result := e.hostAPIGateway.Call(ctx, callReq)
	if result.Error != nil {
		return nil, fmt.Errorf("clipboard action %s failed: %s", action.ActionID, result.Error.Message)
	}
	return result.Output, nil
}

func (e *UIActionExecutor) executeHostCommand(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage, identity runtime_supervisor.RuntimeIdentity) (json.RawMessage, error) {
	command := action.Target.Command
	if command == "" {
		command = action.ActionID
	}
	if e.hostCommandRegistry == nil {
		return nil, fmt.Errorf("host command registry not configured")
	}

	hostCmdExecCtx := HostCommandExecContext{
		ExtensionID:          execCtx.ExtensionID,
		ModuleID:             execCtx.ModuleID,
		Generation:           execCtx.Generation,
		SessionID:            execCtx.SessionID,
		ScopeSnapshotID:      execCtx.ScopeSnapshotID,
		PermissionSnapshotID: execCtx.PermissionSnapshotID,
	}

	operationID := fmt.Sprintf("host-cmd-%s", uuid.NewString())
	invocationID := fmt.Sprintf("host-cmd-inv-%s", uuid.NewString())
	now := time.Now().UTC()

	if e.operationRepo != nil {
		op := sqlite.Operation{
			OperationID:   operationID,
			OperationType: "host_command",
			ExtensionID:   domain.ExtensionID(execCtx.ExtensionID),
			Status:        "running",
			StartedAt:     now,
		}
		_ = e.operationRepo.PutOperation(ctx, op)
	}

	result, err := e.hostCommandRegistry.Execute(ctx, command, hostCmdExecCtx, input)

	finishedAt := time.Now().UTC()
	if e.operationRepo != nil {
		op := sqlite.Operation{
			OperationID:   operationID,
			OperationType: "host_command",
			ExtensionID:   domain.ExtensionID(execCtx.ExtensionID),
			FinishedAt:    &finishedAt,
		}
		inv := sqlite.Invocation{
			InvocationID:   invocationID,
			OperationID:    operationID,
			ContributionID: execCtx.ContributionID,
			StartedAt:      now,
			FinishedAt:     &finishedAt,
		}
		if err != nil {
			op.Status = "failed"
			inv.Status = "failed"
			var hcErr *HostCommandError
			if AsHostCommandError(err, &hcErr) {
				op.ErrorCode = hcErr.Code
				op.ErrorMessage = hcErr.Message
				inv.ErrorCode = hcErr.Code
			} else {
				op.ErrorMessage = err.Error()
				inv.ErrorCode = "HOST_COMMAND_EXECUTION_FAILED"
			}
		} else {
			op.Status = "succeeded"
			inv.Status = "succeeded"
		}
		_ = e.operationRepo.PutOperation(ctx, op)
		_ = e.operationRepo.PutInvocation(ctx, inv)
	}

	if err != nil {
		var hcErr *HostCommandError
		if AsHostCommandError(err, &hcErr) {
			return nil, fmt.Errorf("host command action %s failed: %s: %s", action.ActionID, hcErr.Code, hcErr.Message)
		}
		return nil, fmt.Errorf("host command action %s failed: %w", action.ActionID, err)
	}
	return result, nil
}

func (e *UIActionExecutor) executeOpenResource(ctx context.Context, execCtx UIActionExecContext, action *ui_contribution.UIActionDefinition, input json.RawMessage, identity runtime_supervisor.RuntimeIdentity) (json.RawMessage, error) {
	resource := action.Target.Resource
	if resource == "" {
		resource = action.ActionID
	}
	resInput, _ := json.Marshal(map[string]any{
		"path": resource,
		"mode": "r",
	})
	callReq := host_api.CallRequest{
		CallID:               fmt.Sprintf("ui-action-res-%s-%s", execCtx.SessionID, uuid.NewString()),
		RuntimeIdentity:      identity,
		Method:               host_api.MethodResourceOpen,
		Version:              1,
		Input:                resInput,
		ScopeSnapshotID:      execCtx.ScopeSnapshotID,
		PermissionSnapshotID: execCtx.PermissionSnapshotID,
	}
	result := e.hostAPIGateway.Call(ctx, callReq)
	if result.Error != nil {
		return nil, fmt.Errorf("open resource action %s failed: %s", action.ActionID, result.Error.Message)
	}
	return result.Output, nil
}

func findActionByID(def *ui_contribution.UIContributionDefinition, actionID string) *ui_contribution.UIActionDefinition {
	if def == nil {
		return nil
	}
	for i := range def.Actions {
		if def.Actions[i].ActionID == actionID {
			return &def.Actions[i]
		}
	}
	return nil
}
