package share

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/nativebridge"
)

type ShareHandler struct {
	bridge nativebridge.Bridge
}

func NewShareHandler(bridge nativebridge.Bridge) *ShareHandler {
	return &ShareHandler{bridge: bridge}
}

func (h *ShareHandler) Execute(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationSend:
		return h.handleSend(ctx, request)
	case OperationPreviewSupported:
		return h.handlePreviewSupported(ctx, request)
	case OperationStagingCleanup:
		return h.handleStagingCleanup(ctx, request)
	case OperationLimitedDelete:
		return h.handleLimitedDelete(ctx, request)
	default:
		return NewShareError(request, nativebridge.ErrOperationNotSupported, fmt.Sprintf("unsupported operation: %s", request.Operation))
	}
}

func (h *ShareHandler) bridgeCall(ctx context.Context, request nativebridge.Request, operation string, payload map[string]any) nativebridge.Response {
	if h.bridge == nil {
		return NewShareError(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
	}
	done := make(chan nativebridge.Response, 1)
	go func() {
		resp, err := h.bridge.Execute(ctx, nativebridge.Request{
			ProtocolVersion: request.ProtocolVersion,
			RequestId:       request.RequestId,
			Platform:        "ios",
			Operation:       operation,
			Payload:         payload,
		})
		if err != nil {
			done <- NewShareError(request, ErrOutcomeUnknown, err.Error())
			return
		}
		done <- resp
	}()
	select {
	case <-ctx.Done():
		return NewShareError(request, ErrTimeout, operation+" cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *ShareHandler) handleStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationStatus, map[string]any{})
}

func (h *ShareHandler) handleSend(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	text, _ := request.Payload["text"].(string)
	subject, _ := request.Payload["subject"].(string)
	url, _ := request.Payload["url"].(string)
	shareTitle, _ := request.Payload["shareTitle"].(string)

	var resources []string
	if r, ok := request.Payload["resources"].([]any); ok {
		for _, res := range r {
			if s, ok := res.(string); ok && s != "" {
				resources = append(resources, s)
			}
		}
	}

	var preview *IOSSharePreview
	if p, ok := request.Payload["preview"].(map[string]any); ok && p != nil {
		preview = &IOSSharePreview{}
		if title, ok := p["title"].(string); ok {
			preview.Title = title
		}
		if subtitle, ok := p["subtitle"].(string); ok {
			preview.Subtitle = subtitle
		}
		if imageURI, ok := p["imageResourceUri"].(string); ok {
			preview.ImageResourceURI = imageURI
		}
	}

	req := IOSShareSendRequest{
		Text:       text,
		Subject:    subject,
		URL:        url,
		Resources:  resources,
		ShareTitle: shareTitle,
		Preview:    preview,
	}

	if err := ValidateSendRequest(req); err != nil {
		code, msg := MapErrorToNativeBridge(err)
		return NewShareError(request, code, msg)
	}

	payload := map[string]any{}
	if req.Text != "" {
		payload["text"] = req.Text
	}
	if req.Subject != "" {
		payload["subject"] = req.Subject
	}
	if req.URL != "" {
		payload["url"] = req.URL
	}
	if len(req.Resources) > 0 {
		payload["resources"] = req.Resources
	}
	if req.ShareTitle != "" {
		payload["shareTitle"] = req.ShareTitle
	}
	if req.Preview != nil {
		previewPayload := map[string]any{}
		if req.Preview.Title != "" {
			previewPayload["title"] = req.Preview.Title
		}
		if req.Preview.Subtitle != "" {
			previewPayload["subtitle"] = req.Preview.Subtitle
		}
		if req.Preview.ImageResourceURI != "" {
			previewPayload["imageResourceUri"] = req.Preview.ImageResourceURI
		}
		payload["preview"] = previewPayload
	}

	return h.bridgeCall(ctx, request, OperationSend, payload)
}

func (h *ShareHandler) handlePreviewSupported(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationPreviewSupported, map[string]any{})
}

func (h *ShareHandler) handleStagingCleanup(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	payload := map[string]any{}

	if removeStale, ok := request.Payload["removeStale"].(bool); ok {
		payload["removeStale"] = removeStale
	}

	if maxStaleAgeHours, ok := request.Payload["maxStaleAgeHours"].(float64); ok {
		payload["maxStaleAgeHours"] = ClampMaxStaleAgeHours(int(maxStaleAgeHours))
	} else {
		payload["maxStaleAgeHours"] = StagingMaxStaleAgeHours
	}

	return h.bridgeCall(ctx, request, OperationStagingCleanup, payload)
}

func (h *ShareHandler) handleLimitedDelete(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	confirm, _ := request.Payload["confirm"].(bool)

	var photoIDs []string
	if ids, ok := request.Payload["photoIds"].([]any); ok {
		for _, id := range ids {
			if s, ok := id.(string); ok && s != "" {
				photoIDs = append(photoIDs, s)
			}
		}
	}

	if len(photoIDs) == 0 {
		return NewShareError(request, ErrShareResourceInvalid, "missing required field: photoIds")
	}

	if !confirm {
		return NewShareError(request, ErrShareUserIntentRequired, "confirm field must be true for limited delete")
	}

	payload := map[string]any{
		"photoIds": photoIDs,
		"confirm":  confirm,
	}

	return h.bridgeCall(ctx, request, OperationLimitedDelete, payload)
}
