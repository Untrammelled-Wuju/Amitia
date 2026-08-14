package control

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/rpc"
)

const (
	ControlAuthoritySnapshotMethod = "control.authority.snapshot"
	ControlAuthorityTakeoverMethod = "control.authority.takeover"
	ControlAuthorityReleaseMethod  = "control.authority.release"
	EmergencyStopMethod            = "emergency.stop"
)

type AuthorityRPCHandler struct {
	authority *ControlAuthorityManager
	takeover  *TakeoverService
	emergency *EmergencyStopService
}

func NewAuthorityRPCHandler(authority *ControlAuthorityManager, takeover *TakeoverService, emergency *EmergencyStopService) *AuthorityRPCHandler {
	return &AuthorityRPCHandler{authority: authority, takeover: takeover, emergency: emergency}
}

func (h *AuthorityRPCHandler) RegisterHandlers(registry rpc.HandlerRegistry) error {
	for method, handler := range map[rpc.Method]rpc.Handler{
		ControlAuthoritySnapshotMethod: authoritySnapshotRPCHandler{h},
		ControlAuthorityTakeoverMethod: authorityTakeoverRPCHandler{h},
		ControlAuthorityReleaseMethod:  authorityReleaseRPCHandler{h},
		EmergencyStopMethod:            emergencyStopRPCHandler{h},
	} {
		if err := registry.Register(method, handler); err != nil {
			return fmt.Errorf("register %s: %w", method, err)
		}
	}
	return nil
}

type authoritySnapshotRPCHandler struct{ parent *AuthorityRPCHandler }
type authorityTakeoverRPCHandler struct{ parent *AuthorityRPCHandler }
type authorityReleaseRPCHandler struct{ parent *AuthorityRPCHandler }
type emergencyStopRPCHandler struct{ parent *AuthorityRPCHandler }

func (h authoritySnapshotRPCHandler) Handle(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	snapshot, err := h.parent.authority.Get(ctx, request.RuntimeID)
	return authorityRPCResponse(request.ID, snapshot, err), nil
}

func (h authorityTakeoverRPCHandler) Handle(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	var input struct {
		ExpectedEpoch *uint64 `json:"expectedEpoch"`
	}
	if err := json.Unmarshal(request.Payload, &input); err != nil {
		return invalidAuthorityRPCResponse(request.ID, err), nil
	}
	result, err := h.parent.takeover.Takeover(ctx, TakeoverRequest{RuntimeID: request.RuntimeID, PluginID: request.PluginID, Actor: ActorPlugin, ExpectedEpoch: input.ExpectedEpoch})
	return authorityRPCResponse(request.ID, result, err), nil
}

func (h authorityReleaseRPCHandler) Handle(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	var input struct {
		ExpectedEpoch uint64 `json:"expectedEpoch"`
		TargetMode    string `json:"targetMode"`
	}
	if err := json.Unmarshal(request.Payload, &input); err != nil {
		return invalidAuthorityRPCResponse(request.ID, err), nil
	}
	result, err := h.parent.takeover.Release(ctx, ReleaseRequest{RuntimeID: request.RuntimeID, PluginID: request.PluginID, Actor: ActorPlugin, ExpectedEpoch: input.ExpectedEpoch, UseExpected: true, TargetMode: domain.ControlMode(input.TargetMode)})
	return authorityRPCResponse(request.ID, result, err), nil
}

func (h emergencyStopRPCHandler) Handle(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	var input struct {
		IdempotencyKey string `json:"idempotencyKey"`
	}
	if err := json.Unmarshal(request.Payload, &input); err != nil {
		return invalidAuthorityRPCResponse(request.ID, err), nil
	}
	result, err := h.parent.emergency.EmergencyStop(ctx, EmergencyStopRequest{RuntimeID: request.RuntimeID, Actor: EmergencyActorHost, Reason: EmergencyReasonSafetyPolicy, IdempotencyKey: input.IdempotencyKey})
	return authorityRPCResponse(request.ID, result, err), nil
}

func authorityRPCResponse(requestID string, value interface{}, err error) rpc.RPCResponse {
	if err != nil {
		return rpc.RPCResponse{RequestID: requestID, Error: &rpc.RPCRoutedError{Code: string(domain.ErrInvalidState), Message: err.Error()}}
	}
	payload, marshalErr := json.Marshal(value)
	if marshalErr != nil {
		return rpc.RPCResponse{RequestID: requestID, Error: &rpc.RPCRoutedError{Code: string(domain.ErrInternal), Message: marshalErr.Error()}}
	}
	return rpc.RPCResponse{RequestID: requestID, Payload: payload}
}

func invalidAuthorityRPCResponse(requestID string, err error) rpc.RPCResponse {
	return rpc.RPCResponse{RequestID: requestID, Error: &rpc.RPCRoutedError{Code: string(domain.ErrInvalidArgument), Message: err.Error()}}
}
