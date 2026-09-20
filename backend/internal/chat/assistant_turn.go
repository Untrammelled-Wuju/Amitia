package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	assistantTurnStatusRunning     = "running"
	assistantTurnStatusCompleted   = "completed"
	assistantTurnStatusFailed      = "failed"
	assistantTurnStatusCancelled   = "cancelled"
	assistantTurnStatusInterrupted = "interrupted"

	assistantTurnItemThinking   = "thinking"
	assistantTurnItemToolCall   = "tool_call"
	assistantTurnItemToolResult = "tool_result"
	assistantTurnItemText       = "text"
)

type AssistantTurn struct {
	ID              string              `gorm:"column:id;primaryKey" json:"id"`
	ConversationID  string              `gorm:"column:conversation_id;not null;default:'';index" json:"conversationId"`
	CharacterID     string              `gorm:"column:character_id;not null;default:''" json:"characterId"`
	UserMessageID   string              `gorm:"column:user_message_id;not null;default:''" json:"userMessageId"`
	RequestID       string              `gorm:"column:request_id;not null;default:'';index" json:"requestId"`
	ResponseGroupID string              `gorm:"column:response_group_id;not null;default:''" json:"responseGroupId"`
	Sequence        int64               `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	Status          string              `gorm:"column:status;not null;default:running" json:"status"`
	CreatedAt       string              `gorm:"column:created_at;not null;default:''" json:"createdAt"`
	UpdatedAt       string              `gorm:"column:updated_at;not null;default:''" json:"updatedAt"`
	CompletedAt     string              `gorm:"column:completed_at;not null;default:''" json:"completedAt"`
	Items           []AssistantTurnItem `gorm:"-" json:"items"`
}

func (AssistantTurn) TableName() string { return "assistant_turns" }

type AssistantTurnItem struct {
	ID              string `gorm:"column:id;primaryKey" json:"id"`
	TurnID          string `gorm:"column:turn_id;not null;default:'';index" json:"turnId"`
	ConversationID  string `gorm:"column:conversation_id;not null;default:'';index" json:"conversationId"`
	Sequence        int64  `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	ItemType        string `gorm:"column:item_type;not null;default:''" json:"type"`
	Status          string `gorm:"column:status;not null;default:pending" json:"status"`
	CallID          string `gorm:"column:call_id;not null;default:''" json:"callId,omitempty"`
	ToolName        string `gorm:"column:tool_name;not null;default:''" json:"toolName,omitempty"`
	Content         string `gorm:"column:content;not null;default:''" json:"content,omitempty"`
	ArgumentsJSON   string `gorm:"column:arguments_json;not null;default:''" json:"argumentsJson,omitempty"`
	ResultJSON      string `gorm:"column:result_json;not null;default:''" json:"resultJson,omitempty"`
	ErrorCode       string `gorm:"column:error_code;not null;default:''" json:"errorCode,omitempty"`
	DurationMS      int64  `gorm:"column:duration_ms;not null;default:0" json:"durationMs,omitempty"`
	IsFinal         int    `gorm:"column:is_final;not null;default:0" json:"isFinal,omitempty"`
	LegacyMessageID string `gorm:"column:legacy_message_id;not null;default:''" json:"legacyMessageId,omitempty"`
	CreatedAt       string `gorm:"column:created_at;not null;default:''" json:"createdAt"`
	UpdatedAt       string `gorm:"column:updated_at;not null;default:''" json:"updatedAt"`
}

func (AssistantTurnItem) TableName() string { return "assistant_turn_items" }

type AssistantTurnEvent struct {
	ID          string `gorm:"column:id;primaryKey" json:"id"`
	TurnID      string `gorm:"column:turn_id;not null;default:'';index" json:"turnId"`
	ItemID      string `gorm:"column:item_id;not null;default:''" json:"itemId,omitempty"`
	Sequence    int64  `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	EventType   string `gorm:"column:event_type;not null;default:''" json:"eventType"`
	PayloadJSON string `gorm:"column:payload_json;not null;default:''" json:"payloadJson,omitempty"`
	CreatedAt   string `gorm:"column:created_at;not null;default:''" json:"createdAt"`
}

func (AssistantTurnEvent) TableName() string { return "assistant_turn_events" }

type assistantTurnRecorder struct {
	db             *gorm.DB
	TurnID         string
	ConversationID string
	CharacterID    string
	UserMessageID  string
	RequestID      string
	enabled        bool
}

func newAssistantTurnRecorder(db *gorm.DB, conversationID, characterID, userMessageID, requestID string) *assistantTurnRecorder {
	return &assistantTurnRecorder{
		db:             db,
		TurnID:         uuid.NewString(),
		ConversationID: strings.TrimSpace(conversationID),
		CharacterID:    strings.TrimSpace(characterID),
		UserMessageID:  strings.TrimSpace(userMessageID),
		RequestID:      strings.TrimSpace(requestID),
	}
}

func (r *assistantTurnRecorder) Start(ctx context.Context) error {
	if r == nil || r.db == nil {
		return nil
	}
	if !r.db.Migrator().HasTable(&AssistantTurn{}) ||
		!r.db.Migrator().HasTable(&AssistantTurnItem{}) ||
		!r.db.Migrator().HasTable(&AssistantTurnEvent{}) {
		return nil
	}
	r.enabled = true
	now := nowString()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sequence int64
		if err := tx.Model(&AssistantTurn{}).Where("conversation_id = ?", r.ConversationID).Select("COALESCE(MAX(sequence), 0) + 1").Scan(&sequence).Error; err != nil {
			return err
		}
		turn := &AssistantTurn{
			ID:             r.TurnID,
			ConversationID: r.ConversationID,
			CharacterID:    r.CharacterID,
			UserMessageID:  r.UserMessageID,
			RequestID:      r.RequestID,
			Sequence:       sequence,
			Status:         assistantTurnStatusRunning,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := tx.Create(turn).Error; err != nil {
			return err
		}
		return appendAssistantTurnEventTx(tx, r.TurnID, "", "turn.started", map[string]any{
			"turnId":         r.TurnID,
			"conversationId": r.ConversationID,
			"requestId":      r.RequestID,
		}, now)
	})
}

func (r *assistantTurnRecorder) AddThinking(ctx context.Context, content string, durationMS int64) error {
	content = strings.TrimSpace(content)
	if r == nil || content == "" {
		return nil
	}
	return r.addItem(ctx, AssistantTurnItem{
		ItemType:   assistantTurnItemThinking,
		Status:     assistantTurnStatusCompleted,
		Content:    content,
		DurationMS: durationMS,
	})
}

func (r *assistantTurnRecorder) AddText(ctx context.Context, content string) error {
	content = strings.TrimSpace(content)
	if r == nil || content == "" {
		return nil
	}
	return r.addItem(ctx, AssistantTurnItem{
		ItemType: assistantTurnItemText,
		Status:   assistantTurnStatusCompleted,
		Content:  content,
	})
}

func (r *assistantTurnRecorder) AddToolCall(ctx context.Context, callID, toolName, arguments string, status string) error {
	if r == nil {
		return nil
	}
	if strings.TrimSpace(status) == "" {
		status = "running"
	}
	return r.addItem(ctx, AssistantTurnItem{
		ItemType:      assistantTurnItemToolCall,
		Status:        status,
		CallID:        strings.TrimSpace(callID),
		ToolName:      strings.TrimSpace(toolName),
		ArgumentsJSON: normalizeTurnJSON(arguments),
	})
}

func (r *assistantTurnRecorder) AddToolResult(ctx context.Context, callID, toolName, result, status, errorCode string, durationMS int64) error {
	if r == nil || r.db == nil || !r.enabled {
		return nil
	}
	if strings.TrimSpace(status) == "" {
		status = assistantTurnStatusCompleted
	}
	if err := r.db.WithContext(ctx).Model(&AssistantTurnItem{}).Where(
		"turn_id = ? AND item_type = ? AND call_id = ?",
		r.TurnID,
		assistantTurnItemToolCall,
		strings.TrimSpace(callID),
	).Updates(map[string]any{
		"status":      status,
		"duration_ms": durationMS,
		"updated_at":  nowString(),
	}).Error; err != nil {
		return err
	}
	return r.addItem(ctx, AssistantTurnItem{
		ItemType:   assistantTurnItemToolResult,
		Status:     status,
		CallID:     strings.TrimSpace(callID),
		ToolName:   strings.TrimSpace(toolName),
		ResultJSON: normalizeTurnJSON(result),
		ErrorCode:  strings.TrimSpace(errorCode),
		DurationMS: durationMS,
	})
}

func (r *assistantTurnRecorder) Finalize(ctx context.Context, status string) error {
	if r == nil || r.db == nil || !r.enabled {
		return nil
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = assistantTurnStatusCompleted
	}
	now := nowString()
	return r.db.WithContext(ctx).Model(&AssistantTurn{}).Where("id = ?", r.TurnID).Updates(map[string]any{
		"status":       status,
		"updated_at":   now,
		"completed_at": now,
	}).Error
}

func (r *assistantTurnRecorder) addItem(ctx context.Context, item AssistantTurnItem) error {
	if r == nil || r.db == nil || !r.enabled {
		return nil
	}
	now := nowString()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		item.ID = uuid.NewString()
		item.TurnID = r.TurnID
		item.ConversationID = r.ConversationID
		item.CreatedAt = now
		item.UpdatedAt = now
		return appendAssistantTurnItemTx(tx, item)
	})
}

func appendAssistantTurnItemTx(tx *gorm.DB, item AssistantTurnItem) error {
	var sequence int64
	if err := tx.Model(&AssistantTurnItem{}).Where("turn_id = ?", item.TurnID).Select("COALESCE(MAX(sequence), 0) + 1").Scan(&sequence).Error; err != nil {
		return err
	}
	item.Sequence = sequence
	if err := tx.Create(&item).Error; err != nil {
		return err
	}
	payload, _ := json.Marshal(item)
	eventType := "item.completed"
	if item.Status == "running" || item.Status == "pending" || item.Status == "queued" {
		eventType = "item.started"
	}
	if err := appendAssistantTurnEventTx(tx, item.TurnID, item.ID, eventType, json.RawMessage(payload), item.CreatedAt); err != nil {
		return err
	}
	return nil
}

func completeAssistantTurnTx(tx *gorm.DB, turnID, responseGroupID, reply string, messageIDs []string) error {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return nil
	}
	if !tx.Migrator().HasTable(&AssistantTurn{}) ||
		!tx.Migrator().HasTable(&AssistantTurnItem{}) ||
		!tx.Migrator().HasTable(&AssistantTurnEvent{}) {
		return nil
	}
	now := nowString()
	var turn AssistantTurn
	if err := tx.Where("id = ?", turnID).First(&turn).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	legacyMessageID := ""
	if len(messageIDs) > 0 {
		legacyMessageID = strings.TrimSpace(messageIDs[0])
	}
	var existing AssistantTurnItem
	err := tx.Where("turn_id = ? AND item_type = ? AND is_final = 1", turnID, assistantTurnItemText).First(&existing).Error
	if err == nil {
		if err := tx.Model(&AssistantTurnItem{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"content":           strings.TrimSpace(reply),
			"status":            assistantTurnStatusCompleted,
			"legacy_message_id": legacyMessageID,
			"updated_at":        now,
		}).Error; err != nil {
			return err
		}
	} else if err != gorm.ErrRecordNotFound {
		return err
	} else if strings.TrimSpace(reply) != "" {
		item := AssistantTurnItem{
			ID:              uuid.NewString(),
			TurnID:          turnID,
			ConversationID:  turn.ConversationID,
			ItemType:        assistantTurnItemText,
			Status:          assistantTurnStatusCompleted,
			Content:         strings.TrimSpace(reply),
			IsFinal:         1,
			LegacyMessageID: legacyMessageID,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := appendAssistantTurnItemTx(tx, item); err != nil {
			return err
		}
	}
	if err := tx.Model(&AssistantTurn{}).Where("id = ?", turnID).Updates(map[string]any{
		"response_group_id": responseGroupID,
		"status":            assistantTurnStatusCompleted,
		"updated_at":        now,
		"completed_at":      now,
	}).Error; err != nil {
		return err
	}
	return appendAssistantTurnEventTx(tx, turnID, "", "turn.completed", map[string]any{
		"turnId":          turnID,
		"responseGroupId": responseGroupID,
	}, now)
}

func appendAssistantTurnEventTx(tx *gorm.DB, turnID, itemID, eventType string, payload any, createdAt string) error {
	var sequence int64
	if err := tx.Model(&AssistantTurnEvent{}).Where("turn_id = ?", turnID).Select("COALESCE(MAX(sequence), 0) + 1").Scan(&sequence).Error; err != nil {
		return err
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return tx.Create(&AssistantTurnEvent{
		ID:          uuid.NewString(),
		TurnID:      turnID,
		ItemID:      itemID,
		Sequence:    sequence,
		EventType:   eventType,
		PayloadJSON: string(payloadBytes),
		CreatedAt:   createdAt,
	}).Error
}

func normalizeTurnJSON(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if json.Valid([]byte(value)) {
		return value
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%q", value)
	}
	return string(encoded)
}

func nowString() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
