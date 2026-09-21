package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/conversationstream"
	"gorm.io/gorm"
)

const (
	assistantTurnStatusQueued      = "queued"
	assistantTurnStatusStarting    = "starting"
	assistantTurnStatusRunning     = "running"
	assistantTurnStatusWaitingTool = "waiting_tool"
	assistantTurnStatusCompleted   = "completed"
	assistantTurnStatusFailed      = "failed"
	assistantTurnStatusInterrupted = "interrupted"

	assistantTurnItemReasoning  = "reasoning"
	assistantTurnItemToolCall   = "tool_call"
	assistantTurnItemToolResult = "tool_result"
	assistantTurnItemText       = "text"
	assistantTurnItemError      = "error"
)

type AssistantTurn struct {
	ID             string              `gorm:"column:id;primaryKey" json:"id"`
	ConversationID string              `gorm:"column:conversation_id;not null;default:'';index" json:"conversationId"`
	CharacterID    string              `gorm:"column:character_id;not null;default:''" json:"characterId"`
	UserMessageID  string              `gorm:"column:user_message_id;not null;default:''" json:"userMessageId"`
	RequestID      string              `gorm:"column:request_id;not null;default:'';index" json:"requestId"`
	ExecutionID    string              `gorm:"column:execution_id;not null;default:'';index" json:"executionId"`
	ParentTurnID   string              `gorm:"column:parent_turn_id;not null;default:'';index" json:"parentTurnId,omitempty"`
	ParentBlockID  string              `gorm:"column:parent_block_id;not null;default:''" json:"parentBlockId,omitempty"`
	AgentID        string              `gorm:"column:agent_id;not null;default:'';index" json:"agentId,omitempty"`
	Sequence       int64               `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	Status         string              `gorm:"column:status;not null;default:running" json:"status"`
	CreatedAt      string              `gorm:"column:created_at;not null;default:''" json:"createdAt"`
	UpdatedAt      string              `gorm:"column:updated_at;not null;default:''" json:"updatedAt"`
	CompletedAt    string              `gorm:"column:completed_at;not null;default:''" json:"completedAt"`
	Items          []AssistantTurnItem `gorm:"-" json:"items"`
}

func (AssistantTurn) TableName() string { return "assistant_turns" }

type AssistantTurnItem struct {
	ID             string `gorm:"column:id;primaryKey" json:"id"`
	TurnID         string `gorm:"column:turn_id;not null;default:'';index" json:"turnId"`
	ConversationID string `gorm:"column:conversation_id;not null;default:'';index" json:"conversationId"`
	Sequence       int64  `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	ItemType       string `gorm:"column:item_type;not null;default:''" json:"type"`
	Status         string `gorm:"column:status;not null;default:pending" json:"status"`
	Revision       int64  `gorm:"column:revision;not null;default:0" json:"revision"`
	CallID         string `gorm:"column:call_id;not null;default:''" json:"callId,omitempty"`
	ToolName       string `gorm:"column:tool_name;not null;default:''" json:"toolName,omitempty"`
	Content        string `gorm:"column:content;not null;default:''" json:"content,omitempty"`
	ArgumentsJSON  string `gorm:"column:arguments_json;not null;default:''" json:"argumentsJson,omitempty"`
	ResultJSON     string `gorm:"column:result_json;not null;default:''" json:"resultJson,omitempty"`
	ErrorCode      string `gorm:"column:error_code;not null;default:''" json:"errorCode,omitempty"`
	DurationMS     int64  `gorm:"column:duration_ms;not null;default:0" json:"durationMs,omitempty"`
	IsFinal        int    `gorm:"column:is_final;not null;default:0" json:"isFinal,omitempty"`
	MessageID      string `gorm:"column:message_id;not null;default:''" json:"messageId,omitempty"`
	CreatedAt      string `gorm:"column:created_at;not null;default:''" json:"createdAt"`
	UpdatedAt      string `gorm:"column:updated_at;not null;default:''" json:"updatedAt"`
}

func (AssistantTurnItem) TableName() string { return "assistant_turn_items" }

type assistantTurnRecorder struct {
	db             *gorm.DB
	TurnID         string
	ConversationID string
	CharacterID    string
	UserMessageID  string
	RequestID      string
	ExecutionID    string
	TurnSequence   int64
	Provider       string
	enabled        bool
}

func newAssistantTurnRecorder(db *gorm.DB, conversationID, characterID, userMessageID, requestID string, ids ...string) *assistantTurnRecorder {
	turnID := ""
	executionID := ""
	if len(ids) > 0 {
		turnID = ids[0]
	}
	if len(ids) > 1 {
		executionID = ids[1]
	}
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		turnID = uuid.NewString()
	}
	executionID = strings.TrimSpace(executionID)
	if executionID == "" {
		executionID = uuid.NewString()
	}
	return &assistantTurnRecorder{
		db:             db,
		TurnID:         turnID,
		ConversationID: strings.TrimSpace(conversationID),
		CharacterID:    strings.TrimSpace(characterID),
		UserMessageID:  strings.TrimSpace(userMessageID),
		RequestID:      strings.TrimSpace(requestID),
		ExecutionID:    executionID,
	}
}

func (r *assistantTurnRecorder) Start(ctx context.Context) error {
	if r == nil || r.db == nil {
		return nil
	}
	if !r.db.Migrator().HasTable(&AssistantTurn{}) ||
		!r.db.Migrator().HasTable(&AssistantTurnItem{}) {
		return nil
	}
	r.enabled = true
	now := nowString()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing AssistantTurn
		err := tx.Where("id = ?", r.TurnID).First(&existing).Error
		if err == nil {
			r.TurnSequence = existing.Sequence
			if r.ExecutionID == "" {
				r.ExecutionID = existing.ExecutionID
			}
			updates := map[string]any{"status": assistantTurnStatusRunning, "updated_at": now}
			if r.ExecutionID != "" {
				updates["execution_id"] = r.ExecutionID
			}
			if err := tx.Model(&AssistantTurn{}).Where("id = ?", r.TurnID).Updates(updates).Error; err != nil {
				return err
			}
			return nil
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		var sequence int64
		if err := tx.Model(&AssistantTurn{}).Where("conversation_id = ?", r.ConversationID).Select("COALESCE(MAX(sequence), 0) + 1").Scan(&sequence).Error; err != nil {
			return err
		}
		r.TurnSequence = sequence
		turn := &AssistantTurn{
			ID: r.TurnID, ConversationID: r.ConversationID, CharacterID: r.CharacterID, UserMessageID: r.UserMessageID,
			RequestID: r.RequestID, ExecutionID: r.ExecutionID, Sequence: sequence, Status: assistantTurnStatusRunning,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(turn).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if _, err := conversationstream.DefaultManager().Publish(ctx, conversationstream.AgentUIEvent{
		ConversationID: r.ConversationID, RequestID: r.RequestID, ExecutionID: r.ExecutionID, TurnID: r.TurnID,
		TurnSequence: r.TurnSequence, Type: "turn.started", Status: assistantTurnStatusRunning,
	}, true); err != nil {
		return err
	}
	return nil
}

func (r *assistantTurnRecorder) AddToolCall(ctx context.Context, callID, toolName, arguments string, status string) error {
	if r == nil {
		return nil
	}
	callID = strings.TrimSpace(callID)
	toolName = strings.TrimSpace(toolName)
	if strings.TrimSpace(status) == "" {
		status = assistantTurnStatusRunning
	}
	if r.db != nil && r.enabled && callID != "" {
		var existing AssistantTurnItem
		err := r.db.WithContext(ctx).Where("turn_id = ? AND item_type = ? AND call_id = ?", r.TurnID, assistantTurnItemToolCall, callID).Order("sequence ASC").First(&existing).Error
		if err == nil {
			revision := existing.Revision + 1
			if revision < 2 {
				revision = 2
			}
			argumentsJSON := normalizeTurnJSON(arguments)
			updates := map[string]any{"status": status, "tool_name": toolName, "arguments_json": argumentsJSON, "revision": revision, "updated_at": nowString()}
			if err := r.db.WithContext(ctx).Model(&AssistantTurnItem{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
				return err
			}
			_, err = conversationstream.DefaultManager().Publish(ctx, conversationstream.AgentUIEvent{
				ConversationID: r.ConversationID, RequestID: r.RequestID, ExecutionID: r.ExecutionID, TurnID: r.TurnID, TurnSequence: r.TurnSequence,
				BlockID: existing.ID, BlockSequence: existing.Sequence, CallID: callID, Revision: revision, Type: "tool.running", Status: status,
				Payload: map[string]any{"blockType": "tool_call", "toolName": toolName, "arguments": argumentsJSON, "recoveryCheckpoint": true},
			}, true)
			return err
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
	}
	return r.addItem(ctx, AssistantTurnItem{ItemType: assistantTurnItemToolCall, Status: status, CallID: callID, ToolName: toolName, ArgumentsJSON: normalizeTurnJSON(arguments)})
}

func (r *assistantTurnRecorder) AddToolResult(ctx context.Context, callID, toolName, result, status, errorCode string, durationMS int64) error {
	if r == nil || r.db == nil || !r.enabled {
		return nil
	}
	ctx = context.WithoutCancel(ctx)
	if strings.TrimSpace(status) == "" {
		status = assistantTurnStatusCompleted
	}
	callID = strings.TrimSpace(callID)
	toolName = strings.TrimSpace(toolName)
	errorCode = strings.TrimSpace(errorCode)
	now := nowString()
	var toolCallItem AssistantTurnItem
	resultItem := AssistantTurnItem{
		ID:             uuid.NewString(),
		TurnID:         r.TurnID,
		ConversationID: r.ConversationID,
		ItemType:       assistantTurnItemToolResult,
		Status:         status,
		CallID:         callID,
		ToolName:       toolName,
		ResultJSON:     normalizeTurnJSON(result),
		ErrorCode:      errorCode,
		DurationMS:     durationMS,
		Revision:       2,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	var toolRevision int64
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("turn_id = ? AND item_type = ? AND call_id = ?", r.TurnID, assistantTurnItemToolCall, callID).Order("sequence ASC").First(&toolCallItem).Error; err != nil {
			return err
		}
		toolRevision = toolCallItem.Revision + 1
		if toolRevision < 2 {
			toolRevision = 2
		}
		if err := tx.Model(&AssistantTurnItem{}).Where("id = ?", toolCallItem.ID).Updates(map[string]any{
			"status": status, "duration_ms": durationMS, "revision": toolRevision, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		return appendAssistantTurnItemTx(tx, &resultItem)
	}); err != nil {
		return err
	}
	eventType := "tool.completed"
	if status == assistantTurnStatusFailed {
		eventType = "tool.failed"
	} else if status == assistantTurnStatusInterrupted {
		eventType = "tool.interrupted"
	}
	payload := map[string]any{"blockType": "tool_call", "toolName": toolName, "durationMs": durationMS, "recoveryCheckpoint": true}
	if errorCode != "" {
		payload["errorCode"] = errorCode
	}
	if _, err := conversationstream.DefaultManager().Publish(ctx, conversationstream.AgentUIEvent{
		ConversationID: r.ConversationID, RequestID: r.RequestID, ExecutionID: r.ExecutionID, TurnID: r.TurnID, TurnSequence: r.TurnSequence,
		BlockID: toolCallItem.ID, BlockSequence: toolCallItem.Sequence, CallID: callID, Revision: toolRevision, Type: eventType, Status: status, Payload: payload,
	}, true); err != nil {
		return err
	}
	return r.publishItemEvents(ctx, resultItem)
}

func finalizeAssistantTurnFailureByID(db *gorm.DB, turnID string, cause error) error {
	if db == nil {
		return nil
	}
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return nil
	}
	var turn AssistantTurn
	if err := db.Where("id = ?", turnID).First(&turn).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	recorder := &assistantTurnRecorder{
		db:             db,
		TurnID:         turn.ID,
		ConversationID: turn.ConversationID,
		CharacterID:    turn.CharacterID,
		UserMessageID:  turn.UserMessageID,
		RequestID:      turn.RequestID,
		ExecutionID:    turn.ExecutionID,
		TurnSequence:   turn.Sequence,
		enabled:        true,
	}
	return recorder.FinalizeFailure(context.Background(), assistantTurnStatusFailed, cause)
}

func (r *assistantTurnRecorder) Finalize(ctx context.Context, status string) error {
	return r.finalize(ctx, status, nil)
}

func (r *assistantTurnRecorder) FinalizeFailure(ctx context.Context, status string, cause error) error {
	payload := map[string]any{
		"errorCode":          "runtime_error",
		"errorType":          "runtime",
		"retryable":          true,
		"userMessage":        "Agent 执行失败，可重试",
		"internalMessage":    "",
		"provider":           strings.TrimSpace(r.Provider),
		"recoveryCheckpoint": true,
	}
	if cause != nil {
		payload["internalMessage"] = cause.Error()
		var modelErr *TextModelCallError
		if errors.As(cause, &modelErr) {
			payload["errorCode"] = "provider_error"
			payload["errorType"] = "provider"
			payload["internalMessage"] = modelErr.RawError
			raw := strings.ToLower(modelErr.RawError)
			payload["retryable"] = strings.Contains(raw, "429") || strings.Contains(raw, "500") || strings.Contains(raw, "502") || strings.Contains(raw, "503") || strings.Contains(raw, "504") || strings.Contains(raw, "timeout") || strings.Contains(raw, "busy") || strings.Contains(raw, "unavailable")
		}
	}
	if status == assistantTurnStatusInterrupted {
		payload["errorCode"] = "interrupted"
		payload["errorType"] = "interrupt"
		payload["retryable"] = false
		payload["userMessage"] = "已停止生成"
	}
	return r.finalize(ctx, status, payload)
}

func (r *assistantTurnRecorder) finalize(ctx context.Context, status string, payload map[string]any) error {
	if r == nil || r.db == nil || !r.enabled {
		return nil
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = assistantTurnStatusCompleted
	}
	now := nowString()
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&AssistantTurn{}).Where("id = ?", r.TurnID).Updates(map[string]any{
			"status":       status,
			"updated_at":   now,
			"completed_at": now,
		}).Error; err != nil {
			return err
		}
		if status == assistantTurnStatusFailed || status == assistantTurnStatusInterrupted {
			return tx.Model(&AssistantTurnItem{}).Where("turn_id = ? AND status NOT IN ?", r.TurnID, []string{assistantTurnStatusCompleted, assistantTurnStatusFailed, assistantTurnStatusInterrupted}).Updates(map[string]any{
				"status": status, "revision": gorm.Expr("revision + 1"), "updated_at": now,
			}).Error
		}
		return nil
	}); err != nil {
		return err
	}
	if status != assistantTurnStatusFailed && status != assistantTurnStatusInterrupted {
		return nil
	}
	if status == assistantTurnStatusFailed && payload != nil {
		if err := r.persistErrorItem(context.WithoutCancel(ctx), payload); err != nil {
			return err
		}
	}
	eventType := "turn.failed"
	if status == assistantTurnStatusInterrupted {
		eventType = "turn.interrupted"
	}
	if payload == nil {
		payload = map[string]any{"recoveryCheckpoint": true}
	}
	_, err := conversationstream.DefaultManager().Publish(context.Background(), conversationstream.AgentUIEvent{ConversationID: r.ConversationID, RequestID: r.RequestID, ExecutionID: r.ExecutionID, TurnID: r.TurnID, TurnSequence: r.TurnSequence, Type: eventType, Status: status, Payload: payload}, true)
	return err
}

func (r *assistantTurnRecorder) persistErrorItem(ctx context.Context, payload map[string]any) error {
	if r == nil || r.db == nil || !r.enabled || payload == nil {
		return nil
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	userMessage := strings.TrimSpace(fmt.Sprint(payload["userMessage"]))
	errorCode := strings.TrimSpace(fmt.Sprint(payload["errorCode"]))
	now := nowString()
	var existing AssistantTurnItem
	err = r.db.WithContext(ctx).
		Where("turn_id = ? AND item_type = ?", r.TurnID, assistantTurnItemError).
		Order("sequence ASC").
		First(&existing).Error
	if err == nil {
		revision := existing.Revision + 1
		if revision < 2 {
			revision = 2
		}
		if err := r.db.WithContext(ctx).Model(&AssistantTurnItem{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"status":      assistantTurnStatusFailed,
			"content":     userMessage,
			"result_json": string(encoded),
			"error_code":  errorCode,
			"revision":    revision,
			"updated_at":  now,
		}).Error; err != nil {
			return err
		}
		existing.Status = assistantTurnStatusFailed
		existing.Content = userMessage
		existing.ResultJSON = string(encoded)
		existing.ErrorCode = errorCode
		existing.Revision = revision
		existing.UpdatedAt = now
		return r.publishItemEvents(ctx, existing)
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return r.addItem(ctx, AssistantTurnItem{
		ItemType:   assistantTurnItemError,
		Status:     assistantTurnStatusFailed,
		ErrorCode:  errorCode,
		Content:    userMessage,
		ResultJSON: string(encoded),
		Revision:   2,
	})
}

func (r *assistantTurnRecorder) addItem(ctx context.Context, item AssistantTurnItem) error {
	if r == nil || r.db == nil || !r.enabled {
		return nil
	}
	now := nowString()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		item.ID = uuid.NewString()
		if item.Revision <= 0 {
			item.Revision = 1
		}
		if item.Status != assistantTurnStatusRunning && item.Status != assistantTurnStatusQueued && item.Status != "pending" && item.Revision < 2 {
			item.Revision = 2
		}
		item.TurnID = r.TurnID
		item.ConversationID = r.ConversationID
		item.CreatedAt = now
		item.UpdatedAt = now
		return appendAssistantTurnItemTx(tx, &item)
	})
	if err != nil {
		return err
	}
	return r.publishItemEvents(ctx, item)
}

func (r *assistantTurnRecorder) publishItemEvents(ctx context.Context, item AssistantTurnItem) error {
	blockType := item.ItemType
	startedType := "block.started"
	completedType := "block.completed"
	if blockType == "text" || blockType == "reasoning" {
		startedType = blockType + ".started"
		completedType = blockType + ".completed"
	}
	if blockType == assistantTurnItemToolCall {
		startedType = "tool.started"
		completedType = "tool.running"
	}
	if item.Status == assistantTurnStatusFailed {
		switch blockType {
		case "text", "reasoning":
			completedType = blockType + ".failed"
		case assistantTurnItemToolCall:
			completedType = "tool.failed"
		default:
			completedType = "block.failed"
		}
	} else if item.Status == assistantTurnStatusInterrupted {
		switch blockType {
		case "text", "reasoning":
			completedType = blockType + ".interrupted"
		case assistantTurnItemToolCall:
			completedType = "tool.interrupted"
		default:
			completedType = "block.interrupted"
		}
	}
	payload := map[string]any{"blockType": blockType, "content": item.Content, "toolName": item.ToolName, "arguments": item.ArgumentsJSON, "result": item.ResultJSON, "errorCode": item.ErrorCode}
	if item.Status != assistantTurnStatusRunning && item.Status != assistantTurnStatusQueued && item.Status != "pending" {
		payload["recoveryCheckpoint"] = true
	}
	terminal := item.Status != assistantTurnStatusRunning && item.Status != assistantTurnStatusQueued && item.Status != "pending"
	startedRevision := item.Revision
	if terminal && startedRevision > 1 {
		startedRevision--
	}
	startedStatus := item.Status
	if terminal {
		startedStatus = assistantTurnStatusRunning
	}
	if _, err := conversationstream.DefaultManager().Publish(ctx, conversationstream.AgentUIEvent{
		ConversationID: r.ConversationID, RequestID: r.RequestID, ExecutionID: r.ExecutionID, TurnID: r.TurnID, TurnSequence: r.TurnSequence,
		BlockID: item.ID, BlockSequence: item.Sequence, MessageID: item.MessageID, CallID: item.CallID, Revision: startedRevision, Type: startedType, Status: startedStatus, Payload: payload,
	}, true); err != nil {
		return err
	}
	if terminal {
		_, err := conversationstream.DefaultManager().Publish(ctx, conversationstream.AgentUIEvent{
			ConversationID: r.ConversationID, RequestID: r.RequestID, ExecutionID: r.ExecutionID, TurnID: r.TurnID, TurnSequence: r.TurnSequence,
			BlockID: item.ID, BlockSequence: item.Sequence, MessageID: item.MessageID, CallID: item.CallID, Revision: item.Revision, Type: completedType, Status: item.Status, Payload: payload,
		}, true)
		return err
	}
	return nil
}

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
		"errorCode": strings.TrimSpace(errorCode), "errorType": strings.TrimSpace(errorType), "retryable": retryable,
		"userMessage": strings.TrimSpace(userMessage), "internalMessage": encodedInternalMessage,
		"provider": strings.TrimSpace(provider), "recoveryCheckpoint": true,
	}
	recorder := &assistantTurnRecorder{
		db: db, TurnID: turn.ID, ConversationID: turn.ConversationID, CharacterID: turn.CharacterID,
		UserMessageID: turn.UserMessageID, RequestID: turn.RequestID, ExecutionID: turn.ExecutionID,
		TurnSequence: turn.Sequence, enabled: true,
	}
	return recorder.persistErrorItem(ctx, payload)
}

func appendAssistantTurnItemTx(tx *gorm.DB, item *AssistantTurnItem) error {
	var sequence int64
	if err := tx.Model(&AssistantTurnItem{}).Where("turn_id = ?", item.TurnID).Select("COALESCE(MAX(sequence), 0) + 1").Scan(&sequence).Error; err != nil {
		return err
	}
	item.Sequence = sequence
	if err := tx.Create(item).Error; err != nil {
		return err
	}
	return nil
}

func completeAssistantTurnTx(tx *gorm.DB, turnID, reply, messageID string) error {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return nil
	}
	if !tx.Migrator().HasTable(&AssistantTurn{}) ||
		!tx.Migrator().HasTable(&AssistantTurnItem{}) {
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
	messageID = strings.TrimSpace(messageID)
	var existing AssistantTurnItem
	err := tx.Where("turn_id = ? AND item_type = ?", turnID, assistantTurnItemText).Order("sequence DESC").First(&existing).Error
	if err == nil {
		if err := tx.Model(&AssistantTurnItem{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"content":    strings.TrimSpace(reply),
			"revision":   existing.Revision + 1,
			"status":     assistantTurnStatusCompleted,
			"is_final":   1,
			"message_id": messageID,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
	} else if err != gorm.ErrRecordNotFound {
		return err
	} else if strings.TrimSpace(reply) != "" {
		item := AssistantTurnItem{
			ID: uuid.NewString(), TurnID: turnID, ConversationID: turn.ConversationID,
			ItemType: assistantTurnItemText, Status: assistantTurnStatusCompleted, Revision: 2,
			Content: strings.TrimSpace(reply), IsFinal: 1, MessageID: messageID, CreatedAt: now, UpdatedAt: now,
		}
		if err := appendAssistantTurnItemTx(tx, &item); err != nil {
			return err
		}
	}
	if err := tx.Model(&AssistantTurn{}).Where("id = ?", turnID).Updates(map[string]any{
		"status": assistantTurnStatusCompleted, "updated_at": now, "completed_at": now,
	}).Error; err != nil {
		return err
	}
	return nil
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
