// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

func (s *service) loadHistory(convID string) []map[string]string {
	return s.loadHistoryExcluding(convID, "")
}

func (s *service) loadHistoryExcluding(convID, excludeID string) []map[string]string {
	var messages []Message
	query := s.db.Where("conversation_id = ? AND include_in_context = 1", convID)
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}
	query.Order("sequence ASC").Find(&messages)
	if messages == nil {
		messages = []Message{}
	}
	history := make([]map[string]string, len(messages))
	for i, m := range messages {
		history[i] = map[string]string{"role": m.Role, "content": m.Content}
		if m.ImageUrl != "" {
			history[i]["imageUrl"] = m.ImageUrl
		}
	}
	return history
}

func (s *service) findRequestMessages(convID, requestID string) (Message, []Message, bool) {
	if convID == "" || requestID == "" {
		return Message{}, nil, false
	}
	var user Message
	userFound := s.db.Where("conversation_id = ? AND request_id = ? AND role = ?", convID, requestID, "user").Order("sequence ASC").First(&user).Error == nil
	var assistants []Message
	s.db.Where("conversation_id = ? AND request_id = ? AND role = ?", convID, requestID, "assistant").Order("sequence ASC").Find(&assistants)
	return user, assistants, userFound
}
