package externalautomation

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/androidsystem"
)

type ExternalAutomationHandler struct {
	client ExternalAutomationClient
	policy Policy
	mu     sync.RWMutex
}

type ExternalAutomationClient interface {
	Status(ctx context.Context) (CapabilityState, error)
	ResolveApp(ctx context.Context, req ResolveAppRequest) ([]ResolvedApp, error)
	OpenApp(ctx context.Context, req OpenAppRequest) (ActionResult, error)
	ResolveURI(ctx context.Context, req ResolveURIRequest) (ResolvedURI, error)
	OpenURI(ctx context.Context, req OpenURIRequest) (ActionResult, error)
	OpenSettings(ctx context.Context, req OpenSettingsRequest) (ActionResult, error)
	InvokeIntent(ctx context.Context, spec IntentSpec) (ActionResult, error)
	Foreground(ctx context.Context) (ForegroundState, error)
	WaitForeground(ctx context.Context, req WaitForegroundRequest) (ForegroundState, error)
}

func NewExternalAutomationHandler(client ExternalAutomationClient) *ExternalAutomationHandler {
	if client == nil {
		client = NewBlockedExternalAutomationClient()
	}
	return &ExternalAutomationHandler{
		client: client,
		policy: DefaultPolicy(),
	}
}

func (h *ExternalAutomationHandler) Execute(ctx context.Context, request androidsystem.NotificationRequest) androidsystem.NotificationResponse {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationResolveApp:
		return h.handleResolveApp(ctx, request)
	case OperationOpenApp:
		return h.handleOpenApp(ctx, request)
	case OperationResolveURI:
		return h.handleResolveURI(ctx, request)
	case OperationOpenURI:
		return h.handleOpenURI(ctx, request)
	case OperationOpenSettings:
		return h.handleOpenSettings(ctx, request)
	case OperationInvokeIntent:
		return h.handleInvokeIntent(ctx, request)
	case OperationForeground:
		return h.handleForeground(ctx, request)
	case OperationWaitForeground:
		return h.handleWaitForeground(ctx, request)
	default:
		return androidsystem.NotificationResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.NotificationError{
				Code:    AUTOMATION_UNSUPPORTED,
				Message: "unknown external automation operation: " + request.Operation,
			},
		}
	}
}

func (h *ExternalAutomationHandler) handleStatus(ctx context.Context, request androidsystem.NotificationRequest) androidsystem.NotificationResponse {
	state, err := h.client.Status(ctx)
	if err != nil {
		return h.errorResponse(request, err)
	}
	return androidsystem.NotificationResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"supported":            state.Supported,
			"canResolveApps":       state.CanResolveApps,
			"canLaunchApps":        state.CanLaunchApps,
			"canResolveUri":        state.CanResolveURI,
			"canOpenUri":           state.CanOpenURI,
			"canOpenSettings":      state.CanOpenSettings,
			"canInvokeIntent":      state.CanInvokeIntent,
			"canInspectForeground": state.CanInspectForeground,
			"canWaitForeground":    state.CanWaitForeground,
			"state":                state.State,
			"reason":               state.Reason,
		},
	}
}

func (h *ExternalAutomationHandler) handleResolveApp(ctx context.Context, request androidsystem.NotificationRequest) androidsystem.NotificationResponse {
	req := h.parseResolveAppRequest(request.Payload)
	if err := h.policy.ValidateResolveApp(req); err != nil {
		return h.policyErrorResponse(request, err)
	}

	apps, err := h.client.ResolveApp(ctx, req)
	if err != nil {
		return h.errorResponse(request, err)
	}

	return androidsystem.NotificationResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"apps":  apps,
			"count": len(apps),
		},
	}
}

func (h *ExternalAutomationHandler) handleOpenApp(ctx context.Context, request androidsystem.NotificationRequest) androidsystem.NotificationResponse {
	req := h.parseOpenAppRequest(request.Payload)
	if err := h.policy.ValidateOpenApp(req); err != nil {
		return h.policyErrorResponse(request, err)
	}

	result, err := h.client.OpenApp(ctx, req)
	if err != nil {
		return h.errorResponse(request, err)
	}

	return androidsystem.NotificationResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"success":         result.Success,
			"operation":       result.Operation,
			"targetPackage":   result.TargetPackage,
			"targetComponent": result.TargetComponent,
			"resolved":        result.Resolved,
			"started":         result.Started,
			"userActionRequired": result.UserActionRequired,
			"timestamp":       result.Timestamp,
		},
	}
}

func (h *ExternalAutomationHandler) handleResolveURI(ctx context.Context, request androidsystem.NotificationRequest) androidsystem.NotificationResponse {
	req := h.parseResolveURIRequest(request.Payload)
	if err := h.policy.ValidateResolveURI(req); err != nil {
		return h.policyErrorResponse(request, err)
	}

	resolved, err := h.client.ResolveURI(ctx, req)
	if err != nil {
		return h.errorResponse(request, err)
	}

	return androidsystem.NotificationResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"uri":            resolved.URI,
			"scheme":         resolved.Scheme,
			"resolved":       resolved.Resolved,
			"handlers":       resolved.Handlers,
			"defaultHandler": resolved.DefaultHandler,
		},
	}
}

func (h *ExternalAutomationHandler) handleOpenURI(ctx context.Context, request androidsystem.NotificationRequest) androidsystem.NotificationResponse {
	req := h.parseOpenURIRequest(request.Payload)
	if err := h.policy.ValidateOpenURI(req); err != nil {
		return h.policyErrorResponse(request, err)
	}

	result, err := h.client.OpenURI(ctx, req)
	if err != nil {
		return h.errorResponse(request, err)
	}

	return androidsystem.NotificationResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"success":         result.Success,
			"operation":       result.Operation,
			"targetPackage":   result.TargetPackage,
			"targetComponent": result.TargetComponent,
			"userActionRequired": result.UserActionRequired,
			"timestamp":       result.Timestamp,
		},
	}
}

func (h *ExternalAutomationHandler) handleOpenSettings(ctx context.Context, request androidsystem.NotificationRequest) androidsystem.NotificationResponse {
	req := h.parseOpenSettingsRequest(request.Payload)
	if err := h.policy.ValidateOpenSettings(req); err != nil {
		return h.policyErrorResponse(request, err)
	}

	result, err := h.client.OpenSettings(ctx, req)
	if err != nil {
		return h.errorResponse(request, err)
	}

	return androidsystem.NotificationResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"success":           result.Success,
			"operation":         result.Operation,
			"userActionRequired": result.UserActionRequired,
			"timestamp":         result.Timestamp,
		},
	}
}

func (h *ExternalAutomationHandler) handleInvokeIntent(ctx context.Context, request androidsystem.NotificationRequest) androidsystem.NotificationResponse {
	spec := h.parseIntentSpec(request.Payload)
	if err := h.policy.ValidateIntentSpec(spec); err != nil {
		return h.policyErrorResponse(request, err)
	}

	result, err := h.client.InvokeIntent(ctx, spec)
	if err != nil {
		return h.errorResponse(request, err)
	}

	return androidsystem.NotificationResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"success":         result.Success,
			"operation":       result.Operation,
			"targetPackage":   result.TargetPackage,
			"targetComponent": result.TargetComponent,
			"userActionRequired": result.UserActionRequired,
			"timestamp":       result.Timestamp,
		},
	}
}

func (h *ExternalAutomationHandler) handleForeground(ctx context.Context, request androidsystem.NotificationRequest) androidsystem.NotificationResponse {
	state, err := h.client.Foreground(ctx)
	if err != nil {
		return h.errorResponse(request, err)
	}

	return androidsystem.NotificationResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"packageName": state.PackageName,
			"component":   state.Component,
			"label":       state.Label,
			"displayId":   state.DisplayID,
			"observedAt":  state.ObservedAt,
			"source":      state.Source,
			"confidence":  state.Confidence,
		},
	}
}

func (h *ExternalAutomationHandler) handleWaitForeground(ctx context.Context, request androidsystem.NotificationRequest) androidsystem.NotificationResponse {
	req := h.parseWaitForegroundRequest(request.Payload)
	if err := h.policy.ValidateWaitForeground(req); err != nil {
		return h.policyErrorResponse(request, err)
	}

	state, err := h.client.WaitForeground(ctx, req)
	if err != nil {
		return h.errorResponse(request, err)
	}

	return androidsystem.NotificationResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"packageName": state.PackageName,
			"component":   state.Component,
			"label":       state.Label,
			"observedAt":  state.ObservedAt,
			"source":      state.Source,
			"confidence":  state.Confidence,
		},
	}
}

func (h *ExternalAutomationHandler) errorResponse(request androidsystem.NotificationRequest, err error) androidsystem.NotificationResponse {
	var ae *automationError
	if errors.As(err, &ae) {
		return androidsystem.NotificationResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.NotificationError{
				Code:    ae.code,
				Message: ae.message,
			},
		}
	}
	return androidsystem.NotificationResponse{
		RequestID: request.RequestID,
		Status:    "error",
		Error: &androidsystem.NotificationError{
			Code:    AUTOMATION_NATIVE_HOST_UNAVAILABLE,
			Message: err.Error(),
		},
	}
}

func (h *ExternalAutomationHandler) policyErrorResponse(request androidsystem.NotificationRequest, err error) androidsystem.NotificationResponse {
	var ae *automationError
	if errors.As(err, &ae) {
		return androidsystem.NotificationResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.NotificationError{
				Code:    ae.code,
				Message: ae.message,
			},
		}
	}
	return androidsystem.NotificationResponse{
		RequestID: request.RequestID,
		Status:    "error",
		Error: &androidsystem.NotificationError{
			Code:    AUTOMATION_INVALID_REQUEST,
			Message: err.Error(),
		},
	}
}

func (h *ExternalAutomationHandler) parseResolveAppRequest(payload map[string]any) ResolveAppRequest {
	return ResolveAppRequest{
		Query: h.toString(payload, "query"),
	}
}

func (h *ExternalAutomationHandler) parseOpenAppRequest(payload map[string]any) OpenAppRequest {
	return OpenAppRequest{
		PackageName: h.toString(payload, "packageName"),
		Component:   h.toString(payload, "component"),
		Extras:      h.toExtrasMap(payload, "extras"),
		NewTask:     h.toBool(payload, "newTask"),
	}
}

func (h *ExternalAutomationHandler) parseResolveURIRequest(payload map[string]any) ResolveURIRequest {
	return ResolveURIRequest{
		URI:    h.toString(payload, "uri"),
		Action: h.toString(payload, "action"),
	}
}

func (h *ExternalAutomationHandler) parseOpenURIRequest(payload map[string]any) OpenURIRequest {
	return OpenURIRequest{
		URI:            h.toString(payload, "uri"),
		PackageName:    h.toString(payload, "packageName"),
		PreferExternal: h.toBool(payload, "preferExternal"),
	}
}

func (h *ExternalAutomationHandler) parseOpenSettingsRequest(payload map[string]any) OpenSettingsRequest {
	return OpenSettingsRequest{
		Page:        h.toString(payload, "page"),
		PackageName: h.toString(payload, "packageName"),
	}
}

func (h *ExternalAutomationHandler) parseIntentSpec(payload map[string]any) IntentSpec {
	spec := IntentSpec{
		Action:      h.toString(payload, "action"),
		Data:        h.toString(payload, "data"),
		PackageName: h.toString(payload, "packageName"),
		Component:   h.toString(payload, "component"),
		Extras:      h.toExtrasMap(payload, "extras"),
		Mode:        h.toString(payload, "mode"),
	}
	if cats, ok := payload["categories"].([]any); ok {
		for _, c := range cats {
			if s, ok := c.(string); ok {
				spec.Categories = append(spec.Categories, s)
			}
		}
	}
	return spec
}

func (h *ExternalAutomationHandler) parseWaitForegroundRequest(payload map[string]any) WaitForegroundRequest {
	return WaitForegroundRequest{
		PackageName: h.toString(payload, "packageName"),
		Component:   h.toString(payload, "component"),
		TimeoutMS:   h.toInt(payload, "timeoutMs"),
	}
}

func (h *ExternalAutomationHandler) toString(payload map[string]any, key string) string {
	if v, ok := payload[key].(string); ok {
		return v
	}
	return ""
}

func (h *ExternalAutomationHandler) toBool(payload map[string]any, key string) bool {
	if v, ok := payload[key].(bool); ok {
		return v
	}
	return false
}

func (h *ExternalAutomationHandler) toInt(payload map[string]any, key string) int {
	if v, ok := payload[key].(float64); ok {
		return int(v)
	}
	if v, ok := payload[key].(int); ok {
		return v
	}
	return 0
}

func (h *ExternalAutomationHandler) toExtrasMap(payload map[string]any, key string) map[string]any {
	if v, ok := payload[key].(map[string]any); ok {
		return v
	}
	return nil
}

type blockedExternalAutomationClient struct{}

func NewBlockedExternalAutomationClient() ExternalAutomationClient {
	return &blockedExternalAutomationClient{}
}

func (b *blockedExternalAutomationClient) Status(ctx context.Context) (CapabilityState, error) {
	return CapabilityState{
		Supported: false,
		State:     StateUnsupported,
	}, newAutomationError(AUTOMATION_UNSUPPORTED, "android native host source not available; external automation provider blocked")
}

func (b *blockedExternalAutomationClient) ResolveApp(ctx context.Context, req ResolveAppRequest) ([]ResolvedApp, error) {
	return nil, newAutomationError(AUTOMATION_UNSUPPORTED, "android native host source not available; external automation provider blocked")
}

func (b *blockedExternalAutomationClient) OpenApp(ctx context.Context, req OpenAppRequest) (ActionResult, error) {
	return ActionResult{}, newAutomationError(AUTOMATION_UNSUPPORTED, "android native host source not available; external automation provider blocked")
}

func (b *blockedExternalAutomationClient) ResolveURI(ctx context.Context, req ResolveURIRequest) (ResolvedURI, error) {
	return ResolvedURI{}, newAutomationError(AUTOMATION_UNSUPPORTED, "android native host source not available; external automation provider blocked")
}

func (b *blockedExternalAutomationClient) OpenURI(ctx context.Context, req OpenURIRequest) (ActionResult, error) {
	return ActionResult{}, newAutomationError(AUTOMATION_UNSUPPORTED, "android native host source not available; external automation provider blocked")
}

func (b *blockedExternalAutomationClient) OpenSettings(ctx context.Context, req OpenSettingsRequest) (ActionResult, error) {
	return ActionResult{}, newAutomationError(AUTOMATION_UNSUPPORTED, "android native host source not available; external automation provider blocked")
}

func (b *blockedExternalAutomationClient) InvokeIntent(ctx context.Context, spec IntentSpec) (ActionResult, error) {
	return ActionResult{}, newAutomationError(AUTOMATION_UNSUPPORTED, "android native host source not available; external automation provider blocked")
}

func (b *blockedExternalAutomationClient) Foreground(ctx context.Context) (ForegroundState, error) {
	return ForegroundState{}, newAutomationError(AUTOMATION_UNSUPPORTED, "android native host source not available; external automation provider blocked")
}

func (b *blockedExternalAutomationClient) WaitForeground(ctx context.Context, req WaitForegroundRequest) (ForegroundState, error) {
	return ForegroundState{}, newAutomationError(AUTOMATION_UNSUPPORTED, "android native host source not available; external automation provider blocked")
}

func (h *ExternalAutomationHandler) Close() {
	if c, ok := h.client.(interface{ Close() }); ok {
		c.Close()
	}
}

func Now() int64 {
	return time.Now().UnixMilli()
}
