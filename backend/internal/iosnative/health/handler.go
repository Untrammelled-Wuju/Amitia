package health

import (
	"context"

	"github.com/u-ai/backend/internal/nativebridge"
)

const (
	errHealthKitUnavailable    = nativebridge.ErrProviderUnavailable
	errHealthAuthorizationReq  = nativebridge.ErrAuthorizationDenied
	errHealthQueryInvalid      = "HEALTH_QUERY_INVALID"
	errHealthTypeUnsupported   = "HEALTH_TYPE_UNSUPPORTED"
	errHealthWriteNotEnabled   = "HEALTH_WRITE_NOT_ENABLED"
	errHealthDataNotAvailable  = "HEALTH_DATA_NOT_AVAILABLE"
	errNativeBridgeUnavailable = nativebridge.ErrProviderUnavailable
)

type HealthHandler struct {
	bridge nativebridge.Bridge
}

func NewHealthHandler(bridge nativebridge.Bridge) *HealthHandler {
	return &HealthHandler{bridge: bridge}
}

func (h *HealthHandler) Execute(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	switch request.Operation {
	case "health.authorization.status":
		return h.handleAuthorizationStatus(ctx, request)
	case "health.authorization.request":
		return h.handleAuthorizationRequest(ctx, request)
	case "health.profile.read":
		return h.handleProfileRead(ctx, request)
	case "health.samples.query":
		return h.handleSamplesQuery(ctx, request)
	case "health.statistics.query":
		return h.handleStatisticsQuery(ctx, request)
	case "health.workouts.query":
		return h.handleWorkoutsQuery(ctx, request)
	case "health.workouts.detail":
		return h.handleWorkoutsDetail(ctx, request)
	case "health.sleep.query":
		return h.handleSleepQuery(ctx, request)
	case "health.activity.query":
		return h.handleActivityQuery(ctx, request)
	default:
		return nativebridge.Response{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Status:          "error",
			Error: &nativebridge.Error{
				Code:    nativebridge.ErrOperationNotSupported,
				Message: "unknown health operation: " + request.Operation,
			},
		}
	}
}

func (h *HealthHandler) handleAuthorizationStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, errNativeBridgeUnavailable, "ios native bridge is not available")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       "health.authorization.status",
		})
		if err != nil {
			done <- nativebridge.Response{
				ProtocolVersion: request.ProtocolVersion,
				RequestId:       request.RequestId,
				Status:          "error",
				Error: &nativebridge.Error{
					Code:    nativebridge.ErrBridgeTimeout,
					Message: err.Error(),
				},
			}
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return nativebridge.Response{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Status:          "error",
			Error: &nativebridge.Error{
				Code:    nativebridge.ErrBridgeTimeout,
				Message: "health authorization status query cancelled",
			},
		}
	case resp := <-done:
		return resp
	}
}

func (h *HealthHandler) handleAuthorizationRequest(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, errNativeBridgeUnavailable, "ios native bridge is not available")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       "health.authorization.request",
			Payload:         request.Payload,
		})
		if err != nil {
			done <- nativebridge.Response{
				ProtocolVersion: request.ProtocolVersion,
				RequestId:       request.RequestId,
				Status:          "error",
				Error: &nativebridge.Error{
					Code:    nativebridge.ErrBridgeTimeout,
					Message: err.Error(),
				},
			}
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return nativebridge.Response{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Status:          "error",
			Error: &nativebridge.Error{
				Code:    nativebridge.ErrBridgeTimeout,
				Message: "health authorization request cancelled",
			},
		}
	case resp := <-done:
		return resp
	}
}

func (h *HealthHandler) handleProfileRead(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, errNativeBridgeUnavailable, "ios native bridge is not available")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       "health.profile.read",
		})
		if err != nil {
			done <- nativebridge.Response{
				ProtocolVersion: request.ProtocolVersion,
				RequestId:       request.RequestId,
				Status:          "error",
				Error: &nativebridge.Error{
					Code:    nativebridge.ErrBridgeTimeout,
					Message: err.Error(),
				},
			}
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return nativebridge.Response{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Status:          "error",
			Error: &nativebridge.Error{
				Code:    nativebridge.ErrBridgeTimeout,
				Message: "health profile read cancelled",
			},
		}
	case resp := <-done:
		return resp
	}
}

func (h *HealthHandler) handleSamplesQuery(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, errNativeBridgeUnavailable, "ios native bridge is not available")
	}

	typeName, ok := request.Payload["type"].(string)
	if !ok || typeName == "" {
		return h.errorResponse(request, errHealthQueryInvalid, "missing required field: type")
	}

	dt, supported := ResolveHealthType(typeName)
	if !supported {
		return h.errorResponse(request, errHealthTypeUnsupported, "unsupported health type: "+typeName)
	}

	payload := map[string]any{
		"type":       typeName,
		"identifier": dt.Identifier,
		"unit":       dt.Unit,
	}

	if startTime, ok := request.Payload["startTime"].(string); ok {
		payload["startTime"] = startTime
	}
	if endTime, ok := request.Payload["endTime"].(string); ok {
		payload["endTime"] = endTime
	}
	if limit, ok := request.Payload["limit"].(float64); ok {
		payload["limit"] = int(limit)
	}
	if ascending, ok := request.Payload["ascending"].(bool); ok {
		payload["ascending"] = ascending
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       "health.samples.query",
			Payload:         payload,
		})
		if err != nil {
			done <- nativebridge.Response{
				ProtocolVersion: request.ProtocolVersion,
				RequestId:       request.RequestId,
				Status:          "error",
				Error: &nativebridge.Error{
					Code:    nativebridge.ErrBridgeTimeout,
					Message: err.Error(),
				},
			}
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return nativebridge.Response{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Status:          "error",
			Error: &nativebridge.Error{
				Code:    nativebridge.ErrBridgeTimeout,
				Message: "health samples query cancelled",
			},
		}
	case resp := <-done:
		return resp
	}
}

func (h *HealthHandler) handleStatisticsQuery(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, errNativeBridgeUnavailable, "ios native bridge is not available")
	}

	typeName, ok := request.Payload["type"].(string)
	if !ok || typeName == "" {
		return h.errorResponse(request, errHealthQueryInvalid, "missing required field: type")
	}

	_, supported := ResolveHealthType(typeName)
	if !supported {
		return h.errorResponse(request, errHealthTypeUnsupported, "unsupported health type: "+typeName)
	}

	payload := map[string]any{
		"type": typeName,
	}

	if startTime, ok := request.Payload["startTime"].(string); ok {
		payload["startTime"] = startTime
	}
	if endTime, ok := request.Payload["endTime"].(string); ok {
		payload["endTime"] = endTime
	}
	if statistic, ok := request.Payload["statistic"].(string); ok {
		payload["statistic"] = statistic
	}
	if bucket, ok := request.Payload["bucket"].(string); ok {
		payload["bucket"] = bucket
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       "health.statistics.query",
			Payload:         payload,
		})
		if err != nil {
			done <- nativebridge.Response{
				ProtocolVersion: request.ProtocolVersion,
				RequestId:       request.RequestId,
				Status:          "error",
				Error: &nativebridge.Error{
					Code:    nativebridge.ErrBridgeTimeout,
					Message: err.Error(),
				},
			}
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return nativebridge.Response{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Status:          "error",
			Error: &nativebridge.Error{
				Code:    nativebridge.ErrBridgeTimeout,
				Message: "health statistics query cancelled",
			},
		}
	case resp := <-done:
		return resp
	}
}

func (h *HealthHandler) handleWorkoutsQuery(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, errNativeBridgeUnavailable, "ios native bridge is not available")
	}

	payload := map[string]any{}
	if startTime, ok := request.Payload["startTime"].(string); ok {
		payload["startTime"] = startTime
	}
	if endTime, ok := request.Payload["endTime"].(string); ok {
		payload["endTime"] = endTime
	}
	if limit, ok := request.Payload["limit"].(float64); ok {
		payload["limit"] = int(limit)
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       "health.workouts.query",
			Payload:         payload,
		})
		if err != nil {
			done <- nativebridge.Response{
				ProtocolVersion: request.ProtocolVersion,
				RequestId:       request.RequestId,
				Status:          "error",
				Error: &nativebridge.Error{
					Code:    nativebridge.ErrBridgeTimeout,
					Message: err.Error(),
				},
			}
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return nativebridge.Response{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Status:          "error",
			Error: &nativebridge.Error{
				Code:    nativebridge.ErrBridgeTimeout,
				Message: "health workouts query cancelled",
			},
		}
	case resp := <-done:
		return resp
	}
}

func (h *HealthHandler) handleWorkoutsDetail(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, errNativeBridgeUnavailable, "ios native bridge is not available")
	}

	workoutID, ok := request.Payload["workoutId"].(string)
	if !ok || workoutID == "" {
		return h.errorResponse(request, errHealthQueryInvalid, "missing required field: workoutId")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       "health.workouts.detail",
			Payload:         map[string]any{"workoutId": workoutID},
		})
		if err != nil {
			done <- nativebridge.Response{
				ProtocolVersion: request.ProtocolVersion,
				RequestId:       request.RequestId,
				Status:          "error",
				Error: &nativebridge.Error{
					Code:    nativebridge.ErrBridgeTimeout,
					Message: err.Error(),
				},
			}
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return nativebridge.Response{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Status:          "error",
			Error: &nativebridge.Error{
				Code:    nativebridge.ErrBridgeTimeout,
				Message: "health workout detail query cancelled",
			},
		}
	case resp := <-done:
		return resp
	}
}

func (h *HealthHandler) handleSleepQuery(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, errNativeBridgeUnavailable, "ios native bridge is not available")
	}

	payload := map[string]any{}
	if startTime, ok := request.Payload["startTime"].(string); ok {
		payload["startTime"] = startTime
	}
	if endTime, ok := request.Payload["endTime"].(string); ok {
		payload["endTime"] = endTime
	}
	if limit, ok := request.Payload["limit"].(float64); ok {
		payload["limit"] = int(limit)
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       "health.sleep.query",
			Payload:         payload,
		})
		if err != nil {
			done <- nativebridge.Response{
				ProtocolVersion: request.ProtocolVersion,
				RequestId:       request.RequestId,
				Status:          "error",
				Error: &nativebridge.Error{
					Code:    nativebridge.ErrBridgeTimeout,
					Message: err.Error(),
				},
			}
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return nativebridge.Response{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Status:          "error",
			Error: &nativebridge.Error{
				Code:    nativebridge.ErrBridgeTimeout,
				Message: "health sleep query cancelled",
			},
		}
	case resp := <-done:
		return resp
	}
}

func (h *HealthHandler) handleActivityQuery(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, errNativeBridgeUnavailable, "ios native bridge is not available")
	}

	payload := map[string]any{}
	if date, ok := request.Payload["date"].(string); ok {
		payload["date"] = date
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       "health.activity.query",
			Payload:         payload,
		})
		if err != nil {
			done <- nativebridge.Response{
				ProtocolVersion: request.ProtocolVersion,
				RequestId:       request.RequestId,
				Status:          "error",
				Error: &nativebridge.Error{
					Code:    nativebridge.ErrBridgeTimeout,
					Message: err.Error(),
				},
			}
			return
		}
		done <- resp
	}()

	select {
	case <-ctx.Done():
		return nativebridge.Response{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Status:          "error",
			Error: &nativebridge.Error{
				Code:    nativebridge.ErrBridgeTimeout,
				Message: "health activity query cancelled",
			},
		}
	case resp := <-done:
		return resp
	}
}

func (h *HealthHandler) errorResponse(request nativebridge.Request, code, message string) nativebridge.Response {
	return nativebridge.Response{
		ProtocolVersion: request.ProtocolVersion,
		RequestId:       request.RequestId,
		Status:          "error",
		Error: &nativebridge.Error{
			Code:       code,
			Message:    message,
			DomainCode: "HEALTH_DOMAIN",
		},
	}
}
