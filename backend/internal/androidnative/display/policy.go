package display

import "github.com/u-ai/backend/internal/extension/kernel/capability"

const (
	RuntimeID    = "android_native_display"
	MaxDisplays  = 32
	DefaultDisplayID = 0
	MinAPIGestureDisplay = 30
	APITopologyDisplay   = 36
	APIGestureMin        = 30
	APIMultiDisplayWin   = 30
	APIMinLaunchDisplay  = 26
)

var RuntimeBinding = capability.RuntimeBinding{
	RuntimeType: capability.RuntimeTypeAndroid_Native,
	RuntimeID:   RuntimeID,
}

var DefaultSelectionPolicy = DisplaySelectionPolicy{
	PreferExplicit:       true,
	AllowDefaultFallback: false,
	RejectAmbiguous:      true,
}

func SelectionPolicyFromConfig(allowFallback, rejectAmbiguous bool) DisplaySelectionPolicy {
	return DisplaySelectionPolicy{
		PreferExplicit:       true,
		AllowDefaultFallback: allowFallback,
		RejectAmbiguous:      rejectAmbiguous,
	}
}
