package browser

import (
	"context"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "download"},
		{"report.pdf", "report.pdf"},
		{"../../etc/passwd", "passwd"},
		{"C:\\Windows\\System32\\evil.dll", "evil.dll"},
		{"normal-file_name.txt", "normal-file_name.txt"},
		{"/absolute/path/file.doc", "file.doc"},
		{"file/with/slashes.txt", "slashes.txt"},
		{"   spaces.pdf  ", "spaces.pdf"},
		{"file\x00null.exe", "filenull.exe"},
	}

	for _, tt := range tests {
		result := SanitizeFilename(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestDownloadPolicy(t *testing.T) {
	policy := DefaultDownloadPolicy()

	if policy.MaxBytes != DefaultMaxDownloadBytes {
		t.Errorf("MaxBytes = %d, want %d", policy.MaxBytes, DefaultMaxDownloadBytes)
	}

	if policy.Timeout != DefaultDownloadTimeout {
		t.Errorf("Timeout = %v, want %v", policy.Timeout, DefaultDownloadTimeout)
	}

	if policy.Behavior != DownloadBehaviorAllowAndName {
		t.Errorf("Behavior = %s, want %s", policy.Behavior, DownloadBehaviorAllowAndName)
	}
}

func TestScreenshotPolicy(t *testing.T) {
	policy := DefaultScreenshotPolicy()

	if policy.MaxPixels != MaxScreenshotPixels {
		t.Errorf("MaxPixels = %d, want %d", policy.MaxPixels, MaxScreenshotPixels)
	}

	if policy.MaxBytes != MaxScreenshotBytes {
		t.Errorf("MaxBytes = %d, want %d", policy.MaxBytes, MaxScreenshotBytes)
	}

	if policy.MaxDimension != MaxDimension {
		t.Errorf("MaxDimension = %d, want %d", policy.MaxDimension, MaxDimension)
	}

	if !policy.AllowFullPage {
		t.Error("AllowFullPage should be true")
	}

	if policy.DefaultFormat != DefaultScreenshotFormat {
		t.Errorf("DefaultFormat = %s, want %s", policy.DefaultFormat, DefaultScreenshotFormat)
	}

	if len(policy.AllowedFormats) != 3 {
		t.Errorf("AllowedFormats length = %d, want 3", len(policy.AllowedFormats))
	}
}

func TestIsValidScreenshotFormat(t *testing.T) {
	tests := []struct {
		format string
		valid  bool
	}{
		{"png", true},
		{"jpeg", true},
		{"webp", true},
		{"bmp", false},
		{"gif", false},
		{"", false},
		{"PNG", false},
	}

	for _, tt := range tests {
		result := IsValidScreenshotFormat(tt.format)
		if result != tt.valid {
			t.Errorf("IsValidScreenshotFormat(%q) = %v, want %v", tt.format, result, tt.valid)
		}
	}
}

func TestIsValidScreenshotQuality(t *testing.T) {
	tests := []struct {
		format  string
		quality int
		valid   bool
	}{
		{"png", 0, true},
		{"png", 50, false},
		{"png", 100, false},
		{"jpeg", 0, true},
		{"jpeg", 50, true},
		{"jpeg", 100, true},
		{"webp", 0, true},
		{"webp", 80, true},
		{"jpeg", -1, false},
		{"jpeg", 101, false},
	}

	for _, tt := range tests {
		result := IsValidScreenshotQuality(tt.format, tt.quality)
		if result != tt.valid {
			t.Errorf("IsValidScreenshotQuality(%q, %d) = %v, want %v", tt.format, tt.quality, result, tt.valid)
		}
	}
}

func TestIsInputTypeFile(t *testing.T) {
	tests := []struct {
		name       string
		attributes map[string]string
		result     bool
	}{
		{"input", map[string]string{"type": "file"}, true},
		{"input", map[string]string{"type": "text"}, false},
		{"input", map[string]string{}, false},
		{"div", map[string]string{"type": "file"}, false},
		{"INPUT", map[string]string{"type": "FILE"}, true},
	}

	for _, tt := range tests {
		result := IsInputTypeFile(tt.name, tt.attributes)
		if result != tt.result {
			t.Errorf("IsInputTypeFile(%q, %v) = %v, want %v", tt.name, tt.attributes, result, tt.result)
		}
	}
}

func TestIsAcceptableFileType(t *testing.T) {
	tests := []struct {
		accept   string
		mimeType string
		result   bool
	}{
		{"", "application/pdf", true},
		{"image/*", "image/png", true},
		{"image/*", "application/pdf", false},
		{"application/pdf", "application/pdf", true},
		{"application/pdf,image/*", "image/jpeg", true},
		{"application/json", "application/xml", false},
		{"*/*", "anything/type", true},
	}

	for _, tt := range tests {
		result := IsAcceptableFileType(tt.accept, tt.mimeType)
		if result != tt.result {
			t.Errorf("IsAcceptableFileType(%q, %q) = %v, want %v", tt.accept, tt.mimeType, result, tt.result)
		}
	}
}

func TestIsSafeRecoverableURL(t *testing.T) {
	allowedSchemes := []string{"http", "https", "about"}

	tests := []struct {
		url  string
		safe bool
	}{
		{"https://example.com/page", true},
		{"http://example.com/", true},
		{"about:blank", true},
		{"", false},
		{"https://example.com/page?token=secret123", false},
		{"https://example.com/page?code=auth", false},
		{"https://example.com/page#access_token=xxx", false},
		{"file:///etc/passwd", false},
		{"javascript:alert(1)", false},
		{"data:text/html,test", false},
		{"chrome://settings", false},
	}

	for _, tt := range tests {
		result := IsSafeRecoverableURL(tt.url, allowedSchemes)
		if result != tt.safe {
			t.Errorf("IsSafeRecoverableURL(%q) = %v, want %v", tt.url, result, tt.safe)
		}
	}
}

func TestRecoveryStore(t *testing.T) {
	store := NewRecoveryStore(10)

	store.SaveSession(&sessionRecoveryDescriptor{
		sessionID:   "s1",
		state:       SessionStateReady,
		recoverable: true,
	})

	store.SaveSession(&sessionRecoveryDescriptor{
		sessionID:   "s2",
		state:       SessionStateReady,
		recoverable: false,
	})

	recoverable := store.GetRecoverableSessions()
	if len(recoverable) != 1 {
		t.Fatalf("Expected 1 recoverable session, got %d", len(recoverable))
	}
	if recoverable[0].sessionID != "s1" {
		t.Errorf("Expected session s1, got %s", recoverable[0].sessionID)
	}

	store.MarkSessionRecoverable("s1", false)
	recoverable = store.GetRecoverableSessions()
	if len(recoverable) != 0 {
		t.Errorf("Expected 0 recoverable sessions after marking false, got %d", len(recoverable))
	}

	store.SaveTab(&tabRecoveryState{
		tabID:       "t1",
		recoverable: true,
	})

	tabs := store.GetRecoverableTabs()
	if len(tabs) != 1 {
		t.Fatalf("Expected 1 recoverable tab, got %d", len(tabs))
	}

	store.RemoveTab("t1")
	tabs = store.GetRecoverableTabs()
	if len(tabs) != 0 {
		t.Errorf("Expected 0 tabs after removal, got %d", len(tabs))
	}

	store.SaveSession(&sessionRecoveryDescriptor{
		sessionID:   "s3",
		state:       SessionStateReady,
		recoverable: true,
	})
	store.SaveTab(&tabRecoveryState{
		tabID:       "t2",
		recoverable: true,
	})

	store.InvalidateAll()
	recoverable = store.GetRecoverableSessions()
	if len(recoverable) != 0 {
		t.Errorf("Expected 0 recoverable sessions after InvalidateAll, got %d", len(recoverable))
	}

	tabs = store.GetRecoverableTabs()
	if len(tabs) != 0 {
		t.Errorf("Expected 0 recoverable tabs after InvalidateAll, got %d", len(tabs))
	}
}

func TestCrashDetector(t *testing.T) {
	detector := NewCrashDetector()

	if !detector.IsRuntimeOK() {
		t.Error("Runtime should be OK initially")
	}

	detector.MarkTabCrashed("t1")
	if !detector.IsTabCrashed("t1") {
		t.Error("Tab t1 should be marked as crashed")
	}
	if detector.IsTabCrashed("t2") {
		t.Error("Tab t2 should not be marked as crashed")
	}

	detector.MarkRuntimeFailure()
	if detector.IsRuntimeOK() {
		t.Error("Runtime should be marked as failed")
	}

	detector.Reset()
	if !detector.IsRuntimeOK() {
		t.Error("Runtime should be OK after reset")
	}
	if detector.IsTabCrashed("t1") {
		t.Error("Tab t1 should not be crashed after reset")
	}
}

func TestUploadFilenameSanitization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "upload"},
		{"document.pdf", "document.pdf"},
		{"../../../etc/shadow", "shadow"},
		{"path\\to\\file.txt", "file.txt"},
		{"/absolute/path/file.doc", "file.doc"},
	}

	for _, tt := range tests {
		result := SanitizeUploadFilename(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeUploadFilename(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFullPageScreenshotDenied(t *testing.T) {
	transfer := &productionResourceTransfer{
		screenshotPolicy: ScreenshotPolicy{
			AllowFullPage: false,
		},
	}

	_, err := transfer.Screenshot(context.Background(), BrowserScreenshotRequest{
		SessionID: "s1",
		TabID:     "t1",
		FullPage:  true,
	})

	if err == nil {
		t.Fatal("Expected error when full page is not allowed")
	}
}

func TestGetTabState(t *testing.T) {
	store := NewRecoveryStore(10)

	store.SaveTab(&tabRecoveryState{
		tabID:            "t1",
		lastCommittedURL: "https://example.com",
		active:           true,
		recoverable:      true,
	})

	state, ok := store.GetTabState("t1")
	if !ok {
		t.Fatal("Expected to find tab t1")
	}
	if state.lastCommittedURL != "https://example.com" {
		t.Errorf("Expected URL https://example.com, got %s", state.lastCommittedURL)
	}
	if !state.active {
		t.Error("Expected tab to be active")
	}

	_, ok = store.GetTabState("nonexistent")
	if ok {
		t.Error("Expected not to find nonexistent tab")
	}
}

func TestGetSession(t *testing.T) {
	store := NewRecoveryStore(10)

	store.SaveSession(&sessionRecoveryDescriptor{
		sessionID:   "s1",
		state:       SessionStateReady,
		recoverable: true,
	})

	desc, ok := store.GetSession("s1")
	if !ok {
		t.Fatal("Expected to find session s1")
	}
	if desc.state != SessionStateReady {
		t.Errorf("Expected state ready, got %s", desc.state)
	}

	_, ok = store.GetSession("nonexistent")
	if ok {
		t.Error("Expected not to find nonexistent session")
	}
}
