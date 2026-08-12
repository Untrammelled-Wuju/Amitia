package camera

import (
	"encoding/json"
	"testing"
)

func TestCaptureRequest_Validate_DefaultBackCamera(t *testing.T) {
	req := CaptureRequest{}
	if err := req.Validate(); err != nil {
		t.Fatalf("default request should be valid: %v", err)
	}
	if req.ResolveLens() != LensBack {
		t.Fatalf("expected default lens back, got %s", req.ResolveLens())
	}
}

func TestCaptureRequest_Validate_FrontCamera(t *testing.T) {
	lens := LensFront
	req := CaptureRequest{Lens: &lens}
	if err := req.Validate(); err != nil {
		t.Fatalf("front lens should be valid: %v", err)
	}
}

func TestCaptureRequest_Validate_ExternalCamera(t *testing.T) {
	lens := LensExternal
	req := CaptureRequest{Lens: &lens}
	if err := req.Validate(); err != nil {
		t.Fatalf("external lens should be valid: %v", err)
	}
}

func TestCaptureRequest_Validate_InvalidLens(t *testing.T) {
	lens := "side"
	req := CaptureRequest{Lens: &lens}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for invalid lens")
	}
	if camErr, ok := err.(*CameraError); !ok || camErr.Code != CameraNotFound {
		t.Fatalf("expected CAMERA_NOT_FOUND, got %v", err)
	}
}

func TestCaptureRequest_Validate_LensConflict(t *testing.T) {
	camID := "0"
	lens := LensFront
	req := CaptureRequest{CameraID: &camID, Lens: &lens}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for lens+cameraId conflict")
	}
	if camErr, ok := err.(*CameraError); !ok || camErr.Code != CameraLensConflict {
		t.Fatalf("expected CAMERA_LENS_CONFLICT, got %v", err)
	}
}

func TestCaptureRequest_Validate_InvalidFormat(t *testing.T) {
	format := "bmp"
	req := CaptureRequest{Format: &format}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
	if camErr, ok := err.(*CameraError); !ok || camErr.Code != CameraInvalidFormat {
		t.Fatalf("expected CAMERA_INVALID_FORMAT, got %v", err)
	}
}

func TestCaptureRequest_Validate_QualityBelowOne(t *testing.T) {
	q := 0
	req := CaptureRequest{Quality: &q}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for quality < 1")
	}
}

func TestCaptureRequest_Validate_QualityAbove100(t *testing.T) {
	q := 101
	req := CaptureRequest{Quality: &q}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for quality > 100")
	}
}

func TestCaptureRequest_Validate_InvalidFlashMode(t *testing.T) {
	flash := "strobe"
	req := CaptureRequest{FlashMode: &flash}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for invalid flash mode")
	}
	if camErr, ok := err.(*CameraError); !ok || camErr.Code != CameraInvalidFlashMode {
		t.Fatalf("expected CAMERA_INVALID_FLASH_MODE, got %v", err)
	}
}

func TestCaptureRequest_Validate_InvalidMaxWidth(t *testing.T) {
	w := -1
	req := CaptureRequest{MaxWidth: &w}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for negative maxWidth")
	}
	if camErr, ok := err.(*CameraError); !ok || camErr.Code != CameraInvalidSize {
		t.Fatalf("expected CAMERA_INVALID_SIZE, got %v", err)
	}
}

func TestCaptureRequest_Validate_InvalidMaxHeight(t *testing.T) {
	h := 0
	req := CaptureRequest{MaxHeight: &h}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for zero maxHeight")
	}
}

func TestCaptureResult_Valid(t *testing.T) {
	result := CaptureResult{
		ResourceURI: "amitia://temp/android-media/camera/test.jpg",
		MIMEType:    "image/jpeg",
		Width:       1920,
		Height:      1080,
		SizeBytes:   500000,
	}
	if !result.Valid() {
		t.Fatal("expected valid result")
	}
}

func TestCaptureResult_InvalidMissingResourceURI(t *testing.T) {
	result := CaptureResult{
		MIMEType:  "image/jpeg",
		Width:     1920,
		Height:    1080,
		SizeBytes: 500000,
	}
	if result.Valid() {
		t.Fatal("expected invalid result with missing resourceUri")
	}
}

func TestCaptureResult_InvalidZeroSize(t *testing.T) {
	result := CaptureResult{
		ResourceURI: "amitia://temp/android-media/camera/test.jpg",
		MIMEType:    "image/jpeg",
		Width:       1920,
		Height:      1080,
		SizeBytes:   0,
	}
	if result.Valid() {
		t.Fatal("expected invalid result with zero size")
	}
}

func TestCaptureRequest_ResolveFormat(t *testing.T) {
	req := CaptureRequest{}
	if req.ResolveFormat() != "jpeg" {
		t.Fatalf("expected default jpeg, got %s", req.ResolveFormat())
	}

	format := "png"
	req2 := CaptureRequest{Format: &format}
	if req2.ResolveFormat() != "png" {
		t.Fatalf("expected png, got %s", req2.ResolveFormat())
	}
}

func TestCaptureRequest_ResolveFlashMode(t *testing.T) {
	req := CaptureRequest{}
	if req.ResolveFlashMode() != FlashOff {
		t.Fatalf("expected default off, got %s", req.ResolveFlashMode())
	}

	flash := FlashAuto
	req2 := CaptureRequest{FlashMode: &flash}
	if req2.ResolveFlashMode() != FlashAuto {
		t.Fatalf("expected auto, got %s", req2.ResolveFlashMode())
	}
}

func TestCameraError_Error(t *testing.T) {
	err := &CameraError{Code: CameraNotFound, Message: "camera not found"}
	expected := "CAMERA_NOT_FOUND: camera not found"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

func TestCapabilityStateJSON(t *testing.T) {
	state := CapabilityState{
		Supported:          true,
		PermissionState:    PermissionGranted,
		UserActionRequired: false,
		CameraCount:        2,
		DefaultLens:        LensBack,
		CaptureAvailable:   true,
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded CapabilityState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !decoded.Supported {
		t.Fatal("expected supported=true")
	}
	if decoded.PermissionState != PermissionGranted {
		t.Fatalf("expected granted, got %s", decoded.PermissionState)
	}
	if decoded.CameraCount != 2 {
		t.Fatalf("expected 2, got %d", decoded.CameraCount)
	}
}

func TestCameraDeviceJSON(t *testing.T) {
	device := CameraDevice{
		CameraID:          "0",
		LensFacing:        LensBack,
		SensorOrientation: 90,
		FlashAvailable:    true,
		SupportsAutoFocus: true,
		SupportsZoom:      true,
		MaxWidth:          4096,
		MaxHeight:         3072,
	}

	data, err := json.Marshal(device)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded CameraDevice
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.CameraID != "0" {
		t.Fatalf("expected cameraId 0, got %s", decoded.CameraID)
	}
	if decoded.LensFacing != LensBack {
		t.Fatalf("expected back, got %s", decoded.LensFacing)
	}
}
