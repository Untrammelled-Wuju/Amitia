package adb

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type ADBHandler struct {
	config *ADBConfig
	client *ADBClient
}

func NewADBHandler(config *ADBConfig) *ADBHandler {
	client := NewADBClient(config)
	return &ADBHandler{
		config: config,
		client: client,
	}
}

func (h *ADBHandler) Execute(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationDevices:
		return h.handleDevices(ctx, request)
	case OperationExecute:
		return h.handleExecute(ctx, request)
	default:
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    "OPERATION_NOT_SUPPORTED",
				Message: "unsupported adb operation: " + request.Operation,
			},
		}
	}
}

func (h *ADBHandler) handleStatus(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	status := h.client.GetStatus(ctx)
	result := map[string]any{
		"supported":             status.Supported,
		"backend":               status.Backend,
		"serverAvailable":       status.ServerAvailable,
		"deviceCount":           status.DeviceCount,
		"authorizedDeviceCount": status.AuthorizedDeviceCount,
		"defaultDeviceReady":    status.DefaultDeviceReady,
		"state":                 status.State,
	}

	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result:          result,
	}
}

func (h *ADBHandler) handleDevices(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	devices, err := h.client.listDevices(ctx)
	if err != nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    androidnative.PROVIDER_UNAVAILABLE,
				Message: "failed to list adb devices: " + err.Error(),
			},
		}
	}

	deviceList := make([]map[string]any, 0, len(devices))
	for _, d := range devices {
		deviceList = append(deviceList, map[string]any{
			"serial":    d.Serial,
			"state":     d.State,
			"transport": d.Transport,
			"product":   d.Product,
			"model":     d.Model,
			"device":    d.Device,
			"isDefault": d.IsDefault,
		})
	}

	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result: map[string]any{
			"devices": deviceList,
			"count":   len(deviceList),
		},
	}
}

func (h *ADBHandler) handleExecute(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if request.Payload == nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    "INVALID_REQUEST",
				Message: "empty request payload",
			},
		}
	}

	var execReq ADBExecuteRequest
	payloadBytes, err := json.Marshal(request.Payload)
	if err != nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    "INVALID_REQUEST",
				Message: "failed to marshal payload: " + err.Error(),
			},
		}
	}
	if err := json.Unmarshal(payloadBytes, &execReq); err != nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    "INVALID_REQUEST",
				Message: "invalid execute payload: " + err.Error(),
			},
		}
	}

	if err := validateExecuteRequest(&execReq); err != nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    err.Error(),
				Message: "invalid request",
			},
		}
	}

	result, err := h.client.Execute(ctx, execReq)
	if err != nil {
		if policyErr, ok := err.(*PolicyError); ok {
			return capability.AndroidBridgeResponse{
				ProtocolVersion: request.ProtocolVersion,
				RequestID:       request.RequestID,
				Status:          "error",
				Error: &capability.AndroidError{
					Code:       mapPolicyErrorToAndroidError(policyErr),
					Message:    policyErr.Message,
					DomainCode: policyErr.Code,
				},
			}
		}
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    androidnative.PROVIDER_UNAVAILABLE,
				Message: err.Error(),
			},
		}
	}

	return capability.AndroidBridgeResponse{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "success",
		Result: map[string]any{
			"deviceSerial":      result.DeviceSerial,
			"exitCode":          result.ExitCode,
			"stdout":            result.Stdout,
			"stderr":            result.Stderr,
			"durationMs":        result.DurationMs,
			"timedOut":          result.TimedOut,
			"exitCodeAvailable": result.ExitCodeAvailable,
		},
	}
}

func validateExecuteRequest(req *ADBExecuteRequest) *PolicyError {
	if req.Executable == "" {
		return &PolicyError{Code: ADB_INVALID_ARGUMENT, Message: "executable is required"}
	}

	if isShellExecutable(req.Executable) {
		return &PolicyError{Code: ADB_COMMAND_NOT_ALLOWED, Message: "shell executable not allowed"}
	}

	if len(req.Args) > maxArgCount {
		return &PolicyError{Code: ADB_INVALID_ARGUMENT, Message: "too many arguments"}
	}

	for _, arg := range req.Args {
		if len(arg) > maxSingleArgBytes {
			return &PolicyError{Code: ADB_INVALID_ARGUMENT, Message: "argument too long"}
		}
	}

	totalArgBytes := 0
	for _, arg := range req.Args {
		totalArgBytes += len(arg)
	}
	if totalArgBytes > maxTotalArgBytes {
		return &PolicyError{Code: ADB_INVALID_ARGUMENT, Message: "total arguments too large"}
	}

	if len(req.Stdin) > maxInputBytes {
		return &PolicyError{Code: ADB_INPUT_TOO_LARGE, Message: "stdin too large"}
	}

	return nil
}

func mapPolicyErrorToAndroidError(policyErr *PolicyError) string {
	switch policyErr.Code {
	case ADB_DEVICE_UNAUTHORIZED:
		return "USER_ACTION_REQUIRED"
	case ADB_COMMAND_NOT_ALLOWED:
		return "AUTHORIZATION_DENIED"
	case ADB_INVALID_ARGUMENT:
		return "AUTHORIZATION_DENIED"
	case ADB_NO_DEVICE:
		return "PLATFORM_NOT_SUPPORTED"
	case ADB_DEVICE_AMBIGUOUS:
		return "PROVIDER_UNAVAILABLE"
	case ADB_TIMEOUT:
		return "BRIDGE_TIMEOUT"
	case ADB_CANCELLED:
		return "CANCELLED"
	case ADB_DEVICE_OFFLINE, ADB_DEVICE_NOT_FOUND, ADB_DEVICE_DISCONNECTED, ADB_EXECUTION_FAILED, ADB_SERVER_UNAVAILABLE, ADB_UNAVAILABLE, ADB_DEVICE_NO_PERMISSIONS, ADB_DEVICE_LIST_FAILED, ADB_BACKEND_NOT_CONFIGURED:
		return "PROVIDER_UNAVAILABLE"
	case ADB_INPUT_TOO_LARGE, ADB_OUTPUT_TOO_LARGE, ADB_INVALID_RESPONSE:
		return "BRIDGE_INVALID_RESPONSE"
	default:
		return "PROVIDER_UNAVAILABLE"
	}
}

func (h *ADBHandler) InternalExecutor() InternalADBExecutor {
	return h
}

func (h *ADBHandler) ExecuteArgs(
	ctx context.Context,
	deviceSerial string,
	args []string,
	opts InternalADBExecuteOptions,
) (ADBExecuteResult, error) {
	return h.client.ExecuteArgs(ctx, deviceSerial, args, opts)
}
