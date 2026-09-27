package accessibility

import "github.com/u-ai/backend/internal/androidnative"

type AccessibilityState struct {
	PlatformSupported             bool   `json:"platformSupported"`
	ServiceDeclared               bool   `json:"serviceDeclared"`
	EnabledInSettings             bool   `json:"enabledInSettings"`
	Connected                     bool   `json:"connected"`
	CanRetrieveWindowContent      bool   `json:"canRetrieveWindowContent"`
	CanRetrieveInteractiveWindows bool   `json:"canRetrieveInteractiveWindows"`
	IncludeNotImportantViews      bool   `json:"includeNotImportantViews"`
	EnhancedWebAccessibility      bool   `json:"enhancedWebAccessibility"`
	CanPerformGestures            bool   `json:"canPerformGestures"`
	CanTakeScreenshot             bool   `json:"canTakeScreenshot"`
	InteractionReady              bool   `json:"interactionReady"`
	VisualReady                   bool   `json:"visualReady"`
	Ready                         bool   `json:"ready"`
	UserActionRequired            bool   `json:"userActionRequired"`
	State                         string `json:"state"`
	Generation                    int64  `json:"generation"`
}

func MapAccessibilityStateFromResult(result map[string]any) AccessibilityState {
	state := AccessibilityState{}

	if v, ok := result["platformSupported"].(bool); ok {
		state.PlatformSupported = v
	}
	if v, ok := result["serviceDeclared"].(bool); ok {
		state.ServiceDeclared = v
	}
	if v, ok := result["enabledInSettings"].(bool); ok {
		state.EnabledInSettings = v
	}
	if v, ok := result["connected"].(bool); ok {
		state.Connected = v
	}
	if v, ok := result["canRetrieveWindowContent"].(bool); ok {
		state.CanRetrieveWindowContent = v
	}
	if v, ok := result["canRetrieveInteractiveWindows"].(bool); ok {
		state.CanRetrieveInteractiveWindows = v
	}
	if v, ok := result["includeNotImportantViews"].(bool); ok {
		state.IncludeNotImportantViews = v
	}
	if v, ok := result["enhancedWebAccessibility"].(bool); ok {
		state.EnhancedWebAccessibility = v
	}
	if v, ok := result["canPerformGestures"].(bool); ok {
		state.CanPerformGestures = v
	}
	if v, ok := result["canTakeScreenshot"].(bool); ok {
		state.CanTakeScreenshot = v
	}
	if v, ok := result["interactionReady"].(bool); ok {
		state.InteractionReady = v
	}
	if v, ok := result["visualReady"].(bool); ok {
		state.VisualReady = v
	}
	if v, ok := result["ready"].(bool); ok {
		state.Ready = v
	}
	if v, ok := result["userActionRequired"].(bool); ok {
		state.UserActionRequired = v
	}
	if v, ok := result["state"].(string); ok {
		state.State = v
	}
	if v, ok := result["generation"].(float64); ok {
		state.Generation = int64(v)
	}

	return state
}

func DeriveAccessibilityState(result map[string]any) string {
	if v, ok := result["state"].(string); ok && v != "" {
		return v
	}

	enabledInSettings, _ := result["enabledInSettings"].(bool)
	connected, _ := result["connected"].(bool)
	ready, _ := result["ready"].(bool)

	switch {
	case !enabledInSettings:
		return androidnative.AccessibilityStateDisabled
	case !connected:
		return androidnative.AccessibilityStateEnabledNotConnected
	case !ready:
		return androidnative.AccessibilityStateDegraded
	default:
		return androidnative.AccessibilityStateConnected
	}
}
