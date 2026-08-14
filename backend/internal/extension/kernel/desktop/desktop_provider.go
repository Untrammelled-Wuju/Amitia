package desktop

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type DesktopHostProvider struct {
	host *DesktopHost
}

func NewDesktopHostProvider(host *DesktopHost) *DesktopHostProvider {
	return &DesktopHostProvider{host: host}
}

func (p *DesktopHostProvider) Execute(ctx context.Context, request capability.DesktopBridgeRequest) capability.DesktopBridgeResponse {
	if p.host == nil {
		return capability.DesktopBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.DesktopError{
				Code:    "PROVIDER_UNAVAILABLE",
				Message: "desktop host not available",
			},
		}
	}

	operation := request.Operation
	payload := request.Payload

	contribID, _ := payload["contributionId"].(string)
	actionType, _ := payload["actionType"].(string)
	targetID, _ := payload["targetId"].(string)

	if contribID == "" && targetID != "" {
		contribID = targetID
	}

	if actionType == "" {
		actionType = inferActionType(operation)
	}

	scopeCtx := ScopeContext{
		CharacterID:    getStringPayload(payload, "characterId"),
		ConversationID: getStringPayload(payload, "conversationId"),
		ExtensionID:    getStringPayload(payload, "extensionId"),
		Global:         getBoolPayload(payload, "global"),
	}

	result, err := p.host.InvokeAction(ctx, contribID, scopeCtx)
	if err != nil {
		return capability.DesktopBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error:           mapHostError(err),
		}
	}

	resultMap := map[string]any{}
	if len(result) > 0 {
		_ = json.Unmarshal(result, &resultMap)
	}
	resultMap["status"] = "success"

	return capability.DesktopBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result:          resultMap,
	}
}

func (p *DesktopHostProvider) Health(ctx context.Context) capability.HealthStatus {
	if p.host == nil {
		return capability.HealthUnhealthy
	}

	select {
	case <-ctx.Done():
		return capability.HealthUnknown
	default:
		p.host.mu.RLock()
		executor := p.host.actionExecutor
		p.host.mu.RUnlock()
		if executor == nil {
			return capability.HealthDegraded
		}
		return capability.HealthReady
	}
}

func inferActionType(operation string) string {
	switch operation {
	case "host_action":
		return "host_action"
	case "tool_invoke":
		return "tool_invoke"
	case "workflow_execute":
		return "workflow_execute"
	case "task_enqueue":
		return "task_enqueue"
	case "extension_command":
		return "extension_command"
	case "navigation":
		return "navigation"
	case "dialog_open":
		return "dialog_open"
	default:
		return "host_action"
	}
}

func getStringPayload(payload map[string]any, key string) string {
	if v, ok := payload[key].(string); ok {
		return v
	}
	return ""
}

func getBoolPayload(payload map[string]any, key string) bool {
	if v, ok := payload[key].(bool); ok {
		return v
	}
	return false
}

func mapHostError(err error) *capability.DesktopError {
	if err == nil {
		return nil
	}

	code := "EXECUTION_FAILED"
	switch {
	case err == ErrContributionNotFound:
		code = "NOT_FOUND"
	case err == ErrPermissionDenied:
		code = "AUTHORIZATION_DENIED"
	case err == ErrCircuitOpen:
		code = "PROVIDER_UNAVAILABLE"
	case err == ErrQuarantined:
		code = "AUTHORIZATION_DENIED"
	case err == ErrContributionExists:
		code = "CONFLICT"
	}

	domainCode := "desktop.host"
	switch code {
	case "AUTHORIZATION_DENIED":
		domainCode = "desktop.permission"
	case "NOT_FOUND":
		domainCode = "desktop.contribution"
	}

	return &capability.DesktopError{
		Code:       code,
		Message:    err.Error(),
		DomainCode: domainCode,
	}
}
