package system

import (
	"context"
	"errors"
	"strings"
	"time"
)

func (s *service) NotificationsContext(ctx context.Context, operation string, body map[string]interface{}, spaceID, deviceID string) (map[string]interface{}, error) {
	enabledKey := notificationSettingKey(spaceID, deviceID, "enabled")
	subscribedKey := notificationSettingKey(spaceID, deviceID, "subscribed")
	settings := map[string]string{}
	switch operation {
	case "settings", "status", "test":
	case "update":
		if enabled, ok := body["enabled"].(bool); ok {
			settings[enabledKey] = boolSetting(enabled)
			if !enabled {
				settings[subscribedKey] = "false"
			}
		}
	case "subscribe":
		settings[enabledKey], settings[subscribedKey] = "true", "true"
	case "unsubscribe":
		settings[enabledKey], settings[subscribedKey] = "false", "false"
	default:
		return nil, errors.New("未知通知操作")
	}
	current, err := s.applyAppSettings(ctx, settings)
	if err != nil {
		return nil, err
	}
	enabled := current[enabledKey] != "false"
	subscribed := current[subscribedKey] == "true"
	result := map[string]interface{}{"enabled": enabled, "subscribed": subscribed, "deliveryMode": "client-local", "deviceId": strings.TrimSpace(deviceID)}
	if operation == "test" {
		accepted := enabled && subscribed
		result = map[string]interface{}{"accepted": accepted, "sent": false, "checkedAt": time.Now().UTC().Format(time.RFC3339), "deliveryMode": "client-local", "deviceId": strings.TrimSpace(deviceID)}
		if accepted {
			result["reason"] = "client must deliver the local notification"
		} else {
			result["reason"] = "notifications are disabled or not subscribed for this device"
		}
	}
	return result, nil
}
