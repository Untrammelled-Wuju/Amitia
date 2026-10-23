package system

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/chat/importbatch"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

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

func (h *Handler) GetImportsBatches(c *gin.Context) {
	batches, err := h.importRepo.List(c.Request.Context(), webChatSpaceID(c), 200)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "读取导入批次失败", nil)
		return
	}
	items := make([]map[string]any, 0, len(batches))
	for index := range batches {
		items = append(items, importBatchView(&batches[index], nil))
	}
	util.SuccessResponse(c, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) GetImportsBatchDetail(c *gin.Context) {
	batch, items, err := h.importRepo.Get(c.Request.Context(), webChatSpaceID(c), strings.TrimSpace(c.Param("id")))
	if errors.Is(err, importbatch.ErrNotFound) {
		util.ErrorResponse(c, response.DataNotFound, "导入批次不存在", nil)
		return
	}
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "读取导入批次失败", nil)
		return
	}
	util.SuccessResponse(c, importBatchView(batch, items))
}

func (h *Handler) GetImportsBatchSummary(c *gin.Context) {
	batch, items, err := h.importRepo.Get(c.Request.Context(), webChatSpaceID(c), strings.TrimSpace(c.Param("id")))
	if errors.Is(err, importbatch.ErrNotFound) {
		util.ErrorResponse(c, response.DataNotFound, "导入批次不存在", nil)
		return
	}
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "读取导入批次失败", nil)
		return
	}
	util.SuccessResponse(c, map[string]any{"summary": map[string]any{
		"messageCount": len(items),
		"totalTokens":  0,
		"title":        batch.Title,
		"summary":      batch.SummaryText,
	}})
}

func (h *Handler) DeleteImportsBatch(c *gin.Context) {
	err := h.importRepo.DeleteReady(c.Request.Context(), webChatSpaceID(c), strings.TrimSpace(c.Param("id")))
	if errors.Is(err, importbatch.ErrNotFound) {
		util.ErrorResponse(c, response.DataNotFound, "导入批次不存在", nil)
		return
	}
	if errors.Is(err, importbatch.ErrDeleteNotAllowed) {
		util.ErrorResponse(c, response.BusinessError, "已加入设备聊天的导入批次不能删除", nil)
		return
	}
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "删除导入批次失败", nil)
		return
	}
	util.SuccessResponse(c, map[string]any{"deleted": true})
}

func (h *Handler) GenerateImportsBatchSummary(c *gin.Context) {
	batch, items, err := h.importRepo.Get(c.Request.Context(), webChatSpaceID(c), strings.TrimSpace(c.Param("id")))
	if errors.Is(err, importbatch.ErrNotFound) {
		util.ErrorResponse(c, response.DataNotFound, "导入批次不存在", nil)
		return
	}
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "读取导入消息失败", nil)
		return
	}
	if len(items) == 0 {
		util.ErrorResponse(c, response.InvalidParams, "没有可生成摘要的消息", nil)
		return
	}
	lines := make([]string, 0, len(items))
	for index, item := range items {
		if index >= 100 {
			break
		}
		lines = append(lines, fmt.Sprintf("[%s] %s", item.Role, item.Content))
	}
	systemPrompt := "你是一个摘要生成助手。请将以下对话内容总结为一段简洁的摘要，控制在200字以内。必须严格输出JSON格式：{\"summary\": \"摘要内容\"}。"
	userPrompt := "请为以下对话生成摘要：\n\n" + strings.Join(lines, "\n")
	summaryText, _, _, err := h.chatSvc.GenerateWorkshopJSON(context.Background(), systemPrompt, userPrompt)
	if err != nil {
		util.ErrorResponse(c, response.OperationFailed, "生成摘要失败", err.Error())
		return
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(summaryText), &parsed) == nil {
		if value, ok := parsed["summary"].(string); ok && strings.TrimSpace(value) != "" {
			summaryText = value
		}
	}
	summaryText = strings.TrimSpace(summaryText)
	if summaryText == "" {
		util.ErrorResponse(c, response.OperationFailed, "模型未返回有效摘要", nil)
		return
	}
	if err := h.importRepo.SetSummary(c.Request.Context(), webChatSpaceID(c), batch.ID, summaryText); err != nil {
		util.ErrorResponse(c, response.InternalError, "保存导入摘要失败", nil)
		return
	}
	util.SuccessResponse(c, map[string]any{"summary": summaryText})
}

func (h *Handler) ParseImportsText(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "解析参数无效", nil)
		return
	}
	text, _ := body["rawText"].(string)
	if strings.TrimSpace(text) == "" {
		util.ErrorResponse(c, response.InvalidParams, "聊天内容不能为空", nil)
		return
	}
	format, _ := body["format"].(string)
	defaultRole, _ := body["defaultRole"].(string)
	if defaultRole == "" {
		defaultRole = "user"
	}
	userSpeakerInput, assistantSpeakerInput := speakerInputs(body)
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
	if len(messages) == 0 {
		util.ErrorResponse(c, response.InvalidParams, "没有解析出可导入的消息", nil)
		return
	}
	items := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		items = append(items, map[string]any{
			"speaker":    message.Speaker,
			"role":       message.Role,
			"content":    message.Content,
			"confidence": message.Confidence,
			"timestamp":  message.Timestamp,
			"lineNo":     message.LineNo,
		})
	}
	warnItems := make([]map[string]any, 0, len(warnings))
	for _, warning := range warnings {
		warnItems = append(warnItems, map[string]any{
			"type":    warning.Type,
			"message": warning.Message,
			"lineNo":  warning.LineNo,
		})
	}
	util.SuccessResponse(c, map[string]any{
		"detectedFormat": savedFormat,
		"items":          items,
		"warnings":       warnItems,
		"hasHighRisk":    false,
	})
}

func (h *Handler) ConfirmImports(c *gin.Context) {
	actor := security.GetActor(c)
	if actor == nil || actor.SpaceID == "" || actor.DeviceID == "" {
		util.ErrorResponse(c, response.Unauthorized, "设备身份无效", nil)
		return
	}
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "导入参数无效", nil)
		return
	}
	roleID, _ := body["characterId"].(string)
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		util.ErrorResponse(c, response.InvalidParams, "请选择目标角色", nil)
		return
	}
	if err := h.requireWebChatCharacter(roleID, actor.SpaceID.String()); err != nil {
		util.ErrorResponse(c, response.DataNotFound, "角色不存在", nil)
		return
	}
	title, _ := body["title"].(string)
	if strings.TrimSpace(title) == "" {
		title = "已导入的聊天"
	}
	rawItems, ok := body["items"].([]any)
	if !ok || len(rawItems) == 0 {
		util.ErrorResponse(c, response.InvalidParams, "没有可导入的消息", nil)
		return
	}
	if len(rawItems) > 5000 {
		util.ErrorResponse(c, response.InvalidParams, "单次导入消息数量超过上限", nil)
		return
	}
	format, _ := body["detectedFormat"].(string)
	if format == "" {
		format, _ = body["format"].(string)
	}
	defaultRole, _ := body["defaultRole"].(string)
	if defaultRole == "" {
		defaultRole = "user"
	}
	batchID := "imp_" + uuid.NewString()
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	items := make([]importbatch.Item, 0, len(rawItems))
	for index, raw := range rawItems {
		item, ok := raw.(map[string]any)
		if !ok {
			util.ErrorResponse(c, response.InvalidParams, "消息格式无效", nil)
			return
		}
		role, _ := item["role"].(string)
		role = strings.TrimSpace(role)
		if role == "" {
			role = defaultRole
		}
		if role != "user" && role != "assistant" && role != "system" {
			util.ErrorResponse(c, response.InvalidParams, "消息角色无效", nil)
			return
		}
		content, _ := item["content"].(string)
		if strings.TrimSpace(content) == "" {
			util.ErrorResponse(c, response.InvalidParams, "消息内容不能为空", nil)
			return
		}
		speaker, _ := item["speaker"].(string)
		timestamp, _ := item["timestamp"].(string)
		confidence := 0.0
		if value, ok := item["confidence"].(float64); ok {
			confidence = value
		}
		lineNo := index + 1
		if value, ok := item["lineNo"].(float64); ok {
			lineNo = int(value)
		}
		items = append(items, importbatch.Item{
			ID:         fmt.Sprintf("%s-item-%06d", batchID, index),
			BatchID:    batchID,
			Sequence:   int64(index + 1),
			Speaker:    strings.TrimSpace(speaker),
			Role:       role,
			Content:    strings.TrimSpace(content),
			Confidence: confidence,
			Timestamp:  strings.TrimSpace(timestamp),
			LineNo:     lineNo,
			CreatedAt:  now,
		})
	}
	contentHash := importContentHash(roleID, strings.TrimSpace(title), items)
	idempotencyKey, _ := body["idempotencyKey"].(string)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = contentHash
	}
	batch := &importbatch.Batch{
		ID:             batchID,
		SpaceID:        actor.SpaceID.String(),
		DeviceID:       actor.DeviceID.String(),
		RoleID:         roleID,
		Title:          strings.TrimSpace(title),
		Status:         importbatch.StatusReady,
		DetectedFormat: strings.TrimSpace(format),
		ContentHash:    contentHash,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	stored, storedItems, duplicate, err := h.importRepo.CreateOrGet(c.Request.Context(), batch, items)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "保存导入批次失败", nil)
		return
	}
	util.SuccessResponse(c, map[string]any{
		"batchId":      stored.ID,
		"status":       stored.Status,
		"messageCount": len(storedItems),
		"duplicate":    duplicate,
	})
}

func importBatchView(batch *importbatch.Batch, items []importbatch.Item) map[string]any {
	result := map[string]any{
		"id":                          batch.ID,
		"batchId":                     batch.ID,
		"title":                       batch.Title,
		"status":                      batch.Status,
		"roleId":                      batch.RoleID,
		"detectedFormat":              batch.DetectedFormat,
		"summary":                     batch.SummaryText,
		"messageCount":                batch.MessageCount,
		"message_count":               batch.MessageCount,
		"promotedConversationId":      batch.PromotedConversationID,
		"promotedConversationOwnerId": batch.PromotedConversationOwner,
		"legacyConversationId":        batch.LegacyConversationID,
		"createdAt":                   batch.CreatedAt,
		"created_at":                  batch.CreatedAt,
		"updatedAt":                   batch.UpdatedAt,
		"promotedAt":                  batch.PromotedAt,
	}
	if items != nil {
		rows := make([]map[string]any, 0, len(items))
		for _, item := range items {
			rows = append(rows, map[string]any{
				"id":         item.ID,
				"sequence":   item.Sequence,
				"speaker":    item.Speaker,
				"senderName": item.Speaker,
				"role":       item.Role,
				"content":    item.Content,
				"confidence": item.Confidence,
				"timestamp":  item.Timestamp,
				"lineNo":     item.LineNo,
				"createdAt":  item.CreatedAt,
			})
		}
		result["items"] = rows
	}
	return result
}

func importContentHash(roleID, title string, items []importbatch.Item) string {
	canonical := make([]map[string]any, 0, len(items))
	for _, item := range items {
		canonical = append(canonical, map[string]any{
			"sequence":  item.Sequence,
			"role":      item.Role,
			"content":   item.Content,
			"timestamp": item.Timestamp,
		})
	}
	raw, _ := json.Marshal(map[string]any{"roleId": roleID, "title": title, "items": canonical})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func speakerInputs(body map[string]any) (string, string) {
	read := func(key string) string {
		value, ok := body[key]
		if !ok {
			return ""
		}
		switch typed := value.(type) {
		case string:
			return typed
		case []any:
			parts := make([]string, 0, len(typed))
			for _, part := range typed {
				parts = append(parts, fmt.Sprint(part))
			}
			return strings.Join(parts, ",")
		default:
			return ""
		}
	}
	return read("userSpeakerNames"), read("assistantSpeakerNames")
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
		match := re.FindStringSubmatch(line)
		if match != nil {
			messages = append(messages, parsedMessage{
				Speaker:    match[1],
				Role:       defaultRole,
				Content:    strings.TrimSpace(match[2]),
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
	timestampRE := regexp.MustCompile(`^(\d{4}[-/]\d{2}[-/]\d{2}\s+\d{2}:\d{2}(?::\d{2})?)\s+(.+)`)
	speakerRE := regexp.MustCompile(`^([\p{L}\p{N}_\-\p{Han}]{1,30})[\s]*[:：][\s]*(.*)`)
	lines := strings.Split(text, "\n")
	messages := make([]parsedMessage, 0)
	warnings := make([]parseWarning, 0)
	var currentTimestamp string
	var currentSpeaker string
	lineNo := 0
	for _, rawLine := range lines {
		lineNo++
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		timestampMatch := timestampRE.FindStringSubmatch(line)
		if timestampMatch != nil {
			currentTimestamp = timestampMatch[1]
			remainder := strings.TrimSpace(timestampMatch[2])
			speakerMatch := speakerRE.FindStringSubmatch(remainder)
			if speakerMatch != nil {
				currentSpeaker = speakerMatch[1]
				content := strings.TrimSpace(speakerMatch[2])
				if content != "" {
					messages = append(messages, parsedMessage{
						Speaker:    currentSpeaker,
						Role:       defaultRole,
						Content:    content,
						Confidence: 0.85,
						Timestamp:  currentTimestamp,
						LineNo:     lineNo,
					})
				}
			} else {
				currentSpeaker = remainder
			}
		} else if currentSpeaker != "" && currentTimestamp != "" {
			lastIndex := len(messages) - 1
			if lastIndex >= 0 && messages[lastIndex].Speaker == currentSpeaker && messages[lastIndex].Content == "" {
				messages[lastIndex].Content = line
			} else {
				messages = append(messages, parsedMessage{
					Speaker:    currentSpeaker,
					Role:       defaultRole,
					Content:    line,
					Confidence: 0.65,
					Timestamp:  currentTimestamp,
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
	speakerRE := regexp.MustCompile(`^([\p{L}\p{N}_\-\p{Han}]{1,30})[\s]*[:：]`)
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			lineNo += strings.Count(block, "\n") + 2
			continue
		}
		blockLines := strings.Split(block, "\n")
		firstLine := strings.TrimSpace(blockLines[0])
		match := speakerRE.FindStringSubmatch(firstLine)
		var speaker, content string
		confidence := 0.75
		if match != nil {
			speaker = match[1]
			content = strings.TrimSpace(strings.TrimPrefix(firstLine, match[0]))
			confidence = 0.85
		} else {
			content = block
		}
		if len(blockLines) > 1 && match != nil && content == "" {
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
	for index := range messages {
		speaker := strings.TrimSpace(messages[index].Speaker)
		if containsName(userNames, speaker) {
			messages[index].Role = "user"
			messages[index].Confidence = 1
		} else if containsName(assistantNames, speaker) {
			messages[index].Role = "assistant"
			messages[index].Confidence = 1
		} else {
			messages[index].Role = defaultRole
		}
	}
	return messages
}

func splitNames(input string) []string {
	result := make([]string, 0)
	for _, item := range regexp.MustCompile(`[,，;；]`).Split(input, -1) {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func containsName(names []string, speaker string) bool {
	for _, name := range names {
		if strings.EqualFold(name, speaker) {
			return true
		}
	}
	return false
}
