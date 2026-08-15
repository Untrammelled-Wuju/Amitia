package accessibility

import (
	"context"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	OperationStatus       = "accessibility.status"
	OperationOpenSettings = "accessibility.open_settings"
)

type AccessibilityHandler struct {
	bridge androidnative.NativeBridge
}

func NewAccessibilityHandler(bridge androidnative.NativeBridge) *AccessibilityHandler {
	return &AccessibilityHandler{bridge: bridge}
}

func (h *AccessibilityHandler) Execute(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationOpenSettings:
		return h.handleOpenSettings(ctx, request)
	default:
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    "OPERATION_NOT_SUPPORTED",
				Message: "unknown accessibility operation: " + request.Operation,
			},
		}
	}
}

func (h *AccessibilityHandler) handleStatus(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.bridge == nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    androidnative.ACCESSIBILITY_BRIDGE_UNAVAILABLE,
				Message: "android native bridge is not available",
			},
		}
	}

	bridgeReq := androidnative.NativeBridgeRequest{
		ProtocolVersion: request.ProtocolVersion,
		RequestId:       request.RequestID,
		Operation:       OperationStatus,
		Payload:         map[string]any{},
	}

	resp, err := h.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    androidnative.ACCESSIBILITY_BRIDGE_UNAVAILABLE,
				Message: "accessibility status bridge call failed: " + err.Error(),
			},
		}
	}

	return mapNativeBridgeResponse(resp, request.RequestID)
}

func (h *AccessibilityHandler) handleOpenSettings(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.bridge == nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    androidnative.ACCESSIBILITY_BRIDGE_UNAVAILABLE,
				Message: "android native bridge is not available",
			},
		}
	}

	bridgeReq := androidnative.NativeBridgeRequest{
		ProtocolVersion: request.ProtocolVersion,
		RequestId:       request.RequestID,
		Operation:       OperationOpenSettings,
		Payload:         map[string]any{},
	}

	resp, err := h.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    androidnative.ACCESSIBILITY_BRIDGE_UNAVAILABLE,
				Message: "accessibility open_settings bridge call failed: " + err.Error(),
			},
		}
	}

	return mapNativeBridgeResponse(resp, request.RequestID)
}

func mapNativeBridgeResponse(resp androidnative.NativeBridgeResponse, requestID string) capability.AndroidBridgeResponse {
	result := capability.AndroidBridgeResponse{
		ProtocolVersion: resp.ProtocolVersion,
		RequestID:       resp.RequestId,
		Status:          resp.Status,
	}

	if resp.RequestId != requestID {
		result.Status = "error"
		result.Error = &capability.AndroidError{
			Code:    "BRIDGE_INVALID_RESPONSE",
			Message: "response request ID mismatch",
		}
		return result
	}

	if resp.Result != nil {
		result.Result = resp.Result
	}

	if resp.Error != nil {
		result.Error = &capability.AndroidError{
			Code:       resp.Error.Code,
			Message:    resp.Error.Message,
			DomainCode: resp.Error.DomainCode,
		}
	}

	return result
}
