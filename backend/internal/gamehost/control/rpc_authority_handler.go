package control

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
	authority      *ControlAuthorityManager
	takeover       *TakeoverService
	emergency      *EmergencyStopService
	featureChecker NegotiatedFeatureChecker
}

func NewAuthorityRPCHandler(authority *ControlAuthorityManager, takeover *TakeoverService, emergency *EmergencyStopService) *AuthorityRPCHandler {
	return &AuthorityRPCHandler{authority: authority, takeover: takeover, emergency: emergency}
}

func (h *AuthorityRPCHandler) SetNegotiatedFeatureChecker(checker NegotiatedFeatureChecker) {
	if h == nil {
		return
	}
	h.featureChecker = checker
}

func (h *AuthorityRPCHandler) requireSharedControl(request rpc.RPCRequest) *rpc.RPCResponse {
	if h == nil || h.featureChecker == nil || !h.featureChecker.HasNegotiatedCapability(request.ConnectionID, domain.CapabilitySharedControl) {
		response := rpc.RPCResponse{RequestID: request.ID, Error: &rpc.RPCRoutedError{
			Code:    string(domain.ErrPermissionDenied),
			Message: "shared_control was not negotiated for this service connection",
		}}
		return &response
	}
	return nil
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

type emergencyStopRPCResult struct {
	OperationID     string    `json:"operationId"`
	RuntimeID       string    `json:"runtimeId"`
	State           string    `json:"state"`
	Actor           string    `json:"actor"`
	Reason          string    `json:"reason"`
	Success         bool      `json:"success"`
	CriticalFailure bool      `json:"criticalFailure"`
	Residue         []string  `json:"residue,omitempty"`
	StartedAt       time.Time `json:"startedAt"`
	FinishedAt      time.Time `json:"finishedAt"`
}

func (h authoritySnapshotRPCHandler) Handle(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	if denied := h.parent.requireSharedControl(request); denied != nil {
		return *denied, nil
	}
	snapshot, err := h.parent.authority.Get(ctx, request.RuntimeID)
	return authorityRPCResponse(request.ID, snapshot, err), nil
}

func (h authorityTakeoverRPCHandler) Handle(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	if denied := h.parent.requireSharedControl(request); denied != nil {
		return *denied, nil
	}
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
	if denied := h.parent.requireSharedControl(request); denied != nil {
		return *denied, nil
	}
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
	result, err := h.parent.emergency.EmergencyStop(ctx, EmergencyStopRequest{
		RuntimeID:      request.RuntimeID,
		Actor:          EmergencyActorPlugin,
		Reason:         EmergencyReasonPluginRequested,
		IdempotencyKey: input.IdempotencyKey,
	})
	if err != nil {
		return authorityRPCResponse(request.ID, nil, err), nil
	}
	wire := emergencyStopRPCResult{
		OperationID:     result.OperationID,
		RuntimeID:       string(result.RuntimeID),
		State:           string(result.State),
		Actor:           string(result.Actor),
		Reason:          string(result.Reason),
		Success:         result.Success(),
		CriticalFailure: result.CriticalFailure,
		Residue:         append([]string(nil), result.Residue...),
		StartedAt:       result.StartedAt,
		FinishedAt:      result.FinishedAt,
	}
	return authorityRPCResponse(request.ID, wire, nil), nil
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
