package chat

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/conversationstream"
	"gorm.io/gorm"
)

func PersistAssistantTurnError(ctx context.Context, db *gorm.DB, turn AssistantTurn, errorCode, errorType, userMessage, internalMessage, provider string, retryable bool) error {
	if db == nil || strings.TrimSpace(turn.ID) == "" {
		return nil
	}
	var encodedInternalMessage any = internalMessage
	trimmedInternalMessage := strings.TrimSpace(internalMessage)
	if trimmedInternalMessage != "" && json.Valid([]byte(trimmedInternalMessage)) {
		encodedInternalMessage = json.RawMessage(trimmedInternalMessage)
	}
	payload := map[string]any{
		"errorCode":          strings.TrimSpace(errorCode),
		"errorType":          strings.TrimSpace(errorType),
		"retryable":          retryable,
		"userMessage":        strings.TrimSpace(userMessage),
		"internalMessage":    encodedInternalMessage,
		"provider":           strings.TrimSpace(provider),
		"recoveryCheckpoint": true,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	item := AssistantTurnItem{}
	found := false
	if err := db.WithContext(ctx).
		Where("turn_id = ? AND item_type = ?", turn.ID, assistantTurnItemError).
		Order("sequence ASC").
		First(&item).Error; err == nil {
		found = true
	} else if err != gorm.ErrRecordNotFound {
		return err
	}
	if found {
		revision := item.Revision + 1
		if revision < 2 {
			revision = 2
		}
		item.Status = assistantTurnStatusFailed
		item.Content = strings.TrimSpace(userMessage)
		item.ResultJSON = string(encoded)
		item.ErrorCode = strings.TrimSpace(errorCode)
		item.Revision = revision
		item.UpdatedAt = now
		if err := db.WithContext(ctx).Save(&item).Error; err != nil {
			return err
		}
	} else {
		var maxSequence int64
		if err := db.WithContext(ctx).
			Model(&AssistantTurnItem{}).
			Where("turn_id = ?", turn.ID).
			Select("COALESCE(MAX(sequence), 0)").
			Scan(&maxSequence).Error; err != nil {
			return err
		}
		item = AssistantTurnItem{
			ID:             uuid.NewString(),
			TurnID:         turn.ID,
			ConversationID: turn.ConversationID,
			Sequence:       maxSequence + 1,
			ItemType:       assistantTurnItemError,
			Status:         assistantTurnStatusFailed,
			Revision:       2,
			Content:        strings.TrimSpace(userMessage),
			ResultJSON:     string(encoded),
			ErrorCode:      strings.TrimSpace(errorCode),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := db.WithContext(ctx).Create(&item).Error; err != nil {
			return err
		}
	}
	eventPayload := map[string]any{
		"blockType":          assistantTurnItemError,
		"content":            item.Content,
		"toolName":           item.ToolName,
		"arguments":          item.ArgumentsJSON,
		"result":             item.ResultJSON,
		"errorCode":          item.ErrorCode,
		"recoveryCheckpoint": true,
	}
	startedRevision := item.Revision
	if startedRevision > 1 {
		startedRevision--
	}
	if _, err := conversationstream.DefaultManager().Publish(ctx, conversationstream.AgentUIEvent{
		ConversationID: turn.ConversationID,
		RequestID:      turn.RequestID,
		ExecutionID:    turn.ExecutionID,
		TurnID:         turn.ID,
		TurnSequence:   turn.Sequence,
		BlockID:        item.ID,
		BlockSequence:  item.Sequence,
		MessageID:      item.MessageID,
		CallID:         item.CallID,
		Revision:       startedRevision,
		Type:           "block.started",
		Status:         assistantTurnStatusRunning,
		Payload:        eventPayload,
	}, true); err != nil {
		return err
	}
	_, err = conversationstream.DefaultManager().Publish(ctx, conversationstream.AgentUIEvent{
		ConversationID: turn.ConversationID,
		RequestID:      turn.RequestID,
		ExecutionID:    turn.ExecutionID,
		TurnID:         turn.ID,
		TurnSequence:   turn.Sequence,
		BlockID:        item.ID,
		BlockSequence:  item.Sequence,
		MessageID:      item.MessageID,
		CallID:         item.CallID,
		Revision:       item.Revision,
		Type:           "block.failed",
		Status:         assistantTurnStatusFailed,
		Payload:        eventPayload,
	}, true)
	return err
}
