package interaction

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/nativebridge"
)

type ShizukuInteractionExecutor interface {
	Tap(ctx context.Context, x, y int) error
	LongPress(ctx context.Context, x, y int, duration int) error
	InputText(ctx context.Context, text string) error
	Swipe(ctx context.Context, startX, startY, endX, endY, duration int) error
}

type shizukuExecutor struct {
	bridge nativebridge.Bridge
	policy Policy
}

func NewShizukuExecutor(bridge nativebridge.Bridge, policy Policy) ShizukuInteractionExecutor {
	return &shizukuExecutor{
		bridge: bridge,
		policy: policy,
	}
}

func (e *shizukuExecutor) Tap(ctx context.Context, x, y int) error {
	return e.executeInteraction(ctx, "shizuku.execute", map[string]any{
		"executable": "input",
		"args":       []string{"tap", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y)},
	})
}

func (e *shizukuExecutor) LongPress(ctx context.Context, x, y int, duration int) error {
	return e.executeInteraction(ctx, "shizuku.execute", map[string]any{
		"executable": "input",
		"args":       []string{"swipe", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y), fmt.Sprintf("%d", x), fmt.Sprintf("%d", y), fmt.Sprintf("%d", duration)},
	})
}

func (e *shizukuExecutor) InputText(ctx context.Context, text string) error {
	return e.executeInteraction(ctx, "shizuku.execute", map[string]any{
		"executable": "input",
		"args":       []string{"text", text},
	})
}

func (e *shizukuExecutor) Swipe(ctx context.Context, startX, startY, endX, endY, duration int) error {
	return e.executeInteraction(ctx, "shizuku.execute", map[string]any{
		"executable": "input",
		"args":       []string{"swipe", fmt.Sprintf("%d", startX), fmt.Sprintf("%d", startY), fmt.Sprintf("%d", endX), fmt.Sprintf("%d", endY), fmt.Sprintf("%d", duration)},
	})
}

func (e *shizukuExecutor) executeInteraction(ctx context.Context, operation string, payload map[string]any) error {
	if e.bridge == nil {
		return &Error{Code: INTERACTION_SHIZUKU_UNAVAILABLE, Message: "shizuku executor: bridge not connected"}
	}

	req := nativebridge.Request{
		ProtocolVersion: nativebridge.AndroidBridgeProtocolVersion,
		RequestID:       generateShizukuRequestID(),
		Platform:        "android",
		Operation:       operation,
		Payload:         payload,
	}

	resp, err := e.bridge.Execute(ctx, req)
	if err != nil {
		return &Error{Code: INTERACTION_SHIZUKU_UNAVAILABLE, Message: fmt.Sprintf("shizuku execute error: %v", err)}
	}

	if resp.Status != "success" {
		code := "SHIZUKU_EXECUTION_ERROR"
		msg := "shizuku execution failed"
		if resp.Error != nil {
			code = resp.Error.Code
			msg = resp.Error.Message
		}
		return &Error{Code: code, Message: msg}
	}

	return nil
}

var shizukuRequestCounter uint64

func generateShizukuRequestID() string {
	shizukuRequestCounter++
	return fmt.Sprintf("shizuku-%d", shizukuRequestCounter)
}
