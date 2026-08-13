package clipboard

import (
	"context"
	"errors"
	"sync"

	"github.com/u-ai/backend/internal/androidsystem"
)

type ClipboardHandler struct {
	client ClipboardClient
	policy Policy
	mu     sync.RWMutex
}

func NewClipboardHandler(client ClipboardClient) *ClipboardHandler {
	if client == nil {
		client = NewHostClipboardClient(nil)
	}
	return &ClipboardHandler{
		client: client,
		policy: DefaultPolicy(),
	}
}

func (h *ClipboardHandler) Execute(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationReadText:
		return h.handleRead(ctx, request)
	case OperationWriteText:
		return h.handleWrite(ctx, request)
	case OperationClear:
		return h.handleClear(ctx, request)
	default:
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_UNSUPPORTED,
				Message: "unknown clipboard operation: " + request.Operation,
			},
		}
	}
}

func (h *ClipboardHandler) CapabilityState(ctx context.Context) ClipboardCapabilityState {
	state, err := h.client.Status(ctx)
	if err != nil {
		return ClipboardCapabilityState{
			Supported:    false,
			State:        StateUnsupported,
			Reason:       err.Error(),
			MaxTextBytes: h.policy.MaxTextBytes,
		}
	}
	return state
}

func (h *ClipboardHandler) handleStatus(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	state, err := h.client.Status(ctx)
	if err != nil {
		var ce *clipboardError
		if errors.As(err, &ce) {
			return androidsystem.SystemResponse{
				RequestID: request.RequestID,
				Status:    "error",
				Error: &androidsystem.SystemError{
					Code:    ce.code,
					Message: ce.message,
				},
			}
		}
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_UNSUPPORTED,
				Message: err.Error(),
			},
		}
	}

	return androidsystem.SystemResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"supported":              state.Supported,
			"canWrite":               state.CanWrite,
			"canRead":                state.CanRead,
			"appForeground":          state.AppForeground,
			"appHasInputFocus":       state.AppHasInputFocus,
			"readRequiresForeground": state.ReadRequiresForeground,
			"hasPrimaryClip":         state.HasPrimaryClip,
			"supportedMimeTypes":     state.SupportedMimeTypes,
			"maxTextBytes":           state.MaxTextBytes,
			"state":                  state.State,
			"reason":                 state.Reason,
		},
	}
}

func (h *ClipboardHandler) handleRead(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	result, err := h.client.ReadText(ctx)
	if err != nil {
		var ce *clipboardError
		if errors.As(err, &ce) {
			return androidsystem.SystemResponse{
				RequestID: request.RequestID,
				Status:    "error",
				Error: &androidsystem.SystemError{
					Code:    ce.code,
					Message: ce.message,
				},
			}
		}
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    CLIPBOARD_READ_FAILED,
				Message: err.Error(),
			},
		}
	}

	return androidsystem.SystemResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"hasContent":  result.HasContent,
			"text":        result.Text,
			"mimeType":    result.MIMEType,
			"itemCount":   result.ItemCount,
			"truncated":   result.Truncated,
			"sensitive":   result.Sensitive,
			"generation":  result.Generation,
		},
	}
}

func (h *ClipboardHandler) handleWrite(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	text, _ := request.Payload["text"].(string)
	sensitive, _ := request.Payload["sensitive"].(bool)

	if err := h.policy.validateWriteText(text); err != nil {
		var ce *clipboardError
		if errors.As(err, &ce) {
			return androidsystem.SystemResponse{
				RequestID: request.RequestID,
				Status:    "error",
				Error: &androidsystem.SystemError{
					Code:    ce.code,
					Message: ce.message,
				},
			}
		}
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    CLIPBOARD_INPUT_TOO_LARGE,
				Message: err.Error(),
			},
		}
	}

	result, err := h.client.WriteText(ctx, ClipboardWriteRequest{
		Text:      text,
		Sensitive: &sensitive,
	})
	if err != nil {
		var ce *clipboardError
		if errors.As(err, &ce) {
			return androidsystem.SystemResponse{
				RequestID: request.RequestID,
				Status:    "error",
				Error: &androidsystem.SystemError{
					Code:    ce.code,
					Message: ce.message,
				},
			}
		}
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    CLIPBOARD_WRITE_FAILED,
				Message: err.Error(),
			},
		}
	}

	return androidsystem.SystemResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{
			"written":    result.Written,
			"bytes":      result.Bytes,
			"sensitive":  result.Sensitive,
			"generation": result.Generation,
		},
	}
}

func (h *ClipboardHandler) handleClear(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	if err := h.client.Clear(ctx); err != nil {
		var ce *clipboardError
		if errors.As(err, &ce) {
			return androidsystem.SystemResponse{
				RequestID: request.RequestID,
				Status:    "error",
				Error: &androidsystem.SystemError{
					Code:    ce.code,
					Message: ce.message,
				},
			}
		}
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    CLIPBOARD_CLEAR_FAILED,
				Message: err.Error(),
			},
		}
	}

	return androidsystem.SystemResponse{
		RequestID: request.RequestID,
		Status:    "success",
		Result: map[string]any{"cleared": true},
	}
}

func (h *ClipboardHandler) Close() {
	h.client.Close()
}
