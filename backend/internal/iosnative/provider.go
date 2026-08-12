package iosnative

import (
	"context"
	"sync"

	"github.com/u-ai/backend/internal/nativebridge"
)

type Provider struct {
	bridge   nativebridge.Bridge
	handlers map[string]Handler
	mu       sync.RWMutex
}

func NewProvider(bridge nativebridge.Bridge) *Provider {
	return &Provider{
		bridge:   bridge,
		handlers: make(map[string]Handler),
	}
}

func (p *Provider) RegisterHandler(operation string, handler Handler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[operation] = handler
}

func (p *Provider) Execute(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if p.bridge == nil {
		return nativebridge.Response{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &nativebridge.Error{
				Code:    nativebridge.ErrProviderUnavailable,
				Message: "ios native bridge is not available",
			},
		}
	}

	p.mu.RLock()
	handler, ok := p.handlers[request.Operation]
	p.mu.RUnlock()

	if !ok {
		return nativebridge.Response{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &nativebridge.Error{
				Code:    nativebridge.ErrOperationNotSupported,
				Message: "operation not supported: " + request.Operation,
			},
		}
	}

	return handler.Execute(ctx, request)
}

func (p *Provider) Health(ctx context.Context) nativebridge.Health {
	if p.bridge == nil {
		return nativebridge.HealthUnhealthy
	}
	done := make(chan nativebridge.Health, 1)
	go func() {
		done <- p.bridge.Health(ctx)
	}()
	select {
	case <-ctx.Done():
		return nativebridge.HealthUnknown
	case h := <-done:
		return h
	}
}
