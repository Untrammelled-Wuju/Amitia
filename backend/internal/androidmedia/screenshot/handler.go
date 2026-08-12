package screenshot

import (
	"encoding/json"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type CaptureHandler struct {
	capabilityID capability.CapabilityID
}

func NewCaptureHandler() *CaptureHandler {
	id := capability.CapabilityID(
		capability.BuildCapabilityID(
			capability.CapabilitySourceBuiltin,
			"android_media",
			"screenshot_capture",
		),
	)
	return &CaptureHandler{capabilityID: id}
}

func (h *CaptureHandler) CapabilityID() capability.CapabilityID {
	return h.capabilityID
}

func (h *CaptureHandler) BuildPayload(request CaptureRequest) (map[string]any, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}

	payload := map[string]any{}

	format := request.ResolveFormat()
	payload["format"] = string(format)

	if request.DisplayID != nil {
		payload["displayId"] = *request.DisplayID
	} else {
		payload["displayId"] = 0
	}

	if request.Quality != nil {
		payload["quality"] = *request.Quality
	}

	if request.MaxWidth != nil {
		payload["maxWidth"] = *request.MaxWidth
	}

	if request.MaxHeight != nil {
		payload["maxHeight"] = *request.MaxHeight
	}

	return payload, nil
}

func (h *CaptureHandler) NormalizeBridgeResult(invocationID string, raw map[string]any) (capability.UnifiedToolResult, error) {
	result := capability.UnifiedToolResult{
		InvocationID: invocationID,
		Status:       capability.ToolResultStatusFailed,
	}

	if raw == nil {
		result.Error = &capability.ToolError{
			Code:    capability.ErrorCodeExecutionFailed,
			Message: "android screenshot bridge returned nil result",
		}
		return result, nil
	}

	resourceURI, _ := raw["resourceUri"].(string)
	mimeType, _ := raw["mimeType"].(string)

	if resourceURI == "" {
		result.Error = &capability.ToolError{
			Code:    capability.ErrorCodeInvalidResult,
			Message: "android screenshot result missing resourceUri",
		}
		return result, nil
	}

	if mimeType == "" {
		result.Error = &capability.ToolError{
			Code:    capability.ErrorCodeInvalidResult,
			Message: "android screenshot result missing mimeType",
		}
		return result, nil
	}

	structured, err := json.Marshal(raw)
	if err != nil {
		result.Error = &capability.ToolError{
			Code:    capability.ErrorCodeInternalError,
			Message: "failed to marshal screenshot result: " + err.Error(),
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

type HealthHandler struct {
	capabilityID capability.CapabilityID
}

func NewHealthHandler() *HealthHandler {
	id := capability.CapabilityID(
		capability.BuildCapabilityID(
			capability.CapabilitySourceBuiltin,
			"android_media",
			"screenshot_status",
		),
	)
	return &HealthHandler{capabilityID: id}
}

func (h *HealthHandler) CapabilityID() capability.CapabilityID {
	return h.capabilityID
}

func (h *HealthHandler) BuildPayload() map[string]any {
	return map[string]any{
		"kind": "capability_state",
	}
}

func (h *HealthHandler) NormalizeBridgeResult(invocationID string, raw map[string]any) (capability.UnifiedToolResult, error) {
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
				Message: "failed to marshal screenshot status: " + err.Error(),
			},
		}, nil
	}

	result.Structured = structured
	result.Content = []capability.ToolContent{
		{Type: capability.ToolContentText, Text: "screenshot capability state"},
	}

	return result, nil
}

func DefaultArtifactURI(requestID, ext string) string {
	name := SafeResourceName(requestID, ext)
	return "amitia://temp/android-media/screenshots/" + name
}
