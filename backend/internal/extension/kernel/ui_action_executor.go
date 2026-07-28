package kernel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
)

type UIActionExecutor struct {
	hostAPIGateway *host_api.DefaultGateway
}

func NewUIActionExecutor(gateway *host_api.DefaultGateway) *UIActionExecutor {
	return &UIActionExecutor{hostAPIGateway: gateway}
}

func (e *UIActionExecutor) Execute(ctx context.Context, sessionID, extensionID, moduleID string, action *ui_contribution.UIActionDefinition, input json.RawMessage) (json.RawMessage, error) {
	identity := runtime_supervisor.RuntimeIdentity{
		InstanceID:  sessionID,
		ExtensionID: domain.ExtensionID(extensionID),
		ModuleID:    domain.ModuleID(moduleID),
	}

	switch action.Target.Type {
	case ui_contribution.ActionTargetTool:
		return e.executeTool(ctx, sessionID, action, input, identity)
	case ui_contribution.ActionTargetWorkflow:
		return e.executeWorkflow(ctx, sessionID, action, input, identity)
	case ui_contribution.ActionTargetNavigation:
		return e.executeNavigation(ctx, sessionID, action, input, identity)
	case ui_contribution.ActionTargetDialog:
		return e.executeDialog(ctx, sessionID, action, input, identity)
	case ui_contribution.ActionTargetCopy:
		return e.executeClipboardWrite(ctx, sessionID, action, input, identity)
	case ui_contribution.ActionTargetHostCommand:
		return e.executeHostCommand(ctx, sessionID, action, input, identity)
	case ui_contribution.ActionTargetOpenResource:
		return e.executeOpenResource(ctx, sessionID, action, input, identity)
	default:
		return nil, fmt.Errorf("unsupported action target type: %s", action.Target.Type)
	}
}

func (e *UIActionExecutor) executeTool(ctx context.Context, sessionID string, action *ui_contribution.UIActionDefinition, input json.RawMessage, identity runtime_supervisor.RuntimeIdentity) (json.RawMessage, error) {
	toolID := action.Target.ToolID
	if toolID == "" {
		toolID = action.ActionID
	}
	toolInput, _ := json.Marshal(map[string]any{
		"toolId": toolID,
		"input":  json.RawMessage(input),
	})
	callReq := host_api.CallRequest{
		CallID:          fmt.Sprintf("ui-action-tool-%s-%s", sessionID, uuid.NewString()),
		RuntimeIdentity: identity,
		Method:          host_api.MethodToolExecute,
		Version:         1,
		Input:           toolInput,
	}
	result := e.hostAPIGateway.Call(ctx, callReq)
	if result.Error != nil {
		return nil, fmt.Errorf("action %s failed: %s", action.ActionID, result.Error.Message)
	}
	return result.Output, nil
}

func (e *UIActionExecutor) executeWorkflow(ctx context.Context, sessionID string, action *ui_contribution.UIActionDefinition, input json.RawMessage, identity runtime_supervisor.RuntimeIdentity) (json.RawMessage, error) {
	workflowID := action.Target.WorkflowID
	if workflowID == "" {
		workflowID = action.ActionID
	}
	toolInput, _ := json.Marshal(map[string]any{
		"toolId": workflowID,
		"input":  json.RawMessage(input),
	})
	callReq := host_api.CallRequest{
		CallID:          fmt.Sprintf("ui-action-wf-%s-%s", sessionID, uuid.NewString()),
		RuntimeIdentity: identity,
		Method:          host_api.MethodToolExecute,
		Version:         1,
		Input:           toolInput,
	}
	result := e.hostAPIGateway.Call(ctx, callReq)
	if result.Error != nil {
		return nil, fmt.Errorf("workflow action %s failed: %s", action.ActionID, result.Error.Message)
	}
	return result.Output, nil
}

func (e *UIActionExecutor) executeNavigation(ctx context.Context, sessionID string, action *ui_contribution.UIActionDefinition, input json.RawMessage, identity runtime_supervisor.RuntimeIdentity) (json.RawMessage, error) {
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
		CallID:          fmt.Sprintf("ui-action-nav-%s-%s", sessionID, uuid.NewString()),
		RuntimeIdentity: identity,
		Method:          host_api.MethodUINavigate,
		Version:         1,
		Input:           navInput,
	}
	result := e.hostAPIGateway.Call(ctx, callReq)
	if result.Error != nil {
		return nil, fmt.Errorf("navigation action %s failed: %s", action.ActionID, result.Error.Message)
	}
	return result.Output, nil
}

func (e *UIActionExecutor) executeDialog(ctx context.Context, sessionID string, action *ui_contribution.UIActionDefinition, input json.RawMessage, identity runtime_supervisor.RuntimeIdentity) (json.RawMessage, error) {
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
		CallID:          fmt.Sprintf("ui-action-dialog-%s-%s", sessionID, uuid.NewString()),
		RuntimeIdentity: identity,
		Method:          host_api.MethodUIDialog,
		Version:         1,
		Input:           dialogInput,
	}
	result := e.hostAPIGateway.Call(ctx, callReq)
	if result.Error != nil {
		return nil, fmt.Errorf("dialog action %s failed: %s", action.ActionID, result.Error.Message)
	}
	return result.Output, nil
}

func (e *UIActionExecutor) executeClipboardWrite(ctx context.Context, sessionID string, action *ui_contribution.UIActionDefinition, input json.RawMessage, identity runtime_supervisor.RuntimeIdentity) (json.RawMessage, error) {
	return json.Marshal(map[string]any{
		"ok":     true,
		"action": action.ActionID,
		"copied": true,
	})
}

func (e *UIActionExecutor) executeHostCommand(ctx context.Context, sessionID string, action *ui_contribution.UIActionDefinition, input json.RawMessage, identity runtime_supervisor.RuntimeIdentity) (json.RawMessage, error) {
	command := action.Target.Command
	if command == "" {
		command = action.ActionID
	}
	toolInput, _ := json.Marshal(map[string]any{
		"toolId": command,
		"input":  json.RawMessage(input),
	})
	callReq := host_api.CallRequest{
		CallID:          fmt.Sprintf("ui-action-cmd-%s-%s", sessionID, uuid.NewString()),
		RuntimeIdentity: identity,
		Method:          host_api.MethodToolExecute,
		Version:         1,
		Input:           toolInput,
	}
	result := e.hostAPIGateway.Call(ctx, callReq)
	if result.Error != nil {
		return nil, fmt.Errorf("host command action %s failed: %s", action.ActionID, result.Error.Message)
	}
	return result.Output, nil
}

func (e *UIActionExecutor) executeOpenResource(ctx context.Context, sessionID string, action *ui_contribution.UIActionDefinition, input json.RawMessage, identity runtime_supervisor.RuntimeIdentity) (json.RawMessage, error) {
	resource := action.Target.Resource
	if resource == "" {
		resource = action.ActionID
	}
	resInput, _ := json.Marshal(map[string]any{
		"path": resource,
		"mode": "r",
	})
	callReq := host_api.CallRequest{
		CallID:          fmt.Sprintf("ui-action-res-%s-%s", sessionID, uuid.NewString()),
		RuntimeIdentity: identity,
		Method:          host_api.MethodResourceOpen,
		Version:         1,
		Input:           resInput,
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
