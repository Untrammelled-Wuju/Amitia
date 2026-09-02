package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type WorkflowPreflightStatus string

const (
	WorkflowPreflightPass    WorkflowPreflightStatus = "PASS"
	WorkflowPreflightWarning WorkflowPreflightStatus = "WARNING"
	WorkflowPreflightBlocked WorkflowPreflightStatus = "BLOCKED"
)

type WorkflowPreflightCheck struct {
	Code    string                  `json:"code"`
	Status  WorkflowPreflightStatus `json:"status"`
	Message string                  `json:"message"`
	NodeID  string                  `json:"nodeId,omitempty"`
	Details map[string]any          `json:"details,omitempty"`
}

type WorkflowPreflightReport struct {
	Status           WorkflowPreflightStatus  `json:"status"`
	Runnable         bool                     `json:"runnable"`
	WorkflowID       string                   `json:"workflowId"`
	DefinitionHash   string                   `json:"definitionHash,omitempty"`
	SchemaVersion    string                   `json:"schemaVersion,omitempty"`
	TopologicalOrder []string                 `json:"topologicalOrder,omitempty"`
	Checks           []WorkflowPreflightCheck `json:"checks"`
}

func (r *WorkflowPreflightReport) add(code string, status WorkflowPreflightStatus, message, nodeID string, details map[string]any) {
	r.Checks = append(r.Checks, WorkflowPreflightCheck{Code: code, Status: status, Message: message, NodeID: nodeID, Details: details})
	if preflightRank(status) > preflightRank(r.Status) {
		r.Status = status
	}
	r.Runnable = r.Status != WorkflowPreflightBlocked
}

func preflightRank(status WorkflowPreflightStatus) int {
	switch status {
	case WorkflowPreflightBlocked:
		return 3
	case WorkflowPreflightWarning:
		return 2
	case WorkflowPreflightPass:
		return 1
	default:
		return 0
	}
}

func newWorkflowPreflightReport(def workflow.WorkflowDefinition) WorkflowPreflightReport {
	return WorkflowPreflightReport{
		Status:         WorkflowPreflightPass,
		Runnable:       true,
		WorkflowID:     def.ID,
		DefinitionHash: def.DefinitionHash,
		SchemaVersion:  def.SchemaVersion,
		Checks:         make([]WorkflowPreflightCheck, 0, len(def.Nodes)*3+6),
	}
}

func (api *WorkflowAPI) preflightDefinition(ctx context.Context, def workflow.WorkflowDefinition, userID string) WorkflowPreflightReport {
	report := newWorkflowPreflightReport(def)
	if api == nil || api.runtime == nil || api.runtime.Kernel == nil || api.runtime.Kernel.Container() == nil {
		report.add("kernel.available", WorkflowPreflightBlocked, "workflow kernel is unavailable", "", nil)
		return report
	}
	kc := api.runtime.Kernel.Container()

	migrated, err := workflow.MigrateDefinition(def)
	if err != nil {
		report.add("schema.compatible", WorkflowPreflightBlocked, err.Error(), "", map[string]any{"schemaVersion": def.SchemaVersion})
		return report
	}
	def = migrated
	report.SchemaVersion = def.SchemaVersion
	if def.DefinitionHash == "" {
		def.DefinitionHash = workflow.ComputeDefinitionHash(def)
		report.DefinitionHash = def.DefinitionHash
	}
	report.add("schema.compatible", WorkflowPreflightPass, "workflow schema is supported", "", map[string]any{"schemaVersion": def.SchemaVersion})

	if err := api.validateExecutionTargets(def); err != nil {
		report.add("executionTarget.valid", WorkflowPreflightBlocked, err.Error(), "", nil)
	} else {
		report.add("executionTarget.valid", WorkflowPreflightPass, "execution targets are valid for this workflow installation", "", nil)
	}

	compiled, err := workflow.NewCompiler().Compile(def, workflow.DefaultCompileOptions())
	if err != nil {
		report.add("definition.compile", WorkflowPreflightBlocked, err.Error(), "", nil)
	} else {
		report.TopologicalOrder = append([]string(nil), compiled.TopologicalOrder...)
		report.add("definition.compile", WorkflowPreflightPass, "definition, dependencies and DAG compile successfully", "", map[string]any{"nodeCount": len(def.Nodes)})
	}

	api.preflightWorkflowPermissions(ctx, kc, def, userID, &report)
	router := kernelruntime.NewWorkflowExecutionRouter(kc.CapabilityService, kc.ToolRegistry, kc.TaskRuntimeService, kc.DeviceRuntimeSessions)
	execCtx := workflow.ExecutionContext{
		UserID:       strings.TrimSpace(userID),
		WorkflowID:   def.ID,
		ExtensionID:  def.ExtensionID,
		ModuleID:     def.ModuleID,
		InvocationID: "workflow-preflight-" + def.ID,
		RootID:       "workflow-preflight-" + def.ID,
		OperationID:  "workflow-preflight-" + def.ID,
	}

	for _, node := range def.Nodes {
		api.preflightWorkflowNode(ctx, kc, router, def, node, execCtx, &report)
		if node.Compensation == nil {
			continue
		}
		compNode, compErr := workflow.BuildCompensationNode(node)
		if compErr != nil {
			report.add("node.compensation", WorkflowPreflightBlocked, compErr.Error(), node.ID, nil)
			continue
		}
		report.add("node.compensation", WorkflowPreflightPass, "declared Saga compensation is structurally valid", node.ID, map[string]any{"compensationNodeId": compNode.ID})
		api.preflightWorkflowNode(ctx, kc, router, def, compNode, execCtx, &report)
	}

	if report.Status == "" {
		report.Status = WorkflowPreflightPass
		report.Runnable = true
	}
	return report
}

func (api *WorkflowAPI) preflightWorkflowPermissions(ctx context.Context, kc *kernelruntime.Container, def workflow.WorkflowDefinition, userID string, report *WorkflowPreflightReport) {
	permissionsRequired := make([]string, 0, len(def.Permissions))
	permissionsRequired = append(permissionsRequired, def.Permissions...)
	for _, node := range def.Nodes {
		permissionsRequired = append(permissionsRequired, node.Permissions...)
		if node.Compensation != nil {
			permissionsRequired = append(permissionsRequired, node.Compensation.Permissions...)
		}
	}
	permissionsRequired = uniqueWorkflowStrings(permissionsRequired)
	if len(permissionsRequired) == 0 {
		report.add("permissions.available", WorkflowPreflightPass, "workflow does not declare additional permissions", "", nil)
		return
	}
	if kc.PermissionBroker == nil {
		report.add("permissions.available", WorkflowPreflightBlocked, "permission broker is unavailable", "", nil)
		return
	}

	subject := permission.SubjectForExtension(def.ExtensionID)
	if def.ModuleID != "" {
		subject = permission.PermissionSubject{Type: permission.SubjectModule, ID: def.ModuleID, ExtensionID: def.ExtensionID, ModuleID: def.ModuleID}
	}
	requirements := make([]permission.PermissionRequirement, 0, len(permissionsRequired))
	for _, permissionID := range permissionsRequired {
		requirements = append(requirements, permission.PermissionRequirement{PermissionID: permissionID, Scope: permission.ScopeForExtension(def.ExtensionID)})
	}
	decision := kc.PermissionBroker.Evaluate(ctx, permission.PermissionEvaluationRequest{Subject: subject, Requirements: requirements})
	details := map[string]any{"decision": decision.Decision, "required": permissionsRequired}
	switch decision.Decision {
	case permission.DecisionAllow, permission.DecisionAllowPersistent, permission.DecisionAllowOnce, permission.DecisionAllowSession:
		report.add("permissions.available", WorkflowPreflightPass, "declared workflow permissions are available", "", details)
	case permission.DecisionRequireApproval:
		report.add("permissions.available", WorkflowPreflightWarning, "workflow permissions require user approval before execution", "", details)
	default:
		report.add("permissions.available", WorkflowPreflightBlocked, "workflow permissions are denied or unavailable", "", details)
	}
}

func (api *WorkflowAPI) preflightWorkflowNode(ctx context.Context, kc *kernelruntime.Container, router *kernelruntime.WorkflowExecutionRouter, def workflow.WorkflowDefinition, node workflow.WorkflowNode, execCtx workflow.ExecutionContext, report *WorkflowPreflightReport) {
	nodeType := strings.ToLower(strings.TrimSpace(node.Type))
	switch nodeType {
	case "condition", "logic", "extract", "transform", "wait":
		report.add("node.runtime", WorkflowPreflightPass, "built-in workflow node runtime is available", node.ID, map[string]any{"type": node.Type})
	case "nested_workflow", "nested workflow":
		targetID := strings.TrimSpace(node.TargetID)
		if targetID == "" {
			report.add("node.nestedWorkflow", WorkflowPreflightBlocked, "nested workflow target is missing", node.ID, nil)
		} else if nested, ok := kc.WorkflowRegistry.Get(targetID); !ok || !workflowOwnedBy(nested, execCtx.UserID) {
			report.add("node.nestedWorkflow", WorkflowPreflightBlocked, "nested workflow target does not exist or is not accessible", node.ID, map[string]any{"targetId": targetID})
		} else {
			report.add("node.nestedWorkflow", WorkflowPreflightPass, "nested workflow target is available", node.ID, map[string]any{"targetId": targetID})
		}
	case "tool":
		api.preflightWorkflowToolNode(ctx, kc, node, report)
	default:
		binding := node.Runtime
		if binding.RuntimeType == "" {
			binding.RuntimeType = preflightRuntimeType(nodeType)
		}
		if binding.RuntimeID == "" {
			binding.RuntimeID = strings.TrimSpace(node.TargetID)
		}
		if binding.RuntimeType == "" {
			report.add("node.runtime", WorkflowPreflightBlocked, "workflow node has no recognized runtime", node.ID, map[string]any{"type": node.Type})
		} else if kc.AdapterRegistry == nil {
			report.add("node.runtime", WorkflowPreflightBlocked, "runtime adapter registry is unavailable", node.ID, map[string]any{"runtimeType": binding.RuntimeType})
		} else if _, ok := kc.AdapterRegistry.Resolve(binding); !ok && node.ExecutionTarget.Placement != workflow.WorkflowExecutionDevice && node.ExecutionTarget.Placement != workflow.WorkflowExecutionAuto {
			report.add("node.runtime", WorkflowPreflightBlocked, "required runtime adapter is not registered", node.ID, map[string]any{"runtimeType": binding.RuntimeType, "runtimeId": binding.RuntimeID})
		} else {
			report.add("node.runtime", WorkflowPreflightPass, "required runtime adapter is registered", node.ID, map[string]any{"runtimeType": binding.RuntimeType, "runtimeId": binding.RuntimeID})
		}
	}

	placement := node.ExecutionTarget.Placement
	if placement == "" {
		return
	}
	if placement == workflow.WorkflowExecutionDevice || placement == workflow.WorkflowExecutionAuto {
		if router == nil {
			report.add("node.targetReachable", WorkflowPreflightBlocked, "workflow execution router is unavailable", node.ID, nil)
			return
		}
		resolved, err := router.ResolveNode(ctx, node, execCtx)
		if err != nil {
			status := WorkflowPreflightBlocked
			if node.ExecutionTarget.OfflinePolicy == workflow.WorkflowOfflineWait {
				status = WorkflowPreflightWarning
			}
			report.add("node.targetReachable", status, err.Error(), node.ID, map[string]any{"placement": placement, "deviceId": node.ExecutionTarget.DeviceID})
			return
		}
		report.add("node.targetReachable", WorkflowPreflightPass, "device target is reachable and protocol-compatible", node.ID, map[string]any{
			"placement": placement,
			"deviceId":  string(resolved.Route.DeviceID),
			"runtimeId": string(resolved.Route.RuntimeID),
		})
		api.preflightResolvedAndroidDeviceHealth(ctx, node, execCtx.UserID, string(resolved.Route.DeviceID), report)
		return
	}
	report.add("node.targetReachable", WorkflowPreflightPass, "core execution target is reachable", node.ID, map[string]any{"placement": placement})
}

func (api *WorkflowAPI) preflightWorkflowToolNode(ctx context.Context, kc *kernelruntime.Container, node workflow.WorkflowNode, report *WorkflowPreflightReport) {
	if kc.ToolRegistry == nil {
		report.add("node.tool", WorkflowPreflightBlocked, "tool registry is unavailable", node.ID, nil)
		return
	}
	targetID := strings.TrimSpace(node.TargetID)
	if targetID == "" {
		targetID = strings.TrimSpace(node.Runtime.RuntimeID)
	}
	if targetID == "" {
		report.add("node.tool", WorkflowPreflightBlocked, "tool target is missing", node.ID, nil)
		return
	}
	tool, ok := kc.ToolRegistry.Get(ctx, targetID)
	if !ok {
		tool, ok = kc.ToolRegistry.GetByModelName(ctx, targetID)
	}
	if !ok {
		report.add("node.tool", WorkflowPreflightBlocked, "required tool is not registered", node.ID, map[string]any{"targetId": targetID})
		return
	}
	if !tool.Enabled {
		report.add("node.tool", WorkflowPreflightBlocked, "required tool is disabled", node.ID, map[string]any{"toolId": tool.ID})
		return
	}
	state := tool.ComputedState()
	if !state.Executable() {
		status := WorkflowPreflightBlocked
		if state.Health == capability.HealthDegraded && state.PermissionGranted && state.RuntimeReady && state.DependencyReady {
			status = WorkflowPreflightWarning
		}
		report.add("node.tool", status, "required tool is not fully executable", node.ID, map[string]any{"toolId": tool.ID, "state": state, "source": tool.Source})
	} else {
		report.add("node.tool", WorkflowPreflightPass, "required tool is executable", node.ID, map[string]any{"toolId": tool.ID, "source": tool.Source})
	}
	if tool.Source == capability.ToolSourceMCP {
		report.add("node.mcp", WorkflowPreflightPass, "required MCP tool is registered", node.ID, map[string]any{"toolId": tool.ID, "runtimeId": tool.Runtime.RuntimeID})
	}
	for _, rawRef := range tool.SecretReferences {
		ref, err := secret.ParseRef(strings.TrimSpace(rawRef))
		if err != nil || !ref.Valid() {
			report.add("node.secret", WorkflowPreflightBlocked, "tool contains an invalid secret reference", node.ID, map[string]any{"toolId": tool.ID})
			continue
		}
		// The kernel secret broker is deliberately not exposed from Container.
		// Runtime invocation remains the authority for existence/access checks;
		// preflight can still reject malformed references without resolving values.
		report.add("node.secret", WorkflowPreflightWarning, "secret reference syntax is valid; existence/access will be verified by the runtime secret broker", node.ID, map[string]any{"toolId": tool.ID, "secretRef": ref.String()})
	}
}

func (api *WorkflowAPI) preflightResolvedAndroidDeviceHealth(ctx context.Context, node workflow.WorkflowNode, userID, deviceID string, report *WorkflowPreflightReport) {
	if api == nil || api.runtime == nil || api.runtime.WorkflowDeviceControl == nil || report == nil {
		return
	}
	userID = strings.TrimSpace(userID)
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return
	}

	// Android health is an additional dynamic readiness projection, not a
	// replacement for generic Device Mesh routing. Do not block desktop/iOS
	// device nodes merely because they do not expose the Android health mesh
	// operation. If registry platform metadata is unavailable, only probe nodes
	// that explicitly require Android-specific capabilities.
	isAndroid, platformKnown := api.workflowPreflightDeviceIsAndroid(ctx, userID, deviceID)
	if platformKnown && !isAndroid {
		return
	}
	if !platformKnown && !workflowNodeExplicitlyRequiresAndroid(node) {
		return
	}
	raw, err := api.runtime.WorkflowDeviceControl.Invoke(ctx, userID, deviceID, WorkflowMeshAndroidRuntimeHealth, json.RawMessage(`{}`))
	if err != nil {
		status := WorkflowPreflightWarning
		if node.ExecutionTarget.OfflinePolicy != workflow.WorkflowOfflineWait {
			status = WorkflowPreflightBlocked
		}
		report.add("node.deviceHealth", status, "device runtime health is unavailable: "+err.Error(), node.ID, map[string]any{"deviceId": deviceID})
		return
	}
	var health WorkflowAndroidRuntimeHealthStatus
	if err := json.Unmarshal(raw, &health); err != nil {
		report.add("node.deviceHealth", WorkflowPreflightBlocked, "device runtime returned invalid health status", node.ID, map[string]any{"deviceId": deviceID})
		return
	}
	details := map[string]any{
		"deviceId":                     deviceID,
		"runtimeReady":                 health.RuntimeReady,
		"nativeBridgeReady":            health.NativeBridgeReady,
		"accessibilityConfigured":      health.AccessibilityConfigured,
		"accessibilityEnabled":         health.AccessibilityEnabled,
		"accessibilityReady":           health.AccessibilityReady,
		"screenCaptureReady":           health.ScreenCaptureReady,
		"microphoneReady":              health.MicrophoneReady,
		"uiAgentReady":                 health.UIAgentReady,
		"backgroundRestricted":         health.BackgroundRestricted,
		"deviceIdleMode":               health.DeviceIdleMode,
		"powerSaveMode":                health.PowerSaveMode,
		"interactionState":             health.InteractionState,
		"lastRuntimeFailureAtMs":       health.LastRuntimeFailureAtMS,
		"lastRuntimeFailureGeneration": health.LastRuntimeFailureGeneration,
		"lastRuntimeFailureCode":       health.LastRuntimeFailureCode,
		"recoveryAttempt":              health.RecoveryAttempt,
		"nextRecoveryAtMs":             health.NextRecoveryAtMS,
		"recoveryExhausted":            health.RecoveryExhausted,
		"updatedAt":                    health.UpdatedAt,
		"stale":                        health.Stale,
	}
	if health.Stale {
		report.add("node.deviceHealth", WorkflowPreflightWarning, "device health heartbeat is stale; execution will re-check device state", node.ID, details)
		return
	}
	if health.RecoveryExhausted {
		report.add("node.deviceHealth", WorkflowPreflightBlocked, "device runtime automatic recovery is exhausted; user action is required", node.ID, details)
		return
	}
	if !health.RuntimeReady {
		report.add("node.deviceHealth", WorkflowPreflightBlocked, "device workflow runtime is not ready", node.ID, details)
		return
	}
	if workflowNodeNeedsMicrophoneHealth(node) && !health.MicrophoneReady {
		report.add("node.deviceHealth", WorkflowPreflightBlocked, "device microphone permission/capability is not ready", node.ID, details)
		return
	}
	if workflowNodeNeedsUIHealth(node) {
		if !health.NativeBridgeReady {
			report.add("node.deviceHealth", WorkflowPreflightBlocked, "Android native bridge is not ready for UI automation", node.ID, details)
			return
		}
		if !health.AccessibilityConfigured || !health.AccessibilityEnabled || !health.AccessibilityReady || !health.UIAgentReady {
			report.add("node.deviceHealth", WorkflowPreflightBlocked, "Android UI automation requires an enabled and connected Accessibility service", node.ID, details)
			return
		}
		if !health.ScreenCaptureReady {
			report.add("node.deviceHealth.visual", WorkflowPreflightWarning, "screen capture is unavailable; visual escalation may be limited", node.ID, details)
		}
		switch strings.ToUpper(strings.TrimSpace(health.InteractionState)) {
		case "WAITING_UNLOCK":
			report.add("node.deviceInteraction", WorkflowPreflightWarning, "device is locked; UI execution will wait for the user to unlock it", node.ID, details)
		case "WAITING_SCREEN":
			report.add("node.deviceInteraction", WorkflowPreflightWarning, "device screen is off; UI execution will wait for an interactive screen", node.ID, details)
		case "BLOCKED":
			report.add("node.deviceInteraction", WorkflowPreflightWarning, "Android background/Doze restrictions currently block reliable UI interaction", node.ID, details)
		}
	}
	if health.BackgroundRestricted || health.DeviceIdleMode {
		report.add("node.deviceRestrictions", WorkflowPreflightWarning, "device is online but Android background restrictions may delay execution", node.ID, details)
	}
	report.add("node.deviceHealth", WorkflowPreflightPass, "device runtime capability health is ready", node.ID, details)
}

func (api *WorkflowAPI) workflowPreflightDeviceIsAndroid(ctx context.Context, userID, deviceID string) (bool, bool) {
	if api == nil || api.runtime == nil || api.runtime.WorkflowDeviceControl == nil {
		return false, false
	}
	devices, err := api.runtime.WorkflowDeviceControl.ListDevices(ctx, strings.TrimSpace(userID))
	if err != nil {
		return false, false
	}
	for _, device := range devices {
		if strings.TrimSpace(device.DeviceID) != strings.TrimSpace(deviceID) {
			continue
		}
		platform := strings.ToLower(strings.TrimSpace(device.Platform))
		if platform == "" {
			return false, false
		}
		return platform == "android", true
	}
	return false, false
}

func workflowNodeExplicitlyRequiresAndroid(node workflow.WorkflowNode) bool {
	values := []string{node.TargetID, node.Runtime.RuntimeID, node.Runtime.HandlerName}
	values = append(values, node.RequiredCapabilities...)
	for _, value := range values {
		if strings.Contains(strings.ToLower(strings.TrimSpace(value)), "android") {
			return true
		}
	}
	return false
}

func workflowNodeNeedsUIHealth(node workflow.WorkflowNode) bool {
	values := []string{node.TargetID, node.Runtime.RuntimeID, node.Runtime.HandlerName}
	values = append(values, node.RequiredCapabilities...)
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "android.ui.agent.run" || strings.Contains(value, "android.accessibility") || strings.Contains(value, "android.ui") || strings.Contains(value, "android.interaction") || strings.Contains(value, "screen_capture") {
			return true
		}
	}
	return false
}

func workflowNodeNeedsMicrophoneHealth(node workflow.WorkflowNode) bool {
	values := []string{node.TargetID, node.Runtime.RuntimeID, node.Runtime.HandlerName}
	values = append(values, node.RequiredCapabilities...)
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if strings.Contains(value, "microphone") || strings.Contains(value, "audio.record") || strings.Contains(value, "voice") {
			return true
		}
	}
	return false
}

func preflightRuntimeType(nodeType string) capability.RuntimeType {
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

func uniqueWorkflowStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func workflowPreflightBlockedError(report WorkflowPreflightReport) error {
	if report.Runnable {
		return nil
	}
	for _, check := range report.Checks {
		if check.Status == WorkflowPreflightBlocked {
			encoded, _ := json.Marshal(check)
			return fmt.Errorf("workflow preflight blocked: %s", string(encoded))
		}
	}
	return fmt.Errorf("workflow preflight blocked")
}
