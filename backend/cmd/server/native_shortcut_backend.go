package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/iosnative/alarms"
	"github.com/u-ai/backend/internal/iosnative/calendar"
	"github.com/u-ai/backend/internal/iosnative/media"
	"github.com/u-ai/backend/internal/iosnative/reminders"
	"github.com/u-ai/backend/internal/requestidentity"
)

type nativeShortcutActionRequest struct {
	RequestID string         `json:"requestId"`
	ActionID  string         `json:"actionId"`
	Payload   map[string]any `json:"payload"`
}

func newNativeShortcutBackendActionHandler(services *AppServices) func(context.Context, string, string, json.RawMessage) (json.RawMessage, error) {
	return func(ctx context.Context, spaceID string, platform string, raw json.RawMessage) (json.RawMessage, error) {
		if platform != "ios" {
			return nil, fmt.Errorf("shortcut backend actions are only accepted from ios")
		}
		if services == nil || services.KernelContainer == nil || services.KernelContainer.ToolFacade == nil {
			return nil, fmt.Errorf("canonical ToolFacade unavailable")
		}

		var req nativeShortcutActionRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, fmt.Errorf("decode shortcut action request: %w", err)
		}
		req.RequestID = strings.TrimSpace(req.RequestID)
		req.ActionID = strings.TrimSpace(req.ActionID)
		if req.ActionID == "" {
			return nil, fmt.Errorf("missing actionId")
		}
		if req.Payload == nil {
			req.Payload = map[string]any{}
		}
		if req.RequestID == "" {
			req.RequestID = "shortcut-" + req.ActionID
		}

		if req.ActionID == "com.amitia.action.chat" {
			return json.Marshal(map[string]any{
				"status":             "ok",
				"actionId":           req.ActionID,
				"opensRoute":         "/chat",
				"requiresForeground": true,
			})
		}

		toolID, ok := shortcutActionToolID(req.ActionID)
		if !ok {
			return json.Marshal(map[string]any{
				"status":   "error",
				"error":    map[string]any{"code": "ACTION_NOT_AVAILABLE", "message": "unsupported canonical shortcut action"},
				"actionId": req.ActionID,
			})
		}

		input, err := json.Marshal(req.Payload)
		if err != nil {
			return nil, fmt.Errorf("encode shortcut parameters: %w", err)
		}
		scope := kernel.InvocationScope{
			SpaceID:       requestidentity.NormalizeSpaceID(spaceID),
			Channel:       "ios_shortcut",
			Trigger:       "app_intent",
			RequestID:     req.RequestID,
			CorrelationID: req.RequestID,
		}
		result, found := services.KernelContainer.ToolFacade.ExecuteTool(
			ctx,
			capability.CapabilityID(toolID),
			input,
			scope,
			"ios-shortcut-"+req.RequestID,
			"ios-shortcut-"+req.RequestID,
		)
		if !found {
			return json.Marshal(map[string]any{
				"status":   "error",
				"error":    map[string]any{"code": "TOOL_NOT_FOUND", "message": toolID},
				"actionId": req.ActionID,
			})
		}
		if result.Error != nil || strings.EqualFold(result.Status, "FAILED") {
			code := "TOOL_EXECUTION_FAILED"
			message := result.VisibleText
			if result.Error != nil {
				if result.Error.Code != "" {
					code = result.Error.Code
				}
				if result.Error.Message != "" {
					message = result.Error.Message
				}
			}
			return json.Marshal(map[string]any{
				"status":   "error",
				"error":    map[string]any{"code": code, "message": message},
				"actionId": req.ActionID,
			})
		}

		var output any = map[string]any{}
		if len(result.Output) > 0 {
			_ = json.Unmarshal(result.Output, &output)
		}
		return json.Marshal(map[string]any{
			"status":   "ok",
			"actionId": req.ActionID,
			"result":   output,
		})
	}
}

func shortcutActionToolID(actionID string) (string, bool) {
	switch actionID {
	case "com.amitia.action.reminder.add":
		return reminders.ToolIDCreate, true
	case "com.amitia.action.alarm.add":
		return alarms.ToolIDSchedule, true
	case "com.amitia.action.alarm.remove":
		return alarms.ToolIDCancel, true
	case "com.amitia.action.calendar.add":
		return calendar.ToolIDEventsCreate, true
	case "com.amitia.action.media.pick":
		return media.ToolIDPhotosPick, true
	case "com.amitia.action.media.export":
		return media.ToolIDPhotosExport, true
	default:
		return "", false
	}
}
