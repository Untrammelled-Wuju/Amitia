package control

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/rpc"
)

const (
	ControlOutputMethod    = "control.output"
	ControlSinkRegisterMethod = "control.sink.register"
)

type ControlOutputInput struct {
	SinkID    string          `json:"sinkId"`
	ServiceID string          `json:"serviceId,omitempty"`
	Epoch     uint64          `json:"epoch"`
	Kind      string          `json:"kind,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

type ControlOutputResult struct {
	Allowed   bool   `json:"allowed"`
	Reason    string `json:"reason,omitempty"`
	Epoch     uint64 `json:"epoch"`
	Generation uint64 `json:"generation"`
}

type ControlSinkRegisterInput struct {
	SinkID    string `json:"sinkId"`
	Kind      string `json:"kind"`
	ServiceID string `json:"serviceId,omitempty"`
}

type ControlSinkRegisterResult struct {
	Registered bool   `json:"registered"`
	SinkID     string `json:"sinkId"`
}

type ControlHandler struct {
	gate         *PluginOutputGate
	sinkRegistry *ControlSinkRegistry
}

func NewControlHandler(gate *PluginOutputGate, sinkRegistry *ControlSinkRegistry) *ControlHandler {
	return &ControlHandler{
		gate:         gate,
		sinkRegistry: sinkRegistry,
	}
}

func (h *ControlHandler) RegisterHandlers(registry rpc.HandlerRegistry) error {
	if err := registry.Register(ControlOutputMethod, controlOutputHandler{h: h}); err != nil {
		return fmt.Errorf("register control.output handler: %w", err)
	}
	if err := registry.Register(ControlSinkRegisterMethod, controlSinkRegisterHandler{h: h}); err != nil {
		return fmt.Errorf("register control.sink.register handler: %w", err)
	}
	return nil
}

type controlOutputHandler struct {
	h *ControlHandler
}

func (h controlOutputHandler) Handle(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	return h.h.handleControlOutput(ctx, request)
}

type controlSinkRegisterHandler struct {
	h *ControlHandler
}

func (h controlSinkRegisterHandler) Handle(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	return h.h.handleSinkRegister(ctx, request)
}

func (h *ControlHandler) handleControlOutput(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	var input ControlOutputInput
	if err := json.Unmarshal(request.Payload, &input); err != nil {
		return rpc.RPCResponse{
			RequestID: request.ID,
			Error: &rpc.RPCRoutedError{
				Code:    string(domain.ErrInvalidArgument),
				Message: fmt.Sprintf("invalid control output input: %v", err),
			},
		}, nil
	}

	sinkDesc, found := h.sinkRegistry.Resolve(request.RuntimeID, domain.ServiceID(input.ServiceID), input.SinkID)
	if !found {
		result := ControlOutputResult{
			Allowed: false,
			Reason:  "sink_not_found",
		}
		payload, _ := json.Marshal(result)
		return rpc.RPCResponse{
			RequestID: request.ID,
			Payload:   payload,
		}, nil
	}

	intent := ControlOutputIntent{
		OutputID:       request.ID,
		RuntimeID:      request.RuntimeID,
		ServiceID:      domain.ServiceID(input.ServiceID),
		AuthorityEpoch: input.Epoch,
		Kind:           sinkDesc.Kind,
		Payload:        input.Payload,
	}

	peer := TrustedPluginIdentity{
		PluginID:   request.PluginID,
		RuntimeID:  request.RuntimeID,
		ServiceID:  request.ServiceID,
		Generation: sinkDesc.Generation,
	}

	checkReq := OutputCheckRequest{
		Intent: intent,
		Peer:   peer,
		Payload: input.Payload,
	}

	decision, _ := h.gate.Check(ctx, checkReq)

	result := ControlOutputResult{
		Allowed:    decision.Allowed,
		Reason:     string(decision.Reason),
		Epoch:      decision.CurrentEpoch,
		Generation: sinkDesc.Generation,
	}
	payload, _ := json.Marshal(result)

	return rpc.RPCResponse{
		RequestID: request.ID,
		Payload:   payload,
	}, nil
}

func (h *ControlHandler) handleSinkRegister(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	var input ControlSinkRegisterInput
	if err := json.Unmarshal(request.Payload, &input); err != nil {
		return rpc.RPCResponse{
			RequestID: request.ID,
			Error: &rpc.RPCRoutedError{
					Code:    string(domain.ErrInvalidArgument),
					Message: fmt.Sprintf("invalid sink register input: %v", err),
			},
		}, nil
	}

	kind := ControlOutputKind(input.Kind)
	if !IsValidPublicOutputKind(kind) {
		return rpc.RPCResponse{
			RequestID: request.ID,
			Error: &rpc.RPCRoutedError{
					Code:    string(domain.ErrInvalidArgument),
					Message: fmt.Sprintf("invalid output kind: %s", input.Kind),
			},
		}, nil
	}

	sink := ControlSinkDescriptor{
		SinkID:     input.SinkID,
		RuntimeID:  request.RuntimeID,
		PluginID:   request.PluginID,
		ServiceID:  domain.ServiceID(input.ServiceID),
		Kind:       kind,
		Generation: 1,
	}

	if err := h.sinkRegistry.Register(sink); err != nil {
		return rpc.RPCResponse{
			RequestID: request.ID,
			Error: &rpc.RPCRoutedError{
					Code:    string(domain.ErrInvalidArgument),
					Message: fmt.Sprintf("sink registration failed: %v", err),
			},
		}, nil
	}

	result := ControlSinkRegisterResult{
		Registered: true,
		SinkID:     input.SinkID,
	}
	payload, _ := json.Marshal(result)

	return rpc.RPCResponse{
		RequestID: request.ID,
		Payload:   payload,
	}, nil
}
