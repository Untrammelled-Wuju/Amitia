package externalautomation

import (
	"strings"
	"time"
)

type Policy struct {
	MaxResolveResults int

	MaxExtras         int
	MaxExtraValueBytes int

	MaxCategories int

	MaxURIBytes int

	MaxWaitTimeout time.Duration

	AllowedActions map[string]struct{}

	BlockedSchemes map[string]struct{}

	AllowExplicitComponent bool
}

func DefaultPolicy() Policy {
	return Policy{
		MaxResolveResults: 20,

		MaxExtras:         32,
		MaxExtraValueBytes: 16 * 1024,

		MaxCategories: 8,

		MaxURIBytes: 8 * 1024,

		MaxWaitTimeout: 30 * time.Second,

		AllowedActions: map[string]struct{}{
			"android.intent.action.VIEW":   {},
			"android.intent.action.MAIN":   {},
			"android.intent.action.DIAL":   {},
			"android.intent.action.SENDTO": {},
		},

		BlockedSchemes: map[string]struct{}{
			"javascript": {},
			"file":       {},
			"data":       {},
			"intent":     {},
		},

		AllowExplicitComponent: true,
	}
}

func (p Policy) IsActionAllowed(action string) bool {
	if p.AllowedActions == nil {
		return false
	}
	_, ok := p.AllowedActions[action]
	return ok
}

func (p Policy) IsSchemeBlocked(scheme string) bool {
	if p.BlockedSchemes == nil {
		return false
	}
	_, ok := p.BlockedSchemes[strings.ToLower(scheme)]
	return ok
}

func (p Policy) ValidateResolveApp(req ResolveAppRequest) error {
	if strings.TrimSpace(req.Query) == "" {
		return newAutomationError(AUTOMATION_INVALID_REQUEST, "query is required")
	}
	return nil
}

func (p Policy) ValidateOpenApp(req OpenAppRequest) error {
	if strings.TrimSpace(req.PackageName) == "" {
		return newAutomationError(AUTOMATION_INVALID_REQUEST, "packageName is required")
	}
	if len(req.Extras) > p.MaxExtras {
		return newAutomationError(AUTOMATION_INVALID_REQUEST, "too many extras")
	}
	for _, v := range req.Extras {
		if !isValidExtraValue(v) {
			return newAutomationError(AUTOMATION_INVALID_REQUEST, "invalid extra value type")
		}
	}
	return nil
}

func (p Policy) ValidateResolveURI(req ResolveURIRequest) error {
	if strings.TrimSpace(req.URI) == "" {
		return newAutomationError(AUTOMATION_URI_INVALID, "uri is required")
	}
	if len(req.URI) > p.MaxURIBytes {
		return newAutomationError(AUTOMATION_URI_TOO_LARGE, "uri exceeds maximum size")
	}
	return nil
}

func (p Policy) ValidateOpenURI(req OpenURIRequest) error {
	if strings.TrimSpace(req.URI) == "" {
		return newAutomationError(AUTOMATION_URI_INVALID, "uri is required")
	}
	if len(req.URI) > p.MaxURIBytes {
		return newAutomationError(AUTOMATION_URI_TOO_LARGE, "uri exceeds maximum size")
	}
	scheme := extractScheme(req.URI)
	if p.IsSchemeBlocked(scheme) {
		return newAutomationError(AUTOMATION_URI_SCHEME_BLOCKED, "uri scheme is blocked: "+scheme)
	}
	return nil
}

func (p Policy) ValidateOpenSettings(req OpenSettingsRequest) error {
	switch req.Page {
	case SettingsAppDetails, SettingsAccessibility, SettingsOverlay,
		SettingsNotifications, SettingsBattery, SettingsUnknownSources,
		SettingsWireless, SettingsBluetooth, SettingsLocation, SettingsDefaultApps:
		return nil
	default:
		return newAutomationError(AUTOMATION_SETTINGS_UNSUPPORTED, "unsupported settings page: "+req.Page)
	}
}

func (p Policy) ValidateIntentSpec(spec IntentSpec) error {
	if strings.TrimSpace(spec.Action) == "" {
		return newAutomationError(AUTOMATION_INVALID_REQUEST, "intent action is required")
	}
	if !p.IsActionAllowed(spec.Action) {
		return newAutomationError(AUTOMATION_INTENT_ACTION_BLOCKED, "intent action is not allowed: "+spec.Action)
	}
	if spec.Mode != "" && spec.Mode != ModeActivity {
		return newAutomationError(AUTOMATION_INTENT_ACTION_BLOCKED, "intent mode is not allowed: "+spec.Mode)
	}
	if len(spec.Categories) > p.MaxCategories {
		return newAutomationError(AUTOMATION_INVALID_REQUEST, "too many categories")
	}
	if len(spec.Extras) > p.MaxExtras {
		return newAutomationError(AUTOMATION_INVALID_REQUEST, "too many extras")
	}
	for _, v := range spec.Extras {
		if !isValidExtraValue(v) {
			return newAutomationError(AUTOMATION_INVALID_REQUEST, "invalid extra value type")
		}
	}
	if spec.Action == "android.intent.action.SEND" {
		return newAutomationError(AUTOMATION_INTENT_ACTION_BLOCKED, "ACTION_SEND is not allowed, use canonical share capability")
	}
	return nil
}

func (p Policy) ValidateWaitForeground(req WaitForegroundRequest) error {
	if strings.TrimSpace(req.PackageName) == "" {
		return newAutomationError(AUTOMATION_INVALID_REQUEST, "packageName is required")
	}
	if req.TimeoutMS < 0 {
		return newAutomationError(AUTOMATION_INVALID_REQUEST, "timeoutMs must be non-negative")
	}
	if req.TimeoutMS > int(p.MaxWaitTimeout.Milliseconds()) {
		return newAutomationError(AUTOMATION_INVALID_REQUEST, "timeoutMs exceeds maximum allowed")
	}
	return nil
}

func isValidExtraValue(v any) bool {
	switch v.(type) {
	case string, bool, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return true
	case []string:
		return true
	default:
		return false
	}
}

func extractScheme(uri string) string {
	idx := strings.Index(uri, ":")
	if idx <= 0 {
		return ""
	}
	return strings.ToLower(uri[:idx])
}

func NormalizePage(page string) string {
	switch page {
	case SettingsAppDetails, SettingsAccessibility, SettingsOverlay,
		SettingsNotifications, SettingsBattery, SettingsUnknownSources,
		SettingsWireless, SettingsBluetooth, SettingsLocation, SettingsDefaultApps:
		return page
	default:
		return ""
	}
}

func NormalizeMode(mode string) string {
	if mode == ModeActivity {
		return ModeActivity
	}
	return ModeActivity
}
