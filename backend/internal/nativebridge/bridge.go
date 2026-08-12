package nativebridge

import "context"

type Request struct {
	ProtocolVersion int            `json:"protocolVersion"`
	RequestID       string         `json:"requestId"`
	Platform        string         `json:"platform"`
	Operation       string         `json:"operation"`
	Payload         map[string]any `json:"payload,omitempty"`
}

type Response struct {
	ProtocolVersion int            `json:"protocolVersion"`
	RequestID       string         `json:"requestId"`
	Status          string         `json:"status"`
	Result          map[string]any `json:"result,omitempty"`
	Error           *Error         `json:"error,omitempty"`
}

type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	DomainCode string `json:"domainCode,omitempty"`
}

type Health string

const (
	HealthReady     Health = "ready"
	HealthUnhealthy Health = "unhealthy"
	HealthUnknown   Health = "unknown"
)

type Bridge interface {
	Execute(context.Context, Request) (Response, error)
	Health(context.Context) Health
}
