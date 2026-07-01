// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

func (s *service) GetTimeline(page, pageSize int, userID, source, memoryType, timelineType string) ([]map[string]interface{}, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 30
	}

	var allEvents []map[string]interface{}

	if timelineType == "" || timelineType == "memory" || timelineType == "structured" {
		query := s.db.Table("memory_events")
		if source != "" {
			query = query.Where("source = ?", source)
		}
		if memoryType != "" {
			query = query.Where("memory_type = ?", memoryType)
		}
		var events []map[string]interface{}
		err := query.Order("created_at DESC").Find(&events).Error
		if err != nil {
			return nil, 0, err
		}
		if events == nil {
			events = []map[string]interface{}{}
		}
		for _, e := range events {
			e["timelineType"] = "memory"
			allEvents = append(allEvents, e)
		}
	}

	if timelineType == "" || timelineType == "episodic" {
		var episodics []map[string]interface{}
		eq := s.db.Table("episodic_memories")
		if userID != "" {
			eq = eq.Where("user_id = ?", userID)
		}
		err := eq.Order("created_at DESC").Find(&episodics).Error
		if err != nil {
			return nil, 0, err
		}
		if episodics == nil {
			episodics = []map[string]interface{}{}
		}
		for _, e := range episodics {
			e["timelineType"] = "episodic"
			allEvents = append(allEvents, e)
		}
	}

	sort.Slice(allEvents, func(i, j int) bool {
		ti, _ := allEvents[i]["created_at"].(string)
		tj, _ := allEvents[j]["created_at"].(string)
		if ti == "" {
			ti2, _ := allEvents[i]["createdAt"].(string)
			tj2, _ := allEvents[j]["createdAt"].(string)
			return ti2 > tj2
		}
		return ti > tj
	})

	total := int64(len(allEvents))
	start := (page - 1) * pageSize
	if start >= int(total) {
		return []map[string]interface{}{}, total, nil
	}
	end := start + pageSize
	if end > int(total) {
		end = int(total)
	}
	return allEvents[start:end], total, nil
}

func (s *service) logEvent(memoryID, eventType, key, value, memoryType string, importance int, source, characterID string) {
	id := uuid.New().String()
	now := time.Now().Format("2006-01-02 15:04:05")
	s.db.Exec(
		"INSERT INTO memory_events (id, memory_id, event_type, key, value, memory_type, importance, source, character_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, memoryID, eventType, key, value, memoryType, importance, source, characterID, now,
	)
}
