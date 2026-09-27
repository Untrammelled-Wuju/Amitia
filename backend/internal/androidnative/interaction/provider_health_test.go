package interaction

import "testing"

func TestHealthFromBridgeResponseRequiresAccessibilityCapabilities(t *testing.T) {
	health := healthFromBridgeResponse("accessibility", map[string]any{
		"connected":                true,
		"state":                    "connected",
		"canRetrieveWindowContent": false,
	}, nil)
	if health.State != ProviderStateDegraded {
		t.Fatalf("expected degraded health, got %s", health.State)
	}
}

func TestHealthFromBridgeResponseRejectsMissingGestureCapability(t *testing.T) {
	health := healthFromBridgeResponse("accessibility_gesture", map[string]any{
		"connected":        true,
		"gestureAvailable": false,
	}, nil)
	if health.State != ProviderStateDegraded {
		t.Fatalf("expected degraded gesture health, got %s", health.State)
	}
}

func TestHealthFromBridgeResponseKeepsReadyCapabilities(t *testing.T) {
	health := healthFromBridgeResponse("accessibility", map[string]any{
		"connected":                true,
		"state":                    "connected",
		"canRetrieveWindowContent": true,
		"interactionReady":         true,
	}, nil)
	if health.State != ProviderStateReady {
		t.Fatalf("expected ready health, got %s", health.State)
	}
}
