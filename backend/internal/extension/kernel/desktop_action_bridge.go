package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/desktop"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

type DesktopActionBridge struct {
	permissionBroker permission.PermissionBroker
	scopeManager     scope.ScopeManager
	executionKernel  *execution.ExecutionPipeline
	auditRecorder    *DesktopAuditRecorder
}

func NewDesktopActionBridge(
	pb permission.PermissionBroker,
	sm scope.ScopeManager,
	ek *execution.ExecutionPipeline,
) *DesktopActionBridge {
	return &DesktopActionBridge{
		permissionBroker: pb,
		scopeManager:     sm,
		executionKernel:  ek,
		auditRecorder:    NewDesktopAuditRecorder(),
	}
}

func (b *DesktopActionBridge) Check(ctx context.Context, extensionID, permissionID string) (bool, error) {
	if b.permissionBroker == nil {
		return true, nil
	}
	subject := permission.PermissionSubject{
		Type: permission.SubjectExtension,
		ID:   extensionID,
	}
	req := permission.PermissionEvaluationRequest{
		Subject: subject,
		Requirements: []permission.PermissionRequirement{
			{PermissionID: permissionID},
		},
	}
	result := b.permissionBroker.Evaluate(ctx, req)
	b.auditRecorder.RecordPermissionCheck(extensionID, permissionID, string(result.Decision))
	return result.Decision == permission.DecisionAllow, nil
}

func (b *DesktopActionBridge) CheckScope(ctx context.Context, extensionID, scopeType, scopeID string) (bool, error) {
	if b.scopeManager == nil {
		return true, nil
	}
	req := scope.ScopeEvaluationRequest{
		SubjectType: scope.SubjectExtension,
		SubjectID:   extensionID,
		ExtensionID: extensionID,
	}
	if scopeID != "" {
		switch scope.ScopeType(scopeType) {
		case scope.ScopeCharacter:
			req.CharacterID = scopeID
		case scope.ScopeConversation:
			req.ConversationID = scopeID
		case scope.ScopeExtension:
			req.ExtensionID = scopeID
		case scope.ScopeModule:
			req.ModuleID = scopeID
		}
	}
	decision := b.scopeManager.Evaluate(ctx, req)
	b.auditRecorder.RecordScopeCheck(extensionID, scopeType, scopeID, decision.Allowed)
	return decision.Allowed, nil
}

func (b *DesktopActionBridge) ExecuteAction(ctx context.Context, binding desktop.DesktopActionBinding, extensionID string, scopeCtx desktop.ScopeContext) (json.RawMessage, error) {
	startTime := time.Now()

	result, err := b.dispatchAction(ctx, binding, extensionID, scopeCtx)
	duration := time.Since(startTime)

	b.auditRecorder.RecordActionExecution(extensionID, binding.ActionType, binding.TargetID, err == nil, duration)

	return result, err
}

func (b *DesktopActionBridge) dispatchAction(ctx context.Context, binding desktop.DesktopActionBinding, extensionID string, scopeCtx desktop.ScopeContext) (json.RawMessage, error) {
	switch binding.ActionType {
	case "host_action":
		return b.executeHostAction(ctx, binding, extensionID, scopeCtx)
	case "tool_invoke":
		return b.executeToolInvoke(ctx, binding, extensionID, scopeCtx)
	case "workflow_execute":
		return b.executeWorkflow(ctx, binding, extensionID, scopeCtx)
	case "task_enqueue":
		return b.executeTaskEnqueue(ctx, binding, extensionID, scopeCtx)
	case "extension_command":
		return b.executeExtensionCommand(ctx, binding, extensionID, scopeCtx)
	case "navigation":
		return b.executeNavigation(ctx, binding, extensionID, scopeCtx)
	case "dialog_open":
		return b.executeDialogOpen(ctx, binding, extensionID, scopeCtx)
	default:
		return nil, fmt.Errorf("desktop: unsupported action type: %s", binding.ActionType)
	}
}

func (b *DesktopActionBridge) buildInvocation(extensionID string, scopeCtx desktop.ScopeContext) capability.ToolInvocationContext {
	invocationID := fmt.Sprintf("desktop-%s-%d", extensionID, time.Now().UnixNano())
	inv := capability.ToolInvocationContext{
		InvocationID:   invocationID,
		UserID:         "desktop-host",
		ExtensionID:    extensionID,
		CharacterID:    scopeCtx.CharacterID,
		ConversationID: scopeCtx.ConversationID,
		Source:         capability.InvocationSourceUser,
		Metadata: map[string]any{
			"source":      "desktop_action_bridge",
			"scopeGlobal": scopeCtx.Global,
		},
	}
	if scopeCtx.ExtensionID != "" {
		inv.ExtensionID = scopeCtx.ExtensionID
	}
	return inv
}

func (b *DesktopActionBridge) executeViaKernel(ctx context.Context, toolID string, binding desktop.DesktopActionBinding, extensionID string, scopeCtx desktop.ScopeContext) (json.RawMessage, error) {
	if b.executionKernel == nil {
		return nil, fmt.Errorf("desktop: execution kernel not available")
	}
	input := binding.Input
	if len(input) == 0 {
		input = []byte(`{}`)
	}
	toolReq := execution.ToolExecutionRequest{
		ToolID:     capability.CapabilityID(toolID),
		Input:      input,
		Invocation: b.buildInvocation(extensionID, scopeCtx),
	}
	result := b.executionKernel.Execute(ctx, toolReq)
	if result.Status != capability.ToolResultStatusSuccess {
		errMsg := "unknown error"
		if result.Error != nil {
			errMsg = result.Error.Message
		}
		return nil, fmt.Errorf("desktop: kernel execution failed for %s: %s", toolID, errMsg)
	}
	if len(result.Structured) > 0 {
		return result.Structured, nil
	}
	return []byte(`{"status":"success"}`), nil
}

func (b *DesktopActionBridge) executeHostAction(ctx context.Context, binding desktop.DesktopActionBinding, extensionID string, scopeCtx desktop.ScopeContext) (json.RawMessage, error) {
	toolID := "desktop.action.host_action"
	if binding.TargetID != "" {
		toolID = "desktop.action.host_action." + binding.TargetID
	}
	return b.executeViaKernel(ctx, toolID, binding, extensionID, scopeCtx)
}

func (b *DesktopActionBridge) executeToolInvoke(ctx context.Context, binding desktop.DesktopActionBinding, extensionID string, scopeCtx desktop.ScopeContext) (json.RawMessage, error) {
	toolID := binding.TargetID
	if toolID == "" {
		toolID = "desktop.action.tool_invoke"
	}
	return b.executeViaKernel(ctx, toolID, binding, extensionID, scopeCtx)
}

func (b *DesktopActionBridge) executeWorkflow(ctx context.Context, binding desktop.DesktopActionBinding, extensionID string, scopeCtx desktop.ScopeContext) (json.RawMessage, error) {
	toolID := "desktop.action.workflow_execute"
	if binding.TargetID != "" {
		toolID = "desktop.action.workflow_execute." + binding.TargetID
	}
	return b.executeViaKernel(ctx, toolID, binding, extensionID, scopeCtx)
}

func (b *DesktopActionBridge) executeTaskEnqueue(ctx context.Context, binding desktop.DesktopActionBinding, extensionID string, scopeCtx desktop.ScopeContext) (json.RawMessage, error) {
	toolID := "desktop.action.task_enqueue"
	if binding.TargetID != "" {
		toolID = "desktop.action.task_enqueue." + binding.TargetID
	}
	return b.executeViaKernel(ctx, toolID, binding, extensionID, scopeCtx)
}

func (b *DesktopActionBridge) executeExtensionCommand(ctx context.Context, binding desktop.DesktopActionBinding, extensionID string, scopeCtx desktop.ScopeContext) (json.RawMessage, error) {
	toolID := "desktop.action.extension_command"
	if binding.TargetID != "" {
		toolID = "desktop.action.extension_command." + binding.TargetID
	}
	return b.executeViaKernel(ctx, toolID, binding, extensionID, scopeCtx)
}

func (b *DesktopActionBridge) executeNavigation(ctx context.Context, binding desktop.DesktopActionBinding, extensionID string, scopeCtx desktop.ScopeContext) (json.RawMessage, error) {
	toolID := "desktop.action.navigation"
	if binding.TargetID != "" {
		toolID = "desktop.action.navigation." + binding.TargetID
	}
	return b.executeViaKernel(ctx, toolID, binding, extensionID, scopeCtx)
}

func (b *DesktopActionBridge) executeDialogOpen(ctx context.Context, binding desktop.DesktopActionBinding, extensionID string, scopeCtx desktop.ScopeContext) (json.RawMessage, error) {
	toolID := "desktop.action.dialog_open"
	if binding.TargetID != "" {
		toolID = "desktop.action.dialog_open." + binding.TargetID
	}
	return b.executeViaKernel(ctx, toolID, binding, extensionID, scopeCtx)
}

func (b *DesktopActionBridge) GetAuditRecorder() *DesktopAuditRecorder {
	return b.auditRecorder
}

type DesktopAuditEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`
	ExtensionID string    `json:"extensionId"`
	Permission  string    `json:"permission,omitempty"`
	ScopeType   string    `json:"scopeType,omitempty"`
	ScopeID     string    `json:"scopeId,omitempty"`
	ActionType  string    `json:"actionType,omitempty"`
	TargetID    string    `json:"targetId,omitempty"`
	Success     bool      `json:"success"`
	Duration    string    `json:"duration,omitempty"`
	Decision    string    `json:"decision,omitempty"`
}

type DesktopAuditRecorder struct {
	entries []DesktopAuditEntry
	maxSize int
}

func NewDesktopAuditRecorder() *DesktopAuditRecorder {
	return &DesktopAuditRecorder{
		entries: make([]DesktopAuditEntry, 0, 256),
		maxSize: 1000,
	}
}

func (r *DesktopAuditRecorder) RecordPermissionCheck(extensionID, permissionID, decision string) {
	r.entries = append(r.entries, DesktopAuditEntry{
		Timestamp:   time.Now().UTC(),
		Type:        "permission_check",
		ExtensionID: extensionID,
		Permission:  permissionID,
		Decision:    decision,
	})
	r.trim()
}

func (r *DesktopAuditRecorder) RecordScopeCheck(extensionID, scopeType, scopeID string, allowed bool) {
	decision := "deny"
	if allowed {
		decision = "allow"
	}
	r.entries = append(r.entries, DesktopAuditEntry{
		Timestamp:   time.Now().UTC(),
		Type:        "scope_check",
		ExtensionID: extensionID,
		ScopeType:   scopeType,
		ScopeID:     scopeID,
		Decision:    decision,
	})
	r.trim()
}

func (r *DesktopAuditRecorder) RecordActionExecution(extensionID, actionType, targetID string, success bool, duration time.Duration) {
	r.entries = append(r.entries, DesktopAuditEntry{
		Timestamp:   time.Now().UTC(),
		Type:        "action_execution",
		ExtensionID: extensionID,
		ActionType:  actionType,
		TargetID:    targetID,
		Success:     success,
		Duration:    duration.String(),
	})
	r.trim()
}

func (r *DesktopAuditRecorder) ListEntries(filterType string, limit int) []DesktopAuditEntry {
	if limit <= 0 {
		limit = 100
	}
	var result []DesktopAuditEntry
	count := 0
	for i := len(r.entries) - 1; i >= 0 && count < limit; i-- {
		if filterType != "" && r.entries[i].Type != filterType {
			continue
		}
		result = append(result, r.entries[i])
		count++
	}
	return result
}

func (r *DesktopAuditRecorder) trim() {
	if len(r.entries) > r.maxSize {
		r.entries = r.entries[len(r.entries)-r.maxSize:]
	}
}

var _ desktop.PermissionChecker = (*DesktopActionBridge)(nil)
var _ desktop.ScopeChecker = (*DesktopActionBridge)(nil)
var _ desktop.ActionExecutor = (*DesktopActionBridge)(nil)
