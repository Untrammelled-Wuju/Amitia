package androidnative

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type Handler interface {
	Execute(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse
}

type Provider struct {
	bridge   NativeBridge
	handlers map[string]Handler
}

func NewProvider(bridge NativeBridge) *Provider {
	return &Provider{
		bridge:   bridge,
		handlers: make(map[string]Handler),
	}
}

func (p *Provider) RegisterHandler(operation string, handler Handler) {
	p.handlers[operation] = handler
}

func (p *Provider) Execute(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if p.bridge == nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    ACCESSIBILITY_BRIDGE_UNAVAILABLE,
				Message: "android native bridge is not available",
			},
		}
	}

	handler, ok := p.handlers[request.Operation]
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

func (p *Provider) Health(ctx context.Context) capability.HealthStatus {
	if p.bridge == nil {
		return capability.HealthUnhealthy
	}
	done := make(chan NativeBridgeHealth, 1)
	go func() {
		done <- p.bridge.Health(ctx)
	}()
	select {
	case <-ctx.Done():
		return capability.HealthUnknown
	case h := <-done:
		return mapNativeBridgeHealth(h)
	}
}

