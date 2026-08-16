package media

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/nativebridge"
)

type mockMediaBridge struct {
	response nativebridge.Response
	err      error
	calls    []nativebridge.Request
	delay    time.Duration
}

func (m *mockMediaBridge) Execute(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error) {
	m.calls = append(m.calls, req)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nativebridge.Response{}, ctx.Err()
		}
	}
	return m.response, m.err
}

func (m *mockMediaBridge) Health(context.Context) nativebridge.Health {
	return ""
}

func newMockMediaBridge(resp nativebridge.Response, err error) *mockMediaBridge {
	return &mockMediaBridge{response: resp, err: err}
}

func baseMediaRequest(operation string) nativebridge.Request {
	return nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Platform:        "ios",
		Operation:       operation,
		Payload:         map[string]any{},
	}
}

func TestNewMediaHandler(t *testing.T) {
	h := NewMediaHandler(newMockMediaBridge(nativebridge.Response{}, nil))
	if h == nil {
		t.Fatal("NewMediaHandler returned nil")
	}
}

func TestHandler_Execute_UnknownOperation(t *testing.T) {
	h := NewMediaHandler(newMockMediaBridge(nativebridge.Response{}, nil))
	req := baseMediaRequest("media.unknown")
	resp := h.Execute(context.Background(), req)
	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil {
		t.Fatal("expected error object")
	}
	if resp.Error.Code != nativebridge.ErrOperationNotSupported {
		t.Errorf("expected ErrOperationNotSupported, got %s", resp.Error.Code)
	}
}

func TestHandler_Status(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{"supported": true},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if len(bridge.calls) != 1 {
		t.Errorf("expected 1 bridge call, got %d", len(bridge.calls))
	}
}

func TestHandler_PhotosPick_Defaults(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{"items": []any{}},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationPhotosPick)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	kinds, ok := sent["kinds"].([]string)
	if !ok || len(kinds) != 1 || kinds[0] != "image" {
		t.Error("expected default kinds=[image]")
	}
	if sent["selectionLimit"] != DefaultSelectionLimit {
		t.Errorf("expected default selectionLimit=%d", DefaultSelectionLimit)
	}
}

func TestHandler_PhotosPick_WithKinds(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationPhotosPick)
	req.Payload["kinds"] = []any{"image", "video"}
	req.Payload["selectionLimit"] = float64(5)
	req.Payload["ordered"] = true
	req.Payload["maxTotalBytes"] = float64(10485760)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	kinds, ok := sent["kinds"].([]string)
	if !ok || len(kinds) != 2 {
		t.Error("expected 2 kinds")
	}
	if sent["selectionLimit"] != 5 {
		t.Errorf("expected selectionLimit=5, got %v", sent["selectionLimit"])
	}
	if sent["ordered"] != true {
		t.Error("expected ordered=true")
	}
	if sent["maxTotalBytes"] != int64(10485760) {
		t.Error("expected maxTotalBytes=10485760")
	}
}

func TestHandler_PhotosStatus(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{"authorized": true},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationPhotosStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_PhotosList(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{"assets": []any{}},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationPhotosList)
	req.Payload["mediaType"] = "image"
	req.Payload["limit"] = float64(50)
	req.Payload["favorite"] = true
	req.Payload["sort"] = "createdAt"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["mediaType"] != "image" {
		t.Errorf("expected mediaType=image")
	}
	if sent["limit"] != 50 {
		t.Errorf("expected limit=50")
	}
	if sent["favorite"] != true {
		t.Error("expected favorite=true")
	}
}

func TestHandler_PhotosList_Defaults(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationPhotosList)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["limit"] != DefaultPhotoListLimit {
		t.Errorf("expected default limit=%d, got %v", DefaultPhotoListLimit, sent["limit"])
	}
}

func TestHandler_PhotosGet_MissingAssetRef(t *testing.T) {
	h := NewMediaHandler(newMockMediaBridge(nativebridge.Response{}, nil))
	req := baseMediaRequest(OperationPhotosGet)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrPhotoAssetNotFound {
		t.Errorf("expected ErrPhotoAssetNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_PhotosGet_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{"assetRef": "asset-001"},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationPhotosGet)
	req.Payload["assetRef"] = "asset-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if bridge.calls[0].Payload["assetRef"] != "asset-001" {
		t.Error("expected assetRef=asset-001")
	}
}

func TestHandler_PhotosExport_MissingAssetRef(t *testing.T) {
	h := NewMediaHandler(newMockMediaBridge(nativebridge.Response{}, nil))
	req := baseMediaRequest(OperationPhotosExport)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrPhotoAssetNotFound {
		t.Errorf("expected ErrPhotoAssetNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_PhotosExport_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{"resourceUri": "amitia://temp/abc"},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationPhotosExport)
	req.Payload["assetRef"] = "asset-001"
	req.Payload["representation"] = "original"
	req.Payload["networkAccess"] = true
	req.Payload["maxBytes"] = float64(52428800)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["representation"] != "original" {
		t.Error("expected representation=original")
	}
	if sent["networkAccess"] != true {
		t.Error("expected networkAccess=true")
	}
}

func TestHandler_PhotosExport_Defaults(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationPhotosExport)
	req.Payload["assetRef"] = "asset-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["representation"] != "current" {
		t.Error("expected default representation=current")
	}
	if sent["maxBytes"] != MaxExportBytes {
		t.Errorf("expected default maxBytes=%d", MaxExportBytes)
	}
}

func TestHandler_PhotosSave_MissingResourceURI(t *testing.T) {
	h := NewMediaHandler(newMockMediaBridge(nativebridge.Response{}, nil))
	req := baseMediaRequest(OperationPhotosSave)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrResourceURIInvalid {
		t.Errorf("expected ErrResourceURIInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_PhotosSave_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{"saved": true},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationPhotosSave)
	req.Payload["resourceUri"] = "amitia://temp/photo.jpg"
	req.Payload["mediaType"] = "image"
	req.Payload["preserveMetadata"] = true
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["resourceUri"] != "amitia://temp/photo.jpg" {
		t.Error("expected resourceUri")
	}
	if sent["mediaType"] != "image" {
		t.Error("expected mediaType=image")
	}
}

func TestHandler_PhotosDelete_MissingAssetRefs(t *testing.T) {
	h := NewMediaHandler(newMockMediaBridge(nativebridge.Response{}, nil))
	req := baseMediaRequest(OperationPhotosDelete)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrPhotoAssetNotFound {
		t.Errorf("expected ErrPhotoAssetNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_PhotosDelete_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{"deleted": true},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationPhotosDelete)
	req.Payload["assetRefs"] = []any{"asset-001", "asset-002"}
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	refs, ok := sent["assetRefs"].([]string)
	if !ok || len(refs) != 2 {
		t.Error("expected 2 assetRefs")
	}
}

func TestHandler_PhotosManageLimited(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationPhotosManageLimited)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_CameraStatus(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{"available": true},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationCameraStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_CameraDevices(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{"devices": []any{}},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationCameraDevices)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_CameraCapturePhoto(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{"resourceUri": "amitia://temp/photo.jpg"},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationCameraCapturePhoto)
	req.Payload["deviceRef"] = "back"
	req.Payload["quality"] = "high"
	req.Payload["flash"] = "auto"
	req.Payload["format"] = "jpeg"
	req.Payload["mirrorFrontCamera"] = false
	req.Payload["saveToPhotos"] = true
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["deviceRef"] != "back" {
		t.Error("expected deviceRef=back")
	}
	if sent["quality"] != "high" {
		t.Error("expected quality=high")
	}
	if sent["flash"] != "auto" {
		t.Error("expected flash=auto")
	}
	if sent["format"] != "jpeg" {
		t.Error("expected format=jpeg")
	}
}

func TestHandler_CameraCapturePhoto_Defaults(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationCameraCapturePhoto)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["quality"] != "high" {
		t.Error("expected default quality=high")
	}
	if sent["flash"] != "auto" {
		t.Error("expected default flash=auto")
	}
	if sent["format"] != "jpeg" {
		t.Error("expected default format=jpeg")
	}
}

func TestHandler_CameraRecordVideo_MissingDuration(t *testing.T) {
	h := NewMediaHandler(newMockMediaBridge(nativebridge.Response{}, nil))
	req := baseMediaRequest(OperationCameraRecordVideo)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrInvalidRequest {
		t.Errorf("expected ErrInvalidRequest, got %s", resp.Error.Code)
	}
}

func TestHandler_CameraRecordVideo_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{"resourceUri": "amitia://temp/video.mp4"},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationCameraRecordVideo)
	req.Payload["maxDurationMs"] = float64(60000)
	req.Payload["includeAudio"] = true
	req.Payload["quality"] = "high"
	req.Payload["torch"] = "off"
	req.Payload["saveToPhotos"] = true
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["maxDurationMs"] != int64(60000) {
		t.Error("expected maxDurationMs=60000")
	}
	if sent["includeAudio"] != true {
		t.Error("expected includeAudio=true")
	}
}

func TestHandler_AudioStatus(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{"available": true},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationAudioStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_AudioRecord_MissingDuration(t *testing.T) {
	h := NewMediaHandler(newMockMediaBridge(nativebridge.Response{}, nil))
	req := baseMediaRequest(OperationAudioRecord)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrInvalidRequest {
		t.Errorf("expected ErrInvalidRequest, got %s", resp.Error.Code)
	}
}

func TestHandler_AudioRecord_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{"resourceUri": "amitia://temp/audio.m4a"},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationAudioRecord)
	req.Payload["maxDurationMs"] = float64(30000)
	req.Payload["format"] = "m4a"
	req.Payload["sampleRate"] = float64(44100)
	req.Payload["channels"] = float64(1)
	req.Payload["quality"] = "high"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["maxDurationMs"] != int64(30000) {
		t.Error("expected maxDurationMs=30000")
	}
	if sent["format"] != "m4a" {
		t.Error("expected format=m4a")
	}
	if sent["sampleRate"] != 44100 {
		t.Error("expected sampleRate=44100")
	}
	if sent["channels"] != 1 {
		t.Error("expected channels=1")
	}
}

func TestHandler_AudioRecord_Defaults(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-media-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockMediaBridge(expected, nil)
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationAudioRecord)
	req.Payload["maxDurationMs"] = float64(30000)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["format"] != "m4a" {
		t.Error("expected default format=m4a")
	}
	if sent["sampleRate"] != DefaultAudioSampleRate {
		t.Errorf("expected default sampleRate=%d", DefaultAudioSampleRate)
	}
	if sent["channels"] != DefaultAudioChannels {
		t.Errorf("expected default channels=%d", DefaultAudioChannels)
	}
}

func TestHandler_ContextCancel(t *testing.T) {
	bridge := &mockMediaBridge{
		delay: 5 * time.Second,
	}
	h := NewMediaHandler(bridge)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := baseMediaRequest(OperationStatus)
	resp := h.Execute(ctx, req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrTimeout {
		t.Errorf("expected ErrTimeout, got %s", resp.Error.Code)
	}
}

func TestHandler_BridgeError(t *testing.T) {
	bridge := newMockMediaBridge(nativebridge.Response{}, errors.New("bridge failed"))
	h := NewMediaHandler(bridge)

	req := baseMediaRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_BridgeUnavailable(t *testing.T) {
	h := NewMediaHandler(nil)
	req := baseMediaRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrNativeBridgeUnavailable {
		t.Errorf("expected ErrNativeBridgeUnavailable, got %s", resp.Error.Code)
	}
}

func TestAuthorizationStatusFromNative(t *testing.T) {
	tests := []struct {
		native   string
		expected AuthorizationStatus
	}{
		{"authorized", AuthAuthorized},
		{"denied", AuthDenied},
		{"restricted", AuthRestricted},
		{"limited", AuthLimited},
		{"notDetermined", AuthNotDetermined},
		{"invalid", AuthNotDetermined},
	}
	for _, tt := range tests {
		t.Run(tt.native, func(t *testing.T) {
			result := AuthorizationStatusFromNative(tt.native)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsValidPickerKind(t *testing.T) {
	valid := []string{"image", "video", "live_photo"}
	for _, k := range valid {
		if !IsValidPickerKind(k) {
			t.Errorf("expected %s to be valid", k)
		}
	}
	invalid := []string{"audio", "document", ""}
	for _, k := range invalid {
		if IsValidPickerKind(k) {
			t.Errorf("expected %s to be invalid", k)
		}
	}
}

func TestIsValidRepresentation(t *testing.T) {
	valid := []string{"current", "original", "compatible"}
	for _, r := range valid {
		if !IsValidRepresentation(r) {
			t.Errorf("expected %s to be valid", r)
		}
	}
	invalid := []string{"raw", "preview"}
	for _, r := range invalid {
		if IsValidRepresentation(r) {
			t.Errorf("expected %s to be invalid", r)
		}
	}
}

func TestIsValidFlashMode(t *testing.T) {
	for _, f := range []string{"auto", "on", "off"} {
		if !IsValidFlashMode(f) {
			t.Errorf("expected %s to be valid", f)
		}
	}
	if IsValidFlashMode("invalid") {
		t.Error("expected invalid flash mode")
	}
}

func TestClampPhotoListLimit(t *testing.T) {
	if ClampPhotoListLimit(0) != DefaultPhotoListLimit {
		t.Errorf("expected default %d", DefaultPhotoListLimit)
	}
	if ClampPhotoListLimit(999) != MaxPhotoListLimit {
		t.Errorf("expected max %d", MaxPhotoListLimit)
	}
	if ClampPhotoListLimit(100) != 100 {
		t.Error("expected 100")
	}
}

func TestClampSelectionLimit(t *testing.T) {
	if ClampSelectionLimit(0) != DefaultSelectionLimit {
		t.Errorf("expected default %d", DefaultSelectionLimit)
	}
	if ClampSelectionLimit(999) != MaxSelectionLimit {
		t.Errorf("expected max %d", MaxSelectionLimit)
	}
	if ClampSelectionLimit(5) != 5 {
		t.Error("expected 5")
	}
}

func TestClampVideoDuration(t *testing.T) {
	if ClampVideoDuration(0) != DefaultVideoMaxDurationMs {
		t.Errorf("expected default %d", DefaultVideoMaxDurationMs)
	}
	if ClampVideoDuration(999999) != MaxVideoMaxDurationMs {
		t.Errorf("expected max %d", MaxVideoMaxDurationMs)
	}
	if ClampVideoDuration(60000) != 60000 {
		t.Error("expected 60000")
	}
}

func TestClampAudioDuration(t *testing.T) {
	if ClampAudioDuration(0) != DefaultAudioMaxDurationMs {
		t.Errorf("expected default %d", DefaultAudioMaxDurationMs)
	}
	if ClampAudioDuration(999999) != MaxAudioMaxDurationMs {
		t.Errorf("expected max %d", MaxAudioMaxDurationMs)
	}
	if ClampAudioDuration(30000) != 30000 {
		t.Error("expected 30000")
	}
}

func TestValidatePickerRequest(t *testing.T) {
	err := ValidatePickerRequest(MediaPickerRequest{
		Kinds:          []string{"image", "video"},
		SelectionLimit: 5,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidatePickerRequest(MediaPickerRequest{
		Kinds: []string{"invalid"},
	})
	if err == nil {
		t.Error("expected error for invalid kind")
	}
}

func TestValidateCameraCaptureRequest(t *testing.T) {
	err := ValidateCameraCaptureRequest(CameraCaptureRequest{
		Flash:   "auto",
		Quality: "high",
		Format:  "jpeg",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateCameraCaptureRequest(CameraCaptureRequest{
		Flash: "invalid",
	})
	if err == nil {
		t.Error("expected error for invalid flash mode")
	}
}

func TestValidateVideoRecordRequest(t *testing.T) {
	err := ValidateVideoRecordRequest(VideoRecordRequest{
		MaxDurationMs: 60000,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateVideoRecordRequest(VideoRecordRequest{
		MaxDurationMs: 0,
	})
	if err == nil {
		t.Error("expected error for missing duration")
	}

	err = ValidateVideoRecordRequest(VideoRecordRequest{
		MaxDurationMs: 999999,
	})
	if err == nil {
		t.Error("expected error for excessive duration")
	}
}

func TestValidateAudioRecordRequest(t *testing.T) {
	err := ValidateAudioRecordRequest(AudioRecordRequest{
		MaxDurationMs: 30000,
		Format:        "m4a",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateAudioRecordRequest(AudioRecordRequest{
		MaxDurationMs: 0,
	})
	if err == nil {
		t.Error("expected error for missing duration")
	}

	err = ValidateAudioRecordRequest(AudioRecordRequest{
		MaxDurationMs: 30000,
		Format:        "invalid",
	})
	if err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestMapCodeToMessage(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{ErrMediaUnsupported, "media is not supported on this device"},
		{ErrPhotosPickerCancelled, "photos picker was cancelled by user"},
		{ErrCameraCaptureFailed, "camera capture failed"},
		{ErrVideoRecordInterrupted, "video record was interrupted"},
		{"UNKNOWN_CODE", "UNKNOWN_CODE"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := MapCodeToMessage(tt.code)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
