package devicecontrol

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/androidsystem"
	"github.com/u-ai/backend/internal/nativebridge"
)

type Handler struct {
	bridge nativebridge.Bridge
}

func NewHandler(bridge nativebridge.Bridge) *Handler {
	return &Handler{bridge: bridge}
}

func (h *Handler) Execute(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	if h == nil || h.bridge == nil {
		return failure(request, "DEVICE_NATIVE_HOST_UNAVAILABLE", "Android native bridge is unavailable")
	}
	if !supported(request.Operation) {
		return failure(request, "DEVICE_OPERATION_NOT_SUPPORTED", "unsupported Android device operation: "+request.Operation)
	}
	resp, err := h.bridge.Execute(ctx, nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       request.RequestID,
		Platform:        "android",
		Operation:       request.Operation,
		Payload:         request.Payload,
	})
	if err != nil {
		return failure(request, "DEVICE_NATIVE_HOST_UNAVAILABLE", err.Error())
	}
	result := androidsystem.SystemResponse{
		RequestID: resp.RequestId,
		Status:    resp.Status,
		Result:    resp.Result,
	}
	if result.RequestID == "" {
		result.RequestID = request.RequestID
	}
	if resp.Error != nil {
		result.Error = &androidsystem.SystemError{
			Code:       resp.Error.Code,
			Message:    resp.Error.Message,
			DomainCode: resp.Error.DomainCode,
		}
	}
	if result.Status == "" {
		return failure(request, "DEVICE_NATIVE_PROTOCOL_ERROR", "Android native bridge returned an empty status")
	}
	return result
}

func supported(operation string) bool {
	for _, candidate := range Operations {
		if candidate == operation {
			return true
		}
	}
	return false
}

func failure(request androidsystem.SystemRequest, code, message string) androidsystem.SystemResponse {
	if message == "" {
		message = fmt.Sprintf("Android device operation %s failed", request.Operation)
	}
	return androidsystem.SystemResponse{
		RequestID: request.RequestID,
		Status:    "error",
		Error: &androidsystem.SystemError{
			Code:    code,
			Message: message,
		},
	}
}
