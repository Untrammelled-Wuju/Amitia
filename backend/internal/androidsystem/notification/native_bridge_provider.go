package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/androidsystem"
	"github.com/u-ai/backend/internal/nativebridge"
)

type nativeBridgeNotificationProvider struct {
	bridge nativebridge.Bridge
}

func NewNativeBridgeNotificationProvider(bridge nativebridge.Bridge) androidsystem.SystemProvider {
	return &nativeBridgeNotificationProvider{bridge: bridge}
}

func (p *nativeBridgeNotificationProvider) Execute(ctx context.Context, request androidsystem.SystemRequest) androidsystem.SystemResponse {
	bridgeReq := nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       request.RequestID,
		Platform:        "android",
		Operation:       request.Operation,
		Payload:         request.Payload,
	}
	resp, err := p.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return androidsystem.SystemResponse{
			RequestID: request.RequestID,
			Status:    "error",
			Error: &androidsystem.SystemError{
				Code:    androidsystem.NOTIFICATION_UNSUPPORTED,
				Message: err.Error(),
			},
		}
	}
	return androidsystem.SystemResponse{
		RequestID: resp.RequestID,
		Status:    resp.Status,
		Result:    resp.Result,
		Error:     convertBridgeError(resp.Error),
	}
}

func (p *nativeBridgeNotificationProvider) Health(ctx context.Context) androidsystem.NotificationHealthStatus {
	req := nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       generateRequestID(),
		Platform:        "android",
		Operation:       OperationStatus,
		Payload:         map[string]any{},
	}
	_, err := p.bridge.Execute(ctx, req)
	if err != nil {
		return androidsystem.NotificationHealthUnhealthy
	}
	return androidsystem.NotificationHealthReady
}

func convertBridgeError(err *nativebridge.Error) *androidsystem.SystemError {
	if err == nil {
		return nil
	}
	return &androidsystem.SystemError{
		Code:    err.Code,
		Message: err.Message,
	}
}

func generateRequestID() string {
	return fmt.Sprintf("ntf-req-%d-%d", time.Now().UnixNano(), time.Now().UnixMicro())
}
