package iosnative

import (
	"context"
	"fmt"
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

func (p *Provider) RegisterHandler(operation string, handler Handler) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if operation == "" {
		return fmt.Errorf("iosnative: operation is required")
	}

	if handler == nil {
		return fmt.Errorf("iosnative: nil handler for %q", operation)
	}

	if _, exists := p.handlers[operation]; exists {
		return fmt.Errorf("iosnative: duplicate handler registration for operation %q", operation)
	}

	p.handlers[operation] = handler
	return nil
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

func (p *Provider) Cancel(ctx context.Context, requestID string, reason string) error {
	return nil
}
