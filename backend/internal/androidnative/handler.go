package androidnative

import (
	"context"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type Handler interface {
	Execute(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse
}

type Provider struct {
	bridge   NativeBridge
	handlers map[string]Handler
	mu       sync.RWMutex
}

func NewProvider(bridge NativeBridge) *Provider {
	return &Provider{
		bridge:   bridge,
		handlers: make(map[string]Handler),
	}
}

func (p *Provider) RegisterHandler(operation string, handler Handler) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.handlers[operation]; exists {
		return fmt.Errorf("androidnative: duplicate handler registration for operation %q", operation)
	}
	p.handlers[operation] = handler
	return nil
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

	p.mu.RLock()
	handler, ok := p.handlers[request.Operation]
	p.mu.RUnlock()

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
		switch h {
		case NativeBridgeHealthReady:
			return capability.HealthReady
		case NativeBridgeHealthUnhealthy:
			return capability.HealthUnhealthy
		default:
			return capability.HealthUnknown
		}
	}
}

