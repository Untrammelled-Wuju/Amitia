package display

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/u-ai/backend/internal/androidnative"
)

type DisplayBridge interface {
	Execute(ctx context.Context, operation string, payload map[string]any) (map[string]any, error)
}

type NativeBridgeAdapter struct {
	bridge    androidnative.NativeBridge
	requestID uint64
}

func NewDisplayBridgeAdapter(bridge androidnative.NativeBridge) *NativeBridgeAdapter {
	return &NativeBridgeAdapter{bridge: bridge}
}

func (a *NativeBridgeAdapter) Execute(ctx context.Context, operation string, payload map[string]any) (map[string]any, error) {
	if a.bridge == nil {
		return nil, NewError(ErrDisplayNativeBridgeUnavailable, "native bridge not initialized")
	}
	reqID := atomic.AddUint64(&a.requestID, 1)
	requestID := fmt.Sprintf("dsp_%d", reqID)
	req := androidnative.NativeBridgeRequest{
		ProtocolVersion: 1,
		RequestId:       requestID,
		Platform:        "android",
		Operation:       operation,
		Payload:         payload,
	}
	resp, err := a.bridge.Execute(ctx, req)
	if err != nil {
		return nil, NewError(ErrDisplayInvalidResponse, err.Error())
	}
	if resp.Error != nil {
		return nil, NewError(mapNativeDisplayError(resp.Error.Code), resp.Error.Message)
	}
	return resp.Result, nil
}

func mapNativeDisplayError(code string) string {
	switch code {
	case "DISPLAY_UNSUPPORTED":
		return ErrDisplayUnsupported
	case "DISPLAY_NOT_FOUND":
		return ErrDisplayNotFound
	case "DISPLAY_REMOVED":
		return ErrDisplayRemoved
	case "DISPLAY_TARGET_STALE":
		return ErrDisplayTargetStale
	case "DISPLAY_INVALID_ID":
		return ErrDisplayInvalidID
	case "DISPLAY_PRIVATE":
		return ErrDisplayPrivate
	case "DISPLAY_SECURE":
		return ErrDisplaySecure
	case "DISPLAY_INACTIVE":
		return ErrDisplayInactive
	case "DISPLAY_OFF":
		return ErrDisplayOff
	case "DISPLAY_UI_TREE_UNSUPPORTED":
		return ErrDisplayUITreeUnsupported
	case "DISPLAY_GESTURE_UNSUPPORTED":
		return ErrDisplayGestureUnsupported
	case "DISPLAY_SCREENSHOT_UNSUPPORTED":
		return ErrDisplayScreenshotUnsupported
	case "DISPLAY_SCREEN_FRAME_UNSUPPORTED":
		return ErrDisplayScreenFrameUnsupported
	case "DISPLAY_ACTIVITY_LAUNCH_UNSUPPORTED":
		return ErrDisplayActivityLaunchUnsupported
	case "DISPLAY_ACTIVITY_LAUNCH_DENIED":
		return ErrDisplayActivityLaunchDenied
	case "DISPLAY_TOPOLOGY_UNSUPPORTED":
		return ErrDisplayTopologyUnsupported
	case "DISPLAY_AMBIGUOUS":
		return ErrDisplayAmbiguous
	case "DISPLAY_NATIVE_BRIDGE_UNAVAILABLE":
		return ErrDisplayNativeBridgeUnavailable
	case "DISPLAY_INVALID_RESPONSE":
		return ErrDisplayInvalidResponse
	case "DISPLAY_TIMEOUT":
		return ErrDisplayTimeout
	case "DISPLAY_CANCELLED":
		return ErrDisplayCancelled
	default:
		return ErrDisplayInvalidResponse
	}
}

func mustJSONDisplay(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
