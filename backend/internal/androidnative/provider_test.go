package androidnative

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type mockBridge struct {
	executeFunc func(ctx context.Context, req NativeBridgeRequest) (NativeBridgeResponse, error)
	healthFunc  func(ctx context.Context) NativeBridgeHealth
}

func (m *mockBridge) Execute(ctx context.Context, req NativeBridgeRequest) (NativeBridgeResponse, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, req)
	}
	return NativeBridgeResponse{
		ProtocolVersion: req.ProtocolVersion,
		RequestID:       req.RequestID,
		Status:          "success",
	}, nil
}

func (m *mockBridge) Health(ctx context.Context) NativeBridgeHealth {
	if m.healthFunc != nil {
		return m.healthFunc(ctx)
	}
	return NativeBridgeHealthReady
}

func TestProvider_Execute_UnknownOperation(t *testing.T) {
	bridge := &mockBridge{}
	p := NewProvider(bridge)

	resp := p.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       "accessibility.enable",
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != "OPERATION_NOT_SUPPORTED" {
		t.Fatalf("expected OPERATION_NOT_SUPPORTED, got %+v", resp.Error)
	}
}

func TestProvider_Execute_NilBridge(t *testing.T) {
	p := NewProvider(nil)

	resp := p.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       "accessibility.status",
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ACCESSIBILITY_BRIDGE_UNAVAILABLE {
		t.Fatalf("expected ACCESSIBILITY_BRIDGE_UNAVAILABLE, got %+v", resp.Error)
	}
}

func TestProvider_Execute_RegisteredHandler(t *testing.T) {
	bridge := &mockBridge{
		executeFunc: func(ctx context.Context, req NativeBridgeRequest) (NativeBridgeResponse, error) {
			return NativeBridgeResponse{
				ProtocolVersion: req.ProtocolVersion,
				RequestID:       req.RequestID,
				Status:          "success",
				Result: map[string]any{
					"state": "connected",
				},
			}, nil
		},
	}
	p := NewProvider(bridge)

	handler := &testHandler{}
	p.RegisterHandler("accessibility.status", handler)

	resp := p.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Operation:       "accessibility.status",
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s", resp.Status)
	}
}

func TestProvider_Health_NilBridge(t *testing.T) {
	p := NewProvider(nil)
	health := p.Health(context.Background())
	if health != capability.HealthUnhealthy {
		t.Fatalf("expected unhealthy, got %s", health)
	}
}

func TestProvider_Health_Ready(t *testing.T) {
	bridge := &mockBridge{
		healthFunc: func(ctx context.Context) NativeBridgeHealth {
			return NativeBridgeHealthReady
		},
	}
	p := NewProvider(bridge)
	health := p.Health(context.Background())
	if health != capability.HealthReady {
		t.Fatalf("expected ready, got %s", health)
	}
}

func TestNativeProviderAdapter_Execute_UnknownOperation(t *testing.T) {
	bridge := &mockBridge{}
	a := NewNativeProviderAdapter(bridge)

	resp := a.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "test-adapter-1",
		Operation:       "accessibility.unknown",
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != "OPERATION_NOT_SUPPORTED" {
		t.Fatalf("expected OPERATION_NOT_SUPPORTED, got %+v", resp.Error)
	}
}

func TestNativeProviderAdapter_Health_NilBridge(t *testing.T) {
	a := NewNativeProviderAdapter(nil)
	health := a.Health(context.Background())
	if health != capability.HealthUnhealthy {
		t.Fatalf("expected unhealthy, got %s", health)
	}
}

type testHandler struct{}

func (h *testHandler) Execute(ctx context.Context, req capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	return capability.AndroidBridgeResponse{
		ProtocolVersion: req.ProtocolVersion,
		RequestID:       req.RequestID,
		Status:          "success",
	}
}
