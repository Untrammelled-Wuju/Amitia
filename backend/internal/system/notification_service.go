// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"time"
)

func (s *service) GetNotificationsSettings() map[string]interface{} {
	enabled := s.getAppSetting("notifications_enabled") != "false"
	return map[string]interface{}{"enabled": enabled, "webPush": true, "desktopNotify": true}
}

func (s *service) UpdateNotificationsSettings(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["enabled"].(bool); ok {
		if v {
			s.setAppSetting("notifications_enabled", "true")
		} else {
			s.setAppSetting("notifications_enabled", "false")
		}
	}
	return s.GetNotificationsSettings()
}

func (s *service) GetNotificationsStatus() map[string]interface{} {
	enabled := s.getAppSetting("notifications_enabled") != "false"
	return map[string]interface{}{"enabled": enabled}
}

func (s *service) NotificationsSubscribe(body map[string]interface{}) map[string]interface{} {
	s.setAppSetting("notifications_enabled", "true")
	return map[string]interface{}{"subscribed": true}
}

func (s *service) NotificationsUnsubscribe() map[string]interface{} {
	s.setAppSetting("notifications_enabled", "false")
	return map[string]interface{}{"unsubscribed": true}
}

func (s *service) NotificationsTest() map[string]interface{} {
	return map[string]interface{}{"sent": true, "sentAt": time.Now().Format(time.DateTime)}
}
