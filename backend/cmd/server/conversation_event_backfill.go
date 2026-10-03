package main

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/conversationstream"
	"gorm.io/gorm"
)

func backfillConversationEventFiles(db *gorm.DB, store *conversationstream.FileStore) error {
	if db == nil || store == nil {
		return nil
	}
	var turns []chat.AssistantTurn
	if err := db.Order("conversation_id ASC, sequence ASC").Find(&turns).Error; err != nil {
		return err
	}
	ctx := context.Background()
	currentConversationID := ""
	var sequence int64
	skipConversation := false
	for index := range turns {
		turn := turns[index]
		conversationID := strings.TrimSpace(turn.ConversationID)
		if conversationID == "" {
			continue
		}
		if conversationID != currentConversationID {
			currentConversationID = conversationID
			latest, err := store.LatestSequence(ctx, conversationID)
			if err != nil {
				return err
			}
			sequence = latest
			skipConversation = sequence > 0
		}
		if skipConversation {
			continue
		}
		events, err := conversationEventsFromTurn(ctx, db, turn, &sequence)
		if err != nil {
			return err
		}
		for _, event := range events {
			if err := store.Persist(ctx, event); err != nil {
				return err
			}
		}
		if err := writeConversationSnapshot(ctx, store, conversationID); err != nil {
			return err
		}
	}
	return nil
}

func conversationEventsFromTurn(ctx context.Context, db *gorm.DB, turn chat.AssistantTurn, sequence *int64) ([]conversationstream.AgentUIEvent, error) {
	(*sequence)++
	createdAt := eventTime(turn.CreatedAt)
	events := []conversationstream.AgentUIEvent{{
		Version:        conversationstream.ProtocolVersion,
		EventID:        uuid.NewString(),
		EventSequence:  *sequence,
		ConversationID: turn.ConversationID,
		RequestID:      turn.RequestID,
		ExecutionID:    turn.ExecutionID,
		TurnID:         turn.ID,
		TurnSequence:   turn.Sequence,
		Type:           "turn.started",
		Status:         "running",
		Payload:        map[string]any{"recoveryCheckpoint": true},
		CreatedAt:      createdAt,
	}}
	var items []chat.AssistantTurnItem
	if err := db.WithContext(ctx).Where("turn_id = ?", turn.ID).Order("sequence ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	for index := range items {
		item := items[index]
		(*sequence)++
		eventType, eventStatus := conversationEventForItem(item)
		payload := map[string]any{
			"blockType":          item.ItemType,
			"content":            item.Content,
			"arguments":          item.ArgumentsJSON,
			"result":             item.ResultJSON,
			"toolName":           item.ToolName,
			"errorCode":          item.ErrorCode,
			"durationMs":         item.DurationMS,
			"recoveryCheckpoint": true,
		}
		events = append(events, conversationstream.AgentUIEvent{
			Version:        conversationstream.ProtocolVersion,
			EventID:        uuid.NewString(),
			EventSequence:  *sequence,
			ConversationID: turn.ConversationID,
			RequestID:      turn.RequestID,
			ExecutionID:    turn.ExecutionID,
			TurnID:         turn.ID,
			TurnSequence:   turn.Sequence,
			BlockID:        item.ID,
			BlockSequence:  item.Sequence,
			CallID:         item.CallID,
			Revision:       item.Revision,
			Type:           eventType,
			Status:         eventStatus,
			Payload:        payload,
			CreatedAt:      eventTime(item.CreatedAt),
		})
	}
	(*sequence)++
	terminalType, terminalStatus := conversationEventForTurn(turn.Status)
	events = append(events, conversationstream.AgentUIEvent{
		Version:        conversationstream.ProtocolVersion,
		EventID:        uuid.NewString(),
		EventSequence:  *sequence,
		ConversationID: turn.ConversationID,
		RequestID:      turn.RequestID,
		ExecutionID:    turn.ExecutionID,
		TurnID:         turn.ID,
		TurnSequence:   turn.Sequence,
		Type:           terminalType,
		Status:         terminalStatus,
		Payload:        map[string]any{"recoveryCheckpoint": true},
		CreatedAt:      eventTime(turn.CompletedAt),
	})
	return events, nil
}

func conversationEventForItem(item chat.AssistantTurnItem) (string, string) {
	status := strings.TrimSpace(item.Status)
	if status == "" {
		status = "completed"
	}
	switch item.ItemType {
	case "reasoning":
		return terminalEventType("reasoning", status), status
	case "text":
		return terminalEventType("text", status), status
	case "tool_call":
		if status == "running" {
			return "tool.running", status
		}
		return terminalEventType("tool", status), status
	case "tool_result":
		return terminalEventType("block", status), status
	case "error":
		return "block.failed", "failed"
	default:
		return terminalEventType("block", status), status
	}
}

func terminalEventType(prefix, status string) string {
	switch status {
	case "failed":
		return prefix + ".failed"
	case "interrupted":
		return prefix + ".interrupted"
	default:
		return prefix + ".completed"
	}
}

func conversationEventForTurn(status string) (string, string) {
	switch status {
	case "failed":
		return "turn.failed", "failed"
	case "interrupted":
		return "turn.interrupted", "interrupted"
	default:
		return "turn.completed", "completed"
	}
}

func eventTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local); err == nil {
		return parsed.UTC().Format(time.RFC3339Nano)
	}
	return value
}

func writeConversationSnapshot(ctx context.Context, store *conversationstream.FileStore, conversationID string) error {
	_, err := store.Snapshot(ctx, conversationID)
	return err
}
