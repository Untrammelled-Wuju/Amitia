// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

func (s *service) GetMoods() map[string]interface{} {
	var moods []map[string]interface{}
	s.db.Raw("SELECT DISTINCT mood as name, COUNT(*) as count, MAX(created_at) as lastDetected FROM messages WHERE mood IS NOT NULL AND mood != '' GROUP BY mood ORDER BY count DESC").Scan(&moods)
	if moods == nil {
		moods = []map[string]interface{}{}
	}
	return map[string]interface{}{"moods": moods}
}

func (s *service) GetMoodsByConversation(id string) map[string]interface{} {
	var moods []map[string]interface{}
	s.db.Table("messages").Where("conversation_id = ? AND mood IS NOT NULL AND mood != ''", id).Order("created_at DESC").Limit(50).Find(&moods)
	if moods == nil {
		moods = []map[string]interface{}{}
	}
	return map[string]interface{}{"moods": moods, "conversationId": id}
}

func (s *service) DeleteMood(id string) map[string]interface{} {
	s.db.Table("messages").Where("id = ?", id).Update("mood", "")
	return map[string]interface{}{"deleted": true}
}

func (s *service) DeleteMoodsByConversation(id string) map[string]interface{} {
	result := s.db.Table("messages").Where("conversation_id = ?", id).Update("mood", "")
	return map[string]interface{}{"deleted": true, "affectedRows": result.RowsAffected}
}
