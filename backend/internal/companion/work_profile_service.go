// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

func (s *service) GetWorkProfile(characterID string) map[string]interface{} {
	var w WorkProfile
	s.db.Table("work_profiles").Where("character_id = ?", characterID).Limit(1).Find(&w)
	if w.ID == 0 {
		return map[string]interface{}{"enabled": false, "workDays": "MON,TUE,WED,THU,FRI", "workStartTime": "09:00", "workEndTime": "18:00", "lunchBreakStartTime": "12:00", "lunchBreakEndTime": "13:30", "commuteMinMinutes": 15, "commuteMaxMinutes": 45, "prepareMinMinutes": 20, "prepareMaxMinutes": 60, "replyMode": "SHORT_REPLY", "allowOvertime": false, "overtimeProbability": 10, "overtimeMinMinutes": 30, "overtimeMaxMinutes": 180, "overtimeReplyMode": "SHORT_REPLY", "delayedReplyEnabled": false, "commuteHomeShareEnabled": true, "commuteHomeShareProbability": 60}
	}
	return map[string]interface{}{"id": w.ID, "enabled": w.Enabled == 1, "workDays": w.WorkDays, "workStartTime": w.WorkStartTime, "workEndTime": w.WorkEndTime, "lunchBreakStartTime": w.LunchBreakStartTime, "lunchBreakEndTime": w.LunchBreakEndTime, "commuteMinMinutes": w.CommuteMinMinutes, "commuteMaxMinutes": w.CommuteMaxMinutes, "prepareMinMinutes": w.PrepareMinMinutes, "prepareMaxMinutes": w.PrepareMaxMinutes, "replyMode": w.ReplyMode, "allowOvertime": w.AllowOvertime == 1, "overtimeProbability": w.OvertimeProbability, "overtimeMinMinutes": w.OvertimeMinMinutes, "overtimeMaxMinutes": w.OvertimeMaxMinutes, "overtimeReplyMode": w.OvertimeReplyMode, "delayedReplyEnabled": w.DelayedReplyEnabled == 1, "commuteHomeShareEnabled": w.CommuteHomeShareEnabled == 1, "commuteHomeShareProbability": w.CommuteHomeShareProbability}
}

func parseWorkProfileUpdates(body map[string]interface{}) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["enabled"].(bool); ok {
		if v {
			updates["enabled"] = 1
		} else {
			updates["enabled"] = 0
		}
	}
	if v, ok := body["workDays"].(string); ok {
		updates["work_days"] = v
	}
	if v, ok := body["workStartTime"].(string); ok {
		updates["work_start_time"] = v
	}
	if v, ok := body["workEndTime"].(string); ok {
		updates["work_end_time"] = v
	}
	if v, ok := body["lunchBreakStartTime"].(string); ok {
		updates["lunch_break_start_time"] = v
	}
	if v, ok := body["lunchBreakEndTime"].(string); ok {
		updates["lunch_break_end_time"] = v
	}
	if v, ok := body["commuteMinMinutes"].(float64); ok {
		updates["commute_min_minutes"] = int(v)
	}
	if v, ok := body["commuteMaxMinutes"].(float64); ok {
		updates["commute_max_minutes"] = int(v)
	}
	if v, ok := body["prepareMinMinutes"].(float64); ok {
		updates["prepare_min_minutes"] = int(v)
	}
	if v, ok := body["prepareMaxMinutes"].(float64); ok {
		updates["prepare_max_minutes"] = int(v)
	}
	if v, ok := body["replyMode"].(string); ok {
		updates["reply_mode"] = v
	}
	if v, ok := body["allowOvertime"].(bool); ok {
		if v {
			updates["allow_overtime"] = 1
		} else {
			updates["allow_overtime"] = 0
		}
	}
	if v, ok := body["overtimeProbability"].(float64); ok {
		updates["overtime_probability"] = int(v)
	}
	if v, ok := body["overtimeMinMinutes"].(float64); ok {
		updates["overtime_min_minutes"] = int(v)
	}
	if v, ok := body["overtimeMaxMinutes"].(float64); ok {
		updates["overtime_max_minutes"] = int(v)
	}
	if v, ok := body["overtimeReplyMode"].(string); ok {
		updates["overtime_reply_mode"] = v
	}
	if v, ok := body["delayedReplyEnabled"].(bool); ok {
		if v {
			updates["delayed_reply_enabled"] = 1
		} else {
			updates["delayed_reply_enabled"] = 0
		}
	}
	if v, ok := body["commuteHomeShareEnabled"].(bool); ok {
		if v {
			updates["commute_home_share_enabled"] = 1
		} else {
			updates["commute_home_share_enabled"] = 0
		}
	}
	if v, ok := body["commuteHomeShareProbability"].(float64); ok {
		updates["commute_home_share_probability"] = int(v)
	}
	return updates
}

func (s *service) UpdateWorkProfile(body map[string]interface{}, characterID string) map[string]interface{} {
	var count int64
	s.db.Model(&WorkProfile{}).Where("character_id = ?", characterID).Count(&count)
	if count == 0 {
		s.db.Create(&WorkProfile{CharacterID: characterID})
	}
	updates := parseWorkProfileUpdates(body)
	if len(updates) > 0 {
		s.db.Model(&WorkProfile{}).Where("character_id = ?", characterID).Updates(updates)
		go s.scheduleChanged()
	}
	result := map[string]interface{}{"id": 0, "enabled": false, "workDays": "MON,TUE,WED,THU,FRI", "workStartTime": "09:00", "workEndTime": "18:00", "lunchBreakStartTime": "12:00", "lunchBreakEndTime": "13:30", "commuteMinMinutes": 15, "commuteMaxMinutes": 45, "prepareMinMinutes": 20, "prepareMaxMinutes": 60, "replyMode": "SHORT_REPLY", "allowOvertime": false, "overtimeProbability": 10, "overtimeMinMinutes": 30, "overtimeMaxMinutes": 180, "overtimeReplyMode": "SHORT_REPLY", "delayedReplyEnabled": false, "commuteHomeShareEnabled": true, "commuteHomeShareProbability": 60}
	if v, ok := body["enabled"]; ok {
		if b, ok2 := v.(bool); ok2 {
			result["enabled"] = b
		}
	}
	if v, ok := body["workDays"].(string); ok {
		result["workDays"] = v
	}
	if v, ok := body["workStartTime"].(string); ok {
		result["workStartTime"] = v
	}
	if v, ok := body["workEndTime"].(string); ok {
		result["workEndTime"] = v
	}
	if v, ok := body["lunchBreakStartTime"].(string); ok {
		result["lunchBreakStartTime"] = v
	}
	if v, ok := body["lunchBreakEndTime"].(string); ok {
		result["lunchBreakEndTime"] = v
	}
	if v, ok := body["commuteMinMinutes"].(float64); ok {
		result["commuteMinMinutes"] = int(v)
	}
	if v, ok := body["commuteMaxMinutes"].(float64); ok {
		result["commuteMaxMinutes"] = int(v)
	}
	if v, ok := body["prepareMinMinutes"].(float64); ok {
		result["prepareMinMinutes"] = int(v)
	}
	if v, ok := body["prepareMaxMinutes"].(float64); ok {
		result["prepareMaxMinutes"] = int(v)
	}
	if v, ok := body["replyMode"].(string); ok {
		result["replyMode"] = v
	}
	if v, ok := body["allowOvertime"]; ok {
		if b, ok2 := v.(bool); ok2 {
			result["allowOvertime"] = b
		}
	}
	if v, ok := body["overtimeProbability"].(float64); ok {
		result["overtimeProbability"] = int(v)
	}
	if v, ok := body["overtimeMinMinutes"].(float64); ok {
		result["overtimeMinMinutes"] = int(v)
	}
	if v, ok := body["overtimeMaxMinutes"].(float64); ok {
		result["overtimeMaxMinutes"] = int(v)
	}
	if v, ok := body["overtimeReplyMode"].(string); ok {
		result["overtimeReplyMode"] = v
	}
	if v, ok := body["delayedReplyEnabled"]; ok {
		if b, ok2 := v.(bool); ok2 {
			result["delayedReplyEnabled"] = b
		}
	}
	if v, ok := body["commuteHomeShareEnabled"]; ok {
		if b, ok2 := v.(bool); ok2 {
			result["commuteHomeShareEnabled"] = b
		}
	}
	if v, ok := body["commuteHomeShareProbability"].(float64); ok {
		result["commuteHomeShareProbability"] = int(v)
	}
	return result
}
