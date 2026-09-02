package interaction

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/androidnative"
)

type BridgeCoordinateExecutor struct {
	bridge androidnative.NativeBridge
}

func NewBridgeCoordinateExecutor(bridge androidnative.NativeBridge) *BridgeCoordinateExecutor {
	return &BridgeCoordinateExecutor{bridge: bridge}
}

func (e *BridgeCoordinateExecutor) Tap(
	ctx context.Context,
	displayID int,
	x int,
	y int,
) error {
	if e.bridge == nil {
		return &Error{Code: INTERACTION_NATIVE_HOST_UNAVAILABLE, Message: "native bridge not available"}
	}

	bridgeReq := androidnative.NativeBridgeRequest{
		ProtocolVersion: 1,
		RequestId:       "",
		Operation:       "interaction.tap",
		Payload: map[string]any{
			"displayId": displayID,
			"x":         x,
			"y":         y,
		},
	}

	resp, err := e.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: "tap bridge call failed: " + err.Error()}
	}

	if resp.Error != nil {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: resp.Error.Message}
	}

	return nil
}

func (e *BridgeCoordinateExecutor) LongPress(
	ctx context.Context,
	displayID int,
	x int,
	y int,
	duration time.Duration,
) error {
	if e.bridge == nil {
		return &Error{Code: INTERACTION_NATIVE_HOST_UNAVAILABLE, Message: "native bridge not available"}
	}

	bridgeReq := androidnative.NativeBridgeRequest{
		ProtocolVersion: 1,
		RequestId:       "",
		Operation:       "interaction.long_press",
		Payload: map[string]any{
			"displayId":  displayID,
			"x":          x,
			"y":          y,
			"durationMs": duration.Milliseconds(),
		},
	}

	resp, err := e.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: "long press bridge call failed: " + err.Error()}
	}

	if resp.Error != nil {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: resp.Error.Message}
	}

	return nil
}

func (e *BridgeCoordinateExecutor) Swipe(
	ctx context.Context,
	request SwipeRequest,
) error {
	if e.bridge == nil {
		return &Error{Code: INTERACTION_NATIVE_HOST_UNAVAILABLE, Message: "native bridge not available"}
	}

	durationMS := request.DurationMS
	if durationMS <= 0 {
		durationMS = DefaultSwipeDurationMS
	}
	if durationMS < MinSwipeDurationMS {
		durationMS = MinSwipeDurationMS
	}
	if durationMS > MaxSwipeDurationMS {
		durationMS = MaxSwipeDurationMS
	}

	bridgeReq := androidnative.NativeBridgeRequest{
		ProtocolVersion: 1,
		RequestId:       "",
		Operation:       "interaction.swipe",
		Payload: map[string]any{
			"displayId":  request.DisplayID,
			"startX":     request.StartX,
			"startY":     request.StartY,
			"endX":       request.EndX,
			"endY":       request.EndY,
			"durationMs": durationMS,
		},
	}

	resp, err := e.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: "swipe bridge call failed: " + err.Error()}
	}

	if resp.Error != nil {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: resp.Error.Message}
	}

	return nil
}

func validateCoordinate(x, y, minX, minY, maxX, maxY int) error {
	if x < minX || x > maxX || y < minY || y > maxY {
		return &Error{
			Code:    INTERACTION_COORDINATE_INVALID,
			Message: fmt.Sprintf("coordinate (%d,%d) out of bounds [%d,%d,%d,%d]", x, y, minX, minY, maxX, maxY),
		}
	}
	return nil
}

func (e *BridgeCoordinateExecutor) ProbeHealth(ctx context.Context) ProviderCapabilityHealth {
	return probeNativeStatus(ctx, e.bridge, "accessibility_gesture", "interaction.status")
}
