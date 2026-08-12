package health

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/nativebridge"
)

type fakeHealthBridge struct {
	executeFunc func(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error)
	healthFunc  func(ctx context.Context) nativebridge.Health
}

func (f *fakeHealthBridge) Execute(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error) {
	if f.executeFunc != nil {
		return f.executeFunc(ctx, req)
	}
	return nativebridge.Response{
		ProtocolVersion: req.ProtocolVersion,
		RequestID:       req.RequestID,
		Status:          "success",
	}, nil
}

func (f *fakeHealthBridge) Health(ctx context.Context) nativebridge.Health {
	if f.healthFunc != nil {
		return f.healthFunc(ctx)
	}
	return nativebridge.HealthReady
}

func TestHealthHandler_UnknownOperation(t *testing.T) {
	bridge := &fakeHealthBridge{}
	h := NewHealthHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
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

func TestHealthHandler_AuthorizationStatus_NilBridge(t *testing.T) {
	h := NewHealthHandler(nil)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       "health.authorization.status",
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != errNativeBridgeUnavailable {
		t.Fatalf("expected errNativeBridgeUnavailable, got %+v", resp.Error)
	}
}

func TestHealthHandler_SamplesQuery_MissingType(t *testing.T) {
	bridge := &fakeHealthBridge{}
	h := NewHealthHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       "health.samples.query",
		Payload:         map[string]any{},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != errHealthQueryInvalid {
		t.Fatalf("expected errHealthQueryInvalid, got %+v", resp.Error)
	}
}

func TestHealthHandler_SamplesQuery_UnsupportedType(t *testing.T) {
	bridge := &fakeHealthBridge{}
	h := NewHealthHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       "health.samples.query",
		Payload:         map[string]any{"type": "unknown_type"},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != errHealthTypeUnsupported {
		t.Fatalf("expected errHealthTypeUnsupported, got %+v", resp.Error)
	}
}

func TestHealthHandler_SamplesQuery_ValidType(t *testing.T) {
	bridge := &fakeHealthBridge{
		executeFunc: func(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error) {
			return nativebridge.Response{
				ProtocolVersion: req.ProtocolVersion,
				RequestID:       req.RequestID,
				Status:          "success",
				Result: map[string]any{
					"samples": []any{},
				},
			}, nil
		},
	}
	h := NewHealthHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       "health.samples.query",
		Payload:         map[string]any{"type": "stepCount"},
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s", resp.Status)
	}
}

func TestHealthHandler_WorkoutsDetail_MissingID(t *testing.T) {
	bridge := &fakeHealthBridge{}
	h := NewHealthHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       "health.workouts.detail",
		Payload:         map[string]any{},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != errHealthQueryInvalid {
		t.Fatalf("expected errHealthQueryInvalid, got %+v", resp.Error)
	}
}
