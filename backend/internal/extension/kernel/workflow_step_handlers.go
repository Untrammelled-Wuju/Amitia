package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type workflowToolStepHandler struct {
	kernel execution.ExecutionSecurityKernel
	router *WorkflowExecutionRouter
}

func (h workflowToolStepHandler) Execute(ctx context.Context, node workflow.WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	if h.kernel == nil {
		return nil, fmt.Errorf("workflow tool execution kernel not configured")
	}
	execCtx, ok := workflow.ExecutionContextFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("workflow execution context missing")
	}
	targetID := node.TargetID
	if targetID == "" {
		targetID = node.Runtime.RuntimeID
	}
	if targetID == "" {
		return nil, fmt.Errorf("workflow tool target missing")
	}
	invocationID := fmt.Sprintf("%s/%s", execCtx.InvocationID, node.ID)
	workflow.UpdateExecutionTraceMetadata(ctx, node.ExecutionTarget.DeviceID, node.ExecutionTarget.RuntimeID, invocationID)
	invocation := workflowInvocation(execCtx, invocationID)
	if node.ExecutionTarget.Placement != "" {
		resolved, err := h.router.ResolveNode(ctx, node, execCtx)
		if err != nil {
			return nil, err
		}
		workflow.UpdateExecutionTraceMetadata(ctx, string(resolved.InvocationTarget.DeviceID), string(resolved.InvocationTarget.RuntimeID), invocationID)
		invocation.ExecutionTarget = resolved.InvocationTarget
	}
	result := h.kernel.Execute(ctx, execution.ToolExecutionRequest{
		ToolID:     capability.CapabilityID(targetID),
		Input:      input,
		Invocation: invocation,
	})
	return workflowStepOutput(result)
}

type workflowRuntimeStepHandler struct {
	registry    *capability.RuntimeAdapterRegistry
	router      *WorkflowExecutionRouter
	runtimeType capability.RuntimeType
}

func (h workflowRuntimeStepHandler) Execute(ctx context.Context, node workflow.WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	if h.registry == nil {
		return nil, fmt.Errorf("workflow runtime registry not configured")
	}
	execCtx, ok := workflow.ExecutionContextFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("workflow execution context missing")
	}
	binding := node.Runtime
	if binding.RuntimeType == "" {
		binding.RuntimeType = h.runtimeType
	}
	if binding.RuntimeID == "" {
		binding.RuntimeID = node.TargetID
	}
	invocationID := fmt.Sprintf("%s/%s", execCtx.InvocationID, node.ID)
	workflow.UpdateExecutionTraceMetadata(ctx, node.ExecutionTarget.DeviceID, binding.RuntimeID, invocationID)
	invocation := workflowInvocation(execCtx, invocationID)
	if node.ExecutionTarget.Placement != "" {
		resolved, err := h.router.ResolveNode(ctx, node, execCtx)
		if err != nil {
			return nil, err
		}
		workflow.UpdateExecutionTraceMetadata(ctx, string(resolved.InvocationTarget.DeviceID), string(resolved.InvocationTarget.RuntimeID), invocationID)
		invocation.ExecutionTarget = resolved.InvocationTarget
		adapter, ok := h.registry.ResolveRoute(resolved.Route)
		if !ok {
			return nil, fmt.Errorf("workflow runtime adapter not found for resolved target: %s", resolved.Route.Binding.RuntimeType)
		}
		var result capability.UnifiedToolResult
		if routed, ok := adapter.(capability.RoutedRuntimeAdapter); ok {
			result = routed.ExecuteRoute(ctx, resolved.Route, invocation, input)
		} else {
			result = adapter.Execute(ctx, resolved.Route.Binding, invocation, input)
		}
		return workflowStepOutput(result)
	}
	adapter, ok := h.registry.Resolve(binding)
	if !ok {
		return nil, fmt.Errorf("workflow runtime adapter not found: %s", binding.RuntimeType)
	}
	result := adapter.Execute(ctx, binding, invocation, input)
	return workflowStepOutput(result)
}

func workflowInvocation(execCtx workflow.ExecutionContext, invocationID string) capability.ToolInvocationContext {
	source := capability.InvocationSourceWorkflow
	background := false
	if execCtx.ScheduleID != "" {
		source = capability.InvocationSourceScheduledTask
		background = true
	}
	rootID := execCtx.RootID
	if rootID == "" {
		rootID = execCtx.InvocationID
	}
	operationID := execCtx.OperationID
	if operationID == "" {
		operationID = execCtx.InvocationID
	}
	return capability.ToolInvocationContext{
		InvocationID:         invocationID,
		ParentID:             execCtx.InvocationID,
		RootID:               rootID,
		UserID:               execCtx.UserID,
		CharacterID:          execCtx.CharacterID,
		ConversationID:       execCtx.ConversationID,
		ExtensionID:          execCtx.ExtensionID,
		ModuleID:             execCtx.ModuleID,
		Generation:           execCtx.Generation,
		Source:               source,
		IdempotencyKey:       execCtx.IdempotencyKey,
		WorkflowRunID:        execCtx.InvocationID,
		WorkflowNodeID:       execCtx.NodeID,
		LogicalAttempt:       execCtx.LogicalAttempt,
		FencingToken:         execCtx.FencingToken,
		TraceID:              execCtx.TraceID,
		ScheduleID:           execCtx.ScheduleID,
		TriggerID:            execCtx.TriggerID,
		OperationID:          operationID,
		ScopeSnapshotID:      execCtx.ScopeSnapshotID,
		PermissionSnapshotID: execCtx.PermissionSnapID,
		IsBackground:         background,
	}
}

func workflowStepOutput(result capability.UnifiedToolResult) (json.RawMessage, error) {
	if result.Status != capability.ToolResultStatusSuccess {
		if result.Error != nil {
			return nil, result.Error
		}
		return nil, fmt.Errorf("workflow step failed: %s", result.Status)
	}
	if len(result.Structured) > 0 {
		return result.Structured, nil
	}
	for _, content := range result.Content {
		if content.Type == capability.ToolContentStructured && len(content.Data) > 0 {
			return content.Data, nil
		}
		if content.Text != "" {
			if json.Valid([]byte(content.Text)) {
				return json.RawMessage(content.Text), nil
			}
			encoded, err := json.Marshal(content.Text)
			if err != nil {
				return nil, err
			}
			return encoded, nil
		}
	}
	return json.RawMessage(`{}`), nil
}

func registerWorkflowStepHandlers(executor *workflow.WorkflowExecutor, executionKernel execution.ExecutionSecurityKernel, registry *capability.RuntimeAdapterRegistry, router *WorkflowExecutionRouter) {
	executor.RegisterHandler("tool", workflowToolStepHandler{kernel: executionKernel, router: router})
	runtimeTypes := map[string]capability.RuntimeType{
		"task":            capability.RuntimeTypeTask,
		"mcp":             capability.RuntimeTypeMCP,
		"javascript":      capability.RuntimeTypeJavaScript,
		"wasm":            capability.RuntimeTypeWASM,
		"trusted_service": capability.RuntimeTypeTrustedService,
		"trusted service": capability.RuntimeTypeTrustedService,
	}
	for name, runtimeType := range runtimeTypes {
		executor.RegisterHandler(strings.ToLower(name), workflowRuntimeStepHandler{registry: registry, router: router, runtimeType: runtimeType})
	}
	executor.RegisterHandler("nested_workflow", workflow.NestedWorkflowHandler{Executor: executor})
	executor.RegisterHandler("nested workflow", workflow.NestedWorkflowHandler{Executor: executor})
	executor.RegisterHandler("condition", workflow.ConditionHandler{})
	executor.RegisterHandler("logic", workflow.LogicHandler{})
	executor.RegisterHandler("extract", workflow.ExtractHandler{})
	executor.RegisterHandler("transform", workflow.TransformHandler{})
	executor.RegisterHandler("wait", workflow.WaitHandler{})
}
