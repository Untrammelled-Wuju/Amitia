package chat

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func agentToolLedgerIntentID(turnID, conversationID, requestID, callID string) string {
	sum := sha256.Sum256([]byte(turnID + "\x00" + conversationID + "\x00" + requestID + "\x00" + callID))
	return "harness:" + hex.EncodeToString(sum[:])
}

func ReconcileAgentToolCheckpoint(ctx context.Context, db *gorm.DB, turn AssistantTurn, items []AssistantTurnItem) ([]AssistantTurnItem, error) {
	if db == nil || turn.ID == "" || turn.RequestID == "" {
		return items, nil
	}
	completed := make(map[string]bool)
	for _, item := range items {
		if item.ItemType == assistantTurnItemToolResult {
			completed[item.CallID] = true
		}
	}
	mutated := false
	for _, item := range items {
		if item.ItemType != assistantTurnItemToolCall || item.CallID == "" || completed[item.CallID] {
			continue
		}
		id := agentToolLedgerIntentID(turn.ID, turn.ConversationID, turn.RequestID, item.CallID)
		var intent struct {
			Name        string `gorm:"column:tool_name"`
			Arguments   string `gorm:"column:args_json"`
			Status      string `gorm:"column:status"`
			ResultRef   string `gorm:"column:result_ref"`
			AttemptID   string `gorm:"column:attempt_id"`
			TurnID      string `gorm:"column:turn_id"`
			ExecutionID string `gorm:"column:execution_id"`
			InputHash   string `gorm:"column:input_hash"`
		}
		if err := db.WithContext(ctx).Table("tool_call_intents").
			Select("tool_name, args_json, status, result_ref, attempt_id, turn_id, execution_id, input_hash").Where("id = ?", id).First(&intent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		if intent.Name != item.ToolName || intent.Arguments != item.ArgumentsJSON ||
			(intent.Status != "SUCCESS" && intent.Status != "FAILED") {
			continue
		}
		if intent.ResultRef == "" || intent.AttemptID == "" ||
			intent.InputHash == "" || intent.TurnID != turn.ID ||
			intent.ExecutionID != turn.ExecutionID {
			return nil, fmt.Errorf("saved tool result %s has unverified provenance: %w", item.CallID, errAgentToolIndeterminate)
		}
		var audit, resultStatus string
		err := db.WithContext(ctx).Raw(
			"SELECT audit_json, status FROM tool_call_results WHERE id = ? AND intent_id = ? AND idempotency_key = ? AND attempt_id = ? AND input_hash = ? AND turn_id = ? AND execution_id = ?",
			intent.ResultRef, id, id, intent.AttemptID, intent.InputHash, turn.ID, turn.ExecutionID,
		).Row().Scan(&audit, &resultStatus)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("saved tool result %s cannot be matched to its result reference: %w", item.CallID, errAgentToolIndeterminate)
		}
		if err != nil {
			return nil, err
		}
		if resultStatus != intent.Status {
			return nil, fmt.Errorf("saved tool result %s has a contradictory status: %w", item.CallID, errAgentToolIndeterminate)
		}
		var result agentToolLedgerOutcome
		if err := json.Unmarshal([]byte(audit), &result); err != nil {
			return nil, fmt.Errorf("saved tool outcome %s is corrupt: %w", item.CallID, err)
		}
		if intent.Status == "SUCCESS" && (result.HasError || !result.Found) {
			return nil, fmt.Errorf("tool outcome %s has conflicting success evidence", item.CallID)
		}
		if intent.Status == "FAILED" && !result.HasError && result.Found {
			return nil, fmt.Errorf("tool outcome %s has conflicting failure evidence", item.CallID)
		}
		outcome := toolExecOutcome{
			VisibleText: result.VisibleText, Status: result.Status, ForceVoice: result.ForceVoice,
			ErrorCode: result.ErrorCode, ErrorMessage: result.ErrorMessage, Output: result.Output,
			HasError: result.HasError, Found: result.Found,
		}
		status := assistantTurnStatusCompleted
		if outcome.HasError || !outcome.Found {
			status = assistantTurnStatusFailed
		}
		reconcileCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		err = db.WithContext(reconcileCtx).Transaction(func(tx *gorm.DB) error {
			var existing int64
			if err := tx.Model(&AssistantTurnItem{}).
				Where("turn_id = ? AND item_type = ? AND call_id = ?",
					turn.ID, assistantTurnItemToolResult, item.CallID).Count(&existing).Error; err != nil {
				return err
			}
			if existing > 0 {
				return nil
			}
			now := nowString()
			updated := tx.Model(&AssistantTurnItem{}).
				Where("id = ? AND turn_id = ? AND item_type = ? AND status NOT IN ?",
					item.ID, turn.ID, assistantTurnItemToolCall,
					[]string{assistantTurnStatusCompleted, assistantTurnStatusFailed, assistantTurnStatusInterrupted}).
				Updates(map[string]any{
					"status": status, "updated_at": now, "revision": gorm.Expr("revision + 1"),
				})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return errAgentToolIndeterminate
			}
			var sequence int64
			if err := tx.Raw("SELECT COALESCE(MAX(sequence),0)+1 FROM assistant_turn_items WHERE turn_id = ?", turn.ID).
				Scan(&sequence).Error; err != nil {
				return err
			}
			encodedResult, err := json.Marshal(toolResultContent(item.ToolName, outcome))
			if err != nil {
				return err
			}
			checkpoint := AssistantTurnItem{
				ID:     uuid.NewSHA1(uuid.NameSpaceOID, []byte(turn.ID+":"+item.CallID+":tool-ledger")).String(),
				TurnID: turn.ID, ConversationID: turn.ConversationID, Sequence: sequence,
				ItemType: assistantTurnItemToolResult, Status: status, Revision: 2,
				CallID: item.CallID, ToolName: item.ToolName,
				ResultJSON: string(encodedResult),
				ErrorCode:  outcome.ErrorCode, CreatedAt: now, UpdatedAt: now,
			}
			return tx.Create(&checkpoint).Error
		})
		cancel()
		if err != nil {
			return nil, err
		}
		mutated = true
	}
	if !mutated {
		return items, nil
	}
	var refreshed []AssistantTurnItem
	if err := db.WithContext(ctx).Where("turn_id = ?", turn.ID).
		Order("sequence ASC").Find(&refreshed).Error; err != nil {
		return nil, err
	}
	return refreshed, nil
}
