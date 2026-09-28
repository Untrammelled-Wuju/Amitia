package shizuku

import (
	"context"
	"strings"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	OperationStatus            = "shizuku.status"
	OperationInfo              = "shizuku.info"
	OperationRequestPermission = "shizuku.request_permission"
	OperationPermissionCheck   = "shizuku.permission_check"
	OperationExecute           = "shizuku.execute"
	OperationSetEnabled        = "shizuku.set_enabled"
	OperationOpenManager       = "shizuku.open_manager"
	OperationTest              = "shizuku.test"
	OperationUserServiceStatus = "shizuku.user_service.status"
	OperationUserServiceBind   = "shizuku.user_service.bind"
	OperationUserServiceUnbind = "shizuku.user_service.unbind"
	OperationProcessStart      = "shizuku.process.start"
	OperationProcessWrite      = "shizuku.process.write"
	OperationProcessRead       = "shizuku.process.read"
	OperationProcessWait       = "shizuku.process.wait"
	OperationProcessKill       = "shizuku.process.kill"
	OperationSystemService     = "shizuku.system_service.resolve"
	OperationBinderTransact    = "shizuku.binder.transact"
)

var Operations = []string{
	OperationStatus,
	OperationInfo,
	OperationRequestPermission,
	OperationPermissionCheck,
	OperationExecute,
	OperationSetEnabled,
	OperationOpenManager,
	OperationTest,
	OperationUserServiceStatus,
	OperationUserServiceBind,
	OperationUserServiceUnbind,
	OperationProcessStart,
	OperationProcessWrite,
	OperationProcessRead,
	OperationProcessWait,
	OperationProcessKill,
	OperationSystemService,
	OperationBinderTransact,
}

type Handler struct {
	bridge androidnative.NativeBridge
}

func NewHandler(bridge androidnative.NativeBridge) *Handler {
	return &Handler{bridge: bridge}
}

func (h *Handler) Execute(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if !isSupportedOperation(request.Operation) {
		return errorResponse(
			request,
			"OPERATION_NOT_SUPPORTED",
			"unsupported shizuku operation: "+request.Operation,
			"SHIZUKU_OPERATION_NOT_SUPPORTED",
		)
	}
	if h == nil || h.bridge == nil {
		return errorResponse(
			request,
			"PROVIDER_UNAVAILABLE",
			"android native bridge is not available",
			"SHIZUKU_BRIDGE_UNAVAILABLE",
		)
	}

	payload := request.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	if request.Operation == OperationExecute {
		executable, _ := payload["executable"].(string)
		command, _ := payload["command"].(string)
		if strings.TrimSpace(executable) == "" && strings.TrimSpace(command) == "" {
			return errorResponse(
				request,
				"AUTHORIZATION_DENIED",
				"executable or command is required",
				"SHIZUKU_INVALID_REQUEST",
			)
		}
		if strings.TrimSpace(executable) != "" && strings.TrimSpace(command) != "" {
			return errorResponse(
				request,
				"AUTHORIZATION_DENIED",
				"executable and command are mutually exclusive",
				"SHIZUKU_INVALID_REQUEST",
			)
		}
	}

	resp, err := h.bridge.Execute(ctx, androidnative.NativeBridgeRequest{
		ProtocolVersion: request.ProtocolVersion,
		RequestId:       request.RequestID,
		Operation:       request.Operation,
		Payload:         payload,
	})
	if err != nil {
		return errorResponse(
			request,
			"BRIDGE_DISCONNECTED",
			"shizuku bridge call failed: "+err.Error(),
			"SHIZUKU_BRIDGE_UNAVAILABLE",
		)
	}
	return mapResponse(request, resp)
}

func isSupportedOperation(operation string) bool {
	for _, candidate := range Operations {
		if operation == candidate {
			return true
		}
	}
	return false
}

func mapResponse(
	request capability.AndroidBridgeRequest,
	resp androidnative.NativeBridgeResponse,
) capability.AndroidBridgeResponse {
	if resp.RequestId != request.RequestID {
		return errorResponse(
			request,
			"BRIDGE_INVALID_RESPONSE",
			"response request ID mismatch",
			"SHIZUKU_INVALID_RESPONSE",
		)
	}

	result := capability.AndroidBridgeResponse{
		ProtocolVersion: resp.ProtocolVersion,
		RequestID:       resp.RequestId,
		Status:          resp.Status,
		Result:          resp.Result,
	}
	if resp.Error != nil {
		code, domainCode := canonicalError(resp.Error.Code)
		result.Status = "error"
		result.Error = &capability.AndroidError{
			Code:       code,
			Message:    resp.Error.Message,
			DomainCode: domainCode,
		}
	}
	return result
}

func canonicalError(code string) (string, string) {
	switch code {
	case "SHIZUKU_PERMISSION_REQUIRED", "SHIZUKU_PERMISSION_DENIED", "SHIZUKU_PERMISSION_PERMANENTLY_DENIED", "SHIZUKU_DISABLED":
		return "USER_ACTION_REQUIRED", code
	case "SHIZUKU_NOT_INSTALLED", "SHIZUKU_NOT_RUNNING", "SHIZUKU_BINDER_UNAVAILABLE", "SHIZUKU_SERVICE_UNAVAILABLE", "SHIZUKU_MANAGER_OPEN_FAILED", "SHIZUKU_INSTALL_PAGE_OPEN_FAILED":
		return "PROVIDER_UNAVAILABLE", code
	case "SHIZUKU_PERMISSION_TIMEOUT", "SHIZUKU_BIND_TIMEOUT", "SHIZUKU_TIMEOUT":
		return "BRIDGE_TIMEOUT", code
	case "SHIZUKU_INVALID_REQUEST", "SHIZUKU_SERVICE_DEAD", "SHIZUKU_COMMAND_FAILED", "SHIZUKU_EXECUTION_ERROR", "SHIZUKU_RESPONSE_PARSE_ERROR":
		return "AUTHORIZATION_DENIED", code
	default:
		return "PROVIDER_UNAVAILABLE", code
	}
}

func errorResponse(
	request capability.AndroidBridgeRequest,
	code string,
	message string,
	domainCode string,
) capability.AndroidBridgeResponse {
	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "error",
		Error: &capability.AndroidError{
			Code:       code,
			Message:    message,
			DomainCode: domainCode,
		},
	}
}

var _ androidnative.Handler = (*Handler)(nil)
