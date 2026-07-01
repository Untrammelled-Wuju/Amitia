// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"gorm.io/gorm"
)

func (s *service) ListFixedEvents(date string, characterID string) []map[string]interface{} {
	var events []FixedEvent
	q := s.db.Where("character_id = ?", characterID)
	if date != "" {
		dayOfWeek := parseDayOfWeek(date)
		q = q.Where("(week_day = ? OR week_day = -1)", dayOfWeek)
	}
	q.Order("start_time").Find(&events)
	result := make([]map[string]interface{}, len(events))
	for i, e := range events {
		result[i] = map[string]interface{}{"id": e.ID, "title": e.Title, "description": e.Description, "weekDay": e.WeekDay, "startTime": e.StartTime, "endTime": e.EndTime, "eventType": e.EventType, "repeatDays": e.RepeatDays, "prepareMinMinutes": e.PrepareMinMinutes, "prepareMaxMinutes": e.PrepareMaxMinutes, "replyMode": e.ReplyMode, "enabled": e.Enabled == 1}
	}
	return result
}

func (s *service) GetFixedEvent(id int) map[string]interface{} {
	var e FixedEvent
	s.db.First(&e, id)
	return map[string]interface{}{"id": e.ID, "title": e.Title, "description": e.Description, "weekDay": e.WeekDay, "startTime": e.StartTime, "endTime": e.EndTime, "eventType": e.EventType, "repeatDays": e.RepeatDays, "prepareMinMinutes": e.PrepareMinMinutes, "prepareMaxMinutes": e.PrepareMaxMinutes, "replyMode": e.ReplyMode, "enabled": e.Enabled == 1}
}

func (s *service) CreateFixedEvent(body map[string]interface{}, characterID string) map[string]interface{} {
	title := ""
	if v, ok := body["title"].(string); ok {
		title = v
	} else {
		title = "新事件"
	}
	e := FixedEvent{Title: title, CharacterID: characterID, EventType: "CUSTOM_BUSY", Enabled: 1, ReplyMode: "SHORT_REPLY"}
	if v, ok := body["description"].(string); ok {
		e.Description = v
	}
	if v, ok := body["weekDay"].(float64); ok {
		e.WeekDay = int(v)
	}
	if v, ok := body["startTime"].(string); ok {
		e.StartTime = v
	}
	if v, ok := body["endTime"].(string); ok {
		e.EndTime = v
	}
	if v, ok := body["eventType"].(string); ok {
		e.EventType = v
	}
	if v, ok := body["repeatType"].(string); ok {
		e.RepeatType = v
	}
	if v, ok := body["repeatDays"].(string); ok {
		e.RepeatDays = v
	}
	if v, ok := body["prepareMinMinutes"].(float64); ok {
		e.PrepareMinMinutes = int(v)
	}
	if v, ok := body["prepareMaxMinutes"].(float64); ok {
		e.PrepareMaxMinutes = int(v)
	}
	if v, ok := body["replyMode"].(string); ok {
		e.ReplyMode = v
	}
	s.db.Create(&e)
	go s.scheduleChanged()
	return s.GetFixedEvent(e.ID)
}

func (s *service) UpdateFixedEvent(id int, body map[string]interface{}, characterID string) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["title"].(string); ok {
		updates["title"] = v
	}
	if v, ok := body["description"].(string); ok {
		updates["description"] = v
	}
	if v, ok := body["weekDay"].(float64); ok {
		updates["week_day"] = int(v)
	}
	if v, ok := body["startTime"].(string); ok {
		updates["start_time"] = v
	}
	if v, ok := body["endTime"].(string); ok {
		updates["end_time"] = v
	}
	if v, ok := body["eventType"].(string); ok {
		updates["event_type"] = v
	}
	if v, ok := body["repeatDays"].(string); ok {
		updates["repeat_days"] = v
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
	if v, ok := body["enabled"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				updates["enabled"] = 1
			} else {
				updates["enabled"] = 0
			}
		} else if f, ok2 := v.(float64); ok2 {
			updates["enabled"] = int(f)
		}
	}
	if len(updates) > 0 {
		s.db.Model(&FixedEvent{}).Where("id = ? AND character_id = ?", id, characterID).Updates(updates)
		go s.scheduleChanged()
	}
	return s.GetFixedEvent(id)
}

func (s *service) DeleteFixedEvent(id int, characterID string) bool {
	ok := s.db.Where("id = ? AND character_id = ?", id, characterID).Delete(&FixedEvent{}).RowsAffected > 0
	if ok {
		go s.scheduleChanged()
	}
	return ok
}

func (s *service) ToggleFixedEventEnabled(id int) map[string]interface{} {
	s.db.Model(&FixedEvent{}).Where("id = ?", id).Update("enabled", gorm.Expr("CASE WHEN enabled = 1 THEN 0 ELSE 1 END"))
	return s.GetFixedEvent(id)
}
