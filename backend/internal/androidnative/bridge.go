package androidnative

import (
	"context"

	"github.com/u-ai/backend/internal/nativebridge"
)

type NativeBridgeRequest = nativebridge.Request
type NativeBridgeResponse = nativebridge.Response
type NativeBridgeError = nativebridge.Error
type NativeBridgeHealth = nativebridge.Health

const (
	NativeBridgeHealthReady     = nativebridge.HealthReady
	NativeBridgeHealthUnhealthy = nativebridge.HealthUnhealthy
	NativeBridgeHealthUnknown   = nativebridge.HealthUnknown
)

type NativeBridge interface {
	Execute(ctx context.Context, request NativeBridgeRequest) (NativeBridgeResponse, error)
	Health(ctx context.Context) NativeBridgeHealth
}
