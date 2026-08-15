package homekit

import (
	"context"

	"github.com/u-ai/backend/internal/nativebridge"
)

type HomeKitHandler struct {
	bridge nativebridge.Bridge
}

func NewHomeKitHandler(bridge nativebridge.Bridge) *HomeKitHandler {
	return &HomeKitHandler{bridge: bridge}
}

func (h *HomeKitHandler) Execute(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)

	case OperationHomesList:
		return h.handleHomesList(ctx, request)
	case OperationHomesGet:
		return h.handleHomesGet(ctx, request)

	case OperationRoomsList:
		return h.handleRoomsList(ctx, request)
	case OperationZonesList:
		return h.handleZonesList(ctx, request)

	case OperationAccessoriesList:
		return h.handleAccessoriesList(ctx, request)
	case OperationAccessoriesGet:
		return h.handleAccessoriesGet(ctx, request)

	case OperationServicesList:
		return h.handleServicesList(ctx, request)

	case OperationCharacteristicsList:
		return h.handleCharacteristicsList(ctx, request)
	case OperationCharacteristicsRead:
		return h.handleCharacteristicsRead(ctx, request)
	case OperationCharacteristicsWrite:
		return h.handleCharacteristicsWrite(ctx, request)

	case OperationScenesList:
		return h.handleScenesList(ctx, request)
	case OperationScenesGet:
		return h.handleScenesGet(ctx, request)
	case OperationScenesExecute:
		return h.handleScenesExecute(ctx, request)

	case OperationAutomationsList:
		return h.handleAutomationsList(ctx, request)

	default:
		return h.errorResponse(request, nativebridge.ErrOperationNotSupported, "unknown homekit operation: "+request.Operation)
	}
}

func (h *HomeKitHandler) bridgeCall(ctx context.Context, request nativebridge.Request, operation string, payload map[string]any) nativebridge.Response {
	if h.bridge == nil {
		return h.errorResponse(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}

	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestID:       request.RequestID,
			Platform:        "ios",
			Operation:       operation,
			Payload:         payload,
		})
		if err != nil {
			done <- nativebridge.Response{
				ProtocolVersion: request.ProtocolVersion,
				RequestID:       request.RequestID,
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
		return h.errorResponse(request, ErrTimeout, operation+" cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *HomeKitHandler) handleStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationStatus, nil)
}

func (h *HomeKitHandler) handleHomesList(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	payload := map[string]any{}
	if limit, ok := request.Payload["limit"].(float64); ok {
		payload["limit"] = ClampLimit(int(limit), DefaultListLimit, MaxHomeListLimit)
	}
	return h.bridgeCall(ctx, request, OperationHomesList, payload)
}

func (h *HomeKitHandler) handleHomesGet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	homeID, ok := request.Payload["homeId"].(string)
	if !ok || homeID == "" {
		return h.errorResponse(request, ErrHomeNotFound, "missing required field: homeId")
	}
	return h.bridgeCall(ctx, request, OperationHomesGet, map[string]any{"homeId": homeID})
}

func (h *HomeKitHandler) handleRoomsList(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	payload := map[string]any{}
	if homeID, ok := request.Payload["homeId"].(string); ok && homeID != "" {
		payload["homeId"] = homeID
	}
	if limit, ok := request.Payload["limit"].(float64); ok {
		payload["limit"] = ClampLimit(int(limit), DefaultListLimit, MaxRoomListLimit)
	}
	return h.bridgeCall(ctx, request, OperationRoomsList, payload)
}

func (h *HomeKitHandler) handleZonesList(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	payload := map[string]any{}
	if homeID, ok := request.Payload["homeId"].(string); ok && homeID != "" {
		payload["homeId"] = homeID
	}
	if limit, ok := request.Payload["limit"].(float64); ok {
		payload["limit"] = ClampLimit(int(limit), DefaultListLimit, MaxZoneListLimit)
	}
	return h.bridgeCall(ctx, request, OperationZonesList, payload)
}

func (h *HomeKitHandler) handleAccessoriesList(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	payload := map[string]any{}
	if homeID, ok := request.Payload["homeId"].(string); ok && homeID != "" {
		payload["homeId"] = homeID
	}
	if roomID, ok := request.Payload["roomId"].(string); ok && roomID != "" {
		payload["roomId"] = roomID
	}
	if category, ok := request.Payload["category"].(string); ok && category != "" {
		payload["category"] = category
	}
	if nameQuery, ok := request.Payload["nameQuery"].(string); ok && nameQuery != "" {
		payload["nameQuery"] = nameQuery
	}
	if reachableOnly, ok := request.Payload["reachableOnly"].(bool); ok {
		payload["reachableOnly"] = reachableOnly
	}
	if limit, ok := request.Payload["limit"].(float64); ok {
		payload["limit"] = ClampLimit(int(limit), DefaultListLimit, MaxAccessoryListLimit)
	}
	return h.bridgeCall(ctx, request, OperationAccessoriesList, payload)
}

func (h *HomeKitHandler) handleAccessoriesGet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	accessoryID, ok := request.Payload["accessoryId"].(string)
	if !ok || accessoryID == "" {
		return h.errorResponse(request, ErrAccessoryNotFound, "missing required field: accessoryId")
	}
	return h.bridgeCall(ctx, request, OperationAccessoriesGet, map[string]any{"accessoryId": accessoryID})
}

func (h *HomeKitHandler) handleServicesList(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	accessoryID, ok := request.Payload["accessoryId"].(string)
	if !ok || accessoryID == "" {
		return h.errorResponse(request, ErrAccessoryNotFound, "missing required field: accessoryId")
	}
	payload := map[string]any{"accessoryId": accessoryID}
	if limit, ok := request.Payload["limit"].(float64); ok {
		payload["limit"] = ClampLimit(int(limit), DefaultListLimit, MaxServiceListLimit)
	}
	return h.bridgeCall(ctx, request, OperationServicesList, payload)
}

func (h *HomeKitHandler) handleCharacteristicsList(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	serviceID, ok := request.Payload["serviceId"].(string)
	if !ok || serviceID == "" {
		return h.errorResponse(request, ErrServiceNotFound, "missing required field: serviceId")
	}
	payload := map[string]any{"serviceId": serviceID}
	if limit, ok := request.Payload["limit"].(float64); ok {
		payload["limit"] = ClampLimit(int(limit), DefaultListLimit, MaxCharacteristicListLimit)
	}
	return h.bridgeCall(ctx, request, OperationCharacteristicsList, payload)
}

func (h *HomeKitHandler) handleCharacteristicsRead(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	serviceID, ok := request.Payload["serviceId"].(string)
	if !ok || serviceID == "" {
		return h.errorResponse(request, ErrServiceNotFound, "missing required field: serviceId")
	}
	charID, ok := request.Payload["characteristicId"].(string)
	if !ok || charID == "" {
		return h.errorResponse(request, ErrCharacteristicNotFound, "missing required field: characteristicId")
	}
	return h.bridgeCall(ctx, request, OperationCharacteristicsRead, map[string]any{
		"serviceId":         serviceID,
		"characteristicId":  charID,
	})
}

func (h *HomeKitHandler) handleCharacteristicsWrite(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	serviceID, ok := request.Payload["serviceId"].(string)
	if !ok || serviceID == "" {
		return h.errorResponse(request, ErrServiceNotFound, "missing required field: serviceId")
	}
	charID, ok := request.Payload["characteristicId"].(string)
	if !ok || charID == "" {
		return h.errorResponse(request, ErrCharacteristicNotFound, "missing required field: characteristicId")
	}

	value, ok := request.Payload["value"].(map[string]any)
	if !ok || value == nil {
		return h.errorResponse(request, ErrValueTypeInvalid, "missing required field: value")
	}

	return h.bridgeCall(ctx, request, OperationCharacteristicsWrite, map[string]any{
		"serviceId":        serviceID,
		"characteristicId": charID,
		"value":            value,
	})
}

func (h *HomeKitHandler) handleScenesList(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	payload := map[string]any{}
	if homeID, ok := request.Payload["homeId"].(string); ok && homeID != "" {
		payload["homeId"] = homeID
	}
	if limit, ok := request.Payload["limit"].(float64); ok {
		payload["limit"] = ClampLimit(int(limit), DefaultSceneListLimit, MaxSceneListLimit)
	}
	return h.bridgeCall(ctx, request, OperationScenesList, payload)
}

func (h *HomeKitHandler) handleScenesGet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	sceneID, ok := request.Payload["sceneId"].(string)
	if !ok || sceneID == "" {
		return h.errorResponse(request, ErrSceneNotFound, "missing required field: sceneId")
	}
	return h.bridgeCall(ctx, request, OperationScenesGet, map[string]any{"sceneId": sceneID})
}

func (h *HomeKitHandler) handleScenesExecute(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	sceneID, ok := request.Payload["sceneId"].(string)
	if !ok || sceneID == "" {
		return h.errorResponse(request, ErrSceneNotFound, "missing required field: sceneId")
	}
	return h.bridgeCall(ctx, request, OperationScenesExecute, map[string]any{"sceneId": sceneID})
}

func (h *HomeKitHandler) handleAutomationsList(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	payload := map[string]any{}
	if homeID, ok := request.Payload["homeId"].(string); ok && homeID != "" {
		payload["homeId"] = homeID
	}
	if limit, ok := request.Payload["limit"].(float64); ok {
		payload["limit"] = ClampLimit(int(limit), DefaultAutomationListLimit, MaxAutomationListLimit)
	}
	return h.bridgeCall(ctx, request, OperationAutomationsList, payload)
}

func (h *HomeKitHandler) errorResponse(request nativebridge.Request, code, message string) nativebridge.Response {
	return nativebridge.Response{
		ProtocolVersion: request.ProtocolVersion,
		RequestID:       request.RequestID,
		Status:          "error",
		Error: &nativebridge.Error{
			Code:       code,
			Message:    message,
			DomainCode: "HOMEKIT_DOMAIN",
		},
	}
}

