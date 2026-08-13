package notification

import (
	"context"

	"github.com/u-ai/backend/internal/androidsystem"
)

func (h *NotificationHandler) handleStatus(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	resp := h.provider.Execute(ctx, androidsystem.SystemRequest{
		RequestID: request.RequestID,
		Operation: OperationStatus,
		Payload:   map[string]any{},
	})

	h.RefreshState(ctx)

	if resp.Status == "success" {
		return resp
	}

	return androidsystem.SystemResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"supported":              h.state.Supported,
			"listenerDeclared":       h.state.ListenerDeclared,
			"listenerGranted":        h.state.ListenerGranted,
			"listenerConnected":      h.state.ListenerConnected,
			"postPermissionRequired": h.state.PostPermissionRequired,
			"postPermissionGranted":  h.state.PostPermissionGranted,
			"notificationsEnabled":   h.state.NotificationsEnabled,
			"canRead":                h.state.CanRead,
			"canDismiss":             h.state.CanDismiss,
			"canPost":                h.state.CanPost,
			"userActionRequired":     h.state.UserActionRequired,
			"state":                  h.state.State,
		},
	}
}

func (h *NotificationHandler) handleList(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	h.RefreshState(ctx)

	if !h.state.CanRead {
		if h.state.ListenerGranted && !h.state.ListenerConnected {
			return notConnectedResponse(request.RequestID)
		}
		if !h.state.ListenerGranted {
			return listenerPermissionResponse(request.RequestID)
		}
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_UNSUPPORTED,
				Message: "notification read capability not available",
			},
		}
	}

	payload := map[string]any{}
	limit := DefaultListLimit
	if v, ok := request.Payload["limit"].(float64); ok {
		limit = int(v)
		if limit < 1 {
			limit = DefaultListLimit
		}
		if limit > MaxListLimit {
			limit = MaxListLimit
		}
	}
	payload["limit"] = limit

	if v, ok := request.Payload["packageName"].(string); ok {
		payload["packageName"] = v
	}

	includeOngoing := false
	if v, ok := request.Payload["includeOngoing"].(bool); ok {
		includeOngoing = v
	}
	payload["includeOngoing"] = includeOngoing

	payload["includeOwn"] = false

	resp := h.provider.Execute(ctx, androidsystem.SystemRequest{
		RequestID: request.RequestID,
		Operation: OperationList,
		Payload:   payload,
	})

	if resp.Status == "success" {
		h.sanitizeListResult(resp)
	}

	return resp
}

func (h *NotificationHandler) handleGet(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	ref := stringFrom(request.Payload, "notificationRef")
	if ref == "" {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_NOT_FOUND,
				Message: "notificationRef is required",
			},
		}
	}

	payload := map[string]any{
		"notificationRef": ref,
	}

	if tag, ok := h.store.LookupOwnTag(ref); ok {
		payload["ownTag"] = tag
		payload["own"] = true
	} else if key, ok := h.store.LookupNotification(ref); ok {
		payload["nativeKey"] = key
	} else {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_NOT_FOUND,
				Message: "notification reference not found or stale",
			},
		}
	}

	return h.provider.Execute(ctx, androidsystem.SystemRequest{
		RequestID: request.RequestID,
		Operation: OperationGet,
		Payload:   payload,
	})
}

func (h *NotificationHandler) handlePost(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	title := stringFrom(request.Payload, "title")
	body := stringFrom(request.Payload, "body")

	if title == "" && body == "" {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_POST_FAILED,
				Message: "both title and body are empty",
			},
		}
	}

	title = Truncate(title, PostTitleMax)
	body = Truncate(body, PostBodyMax)

	payload := map[string]any{
		"title": title,
		"body":  body,
	}

	channel := stringFrom(request.Payload, "channel")
	if channel == "" {
		channel = ChannelAgentID
	}
	if channel != ChannelAgentID && channel != ChannelTaskID {
		channel = ChannelAgentID
	}
	payload["channel"] = channel

	if v, ok := request.Payload["silent"].(bool); ok {
		payload["silent"] = v
	}

	return h.provider.Execute(ctx, androidsystem.SystemRequest{
		RequestID: request.RequestID,
		Operation: OperationPost,
		Payload:   payload,
	})
}

func (h *NotificationHandler) handleCancelOwn(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	ref := stringFrom(request.Payload, "notificationRef")
	if ref == "" {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_CANCEL_FAILED,
				Message: "notificationRef is required",
			},
		}
	}

	tag, ok := h.store.LookupOwnTag(ref)
	if !ok {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_NOT_FOUND,
				Message: "own notification reference not found",
			},
		}
	}

	return h.provider.Execute(ctx, androidsystem.SystemRequest{
		RequestID: request.RequestID,
		Operation: OperationCancelOwn,
		Payload: map[string]any{
			"notificationRef": ref,
			"tag":             tag,
		},
	})
}

func (h *NotificationHandler) handleDismiss(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	h.RefreshState(ctx)

	if !h.state.CanDismiss {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_LISTENER_PERMISSION_REQUIRED,
				Message: "notification dismiss capability not available",
			},
		}
	}

	ref := stringFrom(request.Payload, "notificationRef")
	if ref == "" {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_NOT_FOUND,
				Message: "notificationRef is required",
			},
		}
	}

	key, ok := h.store.LookupNotification(ref)
	if !ok {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_NOT_FOUND,
				Message: "notification reference not found or stale",
			},
		}
	}

	return h.provider.Execute(ctx, androidsystem.SystemRequest{
		RequestID: request.RequestID,
		Operation: OperationDismiss,
		Payload: map[string]any{
			"notificationRef": ref,
			"nativeKey":       key,
		},
	})
}

func (h *NotificationHandler) handleOpen(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	h.RefreshState(ctx)

	if !h.state.CanDismiss {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_LISTENER_PERMISSION_REQUIRED,
				Message: "notification open capability not available",
			},
		}
	}

	ref := stringFrom(request.Payload, "notificationRef")
	if ref == "" {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_NOT_FOUND,
				Message: "notificationRef is required",
			},
		}
	}

	key, ok := h.store.LookupNotification(ref)
	if !ok {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_NOT_FOUND,
				Message: "notification reference not found or stale",
			},
		}
	}

	return h.provider.Execute(ctx, androidsystem.SystemRequest{
		RequestID: request.RequestID,
		Operation: OperationOpen,
		Payload: map[string]any{
			"notificationRef": ref,
			"nativeKey":       key,
		},
	})
}

func (h *NotificationHandler) handleInvokeAction(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	h.RefreshState(ctx)

	if !h.state.CanDismiss {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_LISTENER_PERMISSION_REQUIRED,
				Message: "notification action capability not available",
			},
		}
	}

	ref := stringFrom(request.Payload, "notificationRef")
	actionRef := stringFrom(request.Payload, "actionRef")
	if ref == "" || actionRef == "" {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_ACTION_NOT_FOUND,
				Message: "notificationRef and actionRef are required",
			},
		}
	}

	key, ok := h.store.LookupNotification(ref)
	if !ok {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_NOT_FOUND,
				Message: "notification reference not found or stale",
			},
		}
	}

	return h.provider.Execute(ctx, androidsystem.SystemRequest{
		RequestID: request.RequestID,
		Operation: OperationInvokeAction,
		Payload: map[string]any{
			"notificationRef": ref,
			"actionRef":       actionRef,
			"nativeKey":       key,
		},
	})
}

func (h *NotificationHandler) sanitizeListResult(resp androidsystem.SystemResponse) {
	if resp.Result == nil {
		return
	}

	notifications, ok := resp.Result["notifications"].([]any)
	if !ok {
		return
	}

	for _, n := range notifications {
		m, ok := n.(map[string]any)
		if !ok {
			continue
		}
		if title, ok := m["title"].(string); ok {
			m["title"] = Truncate(title, TitleMaxChars)
		}
		if text, ok := m["text"].(string); ok {
			m["text"] = Truncate(text, BodyMaxChars)
		}
		if subText, ok := m["subText"].(string); ok {
			m["subText"] = Truncate(subText, SubTextMaxChars)
		}
	}
}

func listenerPermissionResponse(requestID string) androidsystem.SystemResponse {
	return androidsystem.SystemResponse{
		RequestID: requestID,
		Status:    "error",
		Error: &androidsystem.SystemError{
			Code:    androidsystem.NOTIFICATION_LISTENER_PERMISSION_REQUIRED,
			Message: "notification listener access not granted",
		},
	}
}

func notConnectedResponse(requestID string) androidsystem.SystemResponse {
	return androidsystem.SystemResponse{
		RequestID: requestID,
		Status:    "error",
		Error: &androidsystem.SystemError{
			Code:    androidsystem.NOTIFICATION_LISTENER_NOT_CONNECTED,
			Message: "notification listener not connected",
		},
	}
}
