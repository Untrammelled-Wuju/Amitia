// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import "time"

func (s *service) GetNotificationsSettings() map[string]interface{} {
	enabled := s.getAppSetting("notifications_enabled") != "false"
	subscribed := s.getAppSetting("notifications_subscribed") == "true"
	return map[string]interface{}{
		"enabled":      enabled,
		"subscribed":   subscribed,
		"deliveryMode": "client-local",
	}
}

func (s *service) UpdateNotificationsSettings(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["enabled"].(bool); ok {
		s.setAppSetting("notifications_enabled", boolSetting(v))
		if !v {
			s.setAppSetting("notifications_subscribed", "false")
		}
	}
	return s.GetNotificationsSettings()
}

func (s *service) GetNotificationsStatus() map[string]interface{} {
	settings := s.GetNotificationsSettings()
	return map[string]interface{}{
		"enabled":      settings["enabled"],
		"subscribed":   settings["subscribed"],
		"deliveryMode": "client-local",
	}
}

func (s *service) NotificationsSubscribe(body map[string]interface{}) map[string]interface{} {
	s.setAppSetting("notifications_enabled", "true")
	s.setAppSetting("notifications_subscribed", "true")
	return map[string]interface{}{
		"enabled":      true,
		"subscribed":   true,
		"deliveryMode": "client-local",
	}
}

func (s *service) NotificationsUnsubscribe() map[string]interface{} {
	s.setAppSetting("notifications_subscribed", "false")
	s.setAppSetting("notifications_enabled", "false")
	return map[string]interface{}{
		"enabled":      false,
		"subscribed":   false,
		"deliveryMode": "client-local",
	}
}

func (s *service) NotificationsTest() map[string]interface{} {
	settings := s.GetNotificationsSettings()
	enabled, _ := settings["enabled"].(bool)
	subscribed, _ := settings["subscribed"].(bool)
	accepted := enabled && subscribed
	result := map[string]interface{}{
		"accepted":     accepted,
		"sent":         false,
		"checkedAt":    time.Now().UTC().Format(time.RFC3339),
		"deliveryMode": "client-local",
	}
	if !accepted {
		result["reason"] = "notifications are disabled or not subscribed"
	} else {
		result["reason"] = "client must deliver the local notification"
	}
	return result
}

func boolSetting(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
