package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/chat/importbatch"
)

type ImportActor struct {
	SpaceID  string
	DeviceID string
}

func (h *Handler) ImportBatches(ctx context.Context, spaceID string) (map[string]any, error) {
	batches, err := h.importRepo.List(ctx, spaceID, 200)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(batches))
	for index := range batches {
		items = append(items, importBatchView(&batches[index], nil))
	}
	return map[string]any{"items": items, "total": len(items)}, nil
}

func (h *Handler) ImportBatchDetail(ctx context.Context, spaceID, id string) (map[string]any, error) {
	batch, items, err := h.importRepo.Get(ctx, spaceID, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	return importBatchView(batch, items), nil
}

func (h *Handler) ImportBatchSummary(ctx context.Context, spaceID, id string) (map[string]any, error) {
	batch, items, err := h.importRepo.Get(ctx, spaceID, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	return map[string]any{"summary": map[string]any{
		"messageCount": len(items),
		"totalTokens":  0,
		"title":        batch.Title,
		"summary":      batch.SummaryText,
	}}, nil
}

func (h *Handler) DeleteImportBatch(ctx context.Context, spaceID, id string) error {
	return h.importRepo.DeleteReady(ctx, spaceID, strings.TrimSpace(id))
}

func (h *Handler) GenerateImportBatchSummary(ctx context.Context, spaceID, id string) (map[string]any, error) {
	batch, items, err := h.importRepo.Get(ctx, spaceID, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.New("没有可生成摘要的消息")
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
	summaryText, _, _, err := h.chatSvc.GenerateWorkshopJSON(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(summaryText), &parsed) == nil {
		if value, ok := parsed["summary"].(string); ok && strings.TrimSpace(value) != "" {
			summaryText = value
		}
	}
	summaryText = strings.TrimSpace(summaryText)
	if summaryText == "" {
		return nil, errors.New("模型未返回有效摘要")
	}
	if err := h.importRepo.SetSummary(ctx, spaceID, batch.ID, summaryText); err != nil {
		return nil, err
	}
	return map[string]any{"summary": summaryText}, nil
}

func (h *Handler) ParseImportText(_ context.Context, body map[string]any) (map[string]any, error) {
	text, _ := body["rawText"].(string)
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("聊天内容不能为空")
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
		return nil, errors.New("没有解析出可导入的消息")
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
	return map[string]any{
		"detectedFormat": savedFormat,
		"items":          items,
		"warnings":       warnItems,
		"hasHighRisk":    false,
	}, nil
}

func (h *Handler) ConfirmImport(ctx context.Context, actor ImportActor, body map[string]any) (map[string]any, error) {
	if strings.TrimSpace(actor.SpaceID) == "" || strings.TrimSpace(actor.DeviceID) == "" {
		return nil, errors.New("设备身份无效")
	}
	roleID, _ := body["characterId"].(string)
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return nil, errors.New("请选择目标角色")
	}
	if err := h.requireWebChatCharacter(roleID, actor.SpaceID); err != nil {
		return nil, err
	}
	title, _ := body["title"].(string)
	if strings.TrimSpace(title) == "" {
		title = "已导入的聊天"
	}
	rawItems, ok := body["items"].([]any)
	if !ok || len(rawItems) == 0 {
		return nil, errors.New("没有可导入的消息")
	}
	if len(rawItems) > 5000 {
		return nil, errors.New("单次导入消息数量超过上限")
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
			return nil, errors.New("消息格式无效")
		}
		role, _ := item["role"].(string)
		role = strings.TrimSpace(role)
		if role == "" {
			role = defaultRole
		}
		if role != "user" && role != "assistant" && role != "system" {
			return nil, errors.New("消息角色无效")
		}
		content, _ := item["content"].(string)
		if strings.TrimSpace(content) == "" {
			return nil, errors.New("消息内容不能为空")
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
		SpaceID:        actor.SpaceID,
		DeviceID:       actor.DeviceID,
		RoleID:         roleID,
		Title:          strings.TrimSpace(title),
		Status:         importbatch.StatusReady,
		DetectedFormat: strings.TrimSpace(format),
		ContentHash:    contentHash,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	stored, storedItems, duplicate, err := h.importRepo.CreateOrGet(ctx, batch, items)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"batchId":      stored.ID,
		"status":       stored.Status,
		"messageCount": len(storedItems),
		"duplicate":    duplicate,
	}, nil
}
