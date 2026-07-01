// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"gorm.io/gorm"
)

func (s *service) ListSpecialEvents(characterID string) []map[string]interface{} {
	var events []SpecialEvent
	s.db.Where("character_id = ?", characterID).Order("start_date, start_time").Find(&events)
	result := make([]map[string]interface{}, len(events))
	for i, e := range events {
		result[i] = map[string]interface{}{"id": e.ID, "title": e.Title, "description": e.Description, "startDate": e.StartDate, "endDate": e.EndDate, "startTime": e.StartTime, "endTime": e.EndTime, "eventType": e.EventType, "enabled": e.Enabled == 1, "priority": e.Priority, "activeMessageAllowed": e.ActiveMessageAllowed == 1, "replyMode": e.ReplyMode, "affectSchedule": e.AffectSchedule == 1, "affectSleep": e.AffectSleep == 1, "affectMeal": e.AffectMeal == 1, "affectEnergy": e.AffectEnergy == 1}
	}
	return result
}

func (s *service) CreateSpecialEvent(body map[string]interface{}, characterID string) map[string]interface{} {
	title := ""
	if v, ok := body["title"].(string); ok {
		title = v
	} else {
		title = "特殊事件"
	}
	e := SpecialEvent{Title: title, CharacterID: characterID, EventType: "CUSTOM", Enabled: 1, ReplyMode: "SHORT_REPLY", ActiveMessageAllowed: 1}
	if v, ok := body["description"].(string); ok {
		e.Description = v
	}
	if v, ok := body["startDate"].(string); ok {
		e.StartDate = v
	}
	if v, ok := body["endDate"].(string); ok {
		e.EndDate = v
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
	if v, ok := body["replyMode"].(string); ok {
		e.ReplyMode = v
	}
	if v, ok := body["affectSleep"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				e.AffectSleep = 1
			}
		}
	}
	if v, ok := body["affectSchedule"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				e.AffectSchedule = 1
			}
		}
	}
	if v, ok := body["affectMeal"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				e.AffectMeal = 1
			}
		}
	}
	if v, ok := body["affectEnergy"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				e.AffectEnergy = 1
			}
		}
	}
	if v, ok := body["priority"].(float64); ok {
		e.Priority = int(v)
	}
	s.db.Create(&e)
	go s.scheduleChanged()
	return map[string]interface{}{"id": e.ID, "title": e.Title, "startDate": e.StartDate, "endDate": e.EndDate}
}

func (s *service) UpdateSpecialEvent(id int, body map[string]interface{}, characterID string) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["title"].(string); ok {
		updates["title"] = v
	}
	if v, ok := body["description"].(string); ok {
		updates["description"] = v
	}
	if v, ok := body["startDate"].(string); ok {
		updates["start_date"] = v
	}
	if v, ok := body["endDate"].(string); ok {
		updates["end_date"] = v
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
	if v, ok := body["replyMode"].(string); ok {
		updates["reply_mode"] = v
	}
	if v, ok := body["affectSleep"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				updates["affect_sleep"] = 1
			} else {
				updates["affect_sleep"] = 0
			}
		}
	}
	if v, ok := body["affectSchedule"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				updates["affect_schedule"] = 1
			} else {
				updates["affect_schedule"] = 0
			}
		}
	}
	if v, ok := body["affectMeal"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				updates["affect_meal"] = 1
			} else {
				updates["affect_meal"] = 0
			}
		}
	}
	if v, ok := body["affectEnergy"]; ok {
		if b, ok2 := v.(bool); ok2 {
			if b {
				updates["affect_energy"] = 1
			} else {
				updates["affect_energy"] = 0
			}
		}
	}
	if v, ok := body["priority"].(float64); ok {
		updates["priority"] = int(v)
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
		s.db.Model(&SpecialEvent{}).Where("id = ? AND character_id = ?", id, characterID).Updates(updates)
		go s.scheduleChanged()
	}
	return map[string]interface{}{"id": id, "updated": true}
}

func (s *service) DeleteSpecialEvent(id int, characterID string) bool {
	ok := s.db.Where("id = ? AND character_id = ?", id, characterID).Delete(&SpecialEvent{}).RowsAffected > 0
	if ok {
		go s.scheduleChanged()
	}
	return ok
}

func (s *service) ToggleSpecialEventEnabled(id int) map[string]interface{} {
	s.db.Model(&SpecialEvent{}).Where("id = ?", id).Update("enabled", gorm.Expr("CASE WHEN enabled = 1 THEN 0 ELSE 1 END"))
	return map[string]interface{}{"id": id, "toggled": true}
}
