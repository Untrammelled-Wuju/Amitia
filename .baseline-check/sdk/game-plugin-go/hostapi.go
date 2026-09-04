package sdk

import (
	"context"
	"encoding/json"
)

const (
	MethodHostInvoke = "host.invoke"
)

const HostAPISuccess = "success"

type HostInvokeInput struct {
	Method    string          `json:"method"`
	Version   int             `json:"version,omitempty"`
	Input     json.RawMessage `json:"input"`
	TimeoutMs int             `json:"timeoutMs,omitempty"`
}

type HostInvokeResult struct {
	Status     string          `json:"status"`
	Output     json.RawMessage `json:"output"`
	Method     string          `json:"method"`
	DurationMs int64           `json:"durationMs"`
}

func (c *Client) InvokeHostMethod(ctx context.Context, input HostInvokeInput, opts ...MessageOption) (HostInvokeResult, error) {
	envelope, err := c.SendReservedRequest(ctx, MethodHostInvoke, input, opts...)
	if err != nil {
		return HostInvokeResult{}, err
	}
	var out HostInvokeResult
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return HostInvokeResult{}, NewEncodeError("unmarshal host invoke response: %v", err)
		}
	}
	return out, nil
}

func (c *Client) InvokeHostAPI(ctx context.Context, input HostInvokeInput, opts ...MessageOption) (HostInvokeResult, error) {
	return c.InvokeHostMethod(ctx, input, opts...)
}
