package interaction

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/androidnative"
)

const (
	NodeActionClick          = "click"
	NodeActionLongClick      = "long_click"
	NodeActionSetText        = "set_text"
	NodeActionClearText      = "clear_text"
	NodeActionScrollForward  = "scroll_forward"
	NodeActionScrollBackward = "scroll_backward"
	NodeActionFocus          = "focus"
	NodeActionSelect         = "select"
)

type BridgeAccessibilityExecutor struct {
	bridge androidnative.NativeBridge
}

type AdvancedAccessibilityExecutor interface {
	PerformGlobalAction(ctx context.Context, action string) (map[string]any, error)
	PerformGesture(ctx context.Context, payload map[string]any) (map[string]any, error)
	TakeScreenshot(ctx context.Context, displayID int) (map[string]any, error)
}

func NewBridgeAccessibilityExecutor(bridge androidnative.NativeBridge) *BridgeAccessibilityExecutor {
	return &BridgeAccessibilityExecutor{bridge: bridge}
}

func (e *BridgeAccessibilityExecutor) PerformNodeAction(
	ctx context.Context,
	node ResolvedUINode,
	action string,
	args map[string]any,
) error {
	if e.bridge == nil {
		return &Error{Code: INTERACTION_NATIVE_HOST_UNAVAILABLE, Message: "native bridge not available"}
	}

	if node.NativeRef == "" {
		return &Error{Code: INTERACTION_NODE_STALE, Message: "node native reference is empty"}
	}

	bridgeReq := androidnative.NativeBridgeRequest{
		ProtocolVersion: 1,
		RequestId:       uuid.NewString(),
		Platform:        "android",
		Operation:       "interaction.perform_node_action",
		Payload: map[string]any{
			"nativeRef": node.NativeRef,
			"action":    action,
			"args":      args,
		},
	}

	resp, err := e.bridge.Execute(ctx, bridgeReq)
	if err != nil {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: "bridge call failed: " + err.Error()}
	}

	if resp.Error != nil {
		if strings.Contains(resp.Error.Message, "unsupported") {
			return &Error{Code: INTERACTION_ACTION_UNSUPPORTED, Message: resp.Error.Message}
		}
		if strings.Contains(resp.Error.Message, "stale") || strings.Contains(resp.Error.Message, "expired") {
			return &Error{Code: INTERACTION_NODE_STALE, Message: resp.Error.Message}
		}
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: resp.Error.Message}
	}
	if resp.Status != "success" {
		return &Error{Code: INTERACTION_ACTION_FAILED, Message: "accessibility node action was not completed"}
	}

	if resp.Result != nil {
		if success, ok := resp.Result["success"].(bool); ok && !success {
			if msg, ok := resp.Result["message"].(string); ok {
				return &Error{Code: INTERACTION_ACTION_FAILED, Message: msg}
			}
			return &Error{Code: INTERACTION_ACTION_FAILED, Message: "action failed"}
		}
	}

	return nil
}

func (e *BridgeAccessibilityExecutor) PerformGlobalAction(ctx context.Context, action string) (map[string]any, error) {
	return e.executeOperation(ctx, "interaction.global_action", map[string]any{"action": action})
}

func (e *BridgeAccessibilityExecutor) PerformGesture(ctx context.Context, payload map[string]any) (map[string]any, error) {
	return e.executeOperation(ctx, "interaction.gesture", payload)
}

func (e *BridgeAccessibilityExecutor) TakeScreenshot(ctx context.Context, displayID int) (map[string]any, error) {
	return e.executeOperation(ctx, "interaction.screenshot", map[string]any{"displayId": displayID})
}

func (e *BridgeAccessibilityExecutor) executeOperation(
	ctx context.Context,
	operation string,
	payload map[string]any,
) (map[string]any, error) {
	if e.bridge == nil {
		return nil, &Error{Code: INTERACTION_NATIVE_HOST_UNAVAILABLE, Message: "native bridge not available"}
	}
	resp, err := e.bridge.Execute(ctx, androidnative.NativeBridgeRequest{
		ProtocolVersion: 1,
		RequestId:       uuid.NewString(),
		Platform:        "android",
		Operation:       operation,
		Payload:         payload,
	})
	if err != nil {
		return nil, &Error{Code: INTERACTION_ACTION_FAILED, Message: "bridge call failed: " + err.Error()}
	}
	if resp.Error != nil {
		code := INTERACTION_ACTION_FAILED
		if strings.Contains(strings.ToLower(resp.Error.Message), "unsupported") {
			code = INTERACTION_ACTION_UNSUPPORTED
		}
		return nil, &Error{Code: code, Message: resp.Error.Message}
	}
	if resp.Status != "success" {
		return nil, &Error{Code: INTERACTION_ACTION_FAILED, Message: "accessibility operation was not completed"}
	}
	if resp.Result == nil {
		return map[string]any{}, nil
	}
	return resp.Result, nil
}

func (e *BridgeAccessibilityExecutor) SupportsAction(
	node ResolvedUINode,
	action string,
) bool {
	if node.NativeRef == "" {
		return false
	}

	switch action {
	case NodeActionClick:
		return node.Node.Clickable || containsAction(node.Node.Actions, "click")
	case NodeActionLongClick:
		return node.Node.LongClickable || containsAction(node.Node.Actions, "long_click")
	case NodeActionSetText:
		return node.Node.Editable
	case NodeActionClearText:
		return node.Node.Editable
	case NodeActionScrollForward:
		return node.Node.Scrollable || containsAction(node.Node.Actions, "scroll_forward")
	case NodeActionScrollBackward:
		return node.Node.Scrollable || containsAction(node.Node.Actions, "scroll_backward")
	case NodeActionFocus:
		return node.Node.Focusable
	case NodeActionSelect:
		return true
	default:
		return containsAction(node.Node.Actions, action)
	}
}

func containsAction(actions []string, target string) bool {
	for _, a := range actions {
		if strings.EqualFold(a, target) {
			return true
		}
	}
	return false
}

func formatActionArgs(action string, args map[string]any) string {
	if len(args) == 0 {
		return fmt.Sprintf("action=%s", action)
	}
	return fmt.Sprintf("action=%s args=%v", action, args)
}

func (e *BridgeAccessibilityExecutor) ProbeHealth(ctx context.Context) ProviderCapabilityHealth {
	return probeNativeStatus(ctx, e.bridge, "accessibility", "accessibility.status")
}
