package screenshot

import (
	"encoding/json"
	"testing"
)

func TestCaptureRequest_Validate_Default(t *testing.T) {
	req := CaptureRequest{}
	if err := req.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCaptureRequest_Validate_InvalidFormat(t *testing.T) {
	req := CaptureRequest{
		Format: formatPtr(ScreenshotFormat("bmp")),
	}
	if err := req.Validate(); err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestCaptureRequest_Validate_QualityBounds(t *testing.T) {
	quality0 := 0
	req0 := CaptureRequest{Quality: &quality0}
	if err := req0.Validate(); err == nil {
		t.Fatal("expected error for quality 0")
	}

	quality101 := 101
	req101 := CaptureRequest{Quality: &quality101}
	if err := req101.Validate(); err == nil {
		t.Fatal("expected error for quality 101")
	}

	quality50 := 50
	req50 := CaptureRequest{Quality: &quality50}
	if err := req50.Validate(); err != nil {
		t.Fatalf("unexpected error for quality 50: %v", err)
	}
}

func TestCaptureRequest_Validate_ZeroMaxWidth(t *testing.T) {
	w := 0
	req := CaptureRequest{MaxWidth: &w}
	if err := req.Validate(); err == nil {
		t.Fatal("expected error for maxWidth 0")
	}
}

func TestCaptureRequest_ResolveFormat(t *testing.T) {
	req := CaptureRequest{}
	if got := req.ResolveFormat(); got != FormatPNG {
		t.Errorf("expected png default, got %s", got)
	}

	jpeg := FormatJPEG
	req2 := CaptureRequest{Format: &jpeg}
	if got := req2.ResolveFormat(); got != FormatJPEG {
		t.Errorf("expected jpeg, got %s", got)
	}
}

func TestParseCaptureRequest_Empty(t *testing.T) {
	req, err := ParseCaptureRequest(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Format != nil {
		t.Error("expected nil format for empty request")
	}
}

func TestParseCaptureRequest_ValidJSON(t *testing.T) {
	raw := json.RawMessage(`{"format":"jpeg","quality":80,"maxWidth":1440}`)
	req, err := ParseCaptureRequest(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Format == nil || *req.Format != FormatJPEG {
		t.Error("expected jpeg format")
	}
	if req.Quality == nil || *req.Quality != 80 {
		t.Error("expected quality 80")
	}
	if req.MaxWidth == nil || *req.MaxWidth != 1440 {
		t.Error("expected maxWidth 1440")
	}
}

func TestParseCaptureRequest_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`{invalid`)
	_, err := ParseCaptureRequest(raw)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestScreenshotFormat_MIME(t *testing.T) {
	tests := []struct {
		format ScreenshotFormat
		mime   string
	}{
		{FormatPNG, "image/png"},
		{FormatJPEG, "image/jpeg"},
		{FormatWebP, "image/webp"},
		{ScreenshotFormat("unknown"), "application/octet-stream"},
	}

	for _, tt := range tests {
		if got := tt.format.MIME(); got != tt.mime {
			t.Errorf("format %s: expected mime %s, got %s", tt.format, tt.mime, got)
		}
	}
}

func TestScreenshotFormat_IsValid(t *testing.T) {
	valid := []ScreenshotFormat{FormatPNG, FormatJPEG, FormatWebP}
	for _, f := range valid {
		if !f.IsValid() {
			t.Errorf("expected format %s to be valid", f)
		}
	}

	invalid := []ScreenshotFormat{ScreenshotFormat("bmp"), ScreenshotFormat("tiff"), ScreenshotFormat("")}
	for _, f := range invalid {
		if f.IsValid() {
			t.Errorf("expected format %s to be invalid", f)
		}
	}
}

func TestMapToKernelCode(t *testing.T) {
	tests := []struct {
		domainCode string
		expected   string
	}{
		{ErrUnsupported, "not_available"},
		{ErrAccessibilityDisabled, "permission_denied"},
		{ErrInvalidDisplay, "invalid_input"},
		{ErrIntervalTooShort, "rate_limited"},
		{ErrSecureContent, "restricted_content"},
		{ErrTooLarge, "resource_limit_exceeded"},
		{ErrCancelled, "cancelled"},
		{"UNKNOWN_CODE", "execution_failed"},
	}

	for _, tt := range tests {
		if got := MapToKernelCode(tt.domainCode); got != tt.expected {
			t.Errorf("domain %s: expected %s, got %s", tt.domainCode, tt.expected, got)
		}
	}
}

func TestError_Error(t *testing.T) {
	e := &Error{Code: "TEST_CODE", Message: "test message"}
	if got := e.Error(); got != "TEST_CODE: test message" {
		t.Errorf("expected 'TEST_CODE: test message', got '%s'", got)
	}
}

func TestNativeHostUnavailableError(t *testing.T) {
	err := NativeHostUnavailableError()
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	serr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if serr.Code != ErrBlockedNativeHost {
		t.Errorf("expected code %s, got %s", ErrBlockedNativeHost, serr.Code)
	}
}

func TestFormatResourceInvalid(t *testing.T) {
	err := FormatResourceInvalid("file not found")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	serr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if serr.Code != ErrResourceInvalid {
		t.Errorf("expected code %s, got %s", ErrResourceInvalid, serr.Code)
	}
}
