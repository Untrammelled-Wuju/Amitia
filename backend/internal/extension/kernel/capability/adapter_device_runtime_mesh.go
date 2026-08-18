package capability

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/server"
	"github.com/u-ai/backend/internal/deviceruntime"
	protocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type MeshRuntimePorts struct {
	Hub                *server.ConnectionHub
	SessionLookup      meshSessionLookup
	PendingInvocations *PendingInvocationManager
}

type meshSessionLookup interface {
	GetActiveSession(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) (deviceruntime.RuntimeSession, error)
}

type MeshDeviceRuntimeInvocationPort struct {
	ports *MeshRuntimePorts
}

func NewMeshDeviceRuntimeInvocationPort(ports *MeshRuntimePorts) *MeshDeviceRuntimeInvocationPort {
	return &MeshDeviceRuntimeInvocationPort{ports: ports}
}

func (p *MeshDeviceRuntimeInvocationPort) Execute(ctx context.Context, request DeviceRuntimeInvocationRequest) UnifiedToolResult {
	if p.ports == nil || p.ports.Hub == nil {
		return UnifiedToolResult{
			InvocationID: request.Invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:      ErrorCodeRuntimeUnavailable,
				Message:   "mesh runtime port not configured",
				Retryable: true,
			},
		}
	}

	route := request.Route
	sessionID, generation, ok := p.resolveSession(ctx, route.UserID, route.DeviceID, route.RuntimeID)
	if !ok {
		return UnifiedToolResult{
			InvocationID: request.Invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:      ErrorCodeRuntimeUnavailable,
				Message:   "device session not found or offline",
				Retryable: true,
			},
		}
	}
	if sessionID == "" || generation == 0 {
		return UnifiedToolResult{
			InvocationID: request.Invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:      ErrorCodeRuntimeUnavailable,
				Message:   "device session not active",
				Retryable: true,
			},
		}
	}

	input := request.Input
	if input == nil {
		input = json.RawMessage(`{}`)
	}

	deadline := request.Invocation.DeadlineDuration
	if deadline <= 0 {
		deadline = 30 * time.Second
	}

	invokePayload := protocol.RuntimeInvokePayload{
		InvocationID:         request.Invocation.InvocationID,
		RuntimeType:          string(route.Binding.RuntimeType),
		Handler:              route.Binding.HandlerName,
		Input:                input,
		ProviderID:           route.Binding.ProviderID,
		DeviceID:             route.DeviceID,
		RuntimeID:            route.RuntimeID,
		RuntimeSessionID:     sessionID,
		ConnectionGeneration: generation,
		DeadlineMs:           deadline.Milliseconds(),
		SentAt:               time.Now().UTC(),
	}

	if p.ports.PendingInvocations == nil {
		return UnifiedToolResult{
			InvocationID: request.Invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:    ErrorCodeInternalError,
				Message: "pending invocation manager not configured",
			},
		}
	}

	_, err := p.ports.PendingInvocations.Register(request, request.Invocation.InvocationID, sessionID.String(), generation, deadline)
	if err != nil {
		return UnifiedToolResult{
			InvocationID: request.Invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:    ErrorCodeInternalError,
				Message: fmt.Sprintf("register pending invocation: %v", err),
			},
		}
	}

	if !p.ports.Hub.SendEnvelope(sessionID, generation, protocol.MessageTypeRuntimeInvoke, invokePayload) {
		p.ports.PendingInvocations.Cancel(request.Invocation.InvocationID, "failed to send")
		return UnifiedToolResult{
			InvocationID: request.Invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:      ErrorCodeRuntimeUnavailable,
				Message:   "failed to send invoke to device",
				Retryable: true,
			},
		}
	}

	result, err := p.ports.PendingInvocations.WaitForResult(ctx, request.Invocation.InvocationID)
	if err != nil {
		return UnifiedToolResult{
			InvocationID: request.Invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:      ErrorCodeTimeout,
				Message:   fmt.Sprintf("invocation timed out: %v", err),
				Retryable: true,
			},
		}
	}
	return result
}

func (p *MeshDeviceRuntimeInvocationPort) Health(ctx context.Context, route RuntimeExecutionRoute) HealthStatus {
	if p.ports == nil || p.ports.Hub == nil {
		return HealthUnknown
	}
	sessionID, generation, ok := p.resolveSession(ctx, route.UserID, route.DeviceID, route.RuntimeID)
	if !ok {
		return HealthUnhealthy
	}
	if !p.ports.Hub.IsSessionActive(sessionID, generation) {
		return HealthUnhealthy
	}
	if !p.ports.Hub.HasRecentPong(sessionID, generation, 90*time.Second) {
		return HealthUnhealthy
	}
	return HealthReady
}

func (p *MeshDeviceRuntimeInvocationPort) Cancel(ctx context.Context, request DeviceRuntimeInvocationRequest, reason ToolCancellationReason) error {
	if p.ports == nil || p.ports.Hub == nil {
		return ErrRuntimeCancellationUnsupported{}
	}
	route := request.Route
	sessionID, generation, ok := p.resolveSession(ctx, route.UserID, route.DeviceID, route.RuntimeID)
	if !ok {
		if p.ports.PendingInvocations != nil {
			p.ports.PendingInvocations.Cancel(request.Invocation.InvocationID, string(reason.Code))
		}
		return fmt.Errorf("device session not found")
	}
	cancelPayload := protocol.RuntimeCancelPayload{
		InvocationID:         request.Invocation.InvocationID,
		RuntimeSessionID:     sessionID,
		ConnectionGeneration: generation,
		DeviceID:             route.DeviceID,
		RuntimeID:            route.RuntimeID,
		Reason:               string(reason.Code),
		SentAt:               time.Now().UTC(),
	}
	if !p.ports.Hub.SendEnvelope(sessionID, generation, protocol.MessageTypeRuntimeCancel, cancelPayload) {
		if p.ports.PendingInvocations != nil {
			p.ports.PendingInvocations.Cancel(request.Invocation.InvocationID, string(reason.Code))
		}
		return fmt.Errorf("failed to send cancel to device")
	}
	if p.ports.PendingInvocations != nil {
		p.ports.PendingInvocations.Cancel(request.Invocation.InvocationID, string(reason.Code))
	}
	return nil
}

func (p *MeshDeviceRuntimeInvocationPort) resolveSession(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) (runtimeidentity.RuntimeSessionID, int64, bool) {
	if p.ports.SessionLookup != nil {
		session, err := p.ports.SessionLookup.GetActiveSession(ctx, userID, deviceID, runtimeID)
		if err != nil || session.ID == "" {
			return "", 0, false
		}
		if !session.IsActive() {
			return "", 0, false
		}
		return session.ID, session.ConnectionGeneration, true
	}
	conn, ok := p.ports.Hub.GetByRuntime(userID, deviceID, runtimeID)
	if !ok {
		return "", 0, false
	}
	return conn.SessionID, conn.Generation, true
}
