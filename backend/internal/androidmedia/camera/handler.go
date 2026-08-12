package camera

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type Handler struct {
	bridge androidnative.NativeBridge
	policy Policy
	mu     sync.Mutex
}

func NewHandler(bridge androidnative.NativeBridge, policy Policy) *Handler {
	return &Handler{
		bridge: bridge,
		policy: policy,
	}
}

func (h *Handler) CapabilityID() capability.CapabilityID {
	return capability.CapabilityID(
		capability.BuildCapabilityID(
			capability.CapabilitySourceBuiltin,
			"android_media",
			"camera",
		),
	)
}

func (h *Handler) Status(ctx context.Context) (CapabilityState, error) {
	if h.bridge == nil {
		return CapabilityState{
			Supported:          false,
			PermissionState:    PermissionUnavailable,
			UserActionRequired: false,
			CaptureAvailable:   false,
			Reason:             CameraNativeBridgeUnavailable,
		}, &CameraError{Code: CameraNativeBridgeUnavailable, Message: "android native bridge unavailable"}
	}

	payload := map[string]any{
		"kind": "capability_state",
	}

	raw, err := h.bridge.Execute(ctx, androidnative.NativeBridgeRequest{
		Operation: OperationCameraStatus,
		Payload:   payload,
	})
	if err != nil {
		return CapabilityState{
			Supported:          false,
			PermissionState:    PermissionUnavailable,
			UserActionRequired: false,
			CaptureAvailable:   false,
			Reason:             err.Error(),
		}, &CameraError{Code: CameraNativeBridgeUnavailable, Message: err.Error()}
	}

	result := CapabilityState{
		Supported:          false,
		PermissionState:    PermissionUnknown,
		UserActionRequired: false,
		CaptureAvailable:   false,
	}

	if raw.Result != nil {
		if supported, ok := raw.Result["supported"].(bool); ok {
			result.Supported = supported
		}
		if permState, ok := raw.Result["permissionState"].(string); ok {
			result.PermissionState = PermissionState(permState)
		}
		if userAction, ok := raw.Result["userActionRequired"].(bool); ok {
			result.UserActionRequired = userAction
		}
		if count, ok := raw.Result["cameraCount"].(float64); ok {
			result.CameraCount = int(count)
		}
		if defaultLens, ok := raw.Result["defaultLens"].(string); ok {
			result.DefaultLens = defaultLens
		}
		if captureAvail, ok := raw.Result["captureAvailable"].(bool); ok {
			result.CaptureAvailable = captureAvail
		}
		if reason, ok := raw.Result["reason"].(string); ok {
			result.Reason = reason
		}
	}

	return result, nil
}

func (h *Handler) List(ctx context.Context) ([]CameraDevice, error) {
	if h.bridge == nil {
		return nil, &CameraError{Code: CameraNativeBridgeUnavailable, Message: "android native bridge unavailable"}
	}

	payload := map[string]any{
		"kind": "list_cameras",
	}

	raw, err := h.bridge.Execute(ctx, androidnative.NativeBridgeRequest{
		Operation: OperationCameraList,
		Payload:   payload,
	})
	if err != nil {
		return nil, &CameraError{Code: CameraNativeBridgeUnavailable, Message: err.Error()}
	}

	if raw.Result == nil {
		return nil, &CameraError{Code: CameraInvalidResponse, Message: "camera list returned nil result"}
	}

	camerasRaw, ok := raw.Result["cameras"].([]any)
	if !ok {
		return []CameraDevice{}, nil
	}

	devices := make([]CameraDevice, 0, len(camerasRaw))
	for _, item := range camerasRaw {
		camMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		device := CameraDevice{}
		if id, ok := camMap["cameraId"].(string); ok {
			device.CameraID = id
		}
		if lens, ok := camMap["lensFacing"].(string); ok {
			device.LensFacing = normalizeLensFacing(lens)
		}
		if orient, ok := camMap["sensorOrientation"].(float64); ok {
			device.SensorOrientation = int(orient)
		}
		if flash, ok := camMap["flashAvailable"].(bool); ok {
			device.FlashAvailable = flash
		}
		if af, ok := camMap["supportsAutoFocus"].(bool); ok {
			device.SupportsAutoFocus = af
		}
		if zoom, ok := camMap["supportsZoom"].(bool); ok {
			device.SupportsZoom = zoom
		}
		if mw, ok := camMap["maxWidth"].(float64); ok {
			device.MaxWidth = int(mw)
		}
		if mh, ok := camMap["maxHeight"].(float64); ok {
			device.MaxHeight = int(mh)
		}
		devices = append(devices, device)
	}

	return devices, nil
}

func (h *Handler) Capture(ctx context.Context, request CaptureRequest) (CaptureResult, error) {
	if h.bridge == nil {
		return CaptureResult{}, &CameraError{Code: CameraNativeBridgeUnavailable, Message: "android native bridge unavailable"}
	}

	if err := request.Validate(); err != nil {
		return CaptureResult{}, err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	payload := buildCapturePayload(request, h.policy)

	raw, err := h.bridge.Execute(ctx, androidnative.NativeBridgeRequest{
		Operation: OperationCameraCapture,
		Payload:   payload,
	})
	if err != nil {
		return CaptureResult{}, &CameraError{Code: CameraCaptureFailed, Message: err.Error()}
	}

	if raw.Error != nil {
		return CaptureResult{}, mapBridgeError(raw.Error)
	}

	if raw.Result == nil {
		return CaptureResult{}, &CameraError{Code: CameraInvalidResponse, Message: "camera capture returned nil result"}
	}

	result := CaptureResult{
		EXIFStripped: h.policy.StripSensitiveEXIF,
	}

	if uri, ok := raw.Result["resourceUri"].(string); ok {
		result.ResourceURI = uri
	}
	if mime, ok := raw.Result["mimeType"].(string); ok {
		result.MIMEType = mime
	}
	if w, ok := raw.Result["width"].(float64); ok {
		result.Width = int(w)
	}
	if h2, ok := raw.Result["height"].(float64); ok {
		result.Height = int(h2)
	}
	if cid, ok := raw.Result["cameraId"].(string); ok {
		result.CameraID = cid
	}
	if lens, ok := raw.Result["lensFacing"].(string); ok {
		result.LensFacing = normalizeLensFacing(lens)
	}
	if rot, ok := raw.Result["rotation"].(float64); ok {
		result.Rotation = int(rot)
	}
	if ts, ok := raw.Result["timestampMs"].(float64); ok {
		result.TimestampMs = int64(ts)
	}
	if size, ok := raw.Result["sizeBytes"].(float64); ok {
		result.SizeBytes = int64(size)
	}
	if hash, ok := raw.Result["contentHash"].(string); ok {
		result.ContentHash = hash
	}
	if exif, ok := raw.Result["exifStripped"].(bool); ok {
		result.EXIFStripped = exif
	}

	if !result.Valid() {
		return CaptureResult{}, &CameraError{Code: CameraInvalidResponse, Message: "camera capture returned invalid result"}
	}

	if result.SizeBytes > h.policy.MaxEncodedBytes {
		return CaptureResult{}, &CameraError{Code: CameraTooLarge, Message: "captured image exceeds maximum encoded bytes"}
	}

	if int64(result.Width)*int64(result.Height) > h.policy.MaxCapturePixels {
		return CaptureResult{}, &CameraError{Code: CameraTooLarge, Message: "captured image exceeds maximum pixel count"}
	}

	return result, nil
}

func buildCapturePayload(request CaptureRequest, policy Policy) map[string]any {
	payload := map[string]any{}

	format := policy.ResolveFormat(request.Format)
	payload["format"] = format
	payload["quality"] = policy.ResolveQuality(request.Quality)
	payload["maxWidth"] = policy.ResolveMaxWidth(request.MaxWidth)
	payload["maxHeight"] = policy.ResolveMaxHeight(request.MaxHeight)
	payload["flashMode"] = policy.ResolveFlashMode(request.FlashMode)

	if request.CameraID != nil {
		payload["cameraId"] = *request.CameraID
	}
	if request.Lens != nil {
		payload["lens"] = *request.Lens
	}
	if request.FocusMode != nil {
		payload["focusMode"] = *request.FocusMode
	}
	if request.Rotation != nil {
		payload["rotation"] = *request.Rotation
	}

	return payload
}

func normalizeLensFacing(lens string) string {
	switch lens {
	case LensFront, LensBack, LensExternal:
		return lens
	}
	return LensUnknown
}

func mapBridgeError(bridgeErr *androidnative.NativeBridgeError) *CameraError {
	if bridgeErr == nil {
		return nil
	}

	switch bridgeErr.Code {
	case "PERMISSION_DENIED":
		return &CameraError{Code: CameraPermissionDenied, Message: bridgeErr.Message}
	case "PERMISSION_REQUIRED":
		return &CameraError{Code: CameraPermissionRequired, Message: bridgeErr.Message}
	case "CAMERA_NOT_FOUND":
		return &CameraError{Code: CameraNotFound, Message: bridgeErr.Message}
	case "CAMERA_IN_USE":
		return &CameraError{Code: CameraInUse, Message: bridgeErr.Message}
	case "CAMERA_TIMEOUT":
		return &CameraError{Code: CameraTimeout, Message: bridgeErr.Message}
	case "CAMERA_CANCELLED":
		return &CameraError{Code: CameraCancelled, Message: bridgeErr.Message}
	case "FLASH_UNSUPPORTED":
		return &CameraError{Code: CameraFlashUnsupported, Message: bridgeErr.Message}
	case "ENCODE_FAILED":
		return &CameraError{Code: CameraEncodeFailed, Message: bridgeErr.Message}
	case "UNSUPPORTED":
		return &CameraError{Code: CameraUnsupported, Message: bridgeErr.Message}
	default:
		return &CameraError{Code: CameraCaptureFailed, Message: bridgeErr.Message}
	}
}

type StatusHandler struct {
	capabilityID capability.CapabilityID
	handler      *Handler
}

func NewStatusHandler(handler *Handler) *StatusHandler {
	id := capability.CapabilityID(
		capability.BuildCapabilityID(
			capability.CapabilitySourceBuiltin,
			"android_media",
			"camera_status",
		),
	)
	return &StatusHandler{capabilityID: id, handler: handler}
}

func (h *StatusHandler) CapabilityID() capability.CapabilityID {
	return h.capabilityID
}

func (h *StatusHandler) BuildPayload() map[string]any {
	return map[string]any{
		"kind": "capability_state",
	}
}

func (h *StatusHandler) NormalizeBridgeResult(invocationID string, raw map[string]any) (capability.UnifiedToolResult, error) {
	result := capability.UnifiedToolResult{
		InvocationID: invocationID,
		Status:       capability.ToolResultStatusSuccess,
	}

	structured, err := json.Marshal(raw)
	if err != nil {
		return capability.UnifiedToolResult{
			InvocationID: invocationID,
			Status:       capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:    capability.ErrorCodeInternalError,
				Message: "failed to marshal camera status: " + err.Error(),
			},
		}, nil
	}

	result.Structured = structured
	result.Content = []capability.ToolContent{
		{Type: capability.ToolContentText, Text: "camera capability state"},
	}

	return result, nil
}

type ListHandler struct {
	capabilityID capability.CapabilityID
	handler      *Handler
}

func NewListHandler(handler *Handler) *ListHandler {
	id := capability.CapabilityID(
		capability.BuildCapabilityID(
			capability.CapabilitySourceBuiltin,
			"android_media",
			"camera_list",
		),
	)
	return &ListHandler{capabilityID: id, handler: handler}
}

func (h *ListHandler) CapabilityID() capability.CapabilityID {
	return h.capabilityID
}

func (h *ListHandler) BuildPayload() map[string]any {
	return map[string]any{
		"kind": "list_cameras",
	}
}

func (h *ListHandler) NormalizeBridgeResult(invocationID string, raw map[string]any) (capability.UnifiedToolResult, error) {
	result := capability.UnifiedToolResult{
		InvocationID: invocationID,
		Status:       capability.ToolResultStatusSuccess,
	}

	structured, err := json.Marshal(raw)
	if err != nil {
		return capability.UnifiedToolResult{
			InvocationID: invocationID,
			Status:       capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:    capability.ErrorCodeInternalError,
				Message: "failed to marshal camera list: " + err.Error(),
			},
		}, nil
	}

	result.Structured = structured
	result.Content = []capability.ToolContent{
		{Type: capability.ToolContentText, Text: "camera device list"},
	}

	return result, nil
}

type CaptureHandler struct {
	capabilityID capability.CapabilityID
	handler      *Handler
}

func NewCaptureHandler(handler *Handler) *CaptureHandler {
	id := capability.CapabilityID(
		capability.BuildCapabilityID(
			capability.CapabilitySourceBuiltin,
			"android_media",
			"camera_capture",
		),
	)
	return &CaptureHandler{capabilityID: id, handler: handler}
}

func (h *CaptureHandler) CapabilityID() capability.CapabilityID {
	return h.capabilityID
}

func (h *CaptureHandler) BuildPayload(request CaptureRequest) (map[string]any, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return buildCapturePayload(request, h.handler.policy), nil
}

func (h *CaptureHandler) NormalizeBridgeResult(invocationID string, raw map[string]any) (capability.UnifiedToolResult, error) {
	result := capability.UnifiedToolResult{
		InvocationID: invocationID,
		Status:       capability.ToolResultStatusFailed,
	}

	if raw == nil {
		result.Error = &capability.ToolError{
			Code:    capability.ErrorCodeExecutionFailed,
			Message: "android camera bridge returned nil result",
		}
		return result, nil
	}

	resourceURI, _ := raw["resourceUri"].(string)
	mimeType, _ := raw["mimeType"].(string)

	if resourceURI == "" {
		result.Error = &capability.ToolError{
			Code:    capability.ErrorCodeInvalidResult,
			Message: "android camera result missing resourceUri",
		}
		return result, nil
	}

	if mimeType == "" {
		result.Error = &capability.ToolError{
			Code:    capability.ErrorCodeInvalidResult,
			Message: "android camera result missing mimeType",
		}
		return result, nil
	}

	structured, err := json.Marshal(raw)
	if err != nil {
		result.Error = &capability.ToolError{
			Code:    capability.ErrorCodeInternalError,
			Message: "failed to marshal camera result: " + err.Error(),
		}
		return result, nil
	}

	result.Status = capability.ToolResultStatusSuccess
	result.Structured = structured
	result.Content = []capability.ToolContent{
		{
			Type: capability.ToolContentResourceReference,
			URI:  resourceURI,
		},
	}

	return result, nil
}

func DefaultArtifactURI(requestID, format string) string {
	ext := FormatToExt(format)
	return ArtifactURI(requestID, ext)
}

func CaptureTimeout(policy Policy) time.Duration {
	if policy.MaxCaptureTime > 0 {
		return policy.MaxCaptureTime
	}
	return 30 * time.Second
}
