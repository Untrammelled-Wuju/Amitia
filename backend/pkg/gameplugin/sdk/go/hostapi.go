package sdk

import (
	"context"
	"encoding/json"
)

const (
	MethodHostInvoke = "host.invoke"
)

const (
	HostAPIRead    = "read"
	HostAPIWrite   = "write"
	HostAPIExecute = "execute"
	HostAPINotify  = "notify"

	HostAPISuccess   = "success"
	HostAPIFailed    = "failed"
	HostAPITimeout   = "timeout"
	HostAPICancelled = "cancelled"

	HostAPIPermissionDenied = "permission_denied"
	HostAPIScopeDenied      = "scope_denied"
	HostAPIRateLimited      = "rate_limited"
	HostAPIMethodNotFound   = "method_not_found"
	HostAPIInputInvalid     = "input_invalid"
	HostAPIOutputInvalid    = "output_invalid"
	HostAPIGenerationStale  = "generation_stale"
	HostAPIHostUnavailable  = "host_unavailable"
)

type HostInvokeInput struct {
	Method     string          `json:"method"`
	Version    int             `json:"version,omitempty"`
	Input      json.RawMessage `json:"input"`
	ServiceID  string          `json:"serviceId,omitempty"`
	SideEffect string          `json:"sideEffect,omitempty"`
	RequestID  string          `json:"requestId,omitempty"`
	TimeoutMs  int             `json:"timeoutMs,omitempty"`
}

type HostInvokeResult struct {
	Status     string          `json:"status"`
	Output     json.RawMessage `json:"output,omitempty"`
	Method     string          `json:"method,omitempty"`
	DurationMs int             `json:"durationMs,omitempty"`
	Error      *HostAPIError   `json:"error,omitempty"`
}

type HostAPIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
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
