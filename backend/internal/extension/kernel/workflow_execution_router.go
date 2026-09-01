package kernel

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

// ResolvedWorkflowExecutionTarget is the canonical node-routing decision used
// by Workflow handlers. Tool, MCP, Task, JS, WASM and trusted-service handlers
// share this decision instead of interpreting deviceId/provider fields
// independently.
type ResolvedWorkflowExecutionTarget struct {
	InvocationTarget capability.InvocationExecutionTarget
	Route            capability.RuntimeExecutionRoute
	Explicit         bool
}

type WorkflowExecutionRouter struct {
	capabilities *capability.CapabilityService
	tools        *capability.ToolRegistry
	tasks        *task_runtime.TaskRuntimeService
}

type workflowCapabilityResolutionError struct {
	Decision capability.RoutingDecision
	Err      error
}

func (e *workflowCapabilityResolutionError) Error() string {
	if e == nil || e.Err == nil {
		return "workflow capability resolution failed"
	}
	return e.Err.Error()
}

func (e *workflowCapabilityResolutionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func capabilityResolutionDeviceOffline(err error) bool {
	var resolutionErr *workflowCapabilityResolutionError
	return errors.As(err, &resolutionErr) && resolutionErr.Decision == capability.RoutingDeviceOffline
}

func NewWorkflowExecutionRouter(capabilities *capability.CapabilityService, tools *capability.ToolRegistry, tasks *task_runtime.TaskRuntimeService) *WorkflowExecutionRouter {
	return &WorkflowExecutionRouter{capabilities: capabilities, tools: tools, tasks: tasks}
}

// ResolveNode resolves both capability-backed Tool nodes and runtime-backed
// nodes. A runtime ID (Task definition, MCP server, JS module, etc.) is NOT a
// capability ID, so runtime nodes must never blindly feed RuntimeID into the
// capability resolver. Core execution keeps the original runtime binding;
// remote execution first resolves a provider whose runtime/module matches the
// binding. Task nodes are special only in transport: the TaskRuntime remains
// the local coordinator while its resolved execution target points at the
// selected device.
func (r *WorkflowExecutionRouter) ResolveNode(ctx context.Context, node workflow.WorkflowNode, execCtx workflow.ExecutionContext) (ResolvedWorkflowExecutionTarget, error) {
	target := node.ExecutionTarget
	if target.Placement == "" {
		return ResolvedWorkflowExecutionTarget{}, nil
	}
	if err := target.Validate(); err != nil {
		return ResolvedWorkflowExecutionTarget{}, err
	}
	if r == nil {
		return ResolvedWorkflowExecutionTarget{}, fmt.Errorf("workflow execution router unavailable")
	}

	if strings.EqualFold(strings.TrimSpace(node.Type), "tool") {
		return r.resolveToolNode(ctx, node, execCtx)
	}
	return r.resolveRuntimeNode(ctx, node, execCtx)
}

func (r *WorkflowExecutionRouter) resolveToolNode(ctx context.Context, node workflow.WorkflowNode, execCtx workflow.ExecutionContext) (ResolvedWorkflowExecutionTarget, error) {
	if r.capabilities == nil {
		return ResolvedWorkflowExecutionTarget{}, fmt.Errorf("workflow execution router: capability service unavailable")
	}
	target := node.ExecutionTarget
	targetID := strings.TrimSpace(node.TargetID)
	if targetID == "" {
		targetID = strings.TrimSpace(node.Runtime.RuntimeID)
	}
	if targetID == "" {
		return ResolvedWorkflowExecutionTarget{}, fmt.Errorf("workflow execution router: node %s has no capability target", node.ID)
	}
	capID := capability.CapabilityID(targetID)
	if r.tools != nil {
		if def, ok := r.tools.Get(ctx, targetID); ok && def.CapabilityID != "" {
			capID = def.CapabilityID
		} else if def, ok := r.tools.GetByModelName(ctx, targetID); ok && def.CapabilityID != "" {
			capID = def.CapabilityID
		}
	}

	resolution, err := r.resolveCapability(capID, "", target, execCtx)
	if err != nil {
		return ResolvedWorkflowExecutionTarget{}, r.wrapDeviceUnavailable(target, err)
	}
	return r.resolvedFromCapability(node, resolution, target)
}

func (r *WorkflowExecutionRouter) resolveRuntimeNode(ctx context.Context, node workflow.WorkflowNode, execCtx workflow.ExecutionContext) (ResolvedWorkflowExecutionTarget, error) {
	target := node.ExecutionTarget
	binding := node.Runtime
	if binding.RuntimeID == "" {
		binding.RuntimeID = strings.TrimSpace(node.TargetID)
	}
	if binding.RuntimeType == "" {
		binding.RuntimeType = runtimeTypeForWorkflowNode(node.Type)
	}
	if binding.RuntimeType == "" {
		return ResolvedWorkflowExecutionTarget{}, fmt.Errorf("workflow execution router: node %s has no runtime type", node.ID)
	}

	// local/cloud are both Core placement. Which Core is selected by the
	// WorkflowInstallation/API target (local backend vs Cloud Core), not by a
	// second runtime-provider lookup. This preserves Task/MCP/JS/WASM runtime
	// IDs as runtime IDs rather than misinterpreting them as capability IDs.
	if target.Placement == workflow.WorkflowExecutionLocal || target.Placement == workflow.WorkflowExecutionCloud {
		return ResolvedWorkflowExecutionTarget{
			InvocationTarget: capability.InvocationExecutionTarget{
				Placement: string(capability.ProviderPlacementCore),
				UserID:    runtimeidentity.UserID(execCtx.UserID),
			},
			Route: capability.RuntimeExecutionRoute{
				Binding:   binding,
				Placement: capability.ProviderPlacementCore,
				UserID:    runtimeidentity.UserID(execCtx.UserID),
			},
			Explicit: true,
		}, nil
	}

	if target.Placement != workflow.WorkflowExecutionDevice && target.Placement != workflow.WorkflowExecutionAuto {
		return ResolvedWorkflowExecutionTarget{}, fmt.Errorf("workflow execution router: unsupported runtime placement %q", target.Placement)
	}
	if r.capabilities == nil {
		return ResolvedWorkflowExecutionTarget{}, r.wrapDeviceUnavailable(target, fmt.Errorf("workflow execution router: capability service unavailable"))
	}

	var candidates []*capability.CapabilityProviderDefinition
	if binding.RuntimeType == capability.RuntimeTypeTask {
		defs, err := r.taskProviderCandidates(ctx, binding, node)
		if err != nil {
			return ResolvedWorkflowExecutionTarget{}, r.wrapDeviceUnavailable(target, err)
		}
		candidates = defs
	} else {
		candidates = r.runtimeProviderCandidates(binding, node)
	}
	if len(candidates) == 0 {
		return ResolvedWorkflowExecutionTarget{}, r.wrapDeviceUnavailable(target, fmt.Errorf("workflow execution router: runtime %s/%s is not published by a device capability provider", binding.RuntimeType, binding.RuntimeID))
	}

	var lastErr error
	var offlineErr error
	for _, provider := range candidates {
		if provider == nil || provider.CapabilityID == "" || provider.ID == "" {
			continue
		}
		resolution, err := r.resolveCapability(provider.CapabilityID, provider.ID, target, execCtx)
		if err != nil {
			lastErr = err
			if capabilityResolutionDeviceOffline(err) {
				offlineErr = err
			}
			continue
		}

		// TaskRuntime owns Task lifecycle/checkpoint/recovery and therefore stays
		// on the current Core. Only its execution target is device-resolved; the
		// Task adapter binds that target to TaskRun before dispatch.
		if binding.RuntimeType == capability.RuntimeTypeTask {
			return ResolvedWorkflowExecutionTarget{
				InvocationTarget: resolution.ExecutionTarget,
				Route: capability.RuntimeExecutionRoute{
					Binding:   binding,
					Placement: capability.ProviderPlacementCore,
					UserID:    runtimeidentity.UserID(execCtx.UserID),
				},
				Explicit: true,
			}, nil
		}

		resolved, err := r.resolvedFromCapability(node, resolution, target)
		if err != nil {
			lastErr = err
			continue
		}
		return resolved, nil
	}
	if offlineErr != nil {
		lastErr = offlineErr
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("workflow execution router: no executable device provider for runtime %s/%s", binding.RuntimeType, binding.RuntimeID)
	}
	return ResolvedWorkflowExecutionTarget{}, r.wrapDeviceUnavailable(target, lastErr)
}

func (r *WorkflowExecutionRouter) resolveCapability(capID capability.CapabilityID, requiredProvider capability.ProviderID, target workflow.WorkflowExecutionTarget, execCtx workflow.ExecutionContext) (capability.CapabilityResolution, error) {
	request := capability.CapabilityResolutionRequest{
		CapabilityID: capID,
		UserID:       runtimeidentity.UserID(execCtx.UserID),
		AllowCore:    true,
		AllowDevice:  true,
	}
	if requiredProvider != "" {
		request.RequiredProviderID = requiredProvider
	}
	if target.ProviderID != "" {
		request.RequiredProviderID = capability.ProviderID(target.ProviderID)
		request.PreferredProviderID = capability.ProviderID(target.ProviderID)
	}
	if target.RuntimeID != "" {
		request.PreferredRuntimeID = runtimeidentity.RuntimeID(target.RuntimeID)
	}

	switch target.Placement {
	case workflow.WorkflowExecutionLocal, workflow.WorkflowExecutionCloud:
		request.RequiredPlacement = capability.ProviderPlacementCore
		request.AllowDevice = false
	case workflow.WorkflowExecutionDevice:
		request.RequiredPlacement = capability.ProviderPlacementDevice
		request.RequiredDeviceID = runtimeidentity.DeviceID(target.DeviceID)
		request.AllowCore = false
	case workflow.WorkflowExecutionAuto:
		// auto is intentionally device-only. Capability ranking chooses an online
		// permitted device; it never randomly falls back to Cloud Core.
		request.RequiredPlacement = capability.ProviderPlacementDevice
		request.AllowCore = false
	default:
		return capability.CapabilityResolution{}, fmt.Errorf("workflow execution router: unsupported placement %q", target.Placement)
	}

	resolution, err := r.capabilities.Resolve(request)
	if err != nil {
		return resolution, &workflowCapabilityResolutionError{
			Decision: resolution.Decision,
			Err:      fmt.Errorf("workflow execution router: resolve %s: %w", capID, err),
		}
	}
	if !resolution.HasResult() {
		return capability.CapabilityResolution{}, fmt.Errorf("workflow execution router: no executable provider for %s", capID)
	}
	if target.ProviderInstanceID != "" && string(resolution.ProviderInstance.ID) != target.ProviderInstanceID {
		return capability.CapabilityResolution{}, fmt.Errorf("workflow execution router: requested provider instance %s is unavailable", target.ProviderInstanceID)
	}
	if target.RuntimeID != "" && resolution.ProviderInstance.RuntimeID != runtimeidentity.RuntimeID(target.RuntimeID) {
		return capability.CapabilityResolution{}, fmt.Errorf("workflow execution router: requested runtime %s is unavailable", target.RuntimeID)
	}
	return resolution, nil
}

func (r *WorkflowExecutionRouter) resolvedFromCapability(node workflow.WorkflowNode, resolution capability.CapabilityResolution, target workflow.WorkflowExecutionTarget) (ResolvedWorkflowExecutionTarget, error) {
	binding := resolution.Provider.Runtime
	if node.Runtime.HandlerName != "" {
		binding.HandlerName = node.Runtime.HandlerName
	}
	if node.Runtime.Endpoint != "" {
		binding.Endpoint = node.Runtime.Endpoint
	}
	if node.Runtime.Metadata != nil {
		binding.Metadata = node.Runtime.Metadata
	}

	invTarget := resolution.ExecutionTarget
	if target.ProviderInstanceID != "" {
		invTarget.ProviderInstanceID = target.ProviderInstanceID
	}
	if target.ProviderID != "" {
		invTarget.ProviderID = target.ProviderID
	}

	return ResolvedWorkflowExecutionTarget{
		InvocationTarget: invTarget,
		Route: capability.RuntimeExecutionRoute{
			Binding:                   binding,
			Placement:                 resolution.Provider.Placement,
			ProviderID:                resolution.Provider.ID,
			ProviderInstanceID:        resolution.ProviderInstance.ID,
			ProviderRuntimeInstanceID: resolution.ProviderInstance.RuntimeInstanceID,
			UserID:                    resolution.ProviderInstance.UserID,
			DeviceID:                  resolution.ProviderInstance.DeviceID,
			RuntimeID:                 resolution.ProviderInstance.RuntimeID,
			RemoteDevice:              resolution.Provider.Placement == capability.ProviderPlacementDevice,
		},
		Explicit: true,
	}, nil
}

func (r *WorkflowExecutionRouter) runtimeProviderCandidates(binding capability.RuntimeBinding, node workflow.WorkflowNode) []*capability.CapabilityProviderDefinition {
	if r.capabilities == nil {
		return nil
	}
	providerID := strings.TrimSpace(node.ExecutionTarget.ProviderID)
	if providerID == "" {
		providerID = strings.TrimSpace(binding.ProviderID)
	}
	runtimeID := strings.TrimSpace(binding.RuntimeID)
	targetID := strings.TrimSpace(node.TargetID)
	result := make([]*capability.CapabilityProviderDefinition, 0)
	for _, def := range r.capabilities.ListProviders() {
		if def == nil || def.Placement != capability.ProviderPlacementDevice {
			continue
		}
		if providerID != "" && string(def.ID) != providerID {
			continue
		}
		if !runtimeTypesCompatible(def.Runtime.RuntimeType, binding.RuntimeType) {
			continue
		}
		if runtimeID != "" && def.Runtime.RuntimeID != runtimeID && def.ModuleID != runtimeID {
			continue
		}
		if runtimeID == "" && targetID != "" && def.Runtime.RuntimeID != targetID && def.ModuleID != targetID {
			continue
		}
		result = append(result, def)
	}
	return result
}

func (r *WorkflowExecutionRouter) taskProviderCandidates(ctx context.Context, binding capability.RuntimeBinding, node workflow.WorkflowNode) ([]*capability.CapabilityProviderDefinition, error) {
	if r.tasks == nil {
		return nil, fmt.Errorf("workflow execution router: task runtime service unavailable")
	}
	taskID := strings.TrimSpace(binding.RuntimeID)
	if taskID == "" {
		taskID = strings.TrimSpace(node.TargetID)
	}
	if taskID == "" {
		return nil, fmt.Errorf("workflow execution router: task definition id is required")
	}
	def, err := r.tasks.GetTaskDefinition(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("workflow execution router: get task definition %s: %w", taskID, err)
	}
	if def == nil {
		return nil, fmt.Errorf("workflow execution router: task definition %s not found", taskID)
	}
	providerID := strings.TrimSpace(node.ExecutionTarget.ProviderID)
	result := make([]*capability.CapabilityProviderDefinition, 0)
	for _, provider := range r.capabilities.ListProviders() {
		if provider == nil || provider.Placement != capability.ProviderPlacementDevice {
			continue
		}
		if !runtimeTypesCompatible(provider.Runtime.RuntimeType, capability.RuntimeTypeTask) {
			continue
		}
		if providerID != "" && string(provider.ID) != providerID {
			continue
		}
		if def.ExtensionID != "" && provider.ExtensionID != def.ExtensionID {
			continue
		}
		if def.ModuleID != "" && provider.ModuleID != def.ModuleID {
			continue
		}
		result = append(result, provider)
	}
	return result, nil
}

func (r *WorkflowExecutionRouter) wrapDeviceUnavailable(target workflow.WorkflowExecutionTarget, err error) error {
	if err == nil {
		return nil
	}
	if target.OfflinePolicy == workflow.WorkflowOfflineWait &&
		(target.Placement == workflow.WorkflowExecutionDevice || target.Placement == workflow.WorkflowExecutionAuto) &&
		capabilityResolutionDeviceOffline(err) {
		return &workflow.WorkflowDeviceUnavailableError{DeviceID: target.DeviceID, Cause: err}
	}
	return err
}

func runtimeTypeForWorkflowNode(nodeType string) capability.RuntimeType {
	switch strings.ToLower(strings.TrimSpace(nodeType)) {
	case "task":
		return capability.RuntimeTypeTask
	case "mcp":
		return capability.RuntimeTypeMCP
	case "javascript":
		return capability.RuntimeTypeJavaScript
	case "wasm":
		return capability.RuntimeTypeWASM
	case "trusted_service", "trusted service":
		return capability.RuntimeTypeTrustedService
	default:
		return ""
	}
}

func runtimeTypesCompatible(providerType, requestedType capability.RuntimeType) bool {
	if providerType == requestedType {
		return true
	}
	if requestedType == capability.RuntimeTypeJavaScript && providerType == capability.RuntimeTypePluginJS {
		return true
	}
	if requestedType == capability.RuntimeTypeTrustedService && providerType == capability.RuntimeTypePluginService {
		return true
	}
	return false
}
