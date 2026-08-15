package nativebridge

import (
	"context"
	"testing"
)

func TestRequestRoundTrip(t *testing.T) {
	req := Request{
		ProtocolVersion: 1,
		RequestId:       "test-req-1",
		Platform:        "ios",
		Operation:       "health.samples.query",
		Payload: map[string]any{
			"type":      "stepCount",
			"startTime": "2026-08-01T00:00:00Z",
			"endTime":   "2026-08-12T00:00:00Z",
		},
	}

	if req.Platform != "ios" {
		t.Fatalf("expected platform ios, got %s", req.Platform)
	}
	if req.ProtocolVersion != 1 {
		t.Fatalf("expected protocol version 1, got %d", req.ProtocolVersion)
	}
}

func TestResponseRoundTrip(t *testing.T) {
	resp := Response{
		ProtocolVersion: 1,
		RequestId:       "test-req-1",
		Status:          "success",
		Result: map[string]any{
			"count": 42,
		},
	}

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s", resp.Status)
	}
	if resp.Result["count"] != 42 {
		t.Fatalf("expected count 42, got %v", resp.Result["count"])
	}
}

func TestErrorMapping(t *testing.T) {
	resp := Response{
		ProtocolVersion: 1,
		RequestId:       "test-err-1",
		Status:          "error",
		Error: &Error{
			Code:       ErrAuthorizationDenied,
			Message:    "user denied health access",
			DomainCode: "HEALTH_AUTHORIZATION_REQUIRED",
		},
	}

	if resp.Error == nil {
		t.Fatal("expected error to be non-nil")
	}
	if resp.Error.Code != ErrAuthorizationDenied {
		t.Fatalf("expected authorization denied, got %s", resp.Error.Code)
	}
}

func TestHealthMapping(t *testing.T) {
	tests := []struct {
		name   string
		health Health
	}{
		{"ready", HealthReady},
		{"unhealthy", HealthUnhealthy},
		{"unknown", HealthUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h Health = tt.health
			if h != tt.health {
				t.Fatalf("expected %s, got %s", tt.health, h)
			}
		})
	}
}

type fakeBridge struct {
	executeFunc func(ctx context.Context, req Request) (Response, error)
	healthFunc  func(ctx context.Context) Health
	attached    bool
}

func (f *fakeBridge) Execute(ctx context.Context, req Request) (Response, error) {
	if f.executeFunc != nil {
		return f.executeFunc(ctx, req)
	}
	return Response{
		ProtocolVersion: req.ProtocolVersion,
		RequestId:       req.RequestId,
		Status:          "success",
	}, nil
}

func (f *fakeBridge) Health(ctx context.Context) Health {
	if f.healthFunc != nil {
		return f.healthFunc(ctx)
	}
	return HealthReady
}

func (f *fakeBridge) SessionAttached() bool {
	return f.attached
}

func TestBridgeExecute(t *testing.T) {
	bridge := &fakeBridge{}
	resp, err := bridge.Execute(context.Background(), Request{
		ProtocolVersion: 1,
		RequestId:       "test-1",
		Platform:        "ios",
		Operation:       "health.authorization.status",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %s", resp.Status)
	}
}

func TestBridgeHealth(t *testing.T) {
	bridge := &fakeBridge{}
	h := bridge.Health(context.Background())
	if h != HealthReady {
		t.Fatalf("expected ready, got %s", h)
	}
}

func TestContextCancel(t *testing.T) {
	bridge := &fakeBridge{
		healthFunc: func(ctx context.Context) Health {
			<-ctx.Done()
			return HealthUnknown
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	h := bridge.Health(ctx)
	if h != HealthUnknown {
		t.Fatalf("expected unknown, got %s", h)
	}
}
