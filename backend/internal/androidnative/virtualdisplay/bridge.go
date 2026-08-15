package virtualdisplay

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/u-ai/backend/internal/androidnative"
)

type VirtualBridge interface {
	Execute(ctx context.Context, operation string, payload map[string]any) (map[string]any, error)
}

type NativeBridgeAdapter struct {
	bridge    androidnative.NativeBridge
	requestID uint64
}

func NewNativeBridgeAdapter(bridge androidnative.NativeBridge) *NativeBridgeAdapter {
	return &NativeBridgeAdapter{bridge: bridge}
}

func (a *NativeBridgeAdapter) Execute(ctx context.Context, operation string, payload map[string]any) (map[string]any, error) {
	if a.bridge == nil {
		return nil, NewError(ErrVirtualDisplayUnavailable, "native bridge not initialized")
	}
	reqID := atomic.AddUint64(&a.requestID, 1)
	requestID := fmt.Sprintf("vdr_%d", reqID)
	req := androidnative.NativeBridgeRequest{
		ProtocolVersion: 1,
		RequestId:       requestID,
		Platform:        "android",
		Operation:       operation,
		Payload:         payload,
	}
	resp, err := a.bridge.Execute(ctx, req)
	if err != nil {
		return nil, NewError(ErrVirtualDisplayNative, err.Error())
	}
	if resp.Error != nil {
		return nil, NewError(mapNativeOperationError(resp.Error.Code), resp.Error.Message)
	}
	return resp.Result, nil
}

func mapNativeOperationError(code string) string {
	switch code {
	case "VIRTUAL_DISPLAY_UNSUPPORTED":
		return ErrVirtualDisplayUnsupported
	case "VIRTUAL_DISPLAY_ALREADY_EXISTS":
		return ErrVirtualDisplayAlreadyExists
	case "VIRTUAL_DISPLAY_PROPERTY_NOT_SUPPORTED":
		return ErrVirtualDisplayProperty
	case "VIRTUAL_DISPLAY_CREATE_FAILED":
		return ErrVirtualDisplayCreate
	case "VIRTUAL_DISPLAY_NOT_FOUND":
		return ErrVirtualDisplayNotFound
	case "VIRTUAL_DISPLAY_ID_MISMATCH":
		return ErrVirtualDisplayIdMismatch
	case "VIRTUAL_DISPLAY_RESIZE_FAILED":
		return ErrVirtualDisplayResize
	case "VIRTUAL_DISPLAY_SURFACE_OPERATION_FAILED":
		return ErrVirtualDisplaySurface
	case "VIRTUAL_DISPLAY_OUT_OF_RESOURCES":
		return ErrVirtualDisplayOutOfResources
	case "VIRTUAL_DISPLAY_NATIVE_ERROR":
		return ErrVirtualDisplayNative
	default:
		return ErrVirtualDisplayNative
	}
}
