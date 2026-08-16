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
	Hub           *server.ConnectionHub
	SessionLookup meshSessionLookup
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

	cmdPayload := protocol.CommandPayload{
		CommandID:       uuid.New().String(),
		CommandName:     route.Binding.HandlerName,
		CommandSequence: time.Now().UnixNano(),
		Payload:         input,
	}

	payloadBytes, err := json.Marshal(cmdPayload)
	if err != nil {
		return UnifiedToolResult{
			InvocationID: request.Invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:    ErrorCodeExecutionFailed,
				Message: fmt.Sprintf("marshal command: %v", err),
			},
		}
	}

	env := protocol.Envelope{
		EnvelopeVersion:      1,
		Protocol:             "amitia.device-mesh",
		MessageType:          protocol.MessageTypeCommand,
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

	if !p.ports.Hub.Send(sessionID, generation, envBytes) {
		return UnifiedToolResult{
			InvocationID: request.Invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:      ErrorCodeRuntimeUnavailable,
				Message:   "failed to send command to device",
				Retryable: true,
			},
		}
	}

	return UnifiedToolResult{
		InvocationID: request.Invocation.InvocationID,
		Status:       ToolResultStatusSuccess,
		Metadata: map[string]any{
			"delivered": true,
			"sessionId": sessionID.String(),
		},
	}
}

func (p *MeshDeviceRuntimeInvocationPort) Health(ctx context.Context, route RuntimeExecutionRoute) HealthStatus {
	if p.ports == nil || p.ports.Hub == nil {
		return HealthUnknown
	}
	sessionID, generation, ok := p.resolveSession(ctx, route.UserID, route.DeviceID, route.RuntimeID)
	if !ok {
		return HealthUnhealthy
	}
	if p.ports.Hub.Send(sessionID, generation, nil) {
		return HealthReady
	}
	return HealthUnhealthy
}

func (p *MeshDeviceRuntimeInvocationPort) Cancel(ctx context.Context, request DeviceRuntimeInvocationRequest, reason ToolCancellationReason) error {
	if p.ports == nil || p.ports.Hub == nil {
		return ErrRuntimeCancellationUnsupported{}
	}
	route := request.Route
	sessionID, generation, ok := p.resolveSession(ctx, route.UserID, route.DeviceID, route.RuntimeID)
	if !ok {
		return fmt.Errorf("device session not found")
	}
	cancelPayload := map[string]string{
		"cancelInvocationId": request.Invocation.InvocationID,
		"reason":             string(reason.Code),
	}
	payloadBytes, _ := json.Marshal(cancelPayload)
	if !p.ports.Hub.Send(sessionID, generation, payloadBytes) {
		return fmt.Errorf("failed to send cancel to device")
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
