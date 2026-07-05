// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *service) GetImportsBatches() map[string]interface{} {
	var batches []map[string]interface{}
	s.db.Table("conversations").Where("source = ?", "import").Order("created_at DESC").Find(&batches)
	if batches == nil {
		batches = []map[string]interface{}{}
	}
	return map[string]interface{}{"batches": batches, "total": len(batches)}
}

func (s *service) GetImportsBatchDetail(id string) map[string]interface{} {
	var batch map[string]interface{}
	s.db.Table("conversations").Where("id = ? AND source = ?", id, "import").Limit(1).Scan(&batch)
	if batch == nil {
		batch = map[string]interface{}{}
	}
	var msgCount int64
	s.db.Table("messages").Where("conversation_id = ?", id).Count(&msgCount)
	batch["messageCount"] = msgCount
	return map[string]interface{}{"batch": batch}
}

func (s *service) GetImportsBatchSummary(id string) map[string]interface{} {
	var msgCount, totalTokens int64
	s.db.Table("messages").Where("conversation_id = ?", id).Count(&msgCount)
	s.db.Table("messages").Where("conversation_id = ?", id).Select("COALESCE(SUM(tokens), 0)").Row().Scan(&totalTokens)
	var batch map[string]interface{}
	s.db.Table("conversations").Where("id = ?", id).Limit(1).Scan(&batch)
	return map[string]interface{}{"summary": map[string]interface{}{"messageCount": msgCount, "totalTokens": totalTokens, "title": batch["title"]}}
}

func (s *service) GetImportsBatchMemoryCandidates(id string) map[string]interface{} {
	var msgs []map[string]interface{}
	s.db.Table("messages").Where("conversation_id = ? AND role = ?", id, "user").Order("created_at DESC").Limit(20).Find(&msgs)
	if msgs == nil {
		msgs = []map[string]interface{}{}
	}
	return map[string]interface{}{"candidates": msgs, "conversationId": id}
}

func (s *service) DeleteImportsBatch(id string) map[string]interface{} {
	s.db.Table("messages").Where("conversation_id = ?", id).Delete(nil)
	s.db.Table("conversations").Where("id = ? AND source = ?", id, "import").Delete(nil)
	return map[string]interface{}{"deleted": true}
}

func (s *service) GenerateImportsBatchSummary(id string) map[string]interface{} {
	return s.GetImportsBatchSummary(id)
}

func (s *service) ConfirmImportsBatchMemories(id string) map[string]interface{} {
	var msgs []map[string]interface{}
	s.db.Table("messages").Where("conversation_id = ? AND role = ?", id, "user").Limit(20).Find(&msgs)
	confirmed := 0
	for _, msg := range msgs {
		if content, ok := msg["content"].(string); ok && len(content) > 10 {
			s.db.Table("memories").Create(map[string]interface{}{
				"id":         fmt.Sprintf("mem_%s_%d", id[:8], confirmed),
				"key":        fmt.Sprintf("imported_%d", confirmed),
				"value":      content,
				"source":     "import",
				"created_at": time.Now().Format("2006-01-02 15:04:05"),
			})
			confirmed++
		}
	}
	return map[string]interface{}{"confirmed": true, "memoriesCreated": confirmed}
}

func (s *service) UploadImports(body map[string]interface{}) map[string]interface{} {
	batchId := fmt.Sprintf("imp_%d", time.Now().Unix())
	return map[string]interface{}{"uploaded": true, "batchId": batchId}
}

func (s *service) ParseImportsText(body map[string]interface{}) map[string]interface{} {
	text, _ := body["rawText"].(string)
	defaultRole, _ := body["defaultRole"].(string)
	if defaultRole == "" {
		defaultRole = "user"
	}
	lines := strings.Split(text, "\n")
	messages := []interface{}{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			messages = append(messages, map[string]interface{}{"role": defaultRole, "content": line})
		}
	}
	return map[string]interface{}{"parsed": true, "messages": messages, "count": len(messages)}
}

func (s *service) ConfirmImports(body map[string]interface{}) map[string]interface{} {
	charID, _ := body["characterId"].(string)
	title, _ := body["title"].(string)
	if charID == "" {
		return map[string]interface{}{"code": -1, "message": "请选择目标角色"}
	}
	if title == "" {
		title = "已导入的聊天"
	}

	itemsRaw, ok := body["items"]
	if !ok {
		return map[string]interface{}{"code": -1, "message": "没有可导入的消息"}
	}
	items, ok := itemsRaw.([]interface{})
	if !ok || len(items) == 0 {
		return map[string]interface{}{"code": -1, "message": "没有可导入的消息"}
	}

	now := time.Now()
	convID := uuid.New().String()

	s.db.Exec("INSERT OR IGNORE INTO conversations (id, character_id, title, channel, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		convID, charID, title, "web", "import", now, now)

	count := 0
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		if role == "" {
			defaultRole, _ := body["defaultRole"].(string)
			if defaultRole != "" {
				role = defaultRole
			} else {
				role = "user"
			}
		}
		if content == "" {
			continue
		}
		msgID := uuid.New().String()
		s.db.Exec("INSERT INTO messages (id, conversation_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)",
			msgID, convID, role, content, now.Format("2006-01-02 15:04:05"))
		count++
	}

	batchId := fmt.Sprintf("imp_%d", now.Unix())

	return map[string]interface{}{
		"batchId":        batchId,
		"conversationId": convID,
		"messageCount":   count,
		"confirmed":      true,
	}
}

func (s *service) ImportData(body map[string]interface{}) map[string]interface{} {
	source, _ := body["source"].(string)
	charID, _ := body["characterId"].(string)
	raw, _ := body["raw"].(string)
	if source == "" || charID == "" || raw == "" {
		return map[string]interface{}{"code": -1, "message": "参数不完整"}
	}
	lines := strings.Split(raw, "\n")
	var msgs []map[string]interface{}
	now := time.Now()
	convID := charID + "_import_" + now.Format("20060102150405")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		role := "user"
		if source == "plaintext" && i%2 == 1 {
			role = "assistant"
		}
		msgs = append(msgs, map[string]interface{}{
			"id":              uuid.New().String(),
			"conversation_id": convID,
			"role":            role,
			"content":         line,
			"created_at":      now.Format("2006-01-02 15:04:05"),
		})
	}
	if len(msgs) == 0 {
		return map[string]interface{}{"code": -1, "message": "没有有效的消息"}
	}
	s.db.Exec("INSERT OR IGNORE INTO conversations (id, character_id, title, channel, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		convID, charID, "导入的聊天记录", "web", "import", now, now)
	for _, m := range msgs {
		s.db.Exec("INSERT INTO messages (id, conversation_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)",
			m["id"], m["conversation_id"], m["role"], m["content"], m["created_at"])
	}
	return map[string]interface{}{
		"code":    200,
		"data":    map[string]interface{}{"messageCount": len(msgs), "conversationId": convID},
		"message": fmt.Sprintf("成功导入 %d 条消息", len(msgs)),
	}
}
