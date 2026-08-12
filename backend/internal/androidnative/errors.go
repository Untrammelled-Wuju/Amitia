package androidnative

const (
	PROVIDER_UNAVAILABLE = "PROVIDER_UNAVAILABLE"

	ACCESSIBILITY_UNSUPPORTED            = "ACCESSIBILITY_UNSUPPORTED"
	ACCESSIBILITY_NOT_DECLARED           = "ACCESSIBILITY_NOT_DECLARED"
	ACCESSIBILITY_DISABLED               = "ACCESSIBILITY_DISABLED"
	ACCESSIBILITY_NOT_CONNECTED          = "ACCESSIBILITY_NOT_CONNECTED"
	ACCESSIBILITY_SETTINGS_UNAVAILABLE   = "ACCESSIBILITY_SETTINGS_UNAVAILABLE"
	ACCESSIBILITY_BRIDGE_UNAVAILABLE     = "ACCESSIBILITY_BRIDGE_UNAVAILABLE"
	ACCESSIBILITY_BRIDGE_TIMEOUT         = "ACCESSIBILITY_BRIDGE_TIMEOUT"
	ACCESSIBILITY_INVALID_REQUEST        = "ACCESSIBILITY_INVALID_REQUEST"
)

const (
	PermissionAccessibilityReadState  = "android.accessibility.read_state"
	PermissionAccessibilityOpenSettings = "android.accessibility.open_settings"
)

const (
	AccessibilityStateUnsupported         = "unsupported"
	AccessibilityStateNotDeclared         = "not_declared"
	AccessibilityStateDisabled            = "disabled"
	AccessibilityStateEnabledNotConnected = "enabled_not_connected"
	AccessibilityStateConnected           = "connected"
)
