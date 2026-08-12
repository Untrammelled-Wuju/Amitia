package externalautomation

import (
	"testing"
)

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()

	if p.MaxResolveResults != 20 {
		t.Errorf("expected MaxResolveResults=20, got %d", p.MaxResolveResults)
	}
	if p.MaxExtras != 32 {
		t.Errorf("expected MaxExtras=32, got %d", p.MaxExtras)
	}
	if p.MaxExtraValueBytes != 16*1024 {
		t.Errorf("expected MaxExtraValueBytes=16384, got %d", p.MaxExtraValueBytes)
	}
	if p.MaxCategories != 8 {
		t.Errorf("expected MaxCategories=8, got %d", p.MaxCategories)
	}
	if p.MaxURIBytes != 8*1024 {
		t.Errorf("expected MaxURIBytes=8192, got %d", p.MaxURIBytes)
	}
	if p.MaxWaitTimeout != 30*1000*1000*1000 {
		t.Errorf("expected MaxWaitTimeout=30s, got %v", p.MaxWaitTimeout)
	}
	if !p.AllowExplicitComponent {
		t.Error("expected AllowExplicitComponent=true")
	}
}

func TestIsActionAllowed(t *testing.T) {
	p := DefaultPolicy()

	if !p.IsActionAllowed("android.intent.action.VIEW") {
		t.Error("expected VIEW action to be allowed")
	}
	if !p.IsActionAllowed("android.intent.action.MAIN") {
		t.Error("expected MAIN action to be allowed")
	}
	if !p.IsActionAllowed("android.intent.action.DIAL") {
		t.Error("expected DIAL action to be allowed")
	}
	if p.IsActionAllowed("com.vendor.custom.ACTION") {
		t.Error("expected vendor action to be blocked")
	}
	if p.IsActionAllowed("android.intent.action.SEND") {
		t.Error("expected SEND action to be blocked")
	}
}

func TestIsSchemeBlocked(t *testing.T) {
	p := DefaultPolicy()

	if !p.IsSchemeBlocked("javascript") {
		t.Error("expected javascript scheme to be blocked")
	}
	if !p.IsSchemeBlocked("file") {
		t.Error("expected file scheme to be blocked")
	}
	if !p.IsSchemeBlocked("data") {
		t.Error("expected data scheme to be blocked")
	}
	if !p.IsSchemeBlocked("intent") {
		t.Error("expected intent scheme to be blocked")
	}
	if p.IsSchemeBlocked("https") {
		t.Error("expected https scheme to be allowed")
	}
	if p.IsSchemeBlocked("mailto") {
		t.Error("expected mailto scheme to be allowed")
	}
}

func TestValidateResolveApp(t *testing.T) {
	p := DefaultPolicy()

	err := p.ValidateResolveApp(ResolveAppRequest{Query: "Chrome"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	err = p.ValidateResolveApp(ResolveAppRequest{Query: ""})
	if err == nil {
		t.Error("expected error for empty query")
	}

	err = p.ValidateResolveApp(ResolveAppRequest{Query: "   "})
	if err == nil {
		t.Error("expected error for whitespace query")
	}
}

func TestValidateOpenApp(t *testing.T) {
	p := DefaultPolicy()

	err := p.ValidateOpenApp(OpenAppRequest{PackageName: "com.example.app"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	err = p.ValidateOpenApp(OpenAppRequest{PackageName: ""})
	if err == nil {
		t.Error("expected error for empty packageName")
	}

	extras := make(map[string]any)
	for i := 0; i < p.MaxExtras+1; i++ {
		extras[string(rune(i))] = "value"
	}
	err = p.ValidateOpenApp(OpenAppRequest{PackageName: "com.example.app", Extras: extras})
	if err == nil {
		t.Error("expected error for too many extras")
	}
}

func TestValidateOpenAppInvalidExtraType(t *testing.T) {
	p := DefaultPolicy()

	err := p.ValidateOpenApp(OpenAppRequest{
		PackageName: "com.example.app",
		Extras: map[string]any{
			"key": map[string]any{"nested": "value"},
		},
	})
	if err == nil {
		t.Error("expected error for nested object extra")
	}
}

func TestValidateResolveURI(t *testing.T) {
	p := DefaultPolicy()

	err := p.ValidateResolveURI(ResolveURIRequest{URI: "https://example.com"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	err = p.ValidateResolveURI(ResolveURIRequest{URI: ""})
	if err == nil {
		t.Error("expected error for empty uri")
	}
}

func TestValidateResolveURITooLarge(t *testing.T) {
	p := DefaultPolicy()

	largeURI := make([]byte, p.MaxURIBytes+1)
	err := p.ValidateResolveURI(ResolveURIRequest{URI: string(largeURI)})
	if err == nil {
		t.Error("expected error for too large uri")
	}
}

func TestValidateOpenURI(t *testing.T) {
	p := DefaultPolicy()

	err := p.ValidateOpenURI(OpenURIRequest{URI: "https://example.com"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	err = p.ValidateOpenURI(OpenURIRequest{URI: ""})
	if err == nil {
		t.Error("expected error for empty uri")
	}
}

func TestValidateOpenURIBlockedScheme(t *testing.T) {
	p := DefaultPolicy()

	err := p.ValidateOpenURI(OpenURIRequest{URI: "javascript:alert(1)"})
	if err == nil {
		t.Error("expected error for blocked javascript scheme")
	}

	err = p.ValidateOpenURI(OpenURIRequest{URI: "file:///sdcard/test.txt"})
	if err == nil {
		t.Error("expected error for blocked file scheme")
	}

	err = p.ValidateOpenURI(OpenURIRequest{URI: "data:text/html,<script>alert(1)</script>"})
	if err == nil {
		t.Error("expected error for blocked data scheme")
	}

	err = p.ValidateOpenURI(OpenURIRequest{URI: "intent://scan/#Intent;scheme=zxing;end"})
	if err == nil {
		t.Error("expected error for blocked intent scheme")
	}
}

func TestValidateOpenURIAllowedSchemes(t *testing.T) {
	p := DefaultPolicy()

	allowed := []string{
		"https://example.com",
		"http://example.com",
		"mailto:test@example.com",
		"tel:1234567890",
		"geo:37.7749,-122.4194",
		"market://details?id=com.example.app",
		"myapp://deeplink",
	}

	for _, uri := range allowed {
		err := p.ValidateOpenURI(OpenURIRequest{URI: uri})
		if err != nil {
			t.Errorf("expected no error for uri %s, got %v", uri, err)
		}
	}
}

func TestValidateOpenSettings(t *testing.T) {
	p := DefaultPolicy()

	validPages := []string{
		SettingsAppDetails, SettingsAccessibility, SettingsOverlay,
		SettingsNotifications, SettingsBattery, SettingsUnknownSources,
		SettingsWireless, SettingsBluetooth, SettingsLocation, SettingsDefaultApps,
	}

	for _, page := range validPages {
		err := p.ValidateOpenSettings(OpenSettingsRequest{Page: page})
		if err != nil {
			t.Errorf("expected no error for page %s, got %v", page, err)
		}
	}

	err := p.ValidateOpenSettings(OpenSettingsRequest{Page: "invalid_page"})
	if err == nil {
		t.Error("expected error for invalid page")
	}
}

func TestValidateIntentSpec(t *testing.T) {
	p := DefaultPolicy()

	err := p.ValidateIntentSpec(IntentSpec{Action: "android.intent.action.VIEW"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	err = p.ValidateIntentSpec(IntentSpec{Action: ""})
	if err == nil {
		t.Error("expected error for empty action")
	}

	err = p.ValidateIntentSpec(IntentSpec{Action: "com.vendor.custom.ACTION"})
	if err == nil {
		t.Error("expected error for blocked action")
	}
}

func TestValidateIntentSpecSendRejected(t *testing.T) {
	p := DefaultPolicy()

	err := p.ValidateIntentSpec(IntentSpec{Action: "android.intent.action.SEND"})
	if err == nil {
		t.Error("expected error for SEND action")
	}
	ae, ok := err.(*automationError)
	if !ok || ae.code != AUTOMATION_INTENT_ACTION_BLOCKED {
		t.Errorf("expected AUTOMATION_INTENT_ACTION_BLOCKED, got %v", err)
	}
}

func TestValidateIntentSpecMode(t *testing.T) {
	p := DefaultPolicy()

	err := p.ValidateIntentSpec(IntentSpec{Action: "android.intent.action.VIEW", Mode: ModeActivity})
	if err != nil {
		t.Errorf("expected no error for activity mode, got %v", err)
	}

	err = p.ValidateIntentSpec(IntentSpec{Action: "android.intent.action.VIEW", Mode: "service"})
	if err == nil {
		t.Error("expected error for service mode")
	}

	err = p.ValidateIntentSpec(IntentSpec{Action: "android.intent.action.VIEW", Mode: "broadcast"})
	if err == nil {
		t.Error("expected error for broadcast mode")
	}
}

func TestValidateIntentSpecCategories(t *testing.T) {
	p := DefaultPolicy()

	categories := make([]string, p.MaxCategories+1)
	for i := range categories {
		categories[i] = "category_" + string(rune(i))
	}
	err := p.ValidateIntentSpec(IntentSpec{Action: "android.intent.action.VIEW", Categories: categories})
	if err == nil {
		t.Error("expected error for too many categories")
	}
}

func TestValidateWaitForeground(t *testing.T) {
	p := DefaultPolicy()

	err := p.ValidateWaitForeground(WaitForegroundRequest{PackageName: "com.example.app", TimeoutMS: 5000})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	err = p.ValidateWaitForeground(WaitForegroundRequest{PackageName: ""})
	if err == nil {
		t.Error("expected error for empty packageName")
	}

	err = p.ValidateWaitForeground(WaitForegroundRequest{PackageName: "com.example.app", TimeoutMS: -1})
	if err == nil {
		t.Error("expected error for negative timeout")
	}

	err = p.ValidateWaitForeground(WaitForegroundRequest{PackageName: "com.example.app", TimeoutMS: int(p.MaxWaitTimeout.Milliseconds()) + 1})
	if err == nil {
		t.Error("expected error for timeout exceeding maximum")
	}
}

func TestNormalizePage(t *testing.T) {
	if NormalizePage(SettingsAccessibility) != SettingsAccessibility {
		t.Error("expected accessibility")
	}
	if NormalizePage("invalid") != "" {
		t.Error("expected empty for invalid page")
	}
	if NormalizePage("") != "" {
		t.Error("expected empty for empty page")
	}
}

func TestNormalizeMode(t *testing.T) {
	if NormalizeMode(ModeActivity) != ModeActivity {
		t.Error("expected activity mode")
	}
	if NormalizeMode("invalid") != ModeActivity {
		t.Errorf("expected activity mode for invalid, got %s", NormalizeMode("invalid"))
	}
}

func TestExtractScheme(t *testing.T) {
	tests := []struct {
		uri      string
		expected string
	}{
		{"https://example.com", "https"},
		{"mailto:test@example.com", "mailto"},
		{"tel:1234567890", "tel"},
		{"myapp://deeplink", "myapp"},
		{"no-scheme", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := extractScheme(tt.uri)
		if got != tt.expected {
			t.Errorf("extractScheme(%q) = %q, want %q", tt.uri, got, tt.expected)
		}
	}
}

func TestIsValidExtraValue(t *testing.T) {
	validValues := []any{
		"string", true, int(1), int64(1), float64(1.5),
		[]string{"a", "b"},
	}
	for _, v := range validValues {
		if !isValidExtraValue(v) {
			t.Errorf("expected %v to be valid", v)
		}
	}

	invalidValues := []any{
		map[string]any{"nested": "value"},
		[]int{1, 2},
		struct{}{},
	}
	for _, v := range invalidValues {
		if isValidExtraValue(v) {
			t.Errorf("expected %v to be invalid", v)
		}
	}
}

func TestValidateOpenAppValidExtras(t *testing.T) {
	p := DefaultPolicy()

	err := p.ValidateOpenApp(OpenAppRequest{
		PackageName: "com.example.app",
		Extras: map[string]any{
			"str":   "value",
			"bool":  true,
			"int":   int(42),
			"float": float64(3.14),
			"arr":   []string{"a", "b"},
		},
	})
	if err != nil {
		t.Errorf("expected no error for valid extras, got %v", err)
	}
}

func TestPolicyIsActionAllowedWithNilMap(t *testing.T) {
	p := Policy{}
	if p.IsActionAllowed("android.intent.action.VIEW") {
		t.Error("expected false for nil AllowedActions map")
	}
}

func TestPolicyIsSchemeBlockedWithNilMap(t *testing.T) {
	p := Policy{}
	if p.IsSchemeBlocked("javascript") {
		t.Error("expected false for nil BlockedSchemes map")
	}
}
