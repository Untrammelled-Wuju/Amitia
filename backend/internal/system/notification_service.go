// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// Notification settings are client-local by design. In Cloud mode they must
// therefore be scoped to a concrete client device; otherwise toggling one
// phone/desktop silently changes every other device sharing the same Core.
// Empty device IDs still remain user-scoped on a shared Core. Only the
// historical local single-user path, where both user and device are empty,
// falls back to the legacy global keys.
func (s *service) GetNotificationsSettings(userID, deviceID string) map[string]interface{} {
	enabledKey := notificationSettingKey(userID, deviceID, "enabled")
	subscribedKey := notificationSettingKey(userID, deviceID, "subscribed")
	enabled := s.getAppSetting(enabledKey) != "false"
	subscribed := s.getAppSetting(subscribedKey) == "true"
	return map[string]interface{}{
		"enabled":      enabled,
		"subscribed":   subscribed,
		"deliveryMode": "client-local",
		"deviceId":     strings.TrimSpace(deviceID),
	}
}

func (s *service) UpdateNotificationsSettings(body map[string]interface{}, userID, deviceID string) map[string]interface{} {
	enabledKey := notificationSettingKey(userID, deviceID, "enabled")
	subscribedKey := notificationSettingKey(userID, deviceID, "subscribed")
	if v, ok := body["enabled"].(bool); ok {
		s.setAppSetting(enabledKey, boolSetting(v))
		if !v {
			s.setAppSetting(subscribedKey, "false")
		}
	}
	return s.GetNotificationsSettings(userID, deviceID)
}

func (s *service) GetNotificationsStatus(userID, deviceID string) map[string]interface{} {
	settings := s.GetNotificationsSettings(userID, deviceID)
	return map[string]interface{}{
		"enabled":      settings["enabled"],
		"subscribed":   settings["subscribed"],
		"deliveryMode": "client-local",
		"deviceId":     settings["deviceId"],
	}
}

func (s *service) NotificationsSubscribe(body map[string]interface{}, userID, deviceID string) map[string]interface{} {
	s.setAppSetting(notificationSettingKey(userID, deviceID, "enabled"), "true")
	s.setAppSetting(notificationSettingKey(userID, deviceID, "subscribed"), "true")
	return map[string]interface{}{
		"enabled":      true,
		"subscribed":   true,
		"deliveryMode": "client-local",
		"deviceId":     strings.TrimSpace(deviceID),
	}
}

func (s *service) NotificationsUnsubscribe(userID, deviceID string) map[string]interface{} {
	s.setAppSetting(notificationSettingKey(userID, deviceID, "subscribed"), "false")
	s.setAppSetting(notificationSettingKey(userID, deviceID, "enabled"), "false")
	return map[string]interface{}{
		"enabled":      false,
		"subscribed":   false,
		"deliveryMode": "client-local",
		"deviceId":     strings.TrimSpace(deviceID),
	}
}

func (s *service) NotificationsTest(userID, deviceID string) map[string]interface{} {
	settings := s.GetNotificationsSettings(userID, deviceID)
	enabled, _ := settings["enabled"].(bool)
	subscribed, _ := settings["subscribed"].(bool)
	accepted := enabled && subscribed
	result := map[string]interface{}{
		"accepted":     accepted,
		"sent":         false,
		"checkedAt":    time.Now().UTC().Format(time.RFC3339),
		"deliveryMode": "client-local",
		"deviceId":     strings.TrimSpace(deviceID),
	}
	if !accepted {
		result["reason"] = "notifications are disabled or not subscribed for this device"
	} else {
		result["reason"] = "client must deliver the local notification"
	}
	return result
}

func notificationSettingKey(userID, deviceID, suffix string) string {
	userID = strings.TrimSpace(userID)
	deviceID = strings.TrimSpace(deviceID)
	if userID == "" && deviceID == "" {
		return "notifications_" + suffix
	}
	scope := userID + "\x00" + deviceID
	sum := sha256.Sum256([]byte(scope))
	return "notifications_device_" + hex.EncodeToString(sum[:12]) + "_" + suffix
}

func boolSetting(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
