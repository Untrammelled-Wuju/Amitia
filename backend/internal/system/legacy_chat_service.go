// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/chat"
)

func (s *service) LegacyListConversations() map[string]interface{} {
	var convs []map[string]interface{}
	s.db.Table("conversations").Order("updated_at DESC").Limit(50).Find(&convs)
	if convs == nil {
		convs = []map[string]interface{}{}
	}
	for i, c := range convs {
		var count int64
		s.db.Table("messages").Where("conversation_id = ?", c["id"]).Count(&count)
		convs[i]["messageCount"] = count
	}
	return map[string]interface{}{"conversations": convs}
}

func (s *service) LegacyGetMessages(id string, page, pageSize int) map[string]interface{} {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	var total int64
	s.db.Table("messages").Where("conversation_id = ?", id).Count(&total)

	// 查询足够的原始消息以确保过滤后仍有 pageSize 条
	queryLimit := pageSize
	for {
		var raw []map[string]interface{}
		s.db.Table("messages").Where("conversation_id = ?", id).Order("created_at ASC").Limit(queryLimit).Offset(offset).Find(&raw)
		var msgs []map[string]interface{}
		for _, m := range raw {
			role := fmt.Sprint(m["role"])
			content := fmt.Sprint(m["content"])
			if role == "tool" {
				continue
			}
			if role == "assistant" && (content == "" || content == "<nil>") {
				continue
			}
			if v, ok := m["audio_url"]; ok {
				m["audioUrl"] = v
				delete(m, "audio_url")
			}
			if v, ok := m["audio_duration"]; ok {
				m["audioDuration"] = v
				delete(m, "audio_duration")
			}
			if v, ok := m["msg_type"]; ok {
				m["msgType"] = v
				delete(m, "msg_type")
			}
			if v, ok := m["image_url"]; ok && v != nil && v != "" {
				imageUrl := fmt.Sprint(v)
				if strings.HasPrefix(imageUrl, "data:") {
					newPath := chat.SaveImageFromDataURI(imageUrl)
					if newPath != imageUrl {
						m["imageUrl"] = newPath
						go s.db.Exec("UPDATE messages SET image_url = ? WHERE id = ?", newPath, m["id"])
					} else {
						m["imageUrl"] = imageUrl
					}
				} else {
					m["imageUrl"] = imageUrl
				}
				delete(m, "image_url")
			}
			if v, ok := m["video_url"]; ok && v != nil && v != "" {
				m["videoUrl"] = v
				delete(m, "video_url")
			}
			if v, ok := m["reply_to_message_id"]; ok && v != nil {
				m["replyToMessageId"] = v
				delete(m, "reply_to_message_id")
			}
			if v, ok := m["reply_to_role"]; ok && v != nil {
				m["replyToRole"] = v
				delete(m, "reply_to_role")
			}
			if v, ok := m["reply_to_excerpt"]; ok && v != nil {
				m["replyToExcerpt"] = v
				delete(m, "reply_to_excerpt")
			}
			msgs = append(msgs, m)
		}
		if len(msgs) >= pageSize || int64(offset+queryLimit) >= total {
			if msgs == nil {
				msgs = []map[string]interface{}{}
			}
			totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
			return map[string]interface{}{"items": msgs, "total": total, "page": page, "pageSize": pageSize, "totalPages": totalPages}
		}
		queryLimit += pageSize
		if int64(queryLimit) > total {
			queryLimit = int(total)
		}
	}
}

func (s *service) LegacyDeleteConversation(id string) map[string]interface{} {
	s.db.Table("messages").Where("conversation_id = ?", id).Delete(nil)
	s.db.Table("conversations").Where("id = ?", id).Delete(nil)
	return map[string]interface{}{"deleted": true}
}
