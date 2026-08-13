package sdk

import (
	"context"
	"encoding/json"
)

const (
	MethodHostAPIInvoke     = "host_api.invoke"
	MethodHostAPIQueryCaps  = "host_api.query_capabilities"
	MethodHostAPIRateLimit  = "host_api.rate_limit_status"
)

const (
	HostAPIRead    = "read"
	HostAPIWrite   = "write"
	HostAPIExecute = "execute"
	HostAPINotify  = "notify"

	HostAPISuccess = "success"
	HostAPIFailed  = "failed"
	HostAPITimeout = "timeout"
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

type HostAPIInvokeInput struct {
	Method     string          `json:"method"`
	Version    int             `json:"version,omitempty"`
	Input      json.RawMessage `json:"input"`
	ServiceID  string          `json:"serviceId,omitempty"`
	SideEffect string          `json:"sideEffect,omitempty"`
	RequestID  string          `json:"requestId,omitempty"`
	TimeoutMs  int             `json:"timeoutMs,omitempty"`
}

type HostAPIInvokeResult struct {
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

type HostAPIQueryCapsInput struct {
	Method string `json:"method"`
}

type HostAPIQueryCapsResult struct {
	Method       string `json:"method"`
	Registered   bool   `json:"registered"`
	PermissionID string `json:"permissionId,omitempty"`
	RateLimitMax int    `json:"rateLimitMax,omitempty"`
	TimeoutMs    int    `json:"timeoutMs,omitempty"`
	SideEffect   string `json:"sideEffect,omitempty"`
	Versions     []int  `json:"versions,omitempty"`
}

type HostAPIRateLimitStatusResult struct {
	Limit    int `json:"limit"`
	Remaining int `json:"remaining"`
	ResetAt  int64 `json:"resetAt"`
}

func (c *Client) InvokeHostAPI(ctx context.Context, input HostAPIInvokeInput, opts ...MessageOption) (HostAPIInvokeResult, error) {
	envelope, err := c.SendRequest(ctx, MethodHostAPIInvoke, input, opts...)
	if err != nil {
		return HostAPIInvokeResult{}, err
	}
	var out HostAPIInvokeResult
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return HostAPIInvokeResult{}, NewEncodeError("unmarshal host api invoke response: %v", err)
		}
	}
	return out, nil
}

func (c *Client) QueryHostAPICapabilities(ctx context.Context, input HostAPIQueryCapsInput, opts ...MessageOption) (HostAPIQueryCapsResult, error) {
	envelope, err := c.SendRequest(ctx, MethodHostAPIQueryCaps, input, opts...)
	if err != nil {
		return HostAPIQueryCapsResult{}, err
	}
	var out HostAPIQueryCapsResult
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return HostAPIQueryCapsResult{}, NewEncodeError("unmarshal host api query caps response: %v", err)
		}
	}
	return out, nil
}

func (c *Client) QueryHostAPIRateLimit(ctx context.Context, method string, opts ...MessageOption) (HostAPIRateLimitStatusResult, error) {
	input := map[string]any{"method": method}
	envelope, err := c.SendRequest(ctx, MethodHostAPIRateLimit, input, opts...)
	if err != nil {
		return HostAPIRateLimitStatusResult{}, err
	}
	var out HostAPIRateLimitStatusResult
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return HostAPIRateLimitStatusResult{}, NewEncodeError("unmarshal host api rate limit response: %v", err)
		}
	}
	return out, nil
}
