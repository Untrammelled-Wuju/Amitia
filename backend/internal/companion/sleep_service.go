// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

func (s *service) GetSleepSetting(characterID string) map[string]interface{} {
	var bed, wake string
	var enabled, sleepReplyEnabled int
	var sleepReplyMode string
	err := s.db.Table("sleep_settings").Select("bed_time, wake_time, enabled, COALESCE(sleep_reply_enabled, 0) as sleep_reply_enabled, COALESCE(sleep_reply_mode, 'NO_REPLY') as sleep_reply_mode").Where("character_id = ?", characterID).Limit(1).Row().Scan(&bed, &wake, &enabled, &sleepReplyEnabled, &sleepReplyMode)
	if err != nil {
		return map[string]interface{}{"bedTime": "23:00", "wakeTime": "07:00", "enabled": true, "sleepReplyEnabled": false, "sleepReplyMode": "NO_REPLY"}
	}
	return map[string]interface{}{"bedTime": bed, "wakeTime": wake, "enabled": enabled == 1, "sleepReplyEnabled": sleepReplyEnabled == 1, "sleepReplyMode": sleepReplyMode}
}

func (s *service) UpdateSleepSetting(body map[string]interface{}, characterID string) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["bedTime"].(string); ok {
		updates["bed_time"] = v
	}
	if v, ok := body["wakeTime"].(string); ok {
		updates["wake_time"] = v
	}
	if v, ok := body["enabled"].(bool); ok {
		if v {
			updates["enabled"] = 1
		} else {
			updates["enabled"] = 0
		}
	}
	if v, ok := body["sleepReplyEnabled"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				updates["sleep_reply_enabled"] = 1
			} else {
				updates["sleep_reply_enabled"] = 0
			}
		} else if f, ok2 := v.(float64); ok2 {
			updates["sleep_reply_enabled"] = int(f)
		}
	}
	if v, ok := body["sleepReplyMode"].(string); ok {
		updates["sleep_reply_mode"] = v
	}
	if len(updates) > 0 {
		var c64 int64
		s.db.Table("sleep_settings").Where("character_id = ?", characterID).Count(&c64)
		if c64 == 0 {
			s.db.Exec("INSERT INTO sleep_settings (character_id, bed_time, wake_time, enabled) VALUES (?, '23:00', '07:00', 1)", characterID)
		}
		s.db.Table("sleep_settings").Where("character_id = ?", characterID).Updates(updates)
		go s.scheduleChanged()
	}
	return s.GetSleepSetting(characterID)
}
