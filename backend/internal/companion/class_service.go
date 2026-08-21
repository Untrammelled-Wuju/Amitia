// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package companion

import (
	"fmt"
	"sort"
	"time"
)

func (s *service) ListClassAdjustments(characterID string) []map[string]interface{} {
	var items []ClassAdjustment
	s.db.Where("character_id = ?", characterID).Order("date, slot_index").Find(&items)
	result := make([]map[string]interface{}, len(items))
	for i, a := range items {
		result[i] = map[string]interface{}{"id": a.ID, "date": a.Date, "slotIndex": a.SlotIndex, "className": a.ClassName, "adjustType": a.AdjustType, "description": a.Description}
	}
	return result
}

func (s *service) CreateClassAdjustment(body map[string]interface{}, characterID string) map[string]interface{} {
	a := ClassAdjustment{CharacterID: characterID, AdjustType: "swap"}
	if v, ok := body["date"].(string); ok {
		a.Date = v
	}
	if v, ok := body["slotIndex"].(float64); ok {
		a.SlotIndex = int(v)
	}
	if v, ok := body["className"].(string); ok {
		a.ClassName = v
	}
	if v, ok := body["adjustType"].(string); ok {
		a.AdjustType = v
	}
	if v, ok := body["description"].(string); ok {
		a.Description = v
	}
	s.db.Create(&a)
	go s.scheduleChanged()
	return map[string]interface{}{"id": a.ID, "className": a.ClassName}
}

func (s *service) UpdateClassAdjustment(id int, body map[string]interface{}, characterID string) map[string]interface{} {
	updates := make(map[string]interface{})
	if v, ok := body["date"].(string); ok {
		updates["date"] = v
	}
	if v, ok := body["slotIndex"].(float64); ok {
		updates["slot_index"] = int(v)
	}
	if v, ok := body["className"].(string); ok {
		updates["class_name"] = v
	}
	if v, ok := body["adjustType"].(string); ok {
		updates["adjust_type"] = v
	}
	if v, ok := body["description"].(string); ok {
		updates["description"] = v
	}
	if len(updates) > 0 {
		s.db.Model(&ClassAdjustment{}).Where("id = ? AND character_id = ?", id, characterID).Updates(updates)
		go s.scheduleChanged()
	}
	return map[string]interface{}{"id": id, "updated": true}
}

func (s *service) DeleteClassAdjustment(id int, characterID string) bool {
	ok := s.db.Where("id = ? AND character_id = ?", id, characterID).Delete(&ClassAdjustment{}).RowsAffected > 0
	if ok {
		go s.scheduleChanged()
	}
	return ok
}

func (s *service) GetEffectiveClasses(date string, characterID string) []map[string]interface{} {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	type classSlot struct {
		Title          string `json:"title"`
		StartTime      string `json:"startTime"`
		EndTime        string `json:"endTime"`
		Location       string `json:"location"`
		SourceType     string `json:"sourceType"`
		AdjustmentType string `json:"adjustmentType"`
	}

	var adjustments []ClassAdjustment
	s.db.Where("date = ? AND character_id = ?", date, characterID).Order("slot_index ASC").Find(&adjustments)

	var slots []classSlot
	for _, adj := range adjustments {
		startHour := 8 + adj.SlotIndex
		startTime := fmt.Sprintf("%02d:00", startHour)
		endTime := fmt.Sprintf("%02d:50", startHour)
		slot := classSlot{
			Title:          adj.ClassName,
			StartTime:      startTime,
			EndTime:        endTime,
			Location:       "教室",
			SourceType:     "class_adjustment",
			AdjustmentType: adj.AdjustType,
		}
		if adj.AdjustType == "canceled" {
			continue
		}
		slots = append(slots, slot)
	}

	var specials []SpecialEvent
	s.db.Where("enabled = 1 AND start_date = ? AND character_id = ?", date, characterID).Find(&specials)
	for _, sp := range specials {
		if sp.EventType == "EXAM" || sp.EventType == "EXAM_WEEK" || sp.EventType == "LIBRARY_STUDY" {
			slots = append(slots, classSlot{
				Title: sp.Title, StartTime: sp.StartTime, EndTime: sp.EndTime,
				Location: "", SourceType: "special_event",
				AdjustmentType: sp.EventType,
			})
		}
	}

	sort.Slice(slots, func(i, j int) bool { return slots[i].StartTime < slots[j].StartTime })

	result := make([]map[string]interface{}, len(slots))
	for i, s := range slots {
		result[i] = map[string]interface{}{
			"title": s.Title, "startTime": s.StartTime, "endTime": s.EndTime,
			"location": s.Location, "sourceType": s.SourceType,
			"adjustmentType": s.AdjustmentType,
		}
	}
	if result == nil {
		result = []map[string]interface{}{}
	}
	return []map[string]interface{}{{"date": date, "dayOfWeek": parseDayOfWeek(date), "slots": result}}
}
