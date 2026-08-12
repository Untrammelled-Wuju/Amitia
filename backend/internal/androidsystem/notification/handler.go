package notification

import (
	"context"
	"sync"

	"github.com/u-ai/backend/internal/androidsystem"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type HandlerCapabilityState struct {
	Supported              bool   `json:"supported"`
	ListenerDeclared       bool   `json:"listenerDeclared"`
	ListenerGranted        bool   `json:"listenerGranted"`
	ListenerConnected      bool   `json:"listenerConnected"`
	PostPermissionRequired bool   `json:"postPermissionRequired"`
	PostPermissionGranted  bool   `json:"postPermissionGranted"`
	NotificationsEnabled   bool   `json:"notificationsEnabled"`
	CanRead                bool   `json:"canRead"`
	CanDismiss             bool   `json:"canDismiss"`
	CanPost                bool   `json:"canPost"`
	UserActionRequired     bool   `json:"userActionRequired"`
	State                  string `json:"state"`
}

type NotificationHandler struct {
	provider androidsystem.NotificationProvider
	store    *ProjectionStore
	mu       sync.RWMutex
	state    HandlerCapabilityState
}

func NewNotificationHandler(provider androidsystem.NotificationProvider) *NotificationHandler {
	if provider == nil {
		provider = androidsystem.NewBlockedNotificationProvider(androidsystem.BLOCKED_ANDROID_NATIVE_HOST_SOURCE)
	}
	return &NotificationHandler{
		provider: provider,
		store:    NewProjectionStore(),
		state: HandlerCapabilityState{
			Supported:   false,
			State:       StateUnsupported,
		},
	}
}

func (h *NotificationHandler) Execute(ctx context.Context, request androidsystem.NotificationRequest) androidsystem.NotificationResponse {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationList:
		return h.handleList(ctx, request)
	case OperationGet:
		return h.handleGet(ctx, request)
	case OperationPost:
		return h.handlePost(ctx, request)
	case OperationCancelOwn:
		return h.handleCancelOwn(ctx, request)
	case OperationDismiss:
		return h.handleDismiss(ctx, request)
	case OperationOpen:
		return h.handleOpen(ctx, request)
	case OperationInvokeAction:
		return h.handleInvokeAction(ctx, request)
	default:
		return androidsystem.NotificationResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.NotificationError{
				Code:    androidsystem.NOTIFICATION_UNSUPPORTED,
				Message: "unknown notification operation: " + request.Operation,
			},
		}
	}
}

func (h *NotificationHandler) RefreshState(ctx context.Context) {
	resp := h.provider.Execute(ctx, androidsystem.NotificationRequest{
		RequestID: "internal_state_refresh",
		Operation: OperationStatus,
		Payload:   map[string]any{},
	})

	h.mu.Lock()
	defer h.mu.Unlock()

	if resp.Status == "success" && resp.Result != nil {
		h.state = HandlerCapabilityState{
			Supported:              boolFrom(resp.Result, "supported"),
			ListenerDeclared:       boolFrom(resp.Result, "listenerDeclared"),
			ListenerGranted:        boolFrom(resp.Result, "listenerGranted"),
			ListenerConnected:      boolFrom(resp.Result, "listenerConnected"),
			PostPermissionRequired: boolFrom(resp.Result, "postPermissionRequired"),
			PostPermissionGranted:  boolFrom(resp.Result, "postPermissionGranted"),
			NotificationsEnabled:   boolFrom(resp.Result, "notificationsEnabled"),
			CanRead:                boolFrom(resp.Result, "canRead"),
			CanDismiss:             boolFrom(resp.Result, "canDismiss"),
			CanPost:                boolFrom(resp.Result, "canPost"),
			UserActionRequired:     boolFrom(resp.Result, "userActionRequired"),
			State:                  stringFrom(resp.Result, "state"),
		}
	}
}

func (h *NotificationHandler) State() HandlerCapabilityState {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.state
}

func (h *NotificationHandler) Store() *ProjectionStore {
	return h.store
}

func (h *NotificationHandler) Provider() androidsystem.NotificationProvider {
	return h.provider
}

func (h *NotificationHandler) mapAndroidError(err *androidsystem.NotificationError) *capability.ToolError {
	if err == nil {
		return &capability.ToolError{
			Code:        capability.ErrorCodeExecutionFailed,
			Message:     "notification operation failed",
			UserVisible: true,
		}
	}
	return androidsystem.MapNotificationError(err)
}

func boolFrom(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func intFrom(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

func int64From(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	}
	return 0
}

func stringFrom(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
