package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/conversationstream"
	"strings"
	"time"
)

func (r *assistantTurnRecorder) BeginAgentTask(ctx context.Context, eventType, title string, payload map[string]any) (string, time.Time, error) {
	startedAt := time.Now()
	if r == nil || r.db == nil || !r.enabled {
		return "", startedAt, nil
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Agent task"
	}
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		eventType = "agent_task.started"
	}
	encoded, err := marshalAgentTaskPayload(payload)
	if err != nil {
		return "", startedAt, err
	}
	now := nowString()
	item := AssistantTurnItem{
		ID:             uuid.NewString(),
		TurnID:         r.TurnID,
		ConversationID: r.ConversationID,
		ItemType:       assistantTurnItemAgentTask,
		Status:         assistantTurnStatusRunning,
		Revision:       1,
		Content:        title,
		ResultJSON:     encoded,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := r.rememberItem(&item); err != nil {
		return "", startedAt, err
	}
	if err := r.db.WithContext(ctx).Save(&item).Error; err != nil {
		return "", startedAt, err
	}
	_, err = conversationstream.DefaultManager().Publish(ctx, conversationstream.AgentUIEvent{
		ConversationID: r.ConversationID,
		RequestID:      r.RequestID,
		ExecutionID:    r.ExecutionID,
		TurnID:         r.TurnID,
		TurnSequence:   r.TurnSequence,
		BlockID:        item.ID,
		BlockSequence:  item.Sequence,
		Revision:       item.Revision,
		Type:           eventType,
		Status:         assistantTurnStatusRunning,
		Payload: map[string]any{
			"blockType": assistantTurnItemAgentTask,
			"content":   title,
			"result":    encoded,
		},
	}, true)
	return item.ID, startedAt, err
}

func (r *assistantTurnRecorder) ProgressAgentTask(ctx context.Context, blockID, eventType string, payload map[string]any) error {
	if r == nil || r.db == nil || !r.enabled || strings.TrimSpace(blockID) == "" {
		return nil
	}
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		eventType = "agent_task.progress"
	}
	encoded, err := marshalAgentTaskPayload(payload)
	if err != nil {
		return err
	}
	item, err := r.updateItem(strings.TrimSpace(blockID), func(item *AssistantTurnItem) {
		item.Status = assistantTurnStatusRunning
		item.Revision++
		if item.Revision < 2 {
			item.Revision = 2
		}
		item.ResultJSON = encoded
		item.UpdatedAt = nowString()
	})
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Save(&item).Error; err != nil {
		return err
	}
	_, err = conversationstream.DefaultManager().Publish(ctx, conversationstream.AgentUIEvent{
		ConversationID: r.ConversationID,
		RequestID:      r.RequestID,
		ExecutionID:    r.ExecutionID,
		TurnID:         r.TurnID,
		TurnSequence:   r.TurnSequence,
		BlockID:        item.ID,
		BlockSequence:  item.Sequence,
		Revision:       item.Revision,
		Type:           eventType,
		Status:         assistantTurnStatusRunning,
		Payload: map[string]any{
			"blockType": assistantTurnItemAgentTask,
			"content":   item.Content,
			"result":    encoded,
		},
	}, false)
	return err
}

func (r *assistantTurnRecorder) CompleteAgentTask(ctx context.Context, blockID, eventType, status string, payload map[string]any, startedAt time.Time) error {
	if r == nil || r.db == nil || !r.enabled || strings.TrimSpace(blockID) == "" {
		return nil
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = assistantTurnStatusCompleted
	}
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		switch status {
		case assistantTurnStatusFailed:
			eventType = "agent_task.failed"
		case assistantTurnStatusInterrupted:
			eventType = "agent_task.interrupted"
		default:
			eventType = "agent_task.completed"
		}
	}
	encoded, err := marshalAgentTaskPayload(payload)
	if err != nil {
		return err
	}
	durationMS := int64(0)
	if !startedAt.IsZero() {
		durationMS = time.Since(startedAt).Milliseconds()
		if durationMS < 0 {
			durationMS = 0
		}
	}
	item, err := r.updateItem(strings.TrimSpace(blockID), func(item *AssistantTurnItem) {
		item.Status = status
		item.Revision++
		if item.Revision < 2 {
			item.Revision = 2
		}
		item.ResultJSON = encoded
		item.DurationMS = durationMS
		if code := strings.TrimSpace(fmt.Sprint(payload["errorCode"])); code != "" && code != "<nil>" {
			item.ErrorCode = code
		}
		item.UpdatedAt = nowString()
	})
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Save(&item).Error; err != nil {
		return err
	}
	eventPayload := map[string]any{
		"blockType":          assistantTurnItemAgentTask,
		"content":            item.Content,
		"result":             encoded,
		"durationMs":         durationMS,
		"recoveryCheckpoint": true,
	}
	if item.ErrorCode != "" {
		eventPayload["errorCode"] = item.ErrorCode
	}
	_, err = conversationstream.DefaultManager().Publish(context.WithoutCancel(ctx), conversationstream.AgentUIEvent{
		ConversationID: r.ConversationID,
		RequestID:      r.RequestID,
		ExecutionID:    r.ExecutionID,
		TurnID:         r.TurnID,
		TurnSequence:   r.TurnSequence,
		BlockID:        item.ID,
		BlockSequence:  item.Sequence,
		Revision:       item.Revision,
		Type:           eventType,
		Status:         status,
		Payload:        eventPayload,
	}, true)
	return err
}

func marshalAgentTaskPayload(payload map[string]any) (string, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
