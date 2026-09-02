package interaction

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Execute(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationClick:
		return h.handleClick(ctx, request)
	case OperationLongClick:
		return h.handleLongClick(ctx, request)
	case OperationInputText:
		return h.handleInputText(ctx, request)
	case OperationClearText:
		return h.handleClearText(ctx, request)
	case OperationScroll:
		return h.handleScroll(ctx, request)
	case OperationSwipe:
		return h.handleSwipe(ctx, request)
	case OperationVisualLocate:
		return h.handleVisualLocate(ctx, request)
	case OperationVisualClick:
		return h.handleVisualClick(ctx, request)
	default:
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    "OPERATION_NOT_SUPPORTED",
				Message: "unsupported interaction operation: " + request.Operation,
			},
		}
	}
}

func (h *Handler) handleStatus(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:       "PROVIDER_UNAVAILABLE",
				Message:    "interaction service not initialized",
				DomainCode: INTERACTION_UNAVAILABLE,
			},
		}
	}

	status := h.service.Status(ctx)

	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result: map[string]any{
			"available":                status.Available,
			"accessibilityAction":      status.AccessibilityAction,
			"accessibilityGesture":     status.AccessibilityGesture,
			"coordinateTap":            status.CoordinateTap,
			"textInput":                status.TextInput,
			"scroll":                   status.Scroll,
			"visualLocate":             status.VisualLocate,
			"ocrAvailable":             status.OCRAvailable,
			"imageUnderstandAvailable": status.ImageUnderstandAvailable,
			"rootFallback":             status.RootFallback,
			"adbFallback":              status.ADBFallback,
			"state":                    status.State,
			"healthState":              status.HealthState,
			"reason":                   status.Reason,
			"providers":                status.Providers,
		},
	}
}

func (h *Handler) handleClick(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return h.errorResponse(request, INTERACTION_UNAVAILABLE, "interaction service not initialized")
	}

	var req ClickRequest
	if request.Payload != nil {
		payloadBytes, err := json.Marshal(request.Payload)
		if err != nil {
			return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "failed to marshal payload")
		}
		if err := json.Unmarshal(payloadBytes, &req); err != nil {
			return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "invalid click payload")
		}
	}

	result, err := h.service.Click(ctx, req)
	if err != nil {
		if interErr, ok := err.(*Error); ok {
			return h.errorResponse(request, interErr.Code, interErr.Message)
		}
		return h.errorResponse(request, INTERACTION_ACTION_FAILED, err.Error())
	}

	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result:          interactionResultToMap(result),
	}
}

func (h *Handler) handleLongClick(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return h.errorResponse(request, INTERACTION_UNAVAILABLE, "interaction service not initialized")
	}

	var req LongClickRequest
	if request.Payload != nil {
		payloadBytes, err := json.Marshal(request.Payload)
		if err != nil {
			return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "failed to marshal payload")
		}
		if err := json.Unmarshal(payloadBytes, &req); err != nil {
			return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "invalid long click payload")
		}
	}

	result, err := h.service.LongClick(ctx, req)
	if err != nil {
		if interErr, ok := err.(*Error); ok {
			return h.errorResponse(request, interErr.Code, interErr.Message)
		}
		return h.errorResponse(request, INTERACTION_ACTION_FAILED, err.Error())
	}

	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result:          interactionResultToMap(result),
	}
}

func (h *Handler) handleInputText(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return h.errorResponse(request, INTERACTION_UNAVAILABLE, "interaction service not initialized")
	}

	var req InputTextRequest
	if request.Payload != nil {
		payloadBytes, err := json.Marshal(request.Payload)
		if err != nil {
			return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "failed to marshal payload")
		}
		if err := json.Unmarshal(payloadBytes, &req); err != nil {
			return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "invalid input text payload")
		}
	}

	result, err := h.service.InputText(ctx, req)
	if err != nil {
		if interErr, ok := err.(*Error); ok {
			return h.errorResponse(request, interErr.Code, interErr.Message)
		}
		return h.errorResponse(request, INTERACTION_ACTION_FAILED, err.Error())
	}

	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result:          interactionResultToMap(result),
	}
}

func (h *Handler) handleClearText(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return h.errorResponse(request, INTERACTION_UNAVAILABLE, "interaction service not initialized")
	}

	var req ClearTextRequest
	if request.Payload != nil {
		payloadBytes, err := json.Marshal(request.Payload)
		if err != nil {
			return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "failed to marshal payload")
		}
		if err := json.Unmarshal(payloadBytes, &req); err != nil {
			return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "invalid clear text payload")
		}
	}

	result, err := h.service.ClearText(ctx, req)
	if err != nil {
		if interErr, ok := err.(*Error); ok {
			return h.errorResponse(request, interErr.Code, interErr.Message)
		}
		return h.errorResponse(request, INTERACTION_ACTION_FAILED, err.Error())
	}

	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result:          interactionResultToMap(result),
	}
}

func (h *Handler) handleScroll(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return h.errorResponse(request, INTERACTION_UNAVAILABLE, "interaction service not initialized")
	}

	var req ScrollRequest
	if request.Payload != nil {
		payloadBytes, err := json.Marshal(request.Payload)
		if err != nil {
			return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "failed to marshal payload")
		}
		if err := json.Unmarshal(payloadBytes, &req); err != nil {
			return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "invalid scroll payload")
		}
	}

	result, err := h.service.Scroll(ctx, req)
	if err != nil {
		if interErr, ok := err.(*Error); ok {
			return h.errorResponse(request, interErr.Code, interErr.Message)
		}
		return h.errorResponse(request, INTERACTION_ACTION_FAILED, err.Error())
	}

	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result:          interactionResultToMap(result),
	}
}

func (h *Handler) handleSwipe(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return h.errorResponse(request, INTERACTION_UNAVAILABLE, "interaction service not initialized")
	}

	var req SwipeRequest
	if request.Payload != nil {
		payloadBytes, err := json.Marshal(request.Payload)
		if err != nil {
			return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "failed to marshal payload")
		}
		if err := json.Unmarshal(payloadBytes, &req); err != nil {
			return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "invalid swipe payload")
		}
	}

	result, err := h.service.Swipe(ctx, req)
	if err != nil {
		if interErr, ok := err.(*Error); ok {
			return h.errorResponse(request, interErr.Code, interErr.Message)
		}
		return h.errorResponse(request, INTERACTION_ACTION_FAILED, err.Error())
	}

	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result:          interactionResultToMap(result),
	}
}

func (h *Handler) handleVisualLocate(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return h.errorResponse(request, INTERACTION_UNAVAILABLE, "interaction service not initialized")
	}

	var req VisualLocateRequest
	if request.Payload != nil {
		payloadBytes, err := json.Marshal(request.Payload)
		if err != nil {
			return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "failed to marshal payload")
		}
		if err := json.Unmarshal(payloadBytes, &req); err != nil {
			return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "invalid visual locate payload")
		}
	}

	candidates, err := h.service.VisualLocate(ctx, req)
	if err != nil {
		if interErr, ok := err.(*Error); ok {
			return h.errorResponse(request, interErr.Code, interErr.Message)
		}
		return h.errorResponse(request, INTERACTION_VISUAL_TARGET_NOT_FOUND, err.Error())
	}

	candidateMaps := make([]map[string]any, 0, len(candidates))
	for _, c := range candidates {
		candidateMaps = append(candidateMaps, map[string]any{
			"source":      c.Source,
			"text":        c.Text,
			"description": c.Description,
			"bounds": map[string]any{
				"left":   c.Bounds.Left,
				"top":    c.Bounds.Top,
				"right":  c.Bounds.Right,
				"bottom": c.Bounds.Bottom,
			},
			"centerX":    c.CenterX,
			"centerY":    c.CenterY,
			"confidence": c.Confidence,
			"ocrLineId":  c.OCRLineID,
		})
	}

	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result: map[string]any{
			"candidates": candidateMaps,
			"count":      len(candidateMaps),
		},
	}
}

func (h *Handler) handleVisualClick(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil {
		return h.errorResponse(request, INTERACTION_UNAVAILABLE, "interaction service not initialized")
	}

	var req VisualClickRequest
	if request.Payload != nil {
		payloadBytes, err := json.Marshal(request.Payload)
		if err != nil {
			return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "failed to marshal payload")
		}
		if err := json.Unmarshal(payloadBytes, &req); err != nil {
			return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "invalid visual click payload")
		}
	}

	result, err := h.service.VisualClick(ctx, req)
	if err != nil {
		if interErr, ok := err.(*Error); ok {
			return h.errorResponse(request, interErr.Code, interErr.Message)
		}
		return h.errorResponse(request, INTERACTION_ACTION_FAILED, err.Error())
	}

	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result:          interactionResultToMap(result),
	}
}

func (h *Handler) errorResponse(request capability.AndroidBridgeRequest, domainCode, message string) capability.AndroidBridgeResponse {
	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "error",
		Error: &capability.AndroidError{
			Code:       MapInteractionErrorToCanonical(domainCode),
			Message:    message,
			DomainCode: domainCode,
		},
	}
}

func interactionResultToMap(result InteractionResult) map[string]any {
	resultMap := map[string]any{
		"success":    result.Success,
		"operation":  result.Operation,
		"strategy":   result.Strategy,
		"verified":   result.Verified,
		"durationMs": result.DurationMS,
	}

	if result.SnapshotID != "" {
		resultMap["snapshotId"] = result.SnapshotID
	}
	if result.NodeID != "" {
		resultMap["nodeId"] = result.NodeID
	}
	if result.X != nil {
		resultMap["x"] = *result.X
	}
	if result.Y != nil {
		resultMap["y"] = *result.Y
	}
	if result.DisplayID != 0 {
		resultMap["displayId"] = result.DisplayID
	}
	if result.Verification != "" {
		resultMap["verification"] = result.Verification
	}
	if result.Warning != "" {
		resultMap["warning"] = result.Warning
	}

	return resultMap
}

var _ androidnative.Handler = (*Handler)(nil)
