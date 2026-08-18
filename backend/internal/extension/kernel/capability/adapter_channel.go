package capability

import (
	"context"
	"encoding/json"
	"fmt"
)

type ChannelCallFunc func(
	ctx context.Context,
	handlerName string,
	input json.RawMessage,
) (json.RawMessage, error)

type ChannelHealthFunc func(
	ctx context.Context,
) HealthStatus

type ChannelRuntimeAdapter struct {
	call   ChannelCallFunc
	health ChannelHealthFunc
}

func NewChannelRuntimeAdapter(call ChannelCallFunc, health ChannelHealthFunc) *ChannelRuntimeAdapter {
	return &ChannelRuntimeAdapter{
		call:   call,
		health: health,
	}
}

func (a *ChannelRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeChannel
}

func (a *ChannelRuntimeAdapter) Execute(
	ctx context.Context,
	binding RuntimeBinding,
	invocation ToolInvocationContext,
	input json.RawMessage,
) UnifiedToolResult {
	if a.call == nil {
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:        ErrorCodeExecutionFailed,
				Message:     "channel runtime not configured",
				UserVisible: false,
			},
		}
	}
	output, err := a.call(ctx, binding.HandlerName, input)
	if err != nil {
		if toolErr, ok := err.(*ToolError); ok {
			return UnifiedToolResult{
				InvocationID: invocation.InvocationID,
				Status:       ToolResultStatusFailed,
				Error:        toolErr,
			}
		}
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:        ErrorCodeExecutionFailed,
				Message:     err.Error(),
				UserVisible: true,
			},
		}
	}
	return UnifiedToolResult{
		InvocationID: invocation.InvocationID,
		Status:       ToolResultStatusSuccess,
		Structured:   output,
	}
}

func (a *ChannelRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	if a.health == nil {
		return HealthUnknown
	}
	return a.health(ctx)
}

type ChannelStore interface {
	CreateIntent(ctx context.Context, channel, peerID, contentType string, payload []byte) (string, error)
	IsAvailable(channel string) bool
}

func EncodeChannelResult(intentID string, status string) json.RawMessage {
	result := map[string]any{
		"intentId": intentID,
		"status":   status,
	}
	data, _ := json.Marshal(result)
	return data
}

func DecodeChannelInput(input json.RawMessage) (channel, peerID, contentType string, payload []byte, err error) {
	var req struct {
		Channel     string          `json:"channel"`
		PeerID      string          `json:"peerId"`
		ContentType string          `json:"contentType"`
		Payload     json.RawMessage `json:"payload"`
	}
	if err = json.Unmarshal(input, &req); err != nil {
		return "", "", "", nil, fmt.Errorf("invalid channel delivery input: %w", err)
	}
	return req.Channel, req.PeerID, req.ContentType, req.Payload, nil
}
