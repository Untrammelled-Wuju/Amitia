package screenshot

import (
	"testing"
)

func TestCaptureHandler_CapabilityID(t *testing.T) {
	h := NewCaptureHandler()
	id := h.CapabilityID()
	if string(id) == "" {
		t.Fatal("capability id must not be empty")
	}
}

func TestCaptureHandler_BuildPayload_Defaults(t *testing.T) {
	h := NewCaptureHandler()
	req := CaptureRequest{}

	payload, err := h.BuildPayload(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if payload["format"] != "png" {
		t.Errorf("expected default format png, got %v", payload["format"])
	}
	if payload["displayId"] != 0 {
		t.Errorf("expected default displayId 0, got %v", payload["displayId"])
	}
}

func TestCaptureHandler_BuildPayload_CustomValues(t *testing.T) {
	h := NewCaptureHandler()
	displayID := 1
	quality := 85
	maxWidth := 1080
	maxHeight := 2400

	req := CaptureRequest{
		DisplayID: &displayID,
		Format:    formatPtr(FormatJPEG),
		Quality:   &quality,
		MaxWidth:  &maxWidth,
		MaxHeight: &maxHeight,
	}

	payload, err := h.BuildPayload(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if payload["displayId"] != 1 {
		t.Errorf("expected displayId 1, got %v", payload["displayId"])
	}
	if payload["format"] != "jpeg" {
		t.Errorf("expected format jpeg, got %v", payload["format"])
	}
	if payload["quality"] != 85 {
		t.Errorf("expected quality 85, got %v", payload["quality"])
	}
	if payload["maxWidth"] != 1080 {
		t.Errorf("expected maxWidth 1080, got %v", payload["maxWidth"])
	}
	if payload["maxHeight"] != 2400 {
		t.Errorf("expected maxHeight 2400, got %v", payload["maxHeight"])
	}
}

func TestCaptureHandler_BuildPayload_InvalidFormat(t *testing.T) {
	h := NewCaptureHandler()
	req := CaptureRequest{
		Format: formatPtr(ScreenshotFormat("bmp")),
	}

	_, err := h.BuildPayload(req)
	if err == nil {
		t.Fatal("expected error for invalid format, got nil")
	}
}

func TestCaptureHandler_BuildPayload_InvalidQuality(t *testing.T) {
	h := NewCaptureHandler()
	quality := 101
	req := CaptureRequest{
		Quality: &quality,
	}

	_, err := h.BuildPayload(req)
	if err == nil {
		t.Fatal("expected error for quality > 100, got nil")
	}
}

func TestCaptureHandler_NormalizeBridgeResult_Success(t *testing.T) {
	h := NewCaptureHandler()

	raw := map[string]any{
		"resourceUri": "amitia://temp/android-media/screenshots/test.png",
		"mimeType":    "image/png",
		"width":       1080,
		"height":      2400,
		"displayId":   0,
		"timestampMs": int64(1770000000000),
		"sizeBytes":   int64(857231),
	}

	result, err := h.NormalizeBridgeResult("inv-001", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.InvocationID != "inv-001" {
		t.Errorf("expected invocation id inv-001, got %s", result.InvocationID)
	}
	if result.Status != "success" {
		t.Errorf("expected status success, got %s", result.Status)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected at least one content item")
	}

	found := false
	for _, c := range result.Content {
		if c.Type == "resource_reference" && c.URI == "amitia://temp/android-media/screenshots/test.png" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected resource_reference content with correct URI")
	}
}

func TestCaptureHandler_NormalizeBridgeResult_MissingResourceURI(t *testing.T) {
	h := NewCaptureHandler()

	raw := map[string]any{
		"mimeType": "image/png",
	}

	result, err := h.NormalizeBridgeResult("inv-002", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Errorf("expected status failed, got %s", result.Status)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing resourceUri")
	}
}

func TestCaptureHandler_NormalizeBridgeResult_MissingMIMEType(t *testing.T) {
	h := NewCaptureHandler()

	raw := map[string]any{
		"resourceUri": "amitia://temp/android-media/screenshots/test.png",
	}

	result, err := h.NormalizeBridgeResult("inv-003", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Errorf("expected status failed, got %s", result.Status)
	}
}

func TestCaptureHandler_NormalizeBridgeResult_NilResult(t *testing.T) {
	h := NewCaptureHandler()

	result, err := h.NormalizeBridgeResult("inv-004", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Errorf("expected status failed, got %s", result.Status)
	}
}

func TestHealthHandler_CapabilityID(t *testing.T) {
	h := NewHealthHandler()
	id := h.CapabilityID()
	if string(id) == "" {
		t.Fatal("health handler capability id must not be empty")
	}
}

func TestHealthHandler_BuildPayload(t *testing.T) {
	h := NewHealthHandler()
	payload := h.BuildPayload()
	if payload["kind"] != "capability_state" {
		t.Errorf("expected kind capability_state, got %v", payload["kind"])
	}
}

func TestDefaultArtifactURI(t *testing.T) {
	uri := DefaultArtifactURI("req-abc123", ".png")
	expected := "amitia://temp/android-media/screenshots/req-abc123.png"
	if uri != expected {
		t.Errorf("expected %s, got %s", expected, uri)
	}
}

func TestDefaultArtifactURI_Sanitizes(t *testing.T) {
	uri := DefaultArtifactURI("req/../../etc/passwd", ".png")
	if uri == "" {
		t.Fatal("uri must not be empty")
	}
}

func formatPtr(f ScreenshotFormat) *ScreenshotFormat {
	return &f
}
