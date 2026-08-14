package externalautomation

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/u-ai/backend/internal/nativebridge"
)

type nativeBridgeExternalAutomationClient struct {
	bridge nativebridge.Bridge
}

func NewNativeBridgeExternalAutomationClient(bridge nativebridge.Bridge) ExternalAutomationClient {
	return &nativeBridgeExternalAutomationClient{bridge: bridge}
}

func (c *nativeBridgeExternalAutomationClient) Status(ctx context.Context) (CapabilityState, error) {
	req := nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       generateRequestID(),
		Platform:        "android",
		Operation:       OperationStatus,
		Payload:         map[string]any{},
	}
	resp, err := c.bridge.Execute(ctx, req)
	if err != nil {
		return CapabilityState{}, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return CapabilityState{}, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	return decodeCapabilityState(resp.Result), nil
}

func (c *nativeBridgeExternalAutomationClient) ResolveApp(ctx context.Context, req ResolveAppRequest) ([]ResolvedApp, error) {
	bridgeReq := nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       generateRequestID(),
		Platform:        "android",
		Operation:       OperationResolveApp,
		Payload:         map[string]any{"query": req.Query},
	}
	resp, err := c.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return nil, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return nil, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	if resp.Result == nil {
		return nil, nil
	}
	rawList, ok := resp.Result["apps"].([]any)
	if !ok {
		return nil, nil
	}
	apps := make([]ResolvedApp, 0, len(rawList))
	for _, raw := range rawList {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		app := ResolvedApp{}
		app.PackageName, _ = m["packageName"].(string)
		app.Component, _ = m["component"].(string)
		app.Label, _ = m["label"].(string)
		app.Launchable, _ = m["launchable"].(bool)
		app.SystemApp, _ = m["systemApp"].(bool)
		apps = append(apps, app)
	}
	return apps, nil
}

func (c *nativeBridgeExternalAutomationClient) OpenApp(ctx context.Context, req OpenAppRequest) (ActionResult, error) {
	payload := map[string]any{"packageName": req.PackageName}
	if req.Component != "" {
		payload["component"] = req.Component
	}
	if len(req.Extras) > 0 {
		payload["extras"] = req.Extras
	}
	if req.NewTask {
		payload["newTask"] = true
	}
	bridgeReq := nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       generateRequestID(),
		Platform:        "android",
		Operation:       OperationOpenApp,
		Payload:         payload,
	}
	resp, err := c.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return ActionResult{}, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return ActionResult{}, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	return decodeActionResult(resp.Result), nil
}

func (c *nativeBridgeExternalAutomationClient) ResolveURI(ctx context.Context, req ResolveURIRequest) (ResolvedURI, error) {
	payload := map[string]any{"uri": req.URI}
	if req.Action != "" {
		payload["action"] = req.Action
	}
	bridgeReq := nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       generateRequestID(),
		Platform:        "android",
		Operation:       OperationResolveURI,
		Payload:         payload,
	}
	resp, err := c.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return ResolvedURI{}, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return ResolvedURI{}, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	result := ResolvedURI{}
	if resp.Result != nil {
		result.URI, _ = resp.Result["uri"].(string)
		result.Scheme, _ = resp.Result["scheme"].(string)
		result.Resolved, _ = resp.Result["resolved"].(bool)
	}
	return result, nil
}

func (c *nativeBridgeExternalAutomationClient) OpenURI(ctx context.Context, req OpenURIRequest) (ActionResult, error) {
	payload := map[string]any{"uri": req.URI}
	if req.PackageName != "" {
		payload["packageName"] = req.PackageName
	}
	if req.PreferExternal {
		payload["preferExternal"] = true
	}
	bridgeReq := nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       generateRequestID(),
		Platform:        "android",
		Operation:       OperationOpenURI,
		Payload:         payload,
	}
	resp, err := c.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return ActionResult{}, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return ActionResult{}, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	return decodeActionResult(resp.Result), nil
}

func (c *nativeBridgeExternalAutomationClient) OpenSettings(ctx context.Context, req OpenSettingsRequest) (ActionResult, error) {
	payload := map[string]any{"page": req.Page}
	if req.PackageName != "" {
		payload["packageName"] = req.PackageName
	}
	bridgeReq := nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       generateRequestID(),
		Platform:        "android",
		Operation:       OperationOpenSettings,
		Payload:         payload,
	}
	resp, err := c.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return ActionResult{}, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return ActionResult{}, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	return decodeActionResult(resp.Result), nil
}

func (c *nativeBridgeExternalAutomationClient) InvokeIntent(ctx context.Context, spec IntentSpec) (ActionResult, error) {
	payload := map[string]any{"action": spec.Action}
	if spec.Data != "" {
		payload["data"] = spec.Data
	}
	if spec.PackageName != "" {
		payload["packageName"] = spec.PackageName
	}
	if spec.Component != "" {
		payload["component"] = spec.Component
	}
	if len(spec.Categories) > 0 {
		payload["categories"] = spec.Categories
	}
	if len(spec.Extras) > 0 {
		payload["extras"] = spec.Extras
	}
	if spec.Mode != "" {
		payload["mode"] = spec.Mode
	}
	bridgeReq := nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       generateRequestID(),
		Platform:        "android",
		Operation:       OperationInvokeIntent,
		Payload:         payload,
	}
	resp, err := c.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return ActionResult{}, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return ActionResult{}, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	return decodeActionResult(resp.Result), nil
}

func (c *nativeBridgeExternalAutomationClient) Foreground(ctx context.Context) (ForegroundState, error) {
	req := nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       generateRequestID(),
		Platform:        "android",
		Operation:       OperationForeground,
		Payload:         map[string]any{},
	}
	resp, err := c.bridge.Execute(ctx, req)
	if err != nil {
		return ForegroundState{}, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return ForegroundState{}, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	result := ForegroundState{}
	if resp.Result != nil {
		result.PackageName, _ = resp.Result["packageName"].(string)
		result.Component, _ = resp.Result["component"].(string)
		result.Label, _ = resp.Result["label"].(string)
		result.DisplayID = toInt(resp.Result["displayId"])
		result.ObservedAt = toInt64(resp.Result["observedAt"])
		result.Source, _ = resp.Result["source"].(string)
		result.Confidence, _ = resp.Result["confidence"].(string)
	}
	return result, nil
}

func (c *nativeBridgeExternalAutomationClient) WaitForeground(ctx context.Context, req WaitForegroundRequest) (ForegroundState, error) {
	payload := map[string]any{}
	if req.PackageName != "" {
		payload["packageName"] = req.PackageName
	}
	if req.Component != "" {
		payload["component"] = req.Component
	}
	if req.TimeoutMS != 0 {
		payload["timeoutMs"] = req.TimeoutMS
	}
	bridgeReq := nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       generateRequestID(),
		Platform:        "android",
		Operation:       OperationWaitForeground,
		Payload:         payload,
	}
	resp, err := c.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return ForegroundState{}, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, err.Error())
	}
	if resp.Status != "success" {
		return ForegroundState{}, newAutomationError(AUTOMATION_NATIVE_HOST_UNAVAILABLE, errorMessage(resp))
	}
	result := ForegroundState{}
	if resp.Result != nil {
		result.PackageName, _ = resp.Result["packageName"].(string)
		result.Component, _ = resp.Result["component"].(string)
		result.Label, _ = resp.Result["label"].(string)
		result.DisplayID = toInt(resp.Result["displayId"])
		result.ObservedAt = toInt64(resp.Result["observedAt"])
		result.Source, _ = resp.Result["source"].(string)
		result.Confidence, _ = resp.Result["confidence"].(string)
	}
	return result, nil
}

func decodeCapabilityState(m map[string]any) CapabilityState {
	state := CapabilityState{}
	if m == nil {
		return state
	}
	state.Supported, _ = m["supported"].(bool)
	state.CanResolveApps, _ = m["canResolveApps"].(bool)
	state.CanLaunchApps, _ = m["canLaunchApps"].(bool)
	state.CanResolveURI, _ = m["canResolveUri"].(bool)
	state.CanOpenURI, _ = m["canOpenUri"].(bool)
	state.CanOpenSettings, _ = m["canOpenSettings"].(bool)
	state.CanInvokeIntent, _ = m["canInvokeIntent"].(bool)
	state.CanInspectForeground, _ = m["canInspectForeground"].(bool)
	state.CanWaitForeground, _ = m["canWaitForeground"].(bool)
	state.State, _ = m["state"].(string)
	state.Reason, _ = m["reason"].(string)
	return state
}

func decodeActionResult(m map[string]any) ActionResult {
	r := ActionResult{}
	if m == nil {
		return r
	}
	r.Success, _ = m["success"].(bool)
	r.Operation, _ = m["operation"].(string)
	r.TargetPackage, _ = m["targetPackage"].(string)
	r.TargetComponent, _ = m["targetComponent"].(string)
	r.Resolved, _ = m["resolved"].(bool)
	r.Started, _ = m["started"].(bool)
	r.UserActionRequired, _ = m["userActionRequired"].(bool)
	r.Timestamp = toInt64(m["timestamp"])
	return r
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

func generateRequestID() string {
	requestIDCounter++
	return fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), requestIDCounter)
}
