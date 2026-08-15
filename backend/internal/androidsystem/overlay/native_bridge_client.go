package overlay

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/u-ai/backend/internal/nativebridge"
)

type nativeBridgeOverlayClient struct {
	bridge nativebridge.Bridge
}

func NewNativeBridgeOverlayClient(bridge nativebridge.Bridge) OverlayClient {
	return &nativeBridgeOverlayClient{bridge: bridge}
}

func (c *nativeBridgeOverlayClient) Status(ctx context.Context) (CapabilityState, error) {
	req := nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       generateRequestId(),
		Platform:        "android",
		Operation:       OperationStatus,
		Payload:         map[string]any{},
	}
	resp, err := c.bridge.Execute(ctx, req)
	if err != nil {
		return CapabilityState{}, newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return CapabilityState{}, newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	return decodeCapabilityState(resp.Result), nil
}

func (c *nativeBridgeOverlayClient) RequestPermission(ctx context.Context) (PermissionResult, error) {
	req := nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       generateRequestId(),
		Platform:        "android",
		Operation:       OperationPermissionRequest,
		Payload:         map[string]any{},
	}
	resp, err := c.bridge.Execute(ctx, req)
	if err != nil {
		return PermissionResult{}, newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return PermissionResult{}, newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	result := PermissionResult{}
	if resp.Result != nil {
		result.Opened, _ = resp.Result["opened"].(bool)
		result.UserActionRequired, _ = resp.Result["userActionRequired"].(bool)
		result.PermissionGranted, _ = resp.Result["permissionGranted"].(bool)
	}
	return result, nil
}

func (c *nativeBridgeOverlayClient) Create(ctx context.Context, req CreateRequest) (OverlayInstance, error) {
	payload := map[string]any{"kind": req.Kind}
	if req.Content != nil {
		payload["content"] = req.Content
	}
	appendOverlayGeometryPayload(payload, req.X, req.Y, req.Width, req.Height, req.Gravity, req.Focusable, req.Touchable, req.Draggable, req.TTLms)
	bridgeReq := nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       generateRequestId(),
		Platform:        "android",
		Operation:       OperationCreate,
		Payload:         payload,
	}
	resp, err := c.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return OverlayInstance{}, newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return OverlayInstance{}, newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	return decodeOverlayInstance(resp.Result), nil
}

func (c *nativeBridgeOverlayClient) Update(ctx context.Context, req UpdateRequest) (OverlayInstance, error) {
	payload := map[string]any{"overlayId": req.OverlayID}
	if req.Content != nil {
		payload["content"] = req.Content
	}
	appendOverlayGeometryPayload(payload, req.X, req.Y, req.Width, req.Height, req.Gravity, req.Focusable, req.Touchable, req.Draggable, req.TTLms)
	bridgeReq := nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       generateRequestId(),
		Platform:        "android",
		Operation:       OperationUpdate,
		Payload:         payload,
	}
	resp, err := c.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return OverlayInstance{}, newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return OverlayInstance{}, newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	return decodeOverlayInstance(resp.Result), nil
}

func appendOverlayGeometryPayload(
	payload map[string]any,
	x, y, width, height *int,
	gravity string,
	focusable, touchable, draggable *bool,
	ttlMs *int64,
) {
	if x != nil {
		payload["x"] = *x
	}
	if y != nil {
		payload["y"] = *y
	}
	if width != nil {
		payload["width"] = *width
	}
	if height != nil {
		payload["height"] = *height
	}
	if gravity != "" {
		payload["gravity"] = gravity
	}
	if focusable != nil {
		payload["focusable"] = *focusable
	}
	if touchable != nil {
		payload["touchable"] = *touchable
	}
	if draggable != nil {
		payload["draggable"] = *draggable
	}
	if ttlMs != nil {
		payload["ttlMs"] = *ttlMs
	}
}

func (c *nativeBridgeOverlayClient) Show(ctx context.Context, overlayID string) (OverlayInstance, error) {
	bridgeReq := nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       generateRequestId(),
		Platform:        "android",
		Operation:       OperationShow,
		Payload:         map[string]any{"overlayId": overlayID},
	}
	resp, err := c.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return OverlayInstance{}, newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return OverlayInstance{}, newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	return decodeOverlayInstance(resp.Result), nil
}

func (c *nativeBridgeOverlayClient) Hide(ctx context.Context, overlayID string) (OverlayInstance, error) {
	bridgeReq := nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       generateRequestId(),
		Platform:        "android",
		Operation:       OperationHide,
		Payload:         map[string]any{"overlayId": overlayID},
	}
	resp, err := c.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return OverlayInstance{}, newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return OverlayInstance{}, newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	return decodeOverlayInstance(resp.Result), nil
}

func (c *nativeBridgeOverlayClient) Close(ctx context.Context, overlayID string) error {
	bridgeReq := nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       generateRequestId(),
		Platform:        "android",
		Operation:       OperationClose,
		Payload:         map[string]any{"overlayId": overlayID},
	}
	resp, err := c.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	return nil
}

func (c *nativeBridgeOverlayClient) List(ctx context.Context) ([]OverlayInstance, error) {
	req := nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       generateRequestId(),
		Platform:        "android",
		Operation:       OperationList,
		Payload:         map[string]any{},
	}
	resp, err := c.bridge.Execute(ctx, req)
	if err != nil {
		return nil, newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return nil, newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	if resp.Result == nil {
		return nil, nil
	}
	rawList, ok := resp.Result["overlays"].([]any)
	if !ok {
		return nil, nil
	}
	instances := make([]OverlayInstance, 0, len(rawList))
	for _, raw := range rawList {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		instances = append(instances, decodeOverlayInstance(m))
	}
	return instances, nil
}

func (c *nativeBridgeOverlayClient) CloseAll(ctx context.Context) (int, error) {
	req := nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       generateRequestId(),
		Platform:        "android",
		Operation:       OperationCloseAll,
		Payload:         map[string]any{},
	}
	resp, err := c.bridge.Execute(ctx, req)
	if err != nil {
		return 0, newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return 0, newOverlayError(OVERLAY_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	if resp.Result == nil {
		return 0, nil
	}
	return toInt(resp.Result["closedCount"]), nil
}

func decodeCapabilityState(m map[string]any) CapabilityState {
	state := CapabilityState{}
	if m == nil {
		return state
	}
	state.Supported, _ = m["supported"].(bool)
	state.PermissionRequired, _ = m["permissionRequired"].(bool)
	state.PermissionGranted, _ = m["permissionGranted"].(bool)
	state.NativeHostReady, _ = m["nativeHostReady"].(bool)
	state.CanCreate, _ = m["canCreate"].(bool)
	state.CanUpdate, _ = m["canUpdate"].(bool)
	state.CanInteract, _ = m["canInteract"].(bool)
	state.ActiveCount = toInt(m["activeCount"])
	state.UserActionRequired, _ = m["userActionRequired"].(bool)
	state.State, _ = m["state"].(string)
	return state
}

func decodeOverlayInstance(m map[string]any) OverlayInstance {
	inst := OverlayInstance{}
	if m == nil {
		return inst
	}
	inst.ID, _ = m["overlayId"].(string)
	inst.Kind, _ = m["kind"].(string)
	inst.Visible, _ = m["visible"].(bool)
	inst.Focusable, _ = m["focusable"].(bool)
	inst.Touchable, _ = m["touchable"].(bool)
	inst.X = toInt(m["x"])
	inst.Y = toInt(m["y"])
	inst.Width = toInt(m["width"])
	inst.Height = toInt(m["height"])
	inst.Gravity, _ = m["gravity"].(string)
	inst.DisplayID = toInt(m["displayId"])
	inst.CreatedAt = toInt64(m["createdAt"])
	inst.UpdatedAt = toInt64(m["updatedAt"])
	return inst
}

func errorMessage(resp nativebridge.Response) string {
	if resp.Error != nil && resp.Error.Message != "" {
		return resp.Error.Message
	}
	return "android native host returned status=" + resp.Status
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	}
	return 0
}

var requestIDCounter uint64

func generateRequestId() string {
	requestIDCounter++
	return fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), requestIDCounter)
}
