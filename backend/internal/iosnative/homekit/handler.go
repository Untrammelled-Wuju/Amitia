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
	case OperationScenesCreate:
		return h.handleScenesCreate(ctx, request)
	case OperationScenesUpdate:
		return h.handleScenesUpdate(ctx, request)
	case OperationScenesDelete:
		return h.handleScenesDelete(ctx, request)

	case OperationAutomationsList:
		return h.handleAutomationsList(ctx, request)
	case OperationAutomationsGet:
		return h.handleAutomationsGet(ctx, request)
	case OperationAutomationsCreate:
		return h.handleAutomationsCreate(ctx, request)
	case OperationAutomationsUpdate:
		return h.handleAutomationsUpdate(ctx, request)
	case OperationAutomationsEnable:
		return h.handleAutomationsEnable(ctx, request)
	case OperationAutomationsDelete:
		return h.handleAutomationsDelete(ctx, request)

	case OperationSetupPresent:
		return h.handleSetupPresent(ctx, request)
	case OperationEnableHomeKit:
		return h.handleEnableHomeKit(ctx, request)

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

func (h *HomeKitHandler) handleScenesCreate(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	input, err := extractCreateSceneInput(request.Payload)
	if err != nil {
		return h.errorResponse(request, ErrInvalidResponse, err.Error())
	}
	if validateErr := ValidateCreateSceneInput(*input); validateErr != nil {
		return h.errorResponse(request, ErrInvalidResponse, validateErr.Error())
	}
	return h.bridgeCall(ctx, request, OperationScenesCreate, payloadFromCreateSceneInput(input))
}

func (h *HomeKitHandler) handleScenesUpdate(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	sceneID, ok := request.Payload["sceneId"].(string)
	if !ok || sceneID == "" {
		return h.errorResponse(request, ErrSceneNotFound, "missing required field: sceneId")
	}
	payload := map[string]any{"sceneId": sceneID}
	if name, ok := request.Payload["name"].(string); ok {
		payload["name"] = name
	}
	return h.bridgeCall(ctx, request, OperationScenesUpdate, payload)
}

func (h *HomeKitHandler) handleScenesDelete(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	sceneID, ok := request.Payload["sceneId"].(string)
	if !ok || sceneID == "" {
		return h.errorResponse(request, ErrSceneNotFound, "missing required field: sceneId")
	}
	return h.bridgeCall(ctx, request, OperationScenesDelete, map[string]any{"sceneId": sceneID})
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

func (h *HomeKitHandler) handleAutomationsGet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	autoID, ok := request.Payload["automationId"].(string)
	if !ok || autoID == "" {
		return h.errorResponse(request, ErrAutomationNotFound, "missing required field: automationId")
	}
	return h.bridgeCall(ctx, request, OperationAutomationsGet, map[string]any{"automationId": autoID})
}

func (h *HomeKitHandler) handleAutomationsCreate(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	input, err := extractCreateAutomationInput(request.Payload)
	if err != nil {
		return h.errorResponse(request, ErrInvalidResponse, err.Error())
	}
	if validateErr := ValidateAutomationInput(*input); validateErr != nil {
		return h.errorResponse(request, ErrInvalidResponse, validateErr.Error())
	}
	return h.bridgeCall(ctx, request, OperationAutomationsCreate, payloadFromCreateAutomationInput(input))
}

func (h *HomeKitHandler) handleAutomationsUpdate(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	autoID, ok := request.Payload["automationId"].(string)
	if !ok || autoID == "" {
		return h.errorResponse(request, ErrAutomationNotFound, "missing required field: automationId")
	}
	payload := map[string]any{"automationId": autoID}
	if name, ok := request.Payload["name"].(string); ok {
		payload["name"] = name
	}
	if enabled, ok := request.Payload["enabled"].(bool); ok {
		payload["enabled"] = enabled
	}
	return h.bridgeCall(ctx, request, OperationAutomationsUpdate, payload)
}

func (h *HomeKitHandler) handleAutomationsEnable(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	autoID, ok := request.Payload["automationId"].(string)
	if !ok || autoID == "" {
		return h.errorResponse(request, ErrAutomationNotFound, "missing required field: automationId")
	}
	enabled := true
	if v, ok := request.Payload["enabled"].(bool); ok {
		enabled = v
	}
	return h.bridgeCall(ctx, request, OperationAutomationsEnable, map[string]any{
		"automationId": autoID,
		"enabled":      enabled,
	})
}

func (h *HomeKitHandler) handleAutomationsDelete(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	autoID, ok := request.Payload["automationId"].(string)
	if !ok || autoID == "" {
		return h.errorResponse(request, ErrAutomationNotFound, "missing required field: automationId")
	}
	return h.bridgeCall(ctx, request, OperationAutomationsDelete, map[string]any{"automationId": autoID})
}

func (h *HomeKitHandler) handleSetupPresent(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	payload := map[string]any{}
	if homeID, ok := request.Payload["homeId"].(string); ok {
		payload["homeId"] = homeID
	}
	if roomID, ok := request.Payload["roomId"].(string); ok {
		payload["roomId"] = roomID
	}
	return h.bridgeCall(ctx, request, OperationSetupPresent, payload)
}

func (h *HomeKitHandler) handleEnableHomeKit(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationEnableHomeKit, nil)
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

func extractCreateSceneInput(payload map[string]any) (*CreateSceneInput, error) {
	homeID, ok := payload["homeId"].(string)
	if !ok || homeID == "" {
		return nil, NewValidationError("homeId is required")
	}
	name, ok := payload["name"].(string)
	if !ok || name == "" {
		return nil, NewValidationError("name is required")
	}
	input := &CreateSceneInput{
		HomeID: homeID,
		Name:   name,
	}
	if actionsRaw, ok := payload["actions"].([]any); ok {
		for _, raw := range actionsRaw {
			m, ok := raw.(map[string]any)
			{
			}
			if !ok {
				continue
			}
			action := SceneActionInput{}
			if v, ok := m["accessoryId"].(string); ok {
				action.AccessoryID = v
			}
			if v, ok := m["serviceId"].(string); ok {
				action.ServiceID = v
			}
			if v, ok := m["characteristicId"].(string); ok {
				action.CharacteristicID = v
			}
			if v, ok := m["targetValue"].(map[string]any); ok {
				action.TargetValue = parseCharacteristicValue(v)
			}
			input.Actions = append(input.Actions, action)
		}
	}
	return input, nil
}

func payloadFromCreateSceneInput(input *CreateSceneInput) map[string]any {
	actions := make([]map[string]any, 0, len(input.Actions))
	for _, a := range input.Actions {
		actions = append(actions, map[string]any{
			"accessoryId":      a.AccessoryID,
			"serviceId":        a.ServiceID,
			"characteristicId": a.CharacteristicID,
			"targetValue":      serializeCharacteristicValue(a.TargetValue),
		})
	}
	return map[string]any{
		"homeId":  input.HomeID,
		"name":    input.Name,
		"actions": actions,
	}
}

func extractCreateAutomationInput(payload map[string]any) (*CreateAutomationInput, error) {
	homeID, ok := payload["homeId"].(string)
	if !ok || homeID == "" {
		return nil, NewValidationError("homeId is required")
	}
	name, ok := payload["name"].(string)
	if !ok || name == "" {
		return nil, NewValidationError("name is required")
	}
	autoType, ok := payload["type"].(string)
	if !ok || autoType == "" {
		return nil, NewValidationError("type is required")
	}
	input := &CreateAutomationInput{
		HomeID:      homeID,
		Name:        name,
		Type:        autoType,
	}
	if v, ok := payload["actionSetId"].(string); ok {
		input.ActionSetID = v
	}

	if charRaw, ok := payload["characteristicEvent"].(map[string]any); ok {
		ce := &CharacteristicEventAutomationInput{}
		if v, ok := charRaw["accessoryId"].(string); ok {
			ce.AccessoryID = v
		}
		if v, ok := charRaw["serviceId"].(string); ok {
			ce.ServiceID = v
		}
		if v, ok := charRaw["characteristicId"].(string); ok {
			ce.CharacteristicID = v
		}
		if v, ok := charRaw["targetValue"].(map[string]any); ok {
			ce.TargetValue = parseCharacteristicValue(v)
		}
		input.CharacteristicEvent = ce
	}

	if calRaw, ok := payload["calendarEvent"].(map[string]any); ok {
		ce := &CalendarEventAutomationInput{}
		if v, ok := calRaw["fireAt"].(string); ok {
			ce.FireAt = v
		}
		if v, ok := calRaw["recurrence"].(string); ok {
			ce.Recurrence = v
		}
		if v, ok := calRaw["timezoneOffset"].(float64); ok {
			offset := int(v)
			ce.TimezoneOffset = &offset
		}
		input.CalendarEvent = ce
	}

	if presRaw, ok := payload["presenceEvent"].(map[string]any); ok {
		pe := &PresenceEventAutomationInput{}
		if v, ok := presRaw["event"].(string); ok {
			pe.Event = v
		}
		if v, ok := presRaw["userScope"].(string); ok {
			pe.UserScope = v
		}
		input.PresenceEvent = pe
	}

	return input, nil
}

func payloadFromCreateAutomationInput(input *CreateAutomationInput) map[string]any {
	payload := map[string]any{
		"homeId": input.HomeID,
		"name":   input.Name,
		"type":   input.Type,
	}
	if input.ActionSetID != "" {
		payload["actionSetId"] = input.ActionSetID
	}
	if input.CharacteristicEvent != nil {
		payload["characteristicEvent"] = map[string]any{
			"accessoryId":      input.CharacteristicEvent.AccessoryID,
			"serviceId":        input.CharacteristicEvent.ServiceID,
			"characteristicId": input.CharacteristicEvent.CharacteristicID,
			"targetValue":      serializeCharacteristicValue(input.CharacteristicEvent.TargetValue),
		}
	}
	if input.CalendarEvent != nil {
		cal := map[string]any{
			"fireAt": input.CalendarEvent.FireAt,
		}
		if input.CalendarEvent.Recurrence != "" {
			cal["recurrence"] = input.CalendarEvent.Recurrence
		}
		if input.CalendarEvent.TimezoneOffset != nil {
			cal["timezoneOffset"] = *input.CalendarEvent.TimezoneOffset
		}
		payload["calendarEvent"] = cal
	}
	if input.PresenceEvent != nil {
		payload["presenceEvent"] = map[string]any{
			"event":     input.PresenceEvent.Event,
			"userScope": input.PresenceEvent.UserScope,
		}
	}
	return payload
}

func parseCharacteristicValue(m map[string]any) HomeCharacteristicValue {
	v := HomeCharacteristicValue{}
	if t, ok := m["type"].(string); ok {
		v.Type = t
	}
	switch v.Type {
	case "bool":
		if b, ok := m["bool"].(bool); ok {
			v.Bool = &b
		}
	case "integer":
		if i, ok := m["integer"].(float64); ok {
			i64 := int64(i)
			v.Integer = &i64
		}
	case "float":
		if f, ok := m["float"].(float64); ok {
			v.Float = &f
		}
	case "string":
		if s, ok := m["string"].(string); ok {
			v.String = &s
		}
	case "data":
		if d, ok := m["dataBase64"].(string); ok {
			v.DataBase64 = d
		}
	}
	return v
}

func serializeCharacteristicValue(v HomeCharacteristicValue) map[string]any {
	m := map[string]any{"type": v.Type}
	switch v.Type {
	case "bool":
		if v.Bool != nil {
			m["bool"] = *v.Bool
		}
	case "integer":
		if v.Integer != nil {
			m["integer"] = *v.Integer
		}
	case "float":
		if v.Float != nil {
			m["float"] = *v.Float
		}
	case "string":
		if v.String != nil {
			m["string"] = *v.String
		}
	case "data":
		if v.DataBase64 != "" {
			m["dataBase64"] = v.DataBase64
		}
	}
	return m
}
