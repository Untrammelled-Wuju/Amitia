package accessibility

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type mockAccessibilityBridge struct {
	executeFunc func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error)
}

func (m *mockAccessibilityBridge) Execute(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, req)
	}
	return androidnative.NativeBridgeResponse{
		ProtocolVersion: req.ProtocolVersion,
		RequestID:       req.RequestID,
		Status:          "success",
	}, nil
}

func (m *mockAccessibilityBridge) Health(ctx context.Context) androidnative.NativeBridgeHealth {
	return androidnative.NativeBridgeHealthReady
}

func TestAccessibilityHandler_Status(t *testing.T) {
	bridge := &mockAccessibilityBridge{
		executeFunc: func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				ProtocolVersion: 1,
				RequestID:       req.RequestID,
				Status:          "success",
				Result: map[string]any{
					"platformSupported": true,
					"serviceDeclared":   true,
					"enabledInSettings": true,
					"connected":         true,
					"state":             "connected",
					"generation":        float64(1),
				},
			}, nil
		},
	}
	handler := NewAccessibilityHandler(bridge)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "req-1",
		Operation:       "accessibility.status",
	})

	if resp.Status != "success" {
		t.Fatalf("expected success, got %s", resp.Status)
	}
	if resp.RequestID != "req-1" {
		t.Fatalf("expected request ID req-1, got %s", resp.RequestID)
	}
	if resp.Result["state"] != "connected" {
		t.Fatalf("expected state connected, got %v", resp.Result["state"])
	}
}

func TestAccessibilityHandler_OpenSettings(t *testing.T) {
	bridge := &mockAccessibilityBridge{
		executeFunc: func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				ProtocolVersion: 1,
				RequestID:       req.RequestID,
				Status:          "success",
				Result: map[string]any{
					"opened":            true,
					"userActionRequired": true,
				},
			}, nil
		},
	}
	handler := NewAccessibilityHandler(bridge)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "req-2",
		Operation:       "accessibility.open_settings",
	})

	if resp.Status != "success" {
		t.Fatalf("expected success, got %s", resp.Status)
	}
	if resp.Result["opened"] != true {
		t.Fatalf("expected opened=true, got %v", resp.Result["opened"])
	}
}

func TestAccessibilityHandler_UnknownOperation(t *testing.T) {
	bridge := &mockAccessibilityBridge{}
	handler := NewAccessibilityHandler(bridge)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "req-3",
		Operation:       "accessibility.dump_tree",
	})

	if resp.Status != "error" {
		t.Fatalf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != "OPERATION_NOT_SUPPORTED" {
		t.Fatalf("expected OPERATION_NOT_SUPPORTED, got %+v", resp.Error)
	}
}

func TestAccessibilityHandler_NilBridge(t *testing.T) {
	handler := NewAccessibilityHandler(nil)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "req-4",
		Operation:       "accessibility.status",
	})

	if resp.Status != "error" {
		t.Fatalf("expected error, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != androidnative.ACCESSIBILITY_BRIDGE_UNAVAILABLE {
		t.Fatalf("expected ACCESSIBILITY_BRIDGE_UNAVAILABLE, got %+v", resp.Error)
	}
}

func TestAccessibilityHandler_BridgeError(t *testing.T) {
	bridge := &mockAccessibilityBridge{
		executeFunc: func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{}, context.DeadlineExceeded
		},
	}
	handler := NewAccessibilityHandler(bridge)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "req-5",
		Operation:       "accessibility.status",
	})

	if resp.Status != "error" {
		t.Fatalf("expected error, got %s", resp.Status)
	}
}

func TestAccessibilityHandler_RequestIDMismatch(t *testing.T) {
	bridge := &mockAccessibilityBridge{
		executeFunc: func(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				ProtocolVersion: 1,
				RequestID:       "different-id",
				Status:          "success",
			}, nil
		},
	}
	handler := NewAccessibilityHandler(bridge)

	resp := handler.Execute(context.Background(), capability.AndroidBridgeRequest{
		ProtocolVersion: 1,
		RequestID:       "req-6",
		Operation:       "accessibility.status",
	})

	if resp.Status != "error" {
		t.Fatalf("expected error due to request ID mismatch, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != "BRIDGE_INVALID_RESPONSE" {
		t.Fatalf("expected BRIDGE_INVALID_RESPONSE, got %+v", resp.Error)
	}
}
