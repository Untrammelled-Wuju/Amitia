package androidsystem

import (
	"context"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type NotificationRequest struct {
	RequestID string         `json:"requestId"`
	Operation string         `json:"operation"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type NotificationResponse struct {
	RequestID string              `json:"requestId"`
	Status    string              `json:"status"`
	Result    map[string]any      `json:"result,omitempty"`
	Error     *NotificationError  `json:"error,omitempty"`
}

type NotificationError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	DomainCode string `json:"domainCode,omitempty"`
}

type NotificationProvider interface {
	Execute(ctx context.Context, request NotificationRequest) NotificationResponse
	Health(ctx context.Context) NotificationHealthStatus
}

type NotificationCancellableProvider interface {
	CancelOp(ctx context.Context, requestID string, reason string) error
}

type NotificationHealthStatus string

const (
	NotificationHealthReady     NotificationHealthStatus = "ready"
	NotificationHealthUnhealthy NotificationHealthStatus = "unhealthy"
	NotificationHealthUnknown   NotificationHealthStatus = "unknown"
)

type NotificationCapabilityState struct {
	Supported                bool   `json:"supported"`
	ListenerDeclared         bool   `json:"listenerDeclared"`
	ListenerGranted          bool   `json:"listenerGranted"`
	ListenerConnected        bool   `json:"listenerConnected"`
	PostPermissionRequired   bool   `json:"postPermissionRequired"`
	PostPermissionGranted    bool   `json:"postPermissionGranted"`
	NotificationsEnabled     bool   `json:"notificationsEnabled"`
	CanRead                  bool   `json:"canRead"`
	CanDismiss               bool   `json:"canDismiss"`
	CanPost                  bool   `json:"canPost"`
	UserActionRequired        bool   `json:"userActionRequired"`
	State                    string `json:"state"`
}

type blockedNotificationProvider struct {
	mu     sync.RWMutex
	reason string
}

func NewBlockedNotificationProvider(reason string) NotificationProvider {
	if reason == "" {
		reason = BLOCKED_ANDROID_NATIVE_HOST_SOURCE
	}
	return &blockedNotificationProvider{reason: reason}
}

func (b *blockedNotificationProvider) Execute(ctx context.Context, request NotificationRequest) NotificationResponse {
	b.mu.RLock()
	reason := b.reason
	b.mu.RUnlock()

	return NotificationResponse{
		RequestID: request.RequestID,
		Status:    "error",
		Error: &NotificationError{
			Code:    reason,
			Message: "android native host source not available; notification provider blocked",
		},
	}
}

func (b *blockedNotificationProvider) Health(ctx context.Context) NotificationHealthStatus {
	return NotificationHealthUnhealthy
}

type NotificationRuntimeAdapter interface {
	Supports(binding capability.RuntimeBinding) bool
	Execute(
		ctx context.Context,
		binding capability.RuntimeBinding,
		invocation capability.ToolInvocationContext,
		input []byte,
	) capability.UnifiedToolResult
	Health(ctx context.Context, binding capability.RuntimeBinding) capability.HealthStatus
}

func MapNotificationHealth(status NotificationHealthStatus) capability.HealthStatus {
	switch status {
	case NotificationHealthReady:
		return capability.HealthReady
	case NotificationHealthUnhealthy:
		return capability.HealthUnhealthy
	default:
		return capability.HealthUnknown
	}
}

func MapNotificationError(err *NotificationError) *capability.ToolError {
	if err == nil {
		return &capability.ToolError{
			Code:        capability.ErrorCodeExecutionFailed,
			Message:     "notification operation failed",
			UserVisible: true,
		}
	}

	toolErr := &capability.ToolError{
		Code:        capability.ErrorCodeExecutionFailed,
		Message:     err.Message,
		UserVisible: true,
	}

	if err.DomainCode != "" {
		toolErr.Details = map[string]any{
			"domainCode": err.DomainCode,
		}
	}

	switch err.Code {
	case NOTIFICATION_LISTENER_PERMISSION_REQUIRED,
		NOTIFICATION_POST_PERMISSION_REQUIRED:
		toolErr.Code = capability.ErrorCodePermissionDenied
		toolErr.Retryable = false
	case NOTIFICATION_LISTENER_NOT_CONNECTED:
		toolErr.Code = capability.ErrorCodeNotAvailable
		toolErr.Retryable = false
	case NOTIFICATION_NOT_FOUND:
		toolErr.Code = capability.ErrorCodeNotAvailable
		toolErr.Retryable = false
	case NOTIFICATION_STALE_REFERENCE:
		toolErr.Code = capability.ErrorCodeConflict
		toolErr.Retryable = false
	case NOTIFICATION_NOT_DISMISSIBLE:
		toolErr.Code = capability.ErrorCodePermissionDenied
		toolErr.Retryable = false
	case NOTIFICATION_CONTENT_ACTION_UNAVAILABLE,
		NOTIFICATION_ACTION_NOT_FOUND:
		toolErr.Code = capability.ErrorCodeNotAvailable
		toolErr.Retryable = false
	case NOTIFICATION_ACTION_STALE:
		toolErr.Code = capability.ErrorCodeConflict
		toolErr.Retryable = false
	case NOTIFICATION_REMOTE_INPUT_UNSUPPORTED:
		toolErr.Code = capability.ErrorCodeNotAvailable
		toolErr.Retryable = false
	case NOTIFICATION_SETTINGS_UNAVAILABLE:
		toolErr.Code = capability.ErrorCodeNotAvailable
		toolErr.Retryable = false
	case NOTIFICATION_POST_DISABLED:
		toolErr.Code = capability.ErrorCodePermissionDenied
		toolErr.Retryable = false
	case BLOCKED_ANDROID_NATIVE_HOST_SOURCE:
		toolErr.Code = capability.ErrorCodeNotAvailable
		toolErr.Retryable = false
	case BLOCKED_BY_FROZEN_A_CONTRACT:
		toolErr.Code = capability.ErrorCodeNotAvailable
		toolErr.Retryable = false
	}

	return toolErr
}
