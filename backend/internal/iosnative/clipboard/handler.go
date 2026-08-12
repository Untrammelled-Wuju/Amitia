package clipboard

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/nativebridge"
)

type ClipboardHandler struct {
	bridge nativebridge.Bridge
}

func NewClipboardHandler(bridge nativebridge.Bridge) *ClipboardHandler {
	return &ClipboardHandler{bridge: bridge}
}

func (h *ClipboardHandler) Execute(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	switch request.Operation {
	case OperationStatus:
		return h.handleStatus(ctx, request)
	case OperationDetect:
		return h.handleDetect(ctx, request)
	case OperationRead:
		return h.handleRead(ctx, request)
	case OperationWrite:
		return h.handleWrite(ctx, request)
	case OperationClear:
		return h.handleClear(ctx, request)
	default:
		return NewClipboardError(request, nativebridge.ErrOperationNotSupported, fmt.Sprintf("unsupported operation: %s", request.Operation))
	}
}

func (h *ClipboardHandler) bridgeCall(ctx context.Context, request nativebridge.Request, operation string, payload map[string]any) nativebridge.Response {
	if h.bridge == nil {
		return NewClipboardError(request, ErrNativeBridgeUnavailable, "ios native bridge is not available")
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
			done <- NewClipboardError(request, ErrOutcomeUnknown, err.Error())
			return
		}
		done <- resp
	}()
	select {
	case <-ctx.Done():
		return NewClipboardError(request, ErrTimeout, operation+" cancelled")
	case resp := <-done:
		return resp
	}
}

func (h *ClipboardHandler) handleStatus(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationStatus, map[string]any{})
}

func (h *ClipboardHandler) handleDetect(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	payload := map[string]any{}

	if patterns, ok := request.Payload["patterns"].([]any); ok {
		patternKinds := make([]string, 0, len(patterns))
		for _, p := range patterns {
			if s, ok := p.(string); ok && IsValidPatternKind(PatternKind(s)) {
				patternKinds = append(patternKinds, s)
			}
		}
		if len(patternKinds) == 0 {
			return NewClipboardError(request, ErrDetectionUnsupported, "no valid patterns provided")
		}
		payload["patterns"] = patternKinds
	} else {
		return NewClipboardError(request, ErrDetectionUnsupported, "missing required field: patterns")
	}

	if itemIndexes, ok := request.Payload["itemIndexes"].([]any); ok {
		indexes := make([]int, 0, len(itemIndexes))
		for _, idx := range itemIndexes {
			if n, ok := idx.(float64); ok {
				indexes = append(indexes, int(n))
			}
		}
		payload["itemIndexes"] = indexes
	}

	if includeValues, ok := request.Payload["includeValues"].(bool); ok {
		payload["includeValues"] = includeValues
	}

	return h.bridgeCall(ctx, request, OperationDetect, payload)
}

func (h *ClipboardHandler) handleRead(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	payload := map[string]any{}

	if preferredTypes, ok := request.Payload["preferredTypes"].([]any); ok {
		types := make([]string, 0, len(preferredTypes))
		for _, t := range preferredTypes {
			if s, ok := t.(string); ok && IsValidReadType(ContentType(s)) {
				types = append(types, s)
			}
		}
		payload["preferredTypes"] = types
	}

	if itemIndexes, ok := request.Payload["itemIndexes"].([]any); ok {
		indexes := make([]int, 0, len(itemIndexes))
		for _, idx := range itemIndexes {
			if n, ok := idx.(float64); ok {
				indexes = append(indexes, int(n))
			}
		}
		payload["itemIndexes"] = indexes
	}

	if maxItems, ok := request.Payload["maxItems"].(float64); ok {
		payload["maxItems"] = ClampMaxItems(int(maxItems))
	} else {
		payload["maxItems"] = DefaultMaxItems
	}

	if maxBytes, ok := request.Payload["maxBytes"].(float64); ok {
		payload["maxBytes"] = ClampMaxBytes(int64(maxBytes))
	} else {
		payload["maxBytes"] = MaxClipboardReadBytes
	}

	if materializeBinary, ok := request.Payload["materializeBinary"].(bool); ok {
		payload["materializeBinary"] = materializeBinary
	}

	return h.bridgeCall(ctx, request, OperationRead, payload)
}

func (h *ClipboardHandler) handleWrite(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	itemsRaw, ok := request.Payload["items"].([]any)
	if !ok || len(itemsRaw) == 0 {
		return NewClipboardError(request, ErrWriteValueRequired, "missing required field: items")
	}

	items := make([]map[string]any, 0, len(itemsRaw))
	for _, raw := range itemsRaw {
		itemMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		item := map[string]any{}
		if t, ok := itemMap["type"].(string); ok && IsValidReadType(ContentType(t)) {
			item["type"] = t
		} else {
			continue
		}
		if text, ok := itemMap["text"].(string); ok {
			item["text"] = text
		}
		if url, ok := itemMap["url"].(string); ok {
			item["url"] = url
		}
		if resourceURI, ok := itemMap["resourceUri"].(string); ok {
			item["resourceUri"] = resourceURI
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		return NewClipboardError(request, ErrWriteValueInvalid, "no valid items to write")
	}

	payload := map[string]any{
		"items": items,
	}

	if localOnly, ok := request.Payload["localOnly"].(bool); ok {
		payload["localOnly"] = localOnly
	}

	if expirationSeconds, ok := request.Payload["expirationSeconds"].(float64); ok {
		v := int(expirationSeconds)
		clamped := ClampExpirationSeconds(&v)
		if clamped != nil {
			payload["expirationSeconds"] = *clamped
		}
	}

	return h.bridgeCall(ctx, request, OperationWrite, payload)
}

func (h *ClipboardHandler) handleClear(ctx context.Context, request nativebridge.Request) nativebridge.Response {
	return h.bridgeCall(ctx, request, OperationClear, map[string]any{})
}
