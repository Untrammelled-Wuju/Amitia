package capability

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

type AvailabilityReason string

const (
	ReasonModuleDisabled      AvailabilityReason = "module_disabled"
	ReasonToolDisabled        AvailabilityReason = "tool_disabled"
	ReasonScopeDenied         AvailabilityReason = "scope_denied"
	ReasonPermissionMissing   AvailabilityReason = "permission_missing"
	ReasonRuntimeNotReady     AvailabilityReason = "runtime_not_ready"
	ReasonDependencyMissing   AvailabilityReason = "dependency_missing"
	ReasonMCPDisconnected     AvailabilityReason = "mcp_disconnected"
	ReasonWorkflowInvalid     AvailabilityReason = "workflow_invalid"
	ReasonPluginCircuitOpen   AvailabilityReason = "plugin_circuit_open"
	ReasonPlatformUnsupported AvailabilityReason = "platform_unsupported"
)

type AvailabilityResult struct {
	Visible    bool                 `json:"visible"`
	Executable bool                 `json:"executable"`
	Reasons    []AvailabilityReason `json:"reasons"`
}

type AvailabilityEvaluator interface {
	Evaluate(ctx context.Context, tool ToolDefinition, invocation ToolInvocationContext) AvailabilityResult
}

type DefaultAvailabilityEvaluator struct {
	StateResolver func(ctx context.Context, toolID string) (ToolState, error)
	ScopeManager  scope.ScopeManager
}

func (e *DefaultAvailabilityEvaluator) Evaluate(ctx context.Context, tool ToolDefinition, invocation ToolInvocationContext) AvailabilityResult {
	state := tool.State
	if e.StateResolver != nil {
		if resolved, err := e.StateResolver(ctx, tool.ID); err == nil {
			state = resolved
		}
	}

	result := AvailabilityResult{
		Visible:    state.VisibleToModel(),
		Executable: state.Executable(),
		Reasons:    make([]AvailabilityReason, 0),
	}

	if !state.Installed {
		return result
	}
	if !state.ModuleEnabled {
		result.Reasons = append(result.Reasons, ReasonModuleDisabled)
	}
	if !state.CapabilityEnabled {
		result.Reasons = append(result.Reasons, ReasonToolDisabled)
	}

	if !state.ScopeAllowed || !e.evaluateScope(ctx, tool, invocation) {
		result.Reasons = append(result.Reasons, ReasonScopeDenied)
	}

	if !state.PermissionGranted {
		result.Reasons = append(result.Reasons, ReasonPermissionMissing)
	}
	if !state.RuntimeReady {
		result.Reasons = append(result.Reasons, ReasonRuntimeNotReady)
	}
	if !state.DependencyReady {
		result.Reasons = append(result.Reasons, ReasonDependencyMissing)
	}

	return result
}

func (e *DefaultAvailabilityEvaluator) evaluateScope(ctx context.Context, tool ToolDefinition, invocation ToolInvocationContext) bool {
	if e.ScopeManager == nil {
		return true
	}

	decision := e.ScopeManager.Evaluate(ctx, scope.ScopeEvaluationRequest{
		SubjectType:    scope.SubjectTool,
		SubjectID:      tool.ID,
		CharacterID:    invocation.CharacterID,
		ConversationID: invocation.ConversationID,
		ExtensionID:    invocation.ExtensionID,
		ModuleID:       invocation.ModuleID,
		InvocationID:   invocation.InvocationID,
		Generation:     invocation.Generation,
	})
	return decision.Allowed
}
