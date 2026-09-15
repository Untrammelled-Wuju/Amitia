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
func (s *service) GetNotificationsSettings(spaceID, deviceID string) map[string]interface{} {
	enabledKey := notificationSettingKey(spaceID, deviceID, "enabled")
	subscribedKey := notificationSettingKey(spaceID, deviceID, "subscribed")
	enabled := s.getAppSetting(enabledKey) != "false"
	subscribed := s.getAppSetting(subscribedKey) == "true"
	return map[string]interface{}{
		"enabled":      enabled,
		"subscribed":   subscribed,
		"deliveryMode": "client-local",
		"deviceId":     strings.TrimSpace(deviceID),
	}
}

func (s *service) UpdateNotificationsSettings(body map[string]interface{}, spaceID, deviceID string) map[string]interface{} {
	enabledKey := notificationSettingKey(spaceID, deviceID, "enabled")
	subscribedKey := notificationSettingKey(spaceID, deviceID, "subscribed")
	if v, ok := body["enabled"].(bool); ok {
		s.setAppSetting(enabledKey, boolSetting(v))
		if !v {
			s.setAppSetting(subscribedKey, "false")
		}
	}
	return s.GetNotificationsSettings(spaceID, deviceID)
}

func (s *service) GetNotificationsStatus(spaceID, deviceID string) map[string]interface{} {
	settings := s.GetNotificationsSettings(spaceID, deviceID)
	return map[string]interface{}{
		"enabled":      settings["enabled"],
		"subscribed":   settings["subscribed"],
		"deliveryMode": "client-local",
		"deviceId":     settings["deviceId"],
	}
}

func (s *service) NotificationsSubscribe(body map[string]interface{}, spaceID, deviceID string) map[string]interface{} {
	s.setAppSetting(notificationSettingKey(spaceID, deviceID, "enabled"), "true")
	s.setAppSetting(notificationSettingKey(spaceID, deviceID, "subscribed"), "true")
	return map[string]interface{}{
		"enabled":      true,
		"subscribed":   true,
		"deliveryMode": "client-local",
		"deviceId":     strings.TrimSpace(deviceID),
	}
}

func (s *service) NotificationsUnsubscribe(spaceID, deviceID string) map[string]interface{} {
	s.setAppSetting(notificationSettingKey(spaceID, deviceID, "subscribed"), "false")
	s.setAppSetting(notificationSettingKey(spaceID, deviceID, "enabled"), "false")
	return map[string]interface{}{
		"enabled":      false,
		"subscribed":   false,
		"deliveryMode": "client-local",
		"deviceId":     strings.TrimSpace(deviceID),
	}
}

func (s *service) NotificationsTest(spaceID, deviceID string) map[string]interface{} {
	settings := s.GetNotificationsSettings(spaceID, deviceID)
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

func notificationSettingKey(spaceID, deviceID, suffix string) string {
	spaceID = strings.TrimSpace(spaceID)
	deviceID = strings.TrimSpace(deviceID)
	if spaceID == "" && deviceID == "" {
		return "notifications_" + suffix
	}
	scope := spaceID + "\x00" + deviceID
	sum := sha256.Sum256([]byte(scope))
	return "notifications_device_" + hex.EncodeToString(sum[:12]) + "_" + suffix
}

func boolSetting(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
