// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
)

type importScopedMemoryService interface {
	CreateForSpace(req *memory.CreateMemoryRequest, spaceID string) (*memory.Memory, error)
}

func (h *Handler) importMemoryServiceForSpace() (importScopedMemoryService, bool) {
	svc, ok := h.memorySvc.(importScopedMemoryService)
	return svc, ok
}

func (h *Handler) GetImportsBatches(c *gin.Context) {
	spaceID := webChatSpaceID(c)
	var conversations []map[string]interface{}
	if err := h.webChatOwnedConversationQuery(spaceID).Where("source = ?", "import").Order("created_at DESC").Find(&conversations).Error; err != nil {
		util.ErrorResponse(c, response.InternalError, "读取导入批次失败", nil)
		return
	}
	if conversations == nil {
		conversations = []map[string]interface{}{}
	}
	for _, conv := range conversations {
		var count int64
		if err := h.db.Table("messages").Where("conversation_id = ?", conv["id"]).Count(&count).Error; err != nil {
			util.ErrorResponse(c, response.InternalError, "读取导入消息统计失败", nil)
			return
		}
		conv["totalItems"] = count
		conv["message_count"] = count
		conv["status"] = "completed"
	}
	util.SuccessResponse(c, map[string]interface{}{"items": conversations, "total": len(conversations)})
}

func (h *Handler) GetImportsBatchDetail(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	spaceID := webChatSpaceID(c)
	if _, err := h.requireWebChatImportConversation(id, spaceID); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "导入批次不存在", nil)
		return
	}
	var conv map[string]interface{}
	if err := h.webChatOwnedConversationQuery(spaceID).Where("id = ? AND source = ?", id, "import").Limit(1).Scan(&conv).Error; err != nil {
		util.ErrorResponse(c, response.InternalError, "读取导入批次失败", nil)
		return
	}
	var msgs []map[string]interface{}
	if err := h.db.Table("messages").Where("conversation_id = ?", id).Order("created_at ASC, sequence ASC").Find(&msgs).Error; err != nil {
		util.ErrorResponse(c, response.InternalError, "读取导入消息失败", nil)
		return
	}
	if msgs == nil {
		msgs = []map[string]interface{}{}
	}
	conv["items"] = msgs
	util.SuccessResponse(c, conv)
}

func (h *Handler) GetImportsBatchSummary(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	spaceID := webChatSpaceID(c)
	if _, err := h.requireWebChatImportConversation(id, spaceID); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "导入批次不存在", nil)
		return
	}
	var conv map[string]interface{}
	if err := h.webChatOwnedConversationQuery(spaceID).Where("id = ? AND source = ?", id, "import").Limit(1).Scan(&conv).Error; err != nil {
		util.ErrorResponse(c, response.InternalError, "读取导入批次失败", nil)
		return
	}
	var summaryText string
	if err := h.db.Table("messages").Where("conversation_id = ? AND role = ?", id, "system").Order("created_at DESC").Limit(1).Pluck("content", &summaryText).Error; err != nil {
		util.ErrorResponse(c, response.InternalError, "读取导入摘要失败", nil)
		return
	}
	var msgCount, totalTokens int64
	if err := h.db.Table("messages").Where("conversation_id = ? AND role != ?", id, "system").Count(&msgCount).Error; err != nil {
		util.ErrorResponse(c, response.InternalError, "读取导入统计失败", nil)
		return
	}
	if err := h.db.Table("messages").Where("conversation_id = ?", id).Select("COALESCE(SUM(tokens), 0)").Row().Scan(&totalTokens); err != nil {
		util.ErrorResponse(c, response.InternalError, "读取导入 token 统计失败", nil)
		return
	}
	title := ""
	if t, ok := conv["title"]; ok {
		title = fmt.Sprint(t)
	}
	util.SuccessResponse(c, map[string]interface{}{"summary": map[string]interface{}{
		"messageCount": msgCount,
		"totalTokens":  totalTokens,
		"title":        title,
		"summary":      summaryText,
	}})
}

func (h *Handler) GetImportsBatchMemoryCandidates(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	spaceID := webChatSpaceID(c)
	if _, err := h.requireWebChatImportConversation(id, spaceID); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "导入批次不存在", nil)
		return
	}
	var msgs []map[string]interface{}
	if err := h.db.Table("messages").Where("conversation_id = ? AND role = ?", id, "user").Order("created_at DESC").Limit(20).Find(&msgs).Error; err != nil {
		util.ErrorResponse(c, response.InternalError, "读取导入消息失败", nil)
		return
	}
	if msgs == nil {
		msgs = []map[string]interface{}{}
	}
	candidates := make([]map[string]interface{}, 0)
	for _, msg := range msgs {
		content, _ := msg["content"].(string)
		if len(content) > 10 {
			candidates = append(candidates, map[string]interface{}{
				"key":        fmt.Sprintf("candidate_%d", len(candidates)),
				"value":      content,
				"importance": 5,
			})
		}
	}
	util.SuccessResponse(c, map[string]interface{}{"candidates": candidates, "conversationId": id})
}

func (h *Handler) DeleteImportsBatch(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	spaceID := webChatSpaceID(c)
	if _, err := h.requireWebChatImportConversation(id, spaceID); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "导入批次不存在", nil)
		return
	}
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("messages").Where("conversation_id = ?", id).Delete(nil).Error; err != nil {
			return err
		}
		return webChatOwnerQuery(tx.Table("conversations").Where("id = ? AND source = ?", id, "import"), spaceID).Delete(nil).Error
	}); err != nil {
		util.ErrorResponse(c, response.InternalError, "删除导入批次失败", nil)
		return
	}
	util.SuccessResponse(c, map[string]interface{}{"deleted": true})
}

func (h *Handler) GenerateImportsBatchSummary(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	spaceID := webChatSpaceID(c)
	if _, err := h.requireWebChatImportConversation(id, spaceID); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "导入批次不存在", nil)
		return
	}
	var msgs []map[string]interface{}
	if err := h.db.Table("messages").Where("conversation_id = ? AND role != ?", id, "system").Order("created_at ASC, sequence ASC").Limit(100).Find(&msgs).Error; err != nil {
		util.ErrorResponse(c, response.InternalError, "读取导入消息失败", nil)
		return
	}
	if len(msgs) == 0 {
		util.SuccessResponse(c, map[string]interface{}{"code": -1, "message": "没有可生成摘要的消息"})
		return
	}
	lines := make([]string, 0, len(msgs))
	for _, m := range msgs {
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		lines = append(lines, fmt.Sprintf("[%s] %s", role, content))
	}
	text := strings.Join(lines, "\n")
	systemPrompt := "你是一个摘要生成助手。请将以下对话内容总结为一段简洁的摘要，控制在200字以内。必须严格输出JSON格式：{\"summary\": \"摘要内容\"}。"
	userPrompt := "请为以下对话生成摘要：\n\n" + text
	summaryText, _, _, err := h.chatSvc.GenerateWorkshopJSON(context.Background(), systemPrompt, userPrompt)
	if err != nil {
		util.SuccessResponse(c, map[string]interface{}{"code": -1, "message": fmt.Sprintf("生成摘要失败: %v", err)})
		return
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(summaryText), &parsed); err == nil {
		if v, ok := parsed["summary"].(string); ok {
			summaryText = strings.TrimSpace(v)
		}
	} else {
		summaryText = strings.TrimSpace(summaryText)
	}
	msgID := uuid.New().String()
	now := time.Now().Format("2006-01-02 15:04:05")
	if err := h.db.Exec("INSERT INTO messages (id, conversation_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)", msgID, id, "system", summaryText, now).Error; err != nil {
		util.ErrorResponse(c, response.InternalError, "保存导入摘要失败", nil)
		return
	}
	util.SuccessResponse(c, map[string]interface{}{"summary": summaryText})
}

func (h *Handler) ConfirmImportsBatchMemories(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	spaceID := webChatSpaceID(c)
	if _, err := h.requireWebChatImportConversation(id, spaceID); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "导入批次不存在", nil)
		return
	}
	var body map[string]interface{}
	_ = c.ShouldBindJSON(&body)
	selectedIDs := []string{}
	if raw, ok := body["selectedIds"]; ok {
		if arr, ok2 := raw.([]interface{}); ok2 {
			for _, v := range arr {
				selectedIDs = append(selectedIDs, fmt.Sprint(v))
			}
		}
	}
	var msgs []map[string]interface{}
	q := h.db.Table("messages").Where("conversation_id = ?", id)
	if len(selectedIDs) > 0 {
		q = q.Where("id IN ?", selectedIDs)
	} else {
		q = q.Where("role = ?", "user").Limit(20)
	}
	if err := q.Find(&msgs).Error; err != nil {
		util.ErrorResponse(c, response.InternalError, "读取导入消息失败", nil)
		return
	}
	svc, ok := h.importMemoryServiceForSpace()
	if !ok {
		util.ErrorResponse(c, response.InternalError, "memory service does not provide user-scoped operations", nil)
		return
	}
	confirmed := 0
	for _, msg := range msgs {
		content, _ := msg["content"].(string)
		if len(content) <= 10 {
			continue
		}
		if _, err := svc.CreateForSpace(&memory.CreateMemoryRequest{
			Key:    fmt.Sprintf("imported_%d", confirmed),
			Value:  content,
			Source: "import",
		}, spaceID); err == nil {
			confirmed++
		}
	}
	util.SuccessResponse(c, map[string]interface{}{"confirmed": true, "memoriesCreated": confirmed})
}

func (h *Handler) UploadImports(c *gin.Context) {
	batchId := fmt.Sprintf("imp_%d", time.Now().Unix())
	util.SuccessResponse(c, map[string]interface{}{"uploaded": true, "batchId": batchId})
}

type parsedMessage struct {
	Speaker    string  `json:"speaker"`
	Role       string  `json:"role"`
	Content    string  `json:"content"`
	Confidence float64 `json:"confidence"`
	Timestamp  string  `json:"timestamp"`
	LineNo     int     `json:"lineNo"`
}

type parseWarning struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	LineNo  int    `json:"lineNo"`
}

func detectFormat(text string) string {
	timestampPattern := regexp.MustCompile(`^\d{4}[-/]\d{2}[-/]\d{2}\s+\d{2}:\d{2}`)
	speakerPattern := regexp.MustCompile(`^[\p{L}\p{N}_\-\p{Han}]{1,20}[\s]*[:：]`)
	lines := strings.Split(text, "\n")
	tsCount := 0
	speakerCount := 0
	totalNonBlank := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		totalNonBlank++
		if timestampPattern.MatchString(line) {
			tsCount++
		}
		if speakerPattern.MatchString(line) {
			speakerCount++
		}
	}
	if totalNonBlank == 0 {
		return "auto"
	}
	if float64(tsCount)/float64(totalNonBlank) >= 0.3 {
		return "timestamp"
	}
	if float64(speakerCount)/float64(totalNonBlank) >= 0.3 {
		return "standard"
	}
	return "multiline"
}

func parseStandardFormat(text string, defaultRole string) ([]parsedMessage, []parseWarning) {
	re := regexp.MustCompile(`^([\p{L}\p{N}_\-\p{Han}]{1,30})[\s]*[:：][\s]*(.*)`)
	lines := strings.Split(text, "\n")
	messages := make([]parsedMessage, 0)
	warnings := make([]parseWarning, 0)
	lineNo := 0
	for _, rawLine := range lines {
		lineNo++
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		m := re.FindStringSubmatch(line)
		if m != nil {
			messages = append(messages, parsedMessage{
				Speaker:    m[1],
				Role:       defaultRole,
				Content:    strings.TrimSpace(m[2]),
				Confidence: 0.9,
				LineNo:     lineNo,
			})
		} else {
			warnings = append(warnings, parseWarning{Type: "parse_error", Message: "无法解析此行", LineNo: lineNo})
		}
	}
	return messages, warnings
}

func parseTimestampFormat(text string, defaultRole string) ([]parsedMessage, []parseWarning) {
	tsRe := regexp.MustCompile(`^(\d{4}[-/]\d{2}[-/]\d{2}\s+\d{2}:\d{2}(?::\d{2})?)\s+(.+)`)
	speakerRe := regexp.MustCompile(`^([\p{L}\p{N}_\-\p{Han}]{1,30})[\s]*[:：][\s]*(.*)`)
	lines := strings.Split(text, "\n")
	messages := make([]parsedMessage, 0)
	warnings := make([]parseWarning, 0)
	var currentTs string
	var currentSpeaker string
	lineNo := 0
	for _, rawLine := range lines {
		lineNo++
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		tsMatch := tsRe.FindStringSubmatch(line)
		if tsMatch != nil {
			currentTs = tsMatch[1]
			remainder := strings.TrimSpace(tsMatch[2])
			speakerMatch := speakerRe.FindStringSubmatch(remainder)
			if speakerMatch != nil {
				currentSpeaker = speakerMatch[1]
				content := strings.TrimSpace(speakerMatch[2])
				if content != "" {
					messages = append(messages, parsedMessage{
						Speaker:    currentSpeaker,
						Role:       defaultRole,
						Content:    content,
						Confidence: 0.85,
						Timestamp:  currentTs,
						LineNo:     lineNo,
					})
				}
			} else {
				currentSpeaker = remainder
			}
		} else if currentSpeaker != "" && currentTs != "" {
			lastIdx := len(messages) - 1
			if lastIdx >= 0 && messages[lastIdx].Speaker == currentSpeaker && messages[lastIdx].Content == "" {
				messages[lastIdx].Content = line
			} else {
				messages = append(messages, parsedMessage{
					Speaker:    currentSpeaker,
					Role:       defaultRole,
					Content:    line,
					Confidence: 0.65,
					Timestamp:  currentTs,
					LineNo:     lineNo,
				})
			}
		} else {
			warnings = append(warnings, parseWarning{Type: "parse_error", Message: "无法解析此行", LineNo: lineNo})
		}
	}
	return messages, warnings
}

func parseMultilineFormat(text string, defaultRole string) ([]parsedMessage, []parseWarning) {
	blocks := regexp.MustCompile(`\n\s*\n+`).Split(text, -1)
	messages := make([]parsedMessage, 0)
	warnings := make([]parseWarning, 0)
	lineNo := 1
	speakerRe := regexp.MustCompile(`^([\p{L}\p{N}_\-\p{Han}]{1,30})[\s]*[:：]`)
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			lineNo += strings.Count(block, "\n") + 2
			continue
		}
		blockLines := strings.Split(block, "\n")
		firstLine := strings.TrimSpace(blockLines[0])
		m := speakerRe.FindStringSubmatch(firstLine)
		var speaker, content string
		confidence := 0.75
		if m != nil {
			speaker = m[1]
			content = strings.TrimSpace(strings.TrimPrefix(firstLine, m[0]))
			confidence = 0.85
		} else {
			content = block
		}
		if len(blockLines) > 1 && m != nil && content == "" {
			content = strings.TrimSpace(strings.Join(blockLines[1:], "\n"))
		}
		if content != "" {
			messages = append(messages, parsedMessage{
				Speaker:    speaker,
				Role:       defaultRole,
				Content:    content,
				Confidence: confidence,
				LineNo:     lineNo,
			})
		} else {
			warnings = append(warnings, parseWarning{Type: "empty_content", Message: "空内容块", LineNo: lineNo})
		}
		lineNo += strings.Count(block, "\n") + 2
	}
	return messages, warnings
}

func mapSpeakerNames(messages []parsedMessage, userSpeakerInput, assistantSpeakerInput, defaultRole string) []parsedMessage {
	userNames := splitNames(userSpeakerInput)
	assistantNames := splitNames(assistantSpeakerInput)
	for i := range messages {
		speaker := strings.TrimSpace(messages[i].Speaker)
		if containsName(userNames, speaker) {
			messages[i].Role = "user"
			messages[i].Confidence = 1.0
		} else if containsName(assistantNames, speaker) {
			messages[i].Role = "assistant"
			messages[i].Confidence = 1.0
		} else {
			messages[i].Role = defaultRole
		}
	}
	return messages
}

func splitNames(input string) []string {
	result := make([]string, 0)
	for _, s := range strings.Split(input, ",") {
		for _, s2 := range strings.Split(s, "，") {
			for _, s3 := range strings.Split(s2, ";") {
				for _, s4 := range strings.Split(s3, "；") {
					trimmed := strings.TrimSpace(s4)
					if trimmed != "" {
						result = append(result, trimmed)
					}
				}
			}
		}
	}
	return result
}

func containsName(names []string, speaker string) bool {
	for _, n := range names {
		if strings.EqualFold(n, speaker) {
			return true
		}
	}
	return false
}

func (h *Handler) ParseImportsText(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	text, _ := body["rawText"].(string)
	format, _ := body["format"].(string)
	defaultRole, _ := body["defaultRole"].(string)
	if defaultRole == "" {
		defaultRole = "user"
	}
	var userSpeakerInput, assistantSpeakerInput string
	if v, ok := body["userSpeakerNames"]; ok {
		switch val := v.(type) {
		case string:
			userSpeakerInput = val
		case []interface{}:
			parts := make([]string, 0)
			for _, p := range val {
				parts = append(parts, fmt.Sprint(p))
			}
			userSpeakerInput = strings.Join(parts, ",")
		}
	}
	if v, ok := body["assistantSpeakerNames"]; ok {
		switch val := v.(type) {
		case string:
			assistantSpeakerInput = val
		case []interface{}:
			parts := make([]string, 0)
			for _, p := range val {
				parts = append(parts, fmt.Sprint(p))
			}
			assistantSpeakerInput = strings.Join(parts, ",")
		}
	}

	if format == "" || format == "auto" {
		format = detectFormat(text)
	}

	var messages []parsedMessage
	var warnings []parseWarning

	savedFormat := format
	switch format {
	case "standard":
		messages, warnings = parseStandardFormat(text, defaultRole)
	case "timestamp":
		messages, warnings = parseTimestampFormat(text, defaultRole)
	case "multiline":
		messages, warnings = parseMultilineFormat(text, defaultRole)
	default:
		messages, warnings = parseMultilineFormat(text, defaultRole)
		savedFormat = "multiline"
	}

	if userSpeakerInput != "" || assistantSpeakerInput != "" {
		messages = mapSpeakerNames(messages, userSpeakerInput, assistantSpeakerInput, defaultRole)
	}

	items := make([]map[string]interface{}, 0)
	for _, msg := range messages {
		items = append(items, map[string]interface{}{
			"speaker":    msg.Speaker,
			"role":       msg.Role,
			"content":    msg.Content,
			"confidence": msg.Confidence,
			"timestamp":  msg.Timestamp,
			"lineNo":     msg.LineNo,
		})
	}
	warnItems := make([]map[string]interface{}, 0)
	for _, w := range warnings {
		warnItems = append(warnItems, map[string]interface{}{
			"type":    w.Type,
			"message": w.Message,
			"lineNo":  w.LineNo,
		})
	}

	batchId := fmt.Sprintf("imp_%d", time.Now().Unix())

	util.SuccessResponse(c, map[string]interface{}{
		"detectedFormat": savedFormat,
		"items":          items,
		"warnings":       warnItems,
		"hasHighRisk":    false,
		"batchId":        batchId,
	})
}

func (h *Handler) ConfirmImports(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	charID, _ := body["characterId"].(string)
	title, _ := body["title"].(string)
	if charID == "" {
		util.SuccessResponse(c, map[string]interface{}{"code": -1, "message": "请选择目标角色"})
		return
	}
	spaceID := webChatSpaceID(c)
	if err := h.requireWebChatCharacter(charID, spaceID); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "角色不存在", nil)
		return
	}
	if title == "" {
		title = "已导入的聊天"
	}

	itemsRaw, ok := body["items"]
	if !ok {
		util.SuccessResponse(c, map[string]interface{}{"code": -1, "message": "没有可导入的消息"})
		return
	}

	var items []interface{}
	switch v := itemsRaw.(type) {
	case []interface{}:
		items = v
	default:
		util.SuccessResponse(c, map[string]interface{}{"code": -1, "message": "消息格式错误"})
		return
	}
	if len(items) == 0 {
		util.SuccessResponse(c, map[string]interface{}{"code": -1, "message": "没有可导入的消息"})
		return
	}

	now := time.Now()
	convID := uuid.New().String()
	defaultRole, _ := body["defaultRole"].(string)

	ownerID := requestidentity.NormalizeSpaceID(spaceID)
	if err := h.db.Exec("INSERT OR IGNORE INTO conversations (id, space_id, title, channel, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		convID, ownerID, title, "web", "import", now, now).Error; err != nil {
		util.ErrorResponse(c, response.InternalError, "创建导入会话失败", nil)
		return
	}

	var maxSeq int64
	count := 0
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		if role == "" {
			if defaultRole != "" {
				role = defaultRole
			} else {
				role = "user"
			}
		}
		maxSeq++
		ts, _ := m["timestamp"].(string)
		createdAt := now.Format("2006-01-02 15:04:05")
		if ts != "" {
			if parsed, err := time.Parse("2006-01-02 15:04:05", ts); err == nil {
				createdAt = parsed.Format("2006-01-02 15:04:05")
			} else if parsed, err := time.Parse("2006-01-02 15:04", ts); err == nil {
				createdAt = parsed.Format("2006-01-02 15:04:05")
			} else if parsed, err := time.Parse("2006/01/02 15:04:05", ts); err == nil {
				createdAt = parsed.Format("2006-01-02 15:04:05")
			}
		}
		msgID := uuid.New().String()
		if err := h.db.Exec("INSERT INTO messages (id, conversation_id, character_id, sequence, role, content, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			msgID, convID, charID, maxSeq, role, content, "import", createdAt).Error; err != nil {
			util.ErrorResponse(c, response.InternalError, "保存导入消息失败", nil)
			return
		}
		count++
	}

	batchId := fmt.Sprintf("imp_%d", now.Unix())

	util.SuccessResponse(c, map[string]interface{}{
		"batchId":        batchId,
		"conversationId": convID,
		"messageCount":   count,
		"confirmed":      true,
	})
}

func (h *Handler) DoImportData(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	source, _ := body["source"].(string)
	charID, _ := body["characterId"].(string)
	raw, _ := body["raw"].(string)
	if source == "" || charID == "" || raw == "" {
		util.SuccessResponse(c, map[string]interface{}{"code": -1, "message": "参数不完整"})
		return
	}
	spaceID := webChatSpaceID(c)
	if err := h.requireWebChatCharacter(charID, spaceID); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "角色不存在", nil)
		return
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
		util.SuccessResponse(c, map[string]interface{}{"code": -1, "message": "没有有效的消息"})
		return
	}
	ownerID := requestidentity.NormalizeSpaceID(spaceID)
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("INSERT OR IGNORE INTO conversations (id, space_id, title, channel, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			convID, ownerID, "导入的聊天记录", "web", "import", now, now).Error; err != nil {
			return err
		}
		for _, m := range msgs {
			if err := tx.Exec("INSERT INTO messages (id, conversation_id, character_id, role, content, created_at) VALUES (?, ?, ?, ?, ?, ?)",
				m["id"], m["conversation_id"], charID, m["role"], m["content"], m["created_at"]).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		util.ErrorResponse(c, response.InternalError, "保存导入数据失败", nil)
		return
	}
	util.SuccessResponse(c, map[string]interface{}{
		"code":    200,
		"data":    map[string]interface{}{"messageCount": len(msgs), "conversationId": convID},
		"message": fmt.Sprintf("成功导入 %d 条消息", len(msgs)),
	})
}

func (h *Handler) ExtractImportsMemoryCandidates(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	spaceID := webChatSpaceID(c)
	if _, err := h.requireWebChatImportConversation(id, spaceID); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "导入批次不存在", nil)
		return
	}
	var msgs []map[string]interface{}
	h.db.Table("messages").Where("conversation_id = ? AND role != ?", id, "system").Order("created_at ASC, sequence ASC").Limit(100).Find(&msgs)
	if len(msgs) == 0 {
		util.SuccessResponse(c, map[string]interface{}{"code": -1, "message": "没有可提取的消息"})
		return
	}
	lines := make([]string, 0)
	for _, m := range msgs {
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		if len(content) > 5 {
			lines = append(lines, fmt.Sprintf("[%s] %s", role, content))
		}
	}
	if len(lines) == 0 {
		util.SuccessResponse(c, map[string]interface{}{"code": -1, "message": "消息内容不足"})
		return
	}
	text := strings.Join(lines, "\n")
	systemPrompt := "你是一个记忆提取助手。从以下对话中提取可作为长期记忆的关键信息。返回 JSON，格式为 {\"candidates\": [{\"key\": \"主题\", \"value\": \"具体内容\", \"importance\": 1-10}]}。只提取有长期价值的信息，如用户偏好、重要事件、个人信息、关系等。最多提取10条。"
	userPrompt := "请从以下对话中提取记忆候选：\n\n" + text
	reply, _, _, err := h.chatSvc.GenerateWorkshopJSON(context.Background(), systemPrompt, userPrompt)
	if err != nil {
		var fallback []map[string]interface{}
		for i, m := range msgs {
			content, _ := m["content"].(string)
			if len(content) > 10 && i < 10 {
				fallback = append(fallback, map[string]interface{}{
					"key":        fmt.Sprintf("消息_%d", i+1),
					"value":      content,
					"importance": 3,
				})
			}
		}
		util.SuccessResponse(c, map[string]interface{}{"candidates": fallback, "conversationId": id})
		return
	}
	reply = strings.TrimSpace(reply)
	var result struct {
		Candidates []struct {
			Key        string `json:"key"`
			Value      string `json:"value"`
			Importance int    `json:"importance"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(reply), &result); err != nil {
		var fallback []map[string]interface{}
		for i, m := range msgs {
			content, _ := m["content"].(string)
			if len(content) > 10 && i < 10 {
				fallback = append(fallback, map[string]interface{}{
					"key":        fmt.Sprintf("消息_%d", i+1),
					"value":      content,
					"importance": 3,
				})
			}
		}
		util.SuccessResponse(c, map[string]interface{}{"candidates": fallback, "conversationId": id})
		return
	}
	candidates := make([]map[string]interface{}, 0)
	for _, cand := range result.Candidates {
		if cand.Key != "" && cand.Value != "" {
			imp := cand.Importance
			if imp < 1 {
				imp = 1
			}
			if imp > 10 {
				imp = 10
			}
			candidates = append(candidates, map[string]interface{}{
				"key":        cand.Key,
				"value":      cand.Value,
				"importance": imp,
			})
		}
	}
	util.SuccessResponse(c, map[string]interface{}{"candidates": candidates, "conversationId": id})
}

func (h *Handler) WebChatFromImport(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	convID, _ := body["conversationId"].(string)
	if convID == "" {
		util.SuccessResponse(c, map[string]interface{}{"code": -1, "message": "缺少会话ID"})
		return
	}
	spaceID := webChatSpaceID(c)
	var conv map[string]interface{}
	if err := h.webChatOwnedConversationQuery(spaceID).Where("id = ? AND source = ?", convID, "import").Limit(1).Scan(&conv).Error; err != nil {
		util.ErrorResponse(c, response.InternalError, "读取导入会话失败", nil)
		return
	}
	if len(conv) == 0 {
		util.ErrorResponse(c, response.DataNotFound, "未找到导入的会话", nil)
		return
	}
	util.SuccessResponse(c, map[string]interface{}{"imported": true, "conversationId": convID})
}
