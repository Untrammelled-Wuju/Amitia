package iosnative

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/nativebridge"
)

type IOSBridgeAdapter struct {
	provider *Provider
}

func NewIOSBridgeAdapter(provider *Provider) *IOSBridgeAdapter {
	return &IOSBridgeAdapter{provider: provider}
}

func (a *IOSBridgeAdapter) Execute(ctx context.Context, request capability.IOSBridgeRequest) capability.IOSBridgeResponse {
	req := nativebridge.Request{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Platform:        "ios",
		Operation:       request.Operation,
		Payload:         request.Payload,
	}

	resp := a.provider.Execute(ctx, req)

	return capability.IOSBridgeResponse{
		ProtocolVersion: resp.ProtocolVersion,
		RequestID:       resp.RequestID,
		Status:          resp.Status,
		Result:          resp.Result,
		Error: func() *capability.IOSError {
			if resp.Error == nil {
				return nil
			}
			return &capability.IOSError{
				Code:       resp.Error.Code,
				Message:    resp.Error.Message,
				DomainCode: resp.Error.DomainCode,
			}
		}(),
	}
}

func (a *IOSBridgeAdapter) Health(ctx context.Context) capability.HealthStatus {
	h := a.provider.Health(ctx)
	switch h {
	case nativebridge.HealthReady:
		return capability.HealthReady
	case nativebridge.HealthUnhealthy:
		return capability.HealthUnhealthy
	default:
		return capability.HealthUnknown
	}
}
