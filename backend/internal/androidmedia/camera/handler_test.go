package camera

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type fakeBridge struct {
	executeFunc func(req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error)
	healthFunc  func() androidnative.NativeBridgeHealth
}

func (f *fakeBridge) Execute(ctx context.Context, req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
	if f.executeFunc != nil {
		return f.executeFunc(req)
	}
	return androidnative.NativeBridgeResponse{}, nil
}

func (f *fakeBridge) Health(ctx context.Context) androidnative.NativeBridgeHealth {
	if f.healthFunc != nil {
		return f.healthFunc()
	}
	return androidnative.NativeBridgeHealthReady
}

func TestHandler_Status_Supported(t *testing.T) {
	bridge := &fakeBridge{
		executeFunc: func(req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			if req.Operation != OperationCameraStatus {
				t.Errorf("expected operation %s, got %s", OperationCameraStatus, req.Operation)
			}
			return androidnative.NativeBridgeResponse{
				Result: map[string]any{
					"supported":          true,
					"permissionState":    "granted",
					"cameraCount":        float64(2),
					"defaultLens":        "back",
					"captureAvailable":   true,
				},
			}, nil
		},
	}

	handler := NewHandler(bridge, DefaultPolicy())
	state, err := handler.Status(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !state.Supported {
		t.Fatal("expected supported=true")
	}
	if state.PermissionState != PermissionGranted {
		t.Fatalf("expected granted, got %s", state.PermissionState)
	}
	if state.CameraCount != 2 {
		t.Fatalf("expected 2, got %d", state.CameraCount)
	}
	if state.DefaultLens != "back" {
		t.Fatalf("expected back, got %s", state.DefaultLens)
	}
	if !state.CaptureAvailable {
		t.Fatal("expected captureAvailable=true")
	}
}

func TestHandler_Status_Unsupported(t *testing.T) {
	bridge := &fakeBridge{
		executeFunc: func(req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				Result: map[string]any{
					"supported": false,
					"reason":    "no camera hardware",
				},
			}, nil
		},
	}

	handler := NewHandler(bridge, DefaultPolicy())
	state, err := handler.Status(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state.Supported {
		t.Fatal("expected supported=false")
	}
	if state.Reason != "no camera hardware" {
		t.Fatalf("expected reason, got %s", state.Reason)
	}
}

func TestHandler_Status_BridgeUnavailable(t *testing.T) {
	handler := NewHandler(nil, DefaultPolicy())
	_, err := handler.Status(context.Background())
	if err == nil {
		t.Fatal("expected error when bridge is nil")
	}
	if camErr, ok := err.(*CameraError); !ok || camErr.Code != CameraNativeBridgeUnavailable {
		t.Fatalf("expected CAMERA_NATIVE_BRIDGE_UNAVAILABLE, got %v", err)
	}
}

func TestHandler_List_MultipleCameras(t *testing.T) {
	bridge := &fakeBridge{
		executeFunc: func(req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			if req.Operation != OperationCameraList {
				t.Errorf("expected operation %s, got %s", OperationCameraList, req.Operation)
			}
			return androidnative.NativeBridgeResponse{
				Result: map[string]any{
					"cameras": []any{
						map[string]any{
							"cameraId":          "0",
							"lensFacing":        "back",
							"sensorOrientation": float64(90),
							"flashAvailable":    true,
							"supportsAutoFocus": true,
							"supportsZoom":      true,
							"maxWidth":          float64(4096),
							"maxHeight":         float64(3072),
						},
						map[string]any{
							"cameraId":          "1",
							"lensFacing":        "front",
							"sensorOrientation": float64(270),
							"flashAvailable":    false,
						},
					},
				},
			}, nil
		},
	}

	handler := NewHandler(bridge, DefaultPolicy())
	devices, err := handler.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}

	if devices[0].CameraID != "0" {
		t.Fatalf("expected first cameraId=0, got %s", devices[0].CameraID)
	}
	if devices[0].LensFacing != LensBack {
		t.Fatalf("expected back, got %s", devices[0].LensFacing)
	}
	if !devices[0].FlashAvailable {
		t.Fatal("expected flash available")
	}

	if devices[1].CameraID != "1" {
		t.Fatalf("expected second cameraId=1, got %s", devices[1].CameraID)
	}
	if devices[1].LensFacing != LensFront {
		t.Fatalf("expected front, got %s", devices[1].LensFacing)
	}
}

func TestHandler_List_Empty(t *testing.T) {
	bridge := &fakeBridge{
		executeFunc: func(req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				Result: map[string]any{
					"cameras": []any{},
				},
			}, nil
		},
	}

	handler := NewHandler(bridge, DefaultPolicy())
	devices, err := handler.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(devices) != 0 {
		t.Fatalf("expected 0 devices, got %d", len(devices))
	}
}

func TestHandler_Capture_Success(t *testing.T) {
	bridge := &fakeBridge{
		executeFunc: func(req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			if req.Operation != OperationCameraCapture {
				t.Errorf("expected operation %s, got %s", OperationCameraCapture, req.Operation)
			}
			payload := req.Payload
			if payload["lens"] != "back" {
				t.Errorf("expected lens=back, got %v", payload["lens"])
			}
			return androidnative.NativeBridgeResponse{
				Result: map[string]any{
					"resourceUri":  "amitia://temp/android-media/camera/test.jpg",
					"mimeType":     "image/jpeg",
					"width":        float64(1920),
					"height":       float64(1080),
					"cameraId":     "0",
					"lensFacing":   "back",
					"rotation":     float64(90),
					"timestampMs":  float64(1234567890),
					"sizeBytes":    float64(500000),
					"contentHash":  "sha256:abc123",
				},
			}, nil
		},
	}

	handler := NewHandler(bridge, DefaultPolicy())
	lens := LensBack
	result, err := handler.Capture(context.Background(), CaptureRequest{Lens: &lens})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ResourceURI != "amitia://temp/android-media/camera/test.jpg" {
		t.Fatalf("unexpected resourceUri: %s", result.ResourceURI)
	}
	if result.MIMEType != "image/jpeg" {
		t.Fatalf("unexpected mimeType: %s", result.MIMEType)
	}
	if result.Width != 1920 {
		t.Fatalf("expected width 1920, got %d", result.Width)
	}
	if result.Height != 1080 {
		t.Fatalf("expected height 1080, got %d", result.Height)
	}
	if result.CameraID != "0" {
		t.Fatalf("expected cameraId 0, got %s", result.CameraID)
	}
	if result.LensFacing != LensBack {
		t.Fatalf("expected lensFacing back, got %s", result.LensFacing)
	}
	if !result.EXIFStripped {
		t.Fatal("expected EXIF stripped=true")
	}
}

func TestHandler_Capture_InvalidRequest(t *testing.T) {
	bridge := &fakeBridge{}
	handler := NewHandler(bridge, DefaultPolicy())

	camID := "0"
	lens := LensFront
	_, err := handler.Capture(context.Background(), CaptureRequest{
		CameraID: &camID,
		Lens:     &lens,
	})
	if err == nil {
		t.Fatal("expected error for conflicting cameraId and lens")
	}
}

func TestHandler_Capture_BridgePermissionDenied(t *testing.T) {
	bridge := &fakeBridge{
		executeFunc: func(req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				Error: &androidnative.NativeBridgeError{
					Code:    "PERMISSION_DENIED",
					Message: "user denied CAMERA permission",
				},
			}, nil
		},
	}

	handler := NewHandler(bridge, DefaultPolicy())
	_, err := handler.Capture(context.Background(), CaptureRequest{})
	if err == nil {
		t.Fatal("expected error for permission denied")
	}
	if camErr, ok := err.(*CameraError); !ok || camErr.Code != CameraPermissionDenied {
		t.Fatalf("expected CAMERA_PERMISSION_DENIED, got %v", err)
	}
}

func TestHandler_Capture_InvalidResult(t *testing.T) {
	bridge := &fakeBridge{
		executeFunc: func(req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				Result: map[string]any{
					"resourceUri": "",
					"mimeType":    "",
				},
			}, nil
		},
	}

	handler := NewHandler(bridge, DefaultPolicy())
	_, err := handler.Capture(context.Background(), CaptureRequest{})
	if err == nil {
		t.Fatal("expected error for invalid result")
	}
	if camErr, ok := err.(*CameraError); !ok || camErr.Code != CameraInvalidResponse {
		t.Fatalf("expected CAMERA_INVALID_RESPONSE, got %v", err)
	}
}

func TestHandler_Capture_TooLargeBytes(t *testing.T) {
	bridge := &fakeBridge{
		executeFunc: func(req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{
				Result: map[string]any{
					"resourceUri": "amitia://temp/android-media/camera/test.jpg",
					"mimeType":    "image/jpeg",
					"width":       float64(100),
					"height":      float64(100),
					"sizeBytes":   float64(DefaultMaxEncodedBytes + 1),
				},
			}, nil
		},
	}

	handler := NewHandler(bridge, DefaultPolicy())
	_, err := handler.Capture(context.Background(), CaptureRequest{})
	if err == nil {
		t.Fatal("expected error for too large result")
	}
	if camErr, ok := err.(*CameraError); !ok || camErr.Code != CameraTooLarge {
		t.Fatalf("expected CAMERA_TOO_LARGE, got %v", err)
	}
}

func TestHandler_Capture_BridgeError(t *testing.T) {
	bridge := &fakeBridge{
		executeFunc: func(req androidnative.NativeBridgeRequest) (androidnative.NativeBridgeResponse, error) {
			return androidnative.NativeBridgeResponse{}, errors.New("connection refused")
		},
	}

	handler := NewHandler(bridge, DefaultPolicy())
	_, err := handler.Capture(context.Background(), CaptureRequest{})
	if err == nil {
		t.Fatal("expected error from bridge failure")
	}
}

func TestStatusHandler_NormalizeBridgeResult(t *testing.T) {
	handler := NewStatusHandler(NewHandler(nil, DefaultPolicy()))

	raw := map[string]any{
		"supported": true,
		"cameraCount": float64(2),
	}

	result, err := handler.NormalizeBridgeResult("inv-1", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("expected success, got %s", result.Status)
	}
	if result.InvocationID != "inv-1" {
		t.Fatalf("expected inv-1, got %s", result.InvocationID)
	}
	if result.Structured == nil {
		t.Fatal("expected structured output")
	}

	var decoded map[string]any
	if err := json.Unmarshal(result.Structured, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
}

func TestListHandler_NormalizeBridgeResult(t *testing.T) {
	handler := NewListHandler(NewHandler(nil, DefaultPolicy()))

	raw := map[string]any{
		"cameras": []any{
			map[string]any{"cameraId": "0"},
		},
		"count": float64(1),
	}

	result, err := handler.NormalizeBridgeResult("inv-2", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("expected success, got %s", result.Status)
	}
}

func TestCaptureHandler_BuildPayload(t *testing.T) {
	handler := NewCaptureHandler(NewHandler(nil, DefaultPolicy()))

	lens := LensFront
	quality := 85
	width := 1920
	req := CaptureRequest{Lens: &lens, Quality: &quality, MaxWidth: &width}

	payload, err := handler.BuildPayload(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if payload["lens"] != "front" {
		t.Fatalf("expected front, got %v", payload["lens"])
	}
	if payload["quality"] != 85 {
		t.Fatalf("expected 85, got %v", payload["quality"])
	}
	if payload["maxWidth"] != 1920 {
		t.Fatalf("expected 1920, got %v", payload["maxWidth"])
	}
}

func TestCaptureHandler_NormalizeBridgeResult(t *testing.T) {
	handler := NewCaptureHandler(NewHandler(nil, DefaultPolicy()))

	raw := map[string]any{
		"resourceUri": "amitia://temp/android-media/camera/photo.jpg",
		"mimeType":    "image/jpeg",
		"width":       float64(1920),
		"height":      float64(1080),
		"sizeBytes":   float64(500000),
	}

	result, err := handler.NormalizeBridgeResult("inv-3", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("expected success, got %s", result.Status)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
	if result.Content[0].Type != capability.ToolContentResourceReference {
		t.Fatalf("expected resource reference, got %s", result.Content[0].Type)
	}
	if result.Content[0].URI != "amitia://temp/android-media/camera/photo.jpg" {
		t.Fatalf("expected resource URI, got %s", result.Content[0].URI)
	}
}

func TestCaptureHandler_NormalizeBridgeResult_NilRaw(t *testing.T) {
	handler := NewCaptureHandler(NewHandler(nil, DefaultPolicy()))

	result, err := handler.NormalizeBridgeResult("inv-4", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != capability.ToolResultStatusFailed {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	if result.Error == nil {
		t.Fatal("expected error for nil raw")
	}
}

func TestMapBridgeError_PermissionDenied(t *testing.T) {
	err := &androidnative.NativeBridgeError{Code: "PERMISSION_DENIED", Message: "denied"}
	camErr := mapBridgeError(err)
	if camErr.Code != CameraPermissionDenied {
		t.Fatalf("expected CAMERA_PERMISSION_DENIED, got %s", camErr.Code)
	}
}

func TestMapBridgeError_CameraNotFound(t *testing.T) {
	err := &androidnative.NativeBridgeError{Code: "CAMERA_NOT_FOUND", Message: "camera 99 not found"}
	camErr := mapBridgeError(err)
	if camErr.Code != CameraNotFound {
		t.Fatalf("expected CAMERA_NOT_FOUND, got %s", camErr.Code)
	}
}

func TestMapBridgeError_CameraTimeout(t *testing.T) {
	err := &androidnative.NativeBridgeError{Code: "CAMERA_TIMEOUT", Message: "capture timed out"}
	camErr := mapBridgeError(err)
	if camErr.Code != CameraTimeout {
		t.Fatalf("expected CAMERA_TIMEOUT, got %s", camErr.Code)
	}
}

func TestMapBridgeError_Unknown(t *testing.T) {
	err := &androidnative.NativeBridgeError{Code: "SOMETHING_ELSE", Message: "unknown error"}
	camErr := mapBridgeError(err)
	if camErr.Code != CameraCaptureFailed {
		t.Fatalf("expected CAMERA_CAPTURE_FAILED, got %s", camErr.Code)
	}
}

func TestMapBridgeError_Nil(t *testing.T) {
	if mapBridgeError(nil) != nil {
		t.Fatal("expected nil for nil input")
	}
}

func TestNormalizeLensFacing(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{LensFront, LensFront},
		{LensBack, LensBack},
		{LensExternal, LensExternal},
		{"LEFT", LensUnknown},
		{"", LensUnknown},
	}

	for _, tt := range tests {
		got := normalizeLensFacing(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeLensFacing(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
