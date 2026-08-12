package iosnative

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/nativebridge"
)

type mockNativeBridge struct {
	executeFunc func(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error)
	healthFunc  func(ctx context.Context) nativebridge.Health
}

func (m *mockNativeBridge) Execute(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, req)
	}
	return nativebridge.Response{
		ProtocolVersion: req.ProtocolVersion,
		RequestID:       req.RequestID,
		Status:          "success",
	}, nil
}

func (m *mockNativeBridge) Health(ctx context.Context) nativebridge.Health {
	if m.healthFunc != nil {
		return m.healthFunc(ctx)
	}
	return nativebridge.HealthReady
}

func TestProvider_Execute_UnknownOperation(t *testing.T) {
	bridge := &mockNativeBridge{}
	p := NewProvider(bridge)

	resp := p.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       "health.unknown",
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != nativebridge.ErrOperationNotSupported {
		t.Fatalf("expected OPERATION_NOT_SUPPORTED, got %+v", resp.Error)
	}
}

func TestProvider_Execute_NilBridge(t *testing.T) {
	p := NewProvider(nil)

	resp := p.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       "health.authorization.status",
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != nativebridge.ErrProviderUnavailable {
		t.Fatalf("expected PROVIDER_UNAVAILABLE, got %+v", resp.Error)
	}
}

func TestProvider_Execute_RegisteredHandler(t *testing.T) {
	bridge := &mockNativeBridge{}
	p := NewProvider(bridge)

	handler := &testHandler{}
	p.RegisterHandler(OpHealthAuthorizationStatus, handler)

	resp := p.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       OpHealthAuthorizationStatus,
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s", resp.Status)
	}
}

func TestProvider_Health_NilBridge(t *testing.T) {
	p := NewProvider(nil)
	health := p.Health(context.Background())
	if health != nativebridge.HealthUnhealthy {
		t.Fatalf("expected unhealthy, got %s", health)
	}
}

func TestProvider_Health_Ready(t *testing.T) {
	bridge := &mockNativeBridge{
		healthFunc: func(ctx context.Context) nativebridge.Health {
			return nativebridge.HealthReady
		},
	}
	p := NewProvider(bridge)
	health := p.Health(context.Background())
	if health != nativebridge.HealthReady {
		t.Fatalf("expected ready, got %s", health)
	}
}

type testHandler struct{}

func (h *testHandler) Execute(ctx context.Context, req nativebridge.Request) nativebridge.Response {
	return nativebridge.Response{
		ProtocolVersion: req.ProtocolVersion,
		RequestID:       req.RequestID,
		Status:          "success",
	}
}
