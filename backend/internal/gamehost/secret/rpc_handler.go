package secret

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/rpc"
)

type SecretRPCHandler struct {
	adapter *SecretLeaseAdapter
}

func NewSecretRPCHandler(adapter *SecretLeaseAdapter) *SecretRPCHandler {
	return &SecretRPCHandler{adapter: adapter}
}

func (h *SecretRPCHandler) Register(registry rpc.HandlerRegistry) error {
	if err := registry.Register(rpc.Method("secret.acquire"), rpc.Handler(acquireHandler{h.adapter})); err != nil {
		return fmt.Errorf("register secret.acquire: %w", err)
	}
	if err := registry.Register(rpc.Method("secret.release"), rpc.Handler(releaseHandler{h.adapter})); err != nil {
		return fmt.Errorf("register secret.release: %w", err)
	}
	if err := registry.Register(rpc.Method("secret.query"), rpc.Handler(queryHandler{h.adapter})); err != nil {
		return fmt.Errorf("register secret.query: %w", err)
	}
	return nil
}

type acquireHandler struct {
	adapter *SecretLeaseAdapter
}

func (h acquireHandler) Handle(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	var payload SecretAcquireRequest
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &payload); err != nil {
			return rpc.RPCResponse{Error: &rpc.RPCRoutedError{Code: "INVALID_PAYLOAD", Message: "invalid acquire payload"}}, nil
		}
	}

	if payload.RuntimeID != "" && payload.RuntimeID != string(request.RuntimeID) {
		return rpc.RPCResponse{Error: &rpc.RPCRoutedError{Code: "IDENTITY_MISMATCH", Message: "runtimeId does not match trusted identity"}}, nil
	}
	if payload.ServiceID != "" && payload.ServiceID != string(request.ServiceID) {
		return rpc.RPCResponse{Error: &rpc.RPCRoutedError{Code: "IDENTITY_MISMATCH", Message: "serviceId does not match trusted identity"}}, nil
	}

	if !payload.Ref.Valid() {
		return rpc.RPCResponse{Error: &rpc.RPCRoutedError{Code: "INVALID_REF", Message: "invalid secret ref"}}, nil
	}
	if payload.Generation <= 0 {
		return rpc.RPCResponse{Error: &rpc.RPCRoutedError{Code: "INVALID_GENERATION", Message: "generation must be positive"}}, nil
	}

	result, err := h.adapter.AcquireServiceLease(
		ctx,
		string(request.RuntimeID),
		string(request.PluginID),
		string(request.ServiceID),
		payload.Ref,
		payload.Purpose,
		payload.Required,
		payload.Generation,
	)
	if err != nil {
		return rpc.RPCResponse{Error: &rpc.RPCRoutedError{Code: "LEASE_DENIED", Message: err.Error()}}, nil
	}

	respPayload, _ := json.Marshal(map[string]interface{}{
		"leaseId":   string(result.LeaseID),
		"ref":       string(result.Ref),
		"purpose":   result.Purpose,
		"expiresAt": result.ExpiresAt,
		"granted":   result.Granted,
		"reason":    result.Reason,
	})

	return rpc.RPCResponse{RequestID: request.ID, Payload: respPayload}, nil
}

type releaseHandler struct {
	adapter *SecretLeaseAdapter
}

func (h releaseHandler) Handle(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	return rpc.RPCResponse{Error: &rpc.RPCRoutedError{Code: "NOT_IMPLEMENTED", Message: "use full release RPC"}}, nil
}

type queryHandler struct {
	adapter *SecretLeaseAdapter
}

func (h queryHandler) Handle(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	return rpc.RPCResponse{Error: &rpc.RPCRoutedError{Code: "NOT_IMPLEMENTED", Message: "use full query RPC"}}, nil
}

var _ rpc.Handler = (*acquireHandler)(nil)
