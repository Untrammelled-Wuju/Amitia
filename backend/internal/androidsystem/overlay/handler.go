package overlay

import (
	"context"
	"errors"
	"sync"

	"github.com/u-ai/backend/internal/androidsystem"
)

type OverlayHandler struct {
	client OverlayClient
	policy Policy
	mu     sync.RWMutex
}

type OverlayClient interface {
	Status(ctx context.Context) (CapabilityState, error)
	RequestPermission(ctx context.Context) (PermissionResult, error)

	Create(ctx context.Context, req CreateRequest) (OverlayInstance, error)
	Update(ctx context.Context, req UpdateRequest) (OverlayInstance, error)

	Show(ctx context.Context, overlayID string) (OverlayInstance, error)
	Hide(ctx context.Context, overlayID string) (OverlayInstance, error)

	Close(ctx context.Context, overlayID string) error
	List(ctx context.Context) ([]OverlayInstance, error)
	CloseAll(ctx context.Context) (int, error)
}

func NewOverlayHandler(client OverlayClient) *OverlayHandler {
	if client == nil {
		client = NewBlockedOverlayClient()
	}
	return &OverlayHandler{
		client: client,
		policy: DefaultPolicy(),
	}
}

func (h *OverlayHandler) Execute(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationPermissionRequest:
		return h.handlePermissionRequest(ctx, request)
	case OperationCreate:
		return h.handleCreate(ctx, request)
	case OperationUpdate:
		return h.handleUpdate(ctx, request)
	case OperationShow:
		return h.handleShow(ctx, request)
	case OperationHide:
		return h.handleHide(ctx, request)
	case OperationClose:
		return h.handleClose(ctx, request)
	case OperationList:
		return h.handleList(ctx, request)
	case OperationCloseAll:
		return h.handleCloseAll(ctx, request)
	default:
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_UNSUPPORTED,
				Message: "unknown overlay operation: " + request.Operation,
			},
		}
	}
}

func (h *OverlayHandler) handleStatus(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	state, err := h.client.Status(ctx)
	if err != nil {
		var oe *overlayError
		if errors.As(err, &oe) {
			return androidsystem.SystemResponse{
				RequestID: request.RequestID,
				Status:    "error",
				Error: &androidsystem.SystemError{
					Code:    oe.code,
					Message: oe.message,
				},
			}
		}
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_NATIVE_HOST_UNAVAILABLE,
				Message: err.Error(),
			},
		}
	}

	return androidsystem.SystemResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"supported":          state.Supported,
			"permissionRequired": state.PermissionRequired,
			"permissionGranted":  state.PermissionGranted,
			"nativeHostReady":    state.NativeHostReady,
			"canCreate":          state.CanCreate,
			"canUpdate":          state.CanUpdate,
			"canInteract":        state.CanInteract,
			"activeCount":        state.ActiveCount,
			"userActionRequired": state.UserActionRequired,
			"state":              state.State,
		},
	}
}

func (h *OverlayHandler) handlePermissionRequest(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	result, err := h.client.RequestPermission(ctx)
	if err != nil {
		var oe *overlayError
		if errors.As(err, &oe) {
			return androidsystem.SystemResponse{
				RequestID: request.RequestID,
				Status:    "error",
				Error: &androidsystem.SystemError{
					Code:    oe.code,
					Message: oe.message,
				},
			}
		}
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_PERMISSION_DENIED,
				Message: err.Error(),
			},
		}
	}

	return androidsystem.SystemResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"opened":            result.Opened,
			"userActionRequired": result.UserActionRequired,
			"permissionGranted": result.PermissionGranted,
		},
	}
}

func (h *OverlayHandler) handleCreate(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	h.mu.RLock()
	activeState, err := h.client.Status(ctx)
	h.mu.RUnlock()
	if err != nil {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_NATIVE_HOST_UNAVAILABLE,
				Message: "failed to query overlay status: " + err.Error(),
			},
		}
	}

	if !activeState.PermissionGranted {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_PERMISSION_REQUIRED,
				Message: "overlay permission not granted, please request permission first",
			},
		}
	}

	req := h.parseCreateRequest(request.Payload)

	if err := h.policy.ValidateCreateRequest(req, activeState.ActiveCount); err != nil {
		var oe *overlayError
		if errors.As(err, &oe) {
			return androidsystem.SystemResponse{
				RequestID: request.RequestID,
				Status:    "error",
				Error: &androidsystem.SystemError{
					Code:    oe.code,
					Message: oe.message,
				},
			}
		}
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_INVALID_INPUT,
				Message: err.Error(),
			},
		}
	}

	instance, err := h.client.Create(ctx, req)
	if err != nil {
		var oe *overlayError
		if errors.As(err, &oe) {
			return androidsystem.SystemResponse{
				RequestID: request.RequestID,
				Status:    "error",
				Error: &androidsystem.SystemError{
					Code:    oe.code,
					Message: oe.message,
				},
			}
		}
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_CREATE_FAILED,
				Message: err.Error(),
			},
		}
	}

	return androidsystem.SystemResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"overlay": instance,
		},
	}
}

func (h *OverlayHandler) handleUpdate(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	overlayID, _ := request.Payload["overlayId"].(string)
	if overlayID == "" {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_INVALID_INPUT,
				Message: "overlayId is required",
			},
		}
	}

	req := h.parseUpdateRequest(overlayID, request.Payload)

	if err := h.policy.ValidateUpdateRequest(req); err != nil {
		var oe *overlayError
		if errors.As(err, &oe) {
			return androidsystem.SystemResponse{
				RequestID: request.RequestID,
				Status:    "error",
				Error: &androidsystem.SystemError{
					Code:    oe.code,
					Message: oe.message,
				},
			}
		}
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_INVALID_INPUT,
				Message: err.Error(),
			},
		}
	}

	instance, err := h.client.Update(ctx, req)
	if err != nil {
		var oe *overlayError
		if errors.As(err, &oe) {
			return androidsystem.SystemResponse{
				RequestID: request.RequestID,
				Status:    "error",
				Error: &androidsystem.SystemError{
					Code:    oe.code,
					Message: oe.message,
				},
			}
		}
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_UPDATE_FAILED,
				Message: err.Error(),
			},
		}
	}

	return androidsystem.SystemResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"overlay": instance,
		},
	}
}

func (h *OverlayHandler) handleShow(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	overlayID, _ := request.Payload["overlayId"].(string)
	if overlayID == "" {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_INVALID_INPUT,
				Message: "overlayId is required",
			},
		}
	}

	instance, err := h.client.Show(ctx, overlayID)
	if err != nil {
		var oe *overlayError
		if errors.As(err, &oe) {
			return androidsystem.SystemResponse{
				RequestID: request.RequestID,
				Status:    "error",
				Error: &androidsystem.SystemError{
					Code:    oe.code,
					Message: oe.message,
				},
			}
		}
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_SHOW_FAILED,
				Message: err.Error(),
			},
		}
	}

	return androidsystem.SystemResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"overlay": instance,
		},
	}
}

func (h *OverlayHandler) handleHide(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	overlayID, _ := request.Payload["overlayId"].(string)
	if overlayID == "" {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_INVALID_INPUT,
				Message: "overlayId is required",
			},
		}
	}

	instance, err := h.client.Hide(ctx, overlayID)
	if err != nil {
		var oe *overlayError
		if errors.As(err, &oe) {
			return androidsystem.SystemResponse{
				RequestID: request.RequestID,
				Status:    "error",
				Error: &androidsystem.SystemError{
					Code:    oe.code,
					Message: oe.message,
				},
			}
		}
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_HIDE_FAILED,
				Message: err.Error(),
			},
		}
	}

	return androidsystem.SystemResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"overlay": instance,
		},
	}
}

func (h *OverlayHandler) handleClose(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	overlayID, _ := request.Payload["overlayId"].(string)
	if overlayID == "" {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_INVALID_INPUT,
				Message: "overlayId is required",
			},
		}
	}

	if err := h.client.Close(ctx, overlayID); err != nil {
		var oe *overlayError
		if errors.As(err, &oe) {
			return androidsystem.SystemResponse{
				RequestID: request.RequestID,
				Status:    "error",
				Error: &androidsystem.SystemError{
					Code:    oe.code,
					Message: oe.message,
				},
			}
		}
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_CLOSE_FAILED,
				Message: err.Error(),
			},
		}
	}

	return androidsystem.SystemResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"closed": true,
		},
	}
}

func (h *OverlayHandler) handleList(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	overlays, err := h.client.List(ctx)
	if err != nil {
		var oe *overlayError
		if errors.As(err, &oe) {
			return androidsystem.SystemResponse{
				RequestID: request.RequestID,
				Status:    "error",
				Error: &androidsystem.SystemError{
					Code:    oe.code,
					Message: oe.message,
				},
			}
		}
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_NATIVE_HOST_UNAVAILABLE,
				Message: err.Error(),
			},
		}
	}

	return androidsystem.SystemResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"overlays": overlays,
			"count":    len(overlays),
		},
	}
}

func (h *OverlayHandler) handleCloseAll(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	count, err := h.client.CloseAll(ctx)
	if err != nil {
		var oe *overlayError
		if errors.As(err, &oe) {
			return androidsystem.SystemResponse{
				RequestID: request.RequestID,
				Status:    "error",
				Error: &androidsystem.SystemError{
					Code:    oe.code,
					Message: oe.message,
				},
			}
		}
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    OVERLAY_CLOSE_FAILED,
				Message: err.Error(),
			},
		}
	}

	return androidsystem.SystemResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"closedCount": count,
		},
	}
}

func (h *OverlayHandler) parseCreateRequest(payload map[string]any) CreateRequest {
	req := CreateRequest{
		Kind:    h.toString(payload, "kind"),
		Content: h.toMap(payload, "content"),
	}

	if v, ok := payload["x"].(float64); ok {
		x := int(v)
		req.X = &x
	}
	if v, ok := payload["y"].(float64); ok {
		y := int(v)
		req.Y = &y
	}
	if v, ok := payload["width"].(float64); ok {
		w := int(v)
		req.Width = &w
	}
	if v, ok := payload["height"].(float64); ok {
		h := int(v)
		req.Height = &h
	}

	req.Gravity = h.toString(payload, "gravity")

	if v, ok := payload["focusable"].(bool); ok {
		req.Focusable = &v
	}
	if v, ok := payload["touchable"].(bool); ok {
		req.Touchable = &v
	}
	if v, ok := payload["draggable"].(bool); ok {
		req.Draggable = &v
	}
	if v, ok := payload["ttlMs"].(float64); ok {
		ttl := int64(v)
		req.TTLms = &ttl
	}

	return req
}

func (h *OverlayHandler) parseUpdateRequest(overlayID string, payload map[string]any) UpdateRequest {
	req := UpdateRequest{
		OverlayID: overlayID,
		Content:   h.toMap(payload, "content"),
	}

	if v, ok := payload["x"].(float64); ok {
		x := int(v)
		req.X = &x
	}
	if v, ok := payload["y"].(float64); ok {
		y := int(v)
		req.Y = &y
	}
	if v, ok := payload["width"].(float64); ok {
		w := int(v)
		req.Width = &w
	}
	if v, ok := payload["height"].(float64); ok {
		h := int(v)
		req.Height = &h
	}

	req.Gravity = h.toString(payload, "gravity")

	if v, ok := payload["focusable"].(bool); ok {
		req.Focusable = &v
	}
	if v, ok := payload["touchable"].(bool); ok {
		req.Touchable = &v
	}
	if v, ok := payload["draggable"].(bool); ok {
		req.Draggable = &v
	}
	if v, ok := payload["ttlMs"].(float64); ok {
		ttl := int64(v)
		req.TTLms = &ttl
	}

	return req
}

func (h *OverlayHandler) toString(payload map[string]any, key string) string {
	if v, ok := payload[key].(string); ok {
		return v
	}
	return ""
}

func (h *OverlayHandler) toMap(payload map[string]any, key string) map[string]any {
	if v, ok := payload[key].(map[string]any); ok {
		return v
	}
	return nil
}

type blockedOverlayClient struct{}

func NewBlockedOverlayClient() OverlayClient {
	return &blockedOverlayClient{}
}

func (b *blockedOverlayClient) Status(ctx context.Context) (CapabilityState, error) {
	return CapabilityState{
		Supported: false,
		State:     StateUnsupported,
	}, newOverlayError(OVERLAY_UNSUPPORTED, "android native host source not available; overlay provider blocked")
}

func (b *blockedOverlayClient) RequestPermission(ctx context.Context) (PermissionResult, error) {
	return PermissionResult{}, newOverlayError(OVERLAY_UNSUPPORTED, "android native host source not available; overlay provider blocked")
}

func (b *blockedOverlayClient) Create(ctx context.Context, req CreateRequest) (OverlayInstance, error) {
	return OverlayInstance{}, newOverlayError(OVERLAY_UNSUPPORTED, "android native host source not available; overlay provider blocked")
}

func (b *blockedOverlayClient) Update(ctx context.Context, req UpdateRequest) (OverlayInstance, error) {
	return OverlayInstance{}, newOverlayError(OVERLAY_UNSUPPORTED, "android native host source not available; overlay provider blocked")
}

func (b *blockedOverlayClient) Show(ctx context.Context, overlayID string) (OverlayInstance, error) {
	return OverlayInstance{}, newOverlayError(OVERLAY_UNSUPPORTED, "android native host source not available; overlay provider blocked")
}

func (b *blockedOverlayClient) Hide(ctx context.Context, overlayID string) (OverlayInstance, error) {
	return OverlayInstance{}, newOverlayError(OVERLAY_UNSUPPORTED, "android native host source not available; overlay provider blocked")
}

func (b *blockedOverlayClient) Close(ctx context.Context, overlayID string) error {
	return newOverlayError(OVERLAY_UNSUPPORTED, "android native host source not available; overlay provider blocked")
}

func (b *blockedOverlayClient) List(ctx context.Context) ([]OverlayInstance, error) {
	return []OverlayInstance{}, newOverlayError(OVERLAY_UNSUPPORTED, "android native host source not available; overlay provider blocked")
}

func (b *blockedOverlayClient) CloseAll(ctx context.Context) (int, error) {
	return 0, newOverlayError(OVERLAY_UNSUPPORTED, "android native host source not available; overlay provider blocked")
}

func (h *OverlayHandler) Close() {
}
