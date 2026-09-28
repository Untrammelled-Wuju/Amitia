package interaction

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/androidnative/uitree"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func applyFallbackDefaults(
	payload map[string]any,
	policy Policy,
	coordinate *bool,
	shizuku *bool,
	visual *bool,
	root *bool,
	adb *bool,
) {
	if _, exists := payload["allowCoordinateFallback"]; !exists && coordinate != nil {
		*coordinate = policy.AllowCoordinateFallback
	}
	if _, exists := payload["allowShizukuFallback"]; !exists && shizuku != nil {
		*shizuku = policy.AllowShizukuFallback
	}
	if _, exists := payload["allowVisualFallback"]; !exists && visual != nil {
		*visual = policy.AllowVisualFallback
	}
	if _, exists := payload["allowRootFallback"]; !exists && root != nil {
		*root = policy.AllowRootFallback
	}
	if _, exists := payload["allowAdbFallback"]; !exists && adb != nil {
		*adb = policy.AllowADBFallback
	}
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
	case OperationNodeAction:
		return h.handleNodeAction(ctx, request)
	case OperationGlobalAction:
		return h.handleGlobalAction(ctx, request)
	case OperationGesture:
		return h.handleGesture(ctx, request)
	case OperationScreenshot:
		return h.handleScreenshot(ctx, request)
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

func (h *Handler) handleNodeAction(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.service == nil || h.service.accessibility == nil {
		return h.errorResponse(request, INTERACTION_UNAVAILABLE, "accessibility executor not initialized")
	}
	var req NodeActionRequest
	if err := json.Unmarshal(mustMarshal(request.Payload), &req); err != nil {
		return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "invalid node action payload")
	}
	req.NativeRef = strings.TrimSpace(req.NativeRef)
	req.Action = strings.TrimSpace(req.Action)
	if req.NativeRef == "" || req.Action == "" {
		return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "nativeRef and action are required")
	}
	if err := h.service.accessibility.PerformNodeAction(
		ctx,
		uitree.ResolvedUINode{NativeRef: req.NativeRef},
		req.Action,
		req.Args,
	); err != nil {
		if interErr, ok := err.(*Error); ok {
			return h.errorResponse(request, interErr.Code, interErr.Message)
		}
		return h.errorResponse(request, INTERACTION_ACTION_FAILED, err.Error())
	}
	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result: map[string]any{
			"success": true,
			"action":  req.Action,
		},
	}
}

func (h *Handler) handleGlobalAction(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	advanced, result := h.advancedAccessibility(request)
	if advanced == nil {
		return result
	}
	var req GlobalActionRequest
	if err := json.Unmarshal(mustMarshal(request.Payload), &req); err != nil {
		return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "invalid global action payload")
	}
	req.Action = strings.TrimSpace(req.Action)
	if req.Action == "" {
		return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "action is required")
	}
	output, err := advanced.PerformGlobalAction(ctx, req.Action)
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
		Result:          output,
	}
}

func (h *Handler) handleGesture(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	advanced, result := h.advancedAccessibility(request)
	if advanced == nil {
		return result
	}
	var req GestureRequest
	if err := json.Unmarshal(mustMarshal(request.Payload), &req); err != nil {
		return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "invalid gesture payload")
	}
	if len(req.Strokes) == 0 {
		return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "strokes is required")
	}
	output, err := advanced.PerformGesture(ctx, request.Payload)
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
		Result:          output,
	}
}

func (h *Handler) handleScreenshot(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	advanced, result := h.advancedAccessibility(request)
	if advanced == nil {
		return result
	}
	var req ScreenshotRequest
	if err := json.Unmarshal(mustMarshal(request.Payload), &req); err != nil {
		return h.errorResponse(request, INTERACTION_INVALID_REQUEST, "invalid screenshot payload")
	}
	output, err := advanced.TakeScreenshot(ctx, req.DisplayID)
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
		Result:          output,
	}
}

func (h *Handler) advancedAccessibility(request capability.AndroidBridgeRequest) (AdvancedAccessibilityExecutor, capability.AndroidBridgeResponse) {
	if h.service == nil || h.service.accessibility == nil {
		return nil, capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:       "PROVIDER_UNAVAILABLE",
				Message:    "accessibility executor not initialized",
				DomainCode: INTERACTION_UNAVAILABLE,
			},
		}
	}
	advanced, ok := h.service.accessibility.(AdvancedAccessibilityExecutor)
	if !ok {
		return nil, capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:       "PROVIDER_UNAVAILABLE",
				Message:    "advanced accessibility operations are unavailable",
				DomainCode: INTERACTION_UNSUPPORTED,
			},
		}
	}
	return advanced, capability.AndroidBridgeResponse{}
}

func mustMarshal(value any) []byte {
	data, _ := json.Marshal(value)
	return data
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
	applyFallbackDefaults(
		request.Payload,
		h.service.policy,
		&req.AllowCoordinateFallback,
		&req.AllowShizukuFallback,
		&req.AllowVisualFallback,
		&req.AllowRootFallback,
		&req.AllowADBFallback,
	)

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
	applyFallbackDefaults(
		request.Payload,
		h.service.policy,
		&req.AllowCoordinateFallback,
		&req.AllowShizukuFallback,
		&req.AllowVisualFallback,
		&req.AllowRootFallback,
		&req.AllowADBFallback,
	)

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
	applyFallbackDefaults(
		request.Payload,
		h.service.policy,
		nil,
		&req.AllowShizukuFallback,
		nil,
		&req.AllowRootFallback,
		&req.AllowADBFallback,
	)

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
	applyFallbackDefaults(
		request.Payload,
		h.service.policy,
		&req.AllowCoordinateFallback,
		&req.AllowShizukuFallback,
		nil,
		&req.AllowRootFallback,
		&req.AllowADBFallback,
	)

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
	applyFallbackDefaults(
		request.Payload,
		h.service.policy,
		&req.AllowCoordinateFallback,
		&req.AllowShizukuFallback,
		nil,
		&req.AllowRootFallback,
		&req.AllowADBFallback,
	)

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
