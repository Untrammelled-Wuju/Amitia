package uitree

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/androidnative"
)

type requestIDBridge struct {
	requestIDs []string
}

func (b *requestIDBridge) Execute(_ context.Context, request androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
	b.requestIDs = append(b.requestIDs, request.RequestId)
	return androidnative.NativeBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestId:       request.RequestId,
		Status:          "success",
		Result: map[string]any{
			"connected":                request.Operation == "accessibility.status",
			"canRetrieveWindowContent": request.Operation == "accessibility.status",
			"generation":               1,
			"capturedAt":               1,
			"nodes":                    []any{},
			"windows":                  []any{},
			"truncated":                false,
			"multiWindow":              false,
			"accessibilityConnected":   true,
		},
	}, nil
}

func (b *requestIDBridge) Health(context.Context) androidnative.NativeBridgeHealth {
	return androidnative.NativeBridgeHealthReady
}

func TestAccessibilitySourceUsesRequestID(t *testing.T) {
	bridge := &requestIDBridge{}
	source := NewAccessibilitySource(bridge, DefaultPolicy())

	status := source.Status(context.Background())
	if !status.Available {
		t.Fatalf("expected accessibility source available, got %#v", status)
	}
	if _, err := source.Snapshot(context.Background(), SnapshotRequest{}); err != nil {
		t.Fatalf("unexpected snapshot error: %v", err)
	}
	if len(bridge.requestIDs) != 2 {
		t.Fatalf("request count = %d, want 2", len(bridge.requestIDs))
	}
	for index, requestID := range bridge.requestIDs {
		if requestID == "" {
			t.Fatalf("request %d has empty request ID", index)
		}
	}
}
