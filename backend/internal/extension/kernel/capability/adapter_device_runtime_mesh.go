package capability

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
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

	payloadBytes, err := json.Marshal(invokePayload)
	if err != nil {
		return UnifiedToolResult{
			InvocationID: request.Invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:    ErrorCodeExecutionFailed,
				Message: fmt.Sprintf("marshal invoke payload: %v", err),
			},
		}
	}

	env := protocol.Envelope{
		EnvelopeVersion:      1,
		Protocol:             "amitia.device-mesh",
		MessageType:          protocol.MessageTypeRuntimeInvoke,
		MessageID:            uuid.New().String(),
		UserID:               route.UserID,
		DeviceID:             route.DeviceID,
		RuntimeID:            route.RuntimeID,
		RuntimeSessionID:     sessionID,
		ConnectionGeneration: generation,
		Sequence:             1,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	envBytes, err := json.Marshal(env)
	if err != nil {
		return UnifiedToolResult{
			InvocationID: request.Invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:    ErrorCodeExecutionFailed,
				Message: fmt.Sprintf("marshal envelope: %v", err),
			},
		}
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

	_, err = p.ports.PendingInvocations.Register(request, request.Invocation.InvocationID, sessionID.String(), generation, deadline)
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

	if !p.ports.Hub.Send(sessionID, generation, envBytes) {
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
	cancelPayload := map[string]string{
		"cancelInvocationId": request.Invocation.InvocationID,
		"reason":             string(reason.Code),
	}
	payloadBytes, _ := json.Marshal(cancelPayload)
	if !p.ports.Hub.Send(sessionID, generation, payloadBytes) {
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
