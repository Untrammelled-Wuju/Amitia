package share

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/u-ai/backend/internal/nativebridge"
)

type nativeBridgeShareClient struct {
	bridge nativebridge.Bridge
}

func NewNativeBridgeShareClient(bridge nativebridge.Bridge) ShareClient {
	return &nativeBridgeShareClient{bridge: bridge}
}

func (c *nativeBridgeShareClient) Send(ctx context.Context, req ShareSendRequest) (ShareSendResult, error) {
	payload := map[string]any{}
	if req.Text != "" {
		payload["text"] = req.Text
	}
	if req.Subject != "" {
		payload["subject"] = req.Subject
	}
	if req.MIMEType != "" {
		payload["mimeType"] = req.MIMEType
	}
	if req.ChooserTitle != "" {
		payload["chooserTitle"] = req.ChooserTitle
	}
	if len(req.Resources) > 0 {
		payload["resources"] = req.Resources
	}

	bridgeReq := nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       generateRequestID(),
		Platform:        "android",
		Operation:       OperationSend,
		Payload:         payload,
	}
	resp, err := c.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return ShareSendResult{}, &shareError{code: SHARE_UNAVAILABLE, message: err.Error()}
	}
	if resp.Status != "success" {
		return ShareSendResult{}, &shareError{code: SHARE_UNAVAILABLE, message: errorMessage(resp)}
	}
	result := ShareSendResult{}
	if resp.Result != nil {
		result.Status, _ = resp.Result["status"].(string)
		result.ResourceCount = toInt(resp.Result["resourceCount"])
		result.MIMEType, _ = resp.Result["mimeType"].(string)
		result.UserActionRequired, _ = resp.Result["userActionRequired"].(bool)
	}
	return result, nil
}

func (c *nativeBridgeShareClient) Status(ctx context.Context) (ShareCapabilityState, error) {
	req := nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       generateRequestID(),
		Platform:        "android",
		Operation:       OperationStatus,
		Payload:         map[string]any{},
	}
	resp, err := c.bridge.Execute(ctx, req)
	if err != nil {
		return ShareCapabilityState{}, &shareError{code: SHARE_UNAVAILABLE, message: err.Error()}
	}
	if resp.Status != "success" {
		return ShareCapabilityState{}, &shareError{code: SHARE_UNAVAILABLE, message: errorMessage(resp)}
	}
	state := ShareCapabilityState{}
	if resp.Result != nil {
		state.Supported, _ = resp.Result["supported"].(bool)
		state.CanSend, _ = resp.Result["canSend"].(bool)
		state.CanReceive, _ = resp.Result["canReceive"].(bool)
		state.NativeHostReady, _ = resp.Result["nativeHostReady"].(bool)
		state.MaxResources = toInt(resp.Result["maxResources"])
		state.MaxSingleResourceBytes = toInt64(resp.Result["maxSingleResourceBytes"])
		state.MaxTotalBytes = toInt64(resp.Result["maxTotalBytes"])
		state.State, _ = resp.Result["state"].(string)
	}
	return state, nil
}

func (c *nativeBridgeShareClient) Close() {
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
