package bluetooth

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/nativebridge"
)

type BluetoothHandler struct {
	bridge nativebridge.Bridge
}

func NewBluetoothHandler(bridge nativebridge.Bridge) *BluetoothHandler {
	return &BluetoothHandler{bridge: bridge}
}

func (h *BluetoothHandler) Execute(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationScanStart:
		return h.handleScanStart(ctx, request)
	case OperationScanStop:
		return h.handleScanStop(ctx, request)
	case OperationPeripheralGet:
		return h.handlePeripheralGet(ctx, request)
	case OperationPeripheralConnected:
		return h.handlePeripheralConnected(ctx, request)
	case OperationConnect:
		return h.handleConnect(ctx, request)
	case OperationDisconnect:
		return h.handleDisconnect(ctx, request)
	case OperationServicesDiscover:
		return h.handleServicesDiscover(ctx, request)
	case OperationCharacteristicsDiscover:
		return h.handleCharacteristicsDiscover(ctx, request)
	case OperationDescriptorsDiscover:
		return h.handleDescriptorsDiscover(ctx, request)
	case OperationCharacteristicRead:
		return h.handleCharacteristicRead(ctx, request)
	case OperationCharacteristicWrite:
		return h.handleCharacteristicWrite(ctx, request)
	case OperationCharacteristicSubscribe:
		return h.handleCharacteristicSubscribe(ctx, request)
	case OperationCharacteristicUnsubscribe:
		return h.handleCharacteristicUnsubscribe(ctx, request)
	case OperationDescriptorRead:
		return h.handleDescriptorRead(ctx, request)
	case OperationDescriptorWrite:
		return h.handleDescriptorWrite(ctx, request)
	case OperationRSSIRead:
		return h.handleRSSIRead(ctx, request)
	case OperationPeripheralRoleStart:
		return h.handlePeripheralRoleStart(ctx, request)
	case OperationPeripheralRoleStop:
		return h.handlePeripheralRoleStop(ctx, request)
	default:
		return NewBluetoothError(request, nativebridge.ErrOperationNotSupported, fmt.Sprintf("unsupported operation: %s", request.Operation))
	}
}

func (h *BluetoothHandler) bridgeCall(ctx context.Context, request nativebridge.Request, operation string, payload map[string]any) nativebridge.Response {
	if h.bridge == nil {
		return NewBluetoothError(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
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
			done <- NewBluetoothError(request, ErrOutcomeUnknown, err.Error())
			return
		}
		done <- resp
	}()
	select {
	case <-ctx.Done():
		return NewBluetoothError(request, ErrTimeout, operation+" cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *BluetoothHandler) handleStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationStatus, map[string]any{})
}

func (h *BluetoothHandler) handleScanStart(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	payload := map[string]any{}

	if serviceUUIDs, ok := request.Payload["serviceUuids"].([]any); ok {
		uuids := make([]string, 0, len(serviceUUIDs))
		for _, u := range serviceUUIDs {
			if s, ok := u.(string); ok && IsValidUUID(s) {
				uuids = append(uuids, NormalizeUUID(s))
			}
		}
		payload["serviceUuids"] = uuids
	}

	if durationMs, ok := request.Payload["durationMs"].(float64); ok {
		payload["durationMs"] = ClampScanDuration(int(durationMs))
	} else {
		payload["durationMs"] = DefaultScanDurationMs
	}

	if allowDuplicates, ok := request.Payload["allowDuplicates"].(bool); ok {
		payload["allowDuplicates"] = allowDuplicates
	}

	if namePrefix, ok := request.Payload["namePrefix"].(string); ok && namePrefix != "" {
		payload["namePrefix"] = namePrefix
	}

	if rssiMin, ok := request.Payload["rssiMin"].(float64); ok {
		v := int(rssiMin)
		payload["rssiMin"] = v
	}

	if includeManufacturerData, ok := request.Payload["includeManufacturerData"].(bool); ok {
		payload["includeManufacturerData"] = includeManufacturerData
	}

	if maxResults, ok := request.Payload["maxResults"].(float64); ok {
		payload["maxResults"] = ClampMaxResults(int(maxResults))
	} else {
		payload["maxResults"] = DefaultMaxResults
	}

	return h.bridgeCall(ctx, request, OperationScanStart, payload)
}

func (h *BluetoothHandler) handleScanStop(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationScanStop, map[string]any{})
}

func (h *BluetoothHandler) handlePeripheralGet(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	peripheralID, ok := request.Payload["peripheralId"].(string)
	if !ok || peripheralID == "" {
		return NewBluetoothError(request, ErrInvalidPeripheralID, "missing required field: peripheralId")
	}
	return h.bridgeCall(ctx, request, OperationPeripheralGet, map[string]any{
		"peripheralId": peripheralID,
	})
}

func (h *BluetoothHandler) handlePeripheralConnected(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationPeripheralConnected, map[string]any{})
}

func (h *BluetoothHandler) handleConnect(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	peripheralID, ok := request.Payload["peripheralId"].(string)
	if !ok || peripheralID == "" {
		return NewBluetoothError(request, ErrInvalidPeripheralID, "missing required field: peripheralId")
	}

	payload := map[string]any{
		"peripheralId": peripheralID,
	}

	if timeoutMs, ok := request.Payload["timeoutMs"].(float64); ok {
		payload["timeoutMs"] = ClampConnectTimeout(int(timeoutMs))
	} else {
		payload["timeoutMs"] = DefaultConnectTimeoutMs
	}

	if autoReconnect, ok := request.Payload["autoReconnect"].(bool); ok {
		payload["autoReconnect"] = autoReconnect
	}

	if serviceUUIDs, ok := request.Payload["expectedServiceUuids"].([]any); ok {
		uuids := make([]string, 0, len(serviceUUIDs))
		for _, u := range serviceUUIDs {
			if s, ok := u.(string); ok && IsValidUUID(s) {
				uuids = append(uuids, NormalizeUUID(s))
			}
		}
		payload["expectedServiceUuids"] = uuids
	}

	return h.bridgeCall(ctx, request, OperationConnect, payload)
}

func (h *BluetoothHandler) handleDisconnect(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	peripheralID, ok := request.Payload["peripheralId"].(string)
	if !ok || peripheralID == "" {
		return NewBluetoothError(request, ErrInvalidPeripheralID, "missing required field: peripheralId")
	}
	return h.bridgeCall(ctx, request, OperationDisconnect, map[string]any{
		"peripheralId": peripheralID,
	})
}

func (h *BluetoothHandler) handleServicesDiscover(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	peripheralID, ok := request.Payload["peripheralId"].(string)
	if !ok || peripheralID == "" {
		return NewBluetoothError(request, ErrInvalidPeripheralID, "missing required field: peripheralId")
	}

	payload := map[string]any{
		"peripheralId": peripheralID,
	}

	if serviceUUIDs, ok := request.Payload["serviceUuids"].([]any); ok {
		uuids := make([]string, 0, len(serviceUUIDs))
		for _, u := range serviceUUIDs {
			if s, ok := u.(string); ok && IsValidUUID(s) {
				uuids = append(uuids, NormalizeUUID(s))
			}
		}
		payload["serviceUuids"] = uuids
	}

	return h.bridgeCall(ctx, request, OperationServicesDiscover, payload)
}

func (h *BluetoothHandler) handleCharacteristicsDiscover(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	peripheralID, ok := request.Payload["peripheralId"].(string)
	if !ok || peripheralID == "" {
		return NewBluetoothError(request, ErrInvalidPeripheralID, "missing required field: peripheralId")
	}

	serviceRef, ok := request.Payload["serviceRef"].(string)
	if !ok || serviceRef == "" {
		return NewBluetoothError(request, ErrServiceNotFound, "missing required field: serviceRef")
	}

	payload := map[string]any{
		"peripheralId": peripheralID,
		"serviceRef":   serviceRef,
	}

	if charUUIDs, ok := request.Payload["characteristicUuids"].([]any); ok {
		uuids := make([]string, 0, len(charUUIDs))
		for _, u := range charUUIDs {
			if s, ok := u.(string); ok && IsValidUUID(s) {
				uuids = append(uuids, NormalizeUUID(s))
			}
		}
		payload["characteristicUuids"] = uuids
	}

	return h.bridgeCall(ctx, request, OperationCharacteristicsDiscover, payload)
}

func (h *BluetoothHandler) handleDescriptorsDiscover(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	peripheralID, ok := request.Payload["peripheralId"].(string)
	if !ok || peripheralID == "" {
		return NewBluetoothError(request, ErrInvalidPeripheralID, "missing required field: peripheralId")
	}

	charRef, ok := request.Payload["characteristicRef"].(string)
	if !ok || charRef == "" {
		return NewBluetoothError(request, ErrCharacteristicNotFound, "missing required field: characteristicRef")
	}

	return h.bridgeCall(ctx, request, OperationDescriptorsDiscover, map[string]any{
		"peripheralId":      peripheralID,
		"characteristicRef": charRef,
	})
}

func (h *BluetoothHandler) handleCharacteristicRead(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	peripheralID, ok := request.Payload["peripheralId"].(string)
	if !ok || peripheralID == "" {
		return NewBluetoothError(request, ErrInvalidPeripheralID, "missing required field: peripheralId")
	}

	charRef, ok := request.Payload["characteristicRef"].(string)
	if !ok || charRef == "" {
		return NewBluetoothError(request, ErrCharacteristicNotFound, "missing required field: characteristicRef")
	}

	return h.bridgeCall(ctx, request, OperationCharacteristicRead, map[string]any{
		"peripheralId":      peripheralID,
		"characteristicRef": charRef,
	})
}

func (h *BluetoothHandler) handleCharacteristicWrite(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	charRef, ok := request.Payload["characteristicRef"].(string)
	if !ok || charRef == "" {
		return NewBluetoothError(request, ErrCharacteristicNotFound, "missing required field: characteristicRef")
	}

	valueRaw, ok := request.Payload["value"].(map[string]any)
	if !ok || valueRaw == nil {
		return NewBluetoothError(request, ErrWriteValueInvalid, "missing required field: value")
	}

	value := parseBluetoothValue(valueRaw)
	if value.Encoding != "base64" && value.Encoding != "hex" {
		return NewBluetoothError(request, ErrValueEncodingInvalid, "value encoding must be base64 or hex")
	}

	payload := map[string]any{
		"characteristicRef": charRef,
		"value":             value,
	}

	if mode, ok := request.Payload["mode"].(string); ok && (mode == "with_response" || mode == "without_response") {
		payload["mode"] = mode
	} else {
		payload["mode"] = "with_response"
	}

	if timeoutMs, ok := request.Payload["timeoutMs"].(float64); ok {
		payload["timeoutMs"] = ClampConnectTimeout(int(timeoutMs))
	} else {
		payload["timeoutMs"] = DefaultConnectTimeoutMs
	}

	return h.bridgeCall(ctx, request, OperationCharacteristicWrite, payload)
}

func (h *BluetoothHandler) handleCharacteristicSubscribe(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	peripheralID, ok := request.Payload["peripheralId"].(string)
	if !ok || peripheralID == "" {
		return NewBluetoothError(request, ErrInvalidPeripheralID, "missing required field: peripheralId")
	}

	charRef, ok := request.Payload["characteristicRef"].(string)
	if !ok || charRef == "" {
		return NewBluetoothError(request, ErrCharacteristicNotFound, "missing required field: characteristicRef")
	}

	return h.bridgeCall(ctx, request, OperationCharacteristicSubscribe, map[string]any{
		"peripheralId":      peripheralID,
		"characteristicRef": charRef,
	})
}

func (h *BluetoothHandler) handleCharacteristicUnsubscribe(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	peripheralID, ok := request.Payload["peripheralId"].(string)
	if !ok || peripheralID == "" {
		return NewBluetoothError(request, ErrInvalidPeripheralID, "missing required field: peripheralId")
	}

	charRef, ok := request.Payload["characteristicRef"].(string)
	if !ok || charRef == "" {
		return NewBluetoothError(request, ErrCharacteristicNotFound, "missing required field: characteristicRef")
	}

	return h.bridgeCall(ctx, request, OperationCharacteristicUnsubscribe, map[string]any{
		"peripheralId":      peripheralID,
		"characteristicRef": charRef,
	})
}

func (h *BluetoothHandler) handleDescriptorRead(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	peripheralID, ok := request.Payload["peripheralId"].(string)
	if !ok || peripheralID == "" {
		return NewBluetoothError(request, ErrInvalidPeripheralID, "missing required field: peripheralId")
	}

	descriptorRef, ok := request.Payload["descriptorRef"].(string)
	if !ok || descriptorRef == "" {
		return NewBluetoothError(request, ErrDescriptorNotFound, "missing required field: descriptorRef")
	}

	return h.bridgeCall(ctx, request, OperationDescriptorRead, map[string]any{
		"peripheralId":  peripheralID,
		"descriptorRef": descriptorRef,
	})
}

func (h *BluetoothHandler) handleDescriptorWrite(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	peripheralID, ok := request.Payload["peripheralId"].(string)
	if !ok || peripheralID == "" {
		return NewBluetoothError(request, ErrInvalidPeripheralID, "missing required field: peripheralId")
	}

	descriptorRef, ok := request.Payload["descriptorRef"].(string)
	if !ok || descriptorRef == "" {
		return NewBluetoothError(request, ErrDescriptorNotFound, "missing required field: descriptorRef")
	}

	valueRaw, ok := request.Payload["value"].(map[string]any)
	if !ok || valueRaw == nil {
		return NewBluetoothError(request, ErrWriteValueInvalid, "missing required field: value")
	}

	value := parseBluetoothValue(valueRaw)
	if value.Encoding != "base64" && value.Encoding != "hex" {
		return NewBluetoothError(request, ErrValueEncodingInvalid, "value encoding must be base64 or hex")
	}

	return h.bridgeCall(ctx, request, OperationDescriptorWrite, map[string]any{
		"peripheralId":  peripheralID,
		"descriptorRef": descriptorRef,
		"value":         value,
	})
}

func (h *BluetoothHandler) handleRSSIRead(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	peripheralID, ok := request.Payload["peripheralId"].(string)
	if !ok || peripheralID == "" {
		return NewBluetoothError(request, ErrInvalidPeripheralID, "missing required field: peripheralId")
	}

	return h.bridgeCall(ctx, request, OperationRSSIRead, map[string]any{
		"peripheralId": peripheralID,
	})
}

func (h *BluetoothHandler) handlePeripheralRoleStart(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	payload := map[string]any{}

	if serviceUUIDs, ok := request.Payload["serviceUuids"].([]any); ok {
		uuids := make([]string, 0, len(serviceUUIDs))
		for _, u := range serviceUUIDs {
			if s, ok := u.(string); ok && IsValidUUID(s) {
				uuids = append(uuids, NormalizeUUID(s))
			}
		}
		payload["serviceUuids"] = uuids
	}

	if localName, ok := request.Payload["localName"].(string); ok {
		payload["localName"] = localName
	}

	return h.bridgeCall(ctx, request, OperationPeripheralRoleStart, payload)
}

func (h *BluetoothHandler) handlePeripheralRoleStop(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationPeripheralRoleStop, map[string]any{})
}

func parseBluetoothValue(raw map[string]any) BluetoothValueInput {
	v := BluetoothValueInput{}
	if enc, ok := raw["encoding"].(string); ok {
		v.Encoding = enc
	}
	if b64, ok := raw["base64"].(string); ok {
		v.Base64 = b64
	}
	if hex, ok := raw["hex"].(string); ok {
		v.Hex = hex
	}
	return v
}
