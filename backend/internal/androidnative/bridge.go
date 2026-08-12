package androidnative

import (
	"context"
)

type NativeBridgeRequest struct {
	ProtocolVersion int            `json:"protocolVersion"`
	RequestID       string         `json:"requestId"`
	Operation       string         `json:"operation"`
	Payload         map[string]any `json:"payload,omitempty"`
}

type NativeBridgeResponse struct {
	ProtocolVersion int            `json:"protocolVersion"`
	RequestID       string         `json:"requestId"`
	Status          string         `json:"status"`
	Result          map[string]any `json:"result,omitempty"`
	Error           *NativeBridgeError `json:"error,omitempty"`
}

type NativeBridgeError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	DomainCode string `json:"domainCode,omitempty"`
}

type NativeBridgeHealth string

const (
	NativeBridgeHealthReady     NativeBridgeHealth = "ready"
	NativeBridgeHealthUnhealthy NativeBridgeHealth = "unhealthy"
	NativeBridgeHealthUnknown   NativeBridgeHealth = "unknown"
)

type NativeBridge interface {
	Execute(ctx context.Context, request NativeBridgeRequest) (NativeBridgeResponse, error)
	Health(ctx context.Context) NativeBridgeHealth
}
