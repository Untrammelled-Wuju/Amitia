package accessibility

import (
	"testing"
)

func TestMapAccessibilityStateFromResult(t *testing.T) {
	result := map[string]any{
		"platformSupported":             true,
		"serviceDeclared":               true,
		"enabledInSettings":             true,
		"connected":                     true,
		"canRetrieveWindowContent":      true,
		"canRetrieveInteractiveWindows": true,
		"includeNotImportantViews":      true,
		"enhancedWebAccessibility":      true,
		"canPerformGestures":            true,
		"canTakeScreenshot":             true,
		"interactionReady":              true,
		"visualReady":                   true,
		"ready":                         true,
		"userActionRequired":            false,
		"state":                         "connected",
		"generation":                    float64(4),
	}

	state := MapAccessibilityStateFromResult(result)

	if !state.PlatformSupported {
		t.Fatalf("expected PlatformSupported=true")
	}
	if !state.ServiceDeclared {
		t.Fatalf("expected ServiceDeclared=true")
	}
	if !state.EnabledInSettings {
		t.Fatalf("expected EnabledInSettings=true")
	}
	if !state.Connected {
		t.Fatalf("expected Connected=true")
	}
	if !state.CanRetrieveWindowContent {
		t.Fatalf("expected CanRetrieveWindowContent=true")
	}
	if !state.CanRetrieveInteractiveWindows {
		t.Fatalf("expected CanRetrieveInteractiveWindows=true")
	}
	if !state.IncludeNotImportantViews {
		t.Fatalf("expected IncludeNotImportantViews=true")
	}
	if !state.EnhancedWebAccessibility {
		t.Fatalf("expected EnhancedWebAccessibility=true")
	}
	if !state.CanPerformGestures {
		t.Fatalf("expected CanPerformGestures=true")
	}
	if !state.CanTakeScreenshot {
		t.Fatalf("expected CanTakeScreenshot=true")
	}
	if !state.InteractionReady || !state.VisualReady || !state.Ready {
		t.Fatalf("expected readiness flags to be true")
	}
	if state.UserActionRequired {
		t.Fatalf("expected UserActionRequired=false")
	}
	if state.State != "connected" {
		t.Fatalf("expected state=connected, got %s", state.State)
	}
	if state.Generation != 4 {
		t.Fatalf("expected generation=4, got %d", state.Generation)
	}
}

func TestMapAccessibilityStateFromResult_EmptyResult(t *testing.T) {
	state := MapAccessibilityStateFromResult(map[string]any{})

	if state.PlatformSupported {
		t.Fatalf("expected PlatformSupported=false for empty result")
	}
	if state.State != "" {
		t.Fatalf("expected empty state for empty result")
	}
}

func TestDeriveAccessibilityState_Connected(t *testing.T) {
	result := map[string]any{
		"enabledInSettings": true,
		"connected":         true,
		"state":             "connected",
	}
	state := DeriveAccessibilityState(result)
	if state != "connected" {
		t.Fatalf("expected connected, got %s", state)
	}
}

func TestDeriveAccessibilityState_Disabled(t *testing.T) {
	result := map[string]any{
		"enabledInSettings": false,
	}
	state := DeriveAccessibilityState(result)
	if state != "disabled" {
		t.Fatalf("expected disabled, got %s", state)
	}
}

func TestDeriveAccessibilityState_EnabledNotConnected(t *testing.T) {
	result := map[string]any{
		"enabledInSettings": true,
		"connected":         false,
	}
	state := DeriveAccessibilityState(result)
	if state != "enabled_not_connected" {
		t.Fatalf("expected enabled_not_connected, got %s", state)
	}
}

func TestDeriveAccessibilityState_Degraded(t *testing.T) {
	result := map[string]any{
		"enabledInSettings": true,
		"connected":         true,
		"ready":             false,
	}
	state := DeriveAccessibilityState(result)
	if state != "degraded" {
		t.Fatalf("expected degraded, got %s", state)
	}
}

func TestDeriveAccessibilityState_FallbackNoState(t *testing.T) {
	result := map[string]any{}
	state := DeriveAccessibilityState(result)
	if state != "disabled" {
		t.Fatalf("expected disabled, got %s", state)
	}
}
