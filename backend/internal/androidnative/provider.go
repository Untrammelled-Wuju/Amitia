package androidnative

import (
	"context"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type NativeProviderAdapter struct {
	bridge   NativeBridge
	handlers map[string]Handler
	mu       sync.RWMutex
}

func NewNativeProviderAdapter(bridge NativeBridge) *NativeProviderAdapter {
	return &NativeProviderAdapter{
		bridge:   bridge,
		handlers: make(map[string]Handler),
	}
}

func (a *NativeProviderAdapter) RegisterHandler(operation string, handler Handler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.handlers[operation] = handler
}

func (a *NativeProviderAdapter) Execute(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	a.mu.RLock()
	handler, ok := a.handlers[request.Operation]
	a.mu.RUnlock()

	if !ok {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    "OPERATION_NOT_SUPPORTED",
				Message: "operation not supported: " + request.Operation,
			},
		}
	}

	return handler.Execute(ctx, request)
}

func (a *NativeProviderAdapter) Health(ctx context.Context) capability.HealthStatus {
	if a.bridge == nil {
		return capability.HealthUnhealthy
	}
	done := make(chan NativeBridgeHealth, 1)
	go func() {
		done <- a.bridge.Health(ctx)
	}()
	select {
	case <-ctx.Done():
		return capability.HealthUnknown
	case h := <-done:
		return mapNativeBridgeHealth(h)
	}
}

func mapNativeBridgeHealth(h NativeBridgeHealth) capability.HealthStatus {
	switch h {
	case NativeBridgeHealthReady:
		return capability.HealthReady
	case NativeBridgeHealthUnhealthy:
		return capability.HealthUnhealthy
	default:
		return capability.HealthUnknown
	}
}
