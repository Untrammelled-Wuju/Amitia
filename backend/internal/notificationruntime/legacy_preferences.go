package notificationruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func legacyNotificationSettingKey(spaceID, deviceID, suffix string) string {
	spaceID = strings.TrimSpace(spaceID)
	deviceID = strings.TrimSpace(deviceID)
	if spaceID == "" && deviceID == "" {
		return "notifications_" + suffix
	}
	scope := spaceID + "\x00" + deviceID
	sum := sha256.Sum256([]byte(scope))
	return "notifications_device_" + hex.EncodeToString(sum[:12]) + "_" + suffix
}

func (r *Runtime) migratedSystemNotificationEnabled(ctx context.Context, spaceID, deviceID string) (bool, bool) {
	if r == nil || r.db == nil {
		return false, false
	}
	enabledKey := legacyNotificationSettingKey(spaceID, deviceID, "enabled")
	subscribedKey := legacyNotificationSettingKey(spaceID, deviceID, "subscribed")
	var rows []struct {
		Key   string `gorm:"column:key"`
		Value string `gorm:"column:value"`
	}
	if err := r.db.WithContext(ctx).Table("app_settings").
		Select("key,value").
		Where("key IN ? AND deleted_at IS NULL", []string{enabledKey, subscribedKey}).
		Find(&rows).Error; err != nil {
		return false, false
	}
	if len(rows) == 0 {
		return false, false
	}
	enabled := true
	subscribed := false
	for _, row := range rows {
		switch row.Key {
		case enabledKey:
			enabled = !strings.EqualFold(strings.TrimSpace(row.Value), "false")
		case subscribedKey:
			subscribed = strings.EqualFold(strings.TrimSpace(row.Value), "true")
		}
	}
	return enabled && subscribed, true
}
