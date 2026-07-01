// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"strings"
)

func (s *service) CheckSafety(text string) map[string]interface{} {
	for _, kw := range []string{"suicide", "self-harm", "violence"} {
		if strings.Contains(strings.ToLower(text), kw) {
			return map[string]interface{}{"safe": false, "type": "severe", "reason": "High-risk content detected", "action": "block"}
		}
	}
	for _, kw := range []string{"password", "credit card", "id number", "bank account"} {
		if strings.Contains(strings.ToLower(text), kw) {
			return map[string]interface{}{"safe": false, "type": "privacy", "reason": "Sensitive information detected", "action": "warn"}
		}
	}
	return map[string]interface{}{"safe": true, "type": "", "reason": "", "action": "allow"}
}

func (s *service) SafetyEvents(page, pageSize int) map[string]interface{} {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	s.db.Table("safety_events").Count(&total)
	var items []map[string]interface{}
	offset := (page - 1) * pageSize
	s.db.Raw("SELECT id, conversation_id AS conversationId, event_type AS eventType, description, COALESCE(direction, '') AS direction, handled, created_at AS createdAt FROM safety_events ORDER BY created_at DESC LIMIT ? OFFSET ?", pageSize, offset).Scan(&items)
	if items == nil {
		items = []map[string]interface{}{}
	}
	return map[string]interface{}{"items": items, "total": total}
}

func (s *service) DeleteSafetyEvents() map[string]interface{} {
	s.db.Exec("DELETE FROM safety_events")
	return map[string]interface{}{"deleted": true}
}

func (s *service) HandleSafetyEvent(id string) map[string]interface{} {
	s.db.Table("safety_events").Where("id = ?", id).Update("handled", 1)
	return map[string]interface{}{"handled": true, "id": id}
}

func (s *service) SafetyImportCheck(body map[string]interface{}) map[string]interface{} {
	if text, ok := body["text"].(string); ok {
		return s.CheckSafety(text)
	}
	return map[string]interface{}{"passed": true}
}
