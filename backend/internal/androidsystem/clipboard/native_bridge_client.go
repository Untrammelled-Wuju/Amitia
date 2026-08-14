package clipboard

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/nativebridge"
)

type nativeBridgeClipboardClient struct {
	bridge nativebridge.Bridge
	mu     sync.Mutex
}

func NewNativeBridgeClipboardClient(bridge nativebridge.Bridge) ClipboardClient {
	return &nativeBridgeClipboardClient{bridge: bridge}
}

func (c *nativeBridgeClipboardClient) ReadText(ctx context.Context) (ClipboardReadResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       generateRequestID(),
		Platform:        "android",
		Operation:       OperationReadText,
		Payload:         map[string]any{},
	}
	resp, err := c.bridge.Execute(ctx, req)
	if err != nil {
		return ClipboardReadResult{}, &clipboardError{code: CLIPBOARD_HOST_UNAVAILABLE, message: err.Error()}
	}
	if resp.Status != "success" {
		return ClipboardReadResult{}, &clipboardError{code: CLIPBOARD_HOST_UNAVAILABLE, message: errorMessage(resp)}
	}
	result := ClipboardReadResult{}
	if resp.Result != nil {
		result.HasContent, _ = resp.Result["hasContent"].(bool)
		result.Text, _ = resp.Result["text"].(string)
		result.MIMEType, _ = resp.Result["mimeType"].(string)
		result.ItemCount = toInt(resp.Result["itemCount"])
		result.Truncated, _ = resp.Result["truncated"].(bool)
		result.Sensitive, _ = resp.Result["sensitive"].(bool)
		result.Generation = toUint64(resp.Result["generation"])
	}
	return result, nil
}

func (c *nativeBridgeClipboardClient) WriteText(ctx context.Context, req ClipboardWriteRequest) (WriteResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	payload := map[string]any{"text": req.Text}
	if req.Sensitive != nil && *req.Sensitive {
		payload["sensitive"] = true
	}
	bridgeReq := nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       generateRequestID(),
		Platform:        "android",
		Operation:       OperationWriteText,
		Payload:         payload,
	}
	resp, err := c.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return WriteResult{}, &clipboardError{code: CLIPBOARD_HOST_UNAVAILABLE, message: err.Error()}
	}
	if resp.Status != "success" {
		return WriteResult{}, &clipboardError{code: CLIPBOARD_HOST_UNAVAILABLE, message: errorMessage(resp)}
	}
	result := WriteResult{}
	if resp.Result != nil {
		result.Written, _ = resp.Result["written"].(bool)
		result.Bytes = toInt(resp.Result["bytes"])
		if s, ok := resp.Result["sensitive"].(bool); ok {
			result.Sensitive = s
		} else if req.Sensitive != nil {
			result.Sensitive = *req.Sensitive
		}
		result.Generation = toUint64(resp.Result["generation"])
	}
	return result, nil
}

func (c *nativeBridgeClipboardClient) Status(ctx context.Context) (ClipboardCapabilityState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       generateRequestID(),
		Platform:        "android",
		Operation:       OperationStatus,
		Payload:         map[string]any{},
	}
	resp, err := c.bridge.Execute(ctx, req)
	if err != nil {
		return ClipboardCapabilityState{}, &clipboardError{code: CLIPBOARD_HOST_UNAVAILABLE, message: err.Error()}
	}
	if resp.Status != "success" {
		return ClipboardCapabilityState{}, &clipboardError{code: CLIPBOARD_HOST_UNAVAILABLE, message: errorMessage(resp)}
	}
	state := ClipboardCapabilityState{}
	if resp.Result != nil {
		state.Supported, _ = resp.Result["supported"].(bool)
		state.CanRead, _ = resp.Result["canRead"].(bool)
		state.CanWrite, _ = resp.Result["canWrite"].(bool)
		state.AppForeground, _ = resp.Result["appForeground"].(bool)
		state.AppHasInputFocus, _ = resp.Result["appHasInputFocus"].(bool)
		state.ReadRequiresForeground, _ = resp.Result["readRequiresForeground"].(bool)
		state.HasPrimaryClip, _ = resp.Result["hasPrimaryClip"].(bool)
		state.MaxTextBytes = toInt(resp.Result["maxTextBytes"])
		state.State, _ = resp.Result["state"].(string)
		state.Reason, _ = resp.Result["reason"].(string)
	}
	return state, nil
}

func (c *nativeBridgeClipboardClient) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       generateRequestID(),
		Platform:        "android",
		Operation:       OperationClear,
		Payload:         map[string]any{},
	}
	resp, err := c.bridge.Execute(ctx, req)
	if err != nil {
		return &clipboardError{code: CLIPBOARD_HOST_UNAVAILABLE, message: err.Error()}
	}
	if resp.Status != "success" {
		return &clipboardError{code: CLIPBOARD_HOST_UNAVAILABLE, message: errorMessage(resp)}
	}
	return nil
}

func (c *nativeBridgeClipboardClient) Close() {
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

func toUint64(v any) uint64 {
	switch n := v.(type) {
	case uint64:
		return n
	case int:
		return uint64(n)
	case int64:
		return uint64(n)
	case float64:
		return uint64(n)
	case string:
		i, _ := strconv.ParseUint(n, 10, 64)
		return i
	}
	return 0
}

var requestIDCounter uint64

func generateRequestID() string {
	requestIDCounter++
	return fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), requestIDCounter)
}
