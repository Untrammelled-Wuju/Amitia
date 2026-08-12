package root

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/androidnative"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type RootHandler struct {
	bridge androidnative.NativeBridge
	policy Policy
}

func NewRootHandler(bridge androidnative.NativeBridge) *RootHandler {
	return &RootHandler{
		bridge: bridge,
		policy: DefaultPolicy(),
	}
}

func NewRootHandlerWithPolicy(bridge androidnative.NativeBridge, policy Policy) *RootHandler {
	return &RootHandler{
		bridge: bridge,
		policy: policy,
	}
}

func (h *RootHandler) Execute(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationRequest:
		return h.handleRequest(ctx, request)
	case OperationExecute:
		return h.handleExecute(ctx, request)
	default:
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:    "OPERATION_NOT_SUPPORTED",
				Message: "unsupported root operation: " + request.Operation,
			},
		}
	}
}

func (h *RootHandler) handleStatus(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.bridge == nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:       "PROVIDER_UNAVAILABLE",
				Message:    "android native bridge is not available",
				DomainCode: ROOT_BRIDGE_UNAVAILABLE,
			},
		}
	}

	bridgeReq := androidnative.NativeBridgeRequest{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
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
				Code:       "PROVIDER_UNAVAILABLE",
				Message:    "root status bridge call failed: " + err.Error(),
				DomainCode: ROOT_BRIDGE_UNAVAILABLE,
			},
		}
	}

	return mapNativeBridgeResponse(resp, request.RequestID)
}

func (h *RootHandler) handleRequest(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.bridge == nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:       "PROVIDER_UNAVAILABLE",
				Message:    "android native bridge is not available",
				DomainCode: ROOT_BRIDGE_UNAVAILABLE,
			},
		}
	}

	bridgeReq := androidnative.NativeBridgeRequest{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Operation:       OperationRequest,
		Payload:         map[string]any{},
	}

	resp, err := h.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:       "PROVIDER_UNAVAILABLE",
				Message:    "root request bridge call failed: " + err.Error(),
				DomainCode: ROOT_BRIDGE_UNAVAILABLE,
			},
		}
	}

	return mapNativeBridgeResponse(resp, request.RequestID)
}

func (h *RootHandler) handleExecute(ctx context.Context, request capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	if h.bridge == nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:       "PROVIDER_UNAVAILABLE",
				Message:    "android native bridge is not available",
				DomainCode: ROOT_BRIDGE_UNAVAILABLE,
			},
		}
	}

	if request.Payload == nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:       "AUTHORIZATION_DENIED",
				Message:    "empty request payload",
				DomainCode: ROOT_INVALID_ARGUMENT,
			},
		}
	}

	var execReq ExecuteRequest
	payloadBytes, err := json.Marshal(request.Payload)
	if err != nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:       "AUTHORIZATION_DENIED",
				Message:    "failed to marshal payload: " + err.Error(),
				DomainCode: ROOT_INVALID_ARGUMENT,
			},
		}
	}
	if err := json.Unmarshal(payloadBytes, &execReq); err != nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:       "AUTHORIZATION_DENIED",
				Message:    "invalid execute payload: " + err.Error(),
				DomainCode: ROOT_INVALID_ARGUMENT,
			},
		}
	}

	if policyErr := h.policy.ValidateExecute(&execReq); policyErr != nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:       MapRootPolicyError(policyErr.Code),
				Message:    policyErr.Message,
				DomainCode: policyErr.Code,
			},
		}
	}

	if err := ValidateWorkDir(execReq.WorkDir); err != nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:       "AUTHORIZATION_DENIED",
				Message:    err.Error(),
				DomainCode: ROOT_INVALID_ARGUMENT,
			},
		}
	}

	execReq.TimeoutMS = h.policy.ValidateTimeout(execReq.TimeoutMS)

	bridgePayload := map[string]any{
		"executable": execReq.Executable,
		"args":       execReq.Args,
		"stdin":      execReq.Stdin,
		"env":        execReq.Env,
		"workDir":    execReq.WorkDir,
		"timeoutMs":  execReq.TimeoutMS,
		"mode":       execReq.Mode,
	}

	bridgeReq := androidnative.NativeBridgeRequest{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Operation:       OperationExecute,
		Payload:         bridgePayload,
	}

	resp, err := h.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return capability.AndroidBridgeResponse{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Status:          "error",
			Error: &capability.AndroidError{
				Code:       "PROVIDER_UNAVAILABLE",
				Message:    "root execute bridge call failed: " + err.Error(),
				DomainCode: ROOT_BRIDGE_UNAVAILABLE,
			},
		}
	}

	return mapNativeBridgeResponse(resp, request.RequestID)
}

func (h *RootHandler) InternalExecutor() InternalRootExecutor {
	return h
}

func (h *RootHandler) ExecuteRoot(
	ctx context.Context,
	req ExecuteRequest,
	opts InternalExecuteOptions,
) (ExecuteResult, error) {
	if h.bridge == nil {
		return ExecuteResult{}, &PolicyError{Code: ROOT_BRIDGE_UNAVAILABLE, Message: "bridge unavailable"}
	}

	if policyErr := h.policy.ValidateExecute(&req); policyErr != nil {
		return ExecuteResult{}, policyErr
	}

	if err := ValidateWorkDir(req.WorkDir); err != nil {
		return ExecuteResult{}, &PolicyError{Code: ROOT_INVALID_ARGUMENT, Message: err.Error()}
	}

	timeoutMS := opts.Timeout
	if timeoutMS <= 0 {
		timeoutMS = int64(h.policy.DefaultTimeoutMS)
	}
	if timeoutMS > int64(h.policy.HardTimeoutMS) {
		timeoutMS = int64(h.policy.HardTimeoutMS)
	}

	req.TimeoutMS = int(timeoutMS)

	bridgePayload := map[string]any{
		"executable": req.Executable,
		"args":       req.Args,
		"stdin":      req.Stdin,
		"env":        req.Env,
		"workDir":    req.WorkDir,
		"timeoutMs":  req.TimeoutMS,
		"mode":       req.Mode,
	}

	bridgeReq := androidnative.NativeBridgeRequest{
		ProtocolVersion: androidBridgeProtocolVersion,
		RequestID:       "",
		Operation:       OperationExecute,
		Payload:         bridgePayload,
	}

	resp, err := h.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return ExecuteResult{}, &PolicyError{Code: ROOT_BRIDGE_UNAVAILABLE, Message: err.Error()}
	}

	if resp.Error != nil {
		return ExecuteResult{}, &PolicyError{Code: resp.Error.DomainCode, Message: resp.Error.Message}
	}

	result := ExecuteResult{}
	if resp.Result != nil {
		if v, ok := resp.Result["exitCode"].(float64); ok {
			result.ExitCode = int(v)
		}
		if v, ok := resp.Result["exitCodeAvailable"].(bool); ok {
			result.ExitCodeAvailable = v
		}
		if v, ok := resp.Result["stdout"].(string); ok {
			result.Stdout = v
		}
		if v, ok := resp.Result["stderr"].(string); ok {
			result.Stderr = v
		}
		if v, ok := resp.Result["durationMs"].(float64); ok {
			result.DurationMS = int64(v)
		}
		if v, ok := resp.Result["timedOut"].(bool); ok {
			result.TimedOut = v
		}
	}

	return result, nil
}

const androidBridgeProtocolVersion = 1

func mapNativeBridgeResponse(resp androidnative.NativeBridgeResponse, requestID string) capability.AndroidBridgeResponse {
	result := capability.AndroidBridgeResponse{
		ProtocolVersion: resp.ProtocolVersion,
		RequestID:       resp.RequestID,
		Status:          resp.Status,
	}

	if resp.RequestID != requestID {
		result.Status = "error"
		result.Error = &capability.AndroidError{
			Code:       "BRIDGE_INVALID_RESPONSE",
			Message:    "response request ID mismatch",
			DomainCode: ROOT_INVALID_RESPONSE,
		}
		return result
	}

	if resp.Result != nil {
		result.Result = resp.Result
	}

	if resp.Error != nil {
		result.Error = &capability.AndroidError{
			Code:       MapRootErrorToCanonical(resp.Error.DomainCode),
			Message:    resp.Error.Message,
			DomainCode: resp.Error.DomainCode,
		}
	}

	return result
}
