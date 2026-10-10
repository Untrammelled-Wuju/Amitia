package chat

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/interaction"
	"gorm.io/gorm"
)

var errAgentToolIndeterminate = errors.New("agent tool side effect cannot be proven safe to replay")

type agentToolLedger struct {
	db          *gorm.DB
	intentID    string
	attemptID   string
	inputHash   string
	executionID string
	turnID      string
	call        agentToolCall
	enabled     bool
	unavailable bool
}

var agentToolLedgerNodeID = uuid.NewString()

type agentToolLedgerOutcome struct {
	VisibleText  string          `json:"visibleText"`
	Status       string          `json:"status"`
	ForceVoice   bool            `json:"forceVoice"`
	ErrorCode    string          `json:"errorCode"`
	ErrorMessage string          `json:"errorMessage"`
	Output       json.RawMessage `json:"output"`
	HasError     bool            `json:"hasError"`
	Found        bool            `json:"found"`
}

func newAgentToolLedger(recorder *assistantTurnRecorder, call agentToolCall) *agentToolLedger {
	if recorder == nil || recorder.db == nil {
		return &agentToolLedger{unavailable: true}
	}
	if !recorder.enabled {
		return &agentToolLedger{unavailable: true}
	}
	sum := sha256.Sum256([]byte(call.Name + "\x00" + call.Arguments + "\x00" + call.Scope.SpaceID + "\x00" + call.Scope.CharacterID + "\x00" + call.Scope.PermissionMode))
	return &agentToolLedger{
		db:        recorder.db,
		intentID:  agentToolLedgerIntentID(recorder.TurnID, call.Scope.ConversationID, call.Scope.RequestID, call.ID),
		attemptID: uuid.NewString(), inputHash: hex.EncodeToString(sum[:]), executionID: recorder.ExecutionID,
		turnID: recorder.TurnID, call: call, enabled: true,
	}
}

func (l *agentToolLedger) verifyParentLease(ctx context.Context) error {
	claim, present := interaction.ClaimedExecutionLease(ctx)
	if !present {
		return nil
	}
	if l == nil || l.db == nil || ctx.Err() != nil {
		return fmt.Errorf("parent execution lease context unavailable: %w", errAgentToolIndeterminate)
	}
	var owners int64
	err := l.db.WithContext(ctx).Table("interaction_records").
		Where("id = ? AND owner_instance_id = ? AND status IN ? AND commit_id = '' AND cancel_reason = '' AND heartbeat_at >= ?",
			claim.InteractionID, claim.OwnerInstanceID,
			[]string{"processing", "context_ready"}, time.Now().UTC().Add(-90*time.Second)).
		Count(&owners).Error
	if err != nil {
		return fmt.Errorf("validate parent execution lease: %w", err)
	}
	if owners != 1 {
		return fmt.Errorf("parent execution lease lost or expired: %w", errAgentToolIndeterminate)
	}
	return nil
}

func (l *agentToolLedger) begin(ctx context.Context) (*toolExecOutcome, error) {
	if l.unavailable {
		return nil, fmt.Errorf("durable tool execution is unavailable: %w", errAgentToolIndeterminate)
	}
	if !l.enabled {
		return nil, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	call := l.call
	result := l.db.WithContext(ctx).Exec(
		"INSERT INTO tool_call_intents (id, request_id, conversation_id, character_id, channel, tool_call_id, tool_name, args_json, idempotency_key, status, attempt_id, turn_id, execution_id, input_hash, owner_instance_id, started_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO NOTHING",
		l.intentID, call.Scope.RequestID, call.Scope.ConversationID, call.Scope.CharacterID,
		call.Scope.Channel, call.ID, call.Name, call.Arguments, l.intentID, "PENDING",
		l.attemptID, l.turnID, l.executionID, l.inputHash, agentToolLedgerNodeID, now, now, now,
	)
	if result.Error != nil {
		return nil, fmt.Errorf("persist tool execution intent: %w", result.Error)
	}
	if result.RowsAffected == 1 {
		return nil, nil
	}
	var previous struct {
		RequestID      string `gorm:"column:request_id"`
		ConversationID string `gorm:"column:conversation_id"`
		CharacterID    string `gorm:"column:character_id"`
		CallID         string `gorm:"column:tool_call_id"`
		Name           string `gorm:"column:tool_name"`
		Arguments      string `gorm:"column:args_json"`
		InputHash      string `gorm:"column:input_hash"`
		TurnID         string `gorm:"column:turn_id"`
		Status         string `gorm:"column:status"`
		ResultRef      string `gorm:"column:result_ref"`
		AttemptID      string `gorm:"column:attempt_id"`
		ExecutionID    string `gorm:"column:execution_id"`
	}
	err := l.db.WithContext(ctx).Table("tool_call_intents").
		Select("request_id, conversation_id, character_id, tool_call_id, tool_name, args_json, input_hash, turn_id, status, result_ref, attempt_id, execution_id").
		Where("id = ?", l.intentID).First(&previous).Error
	if err != nil {
		return nil, fmt.Errorf("read persisted tool execution intent: %w", err)
	}
	if previous.RequestID != call.Scope.RequestID || previous.ConversationID != call.Scope.ConversationID ||
		previous.CharacterID != call.Scope.CharacterID || previous.CallID != call.ID ||
		previous.Name != call.Name || previous.Arguments != call.Arguments ||
		previous.TurnID != l.turnID || previous.InputHash != l.inputHash ||
		previous.ExecutionID != l.executionID {
		return nil, fmt.Errorf("tool call %s changed identity or arguments: %w", call.ID, errAgentToolIndeterminate)
	}
	if previous.Status != "SUCCESS" && previous.Status != "FAILED" {
		return nil, fmt.Errorf("tool call %s is not durably completed: %w", call.ID, errAgentToolIndeterminate)
	}
	if previous.ResultRef == "" || previous.AttemptID == "" {
		return nil, fmt.Errorf("tool call %s lacks result provenance: %w", call.ID, errAgentToolIndeterminate)
	}
	var audit, resultStatus string
	err = l.db.WithContext(ctx).Raw(
		"SELECT audit_json, status FROM tool_call_results WHERE id = ? AND intent_id = ? AND idempotency_key = ? AND attempt_id = ? AND input_hash = ? AND turn_id = ? AND execution_id = ?",
		previous.ResultRef, l.intentID, l.intentID, previous.AttemptID, l.inputHash, l.turnID, l.executionID,
	).Row().Scan(&audit, &resultStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("tool call %s was previously dispatched without a durable result: %w", call.ID, errAgentToolIndeterminate)
	}
	if err != nil {
		return nil, fmt.Errorf("read prior tool result: %w", err)
	}
	var saved agentToolLedgerOutcome
	if err := json.Unmarshal([]byte(audit), &saved); err != nil {
		return nil, fmt.Errorf("tool call %s result has invalid evidence: %w", call.ID, errAgentToolIndeterminate)
	}
	if resultStatus != previous.Status ||
		(previous.Status == "SUCCESS" && (saved.HasError || !saved.Found)) ||
		(previous.Status == "FAILED" && !saved.HasError && saved.Found) {
		return nil, fmt.Errorf("tool call %s has contradictory outcome evidence: %w", call.ID, errAgentToolIndeterminate)
	}
	return &toolExecOutcome{
		VisibleText: saved.VisibleText, Status: saved.Status, ForceVoice: saved.ForceVoice,
		ErrorCode: saved.ErrorCode, ErrorMessage: saved.ErrorMessage, Output: saved.Output,
		HasError: saved.HasError, Found: saved.Found,
	}, nil
}

func (l *agentToolLedger) finish(ctx context.Context, outcome toolExecOutcome) error {
	if !l.enabled {
		return nil
	}
	stored := agentToolLedgerOutcome{
		VisibleText: outcome.VisibleText, Status: outcome.Status, ForceVoice: outcome.ForceVoice,
		ErrorCode: outcome.ErrorCode, ErrorMessage: outcome.ErrorMessage, Output: outcome.Output,
		HasError: outcome.HasError, Found: outcome.Found,
	}
	audit, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	status := "SUCCESS"
	if outcome.HasError || !outcome.Found {
		status = "FAILED"
	}
	resultID := uuid.NewString()
	return l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		exists := tx.Exec(
			"UPDATE tool_call_intents SET status = ?, updated_at = ?, finished_at = ?, result_ref = ?, error_class = ? WHERE id = ? AND status = 'PENDING'",
			status, now, now, resultID, outcome.ErrorCode, l.intentID,
		)
		if exists.Error != nil {
			return exists.Error
		}
		if exists.RowsAffected != 1 {
			return errAgentToolIndeterminate
		}
		visible := toolResultContent(l.call.Name, outcome)
		if err := tx.Exec(
			"INSERT INTO tool_call_results (id, intent_id, request_id, conversation_id, character_id, channel, tool_call_id, tool_name, status, content, error_code, visible_text, side_effects_json, external_operation_id, idempotency_key, audit_json, confidence, force_voice, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			resultID, l.intentID, l.call.Scope.RequestID, l.call.Scope.ConversationID,
			l.call.Scope.CharacterID, l.call.Scope.Channel, l.call.ID, l.call.Name, status,
			visible, strings.TrimSpace(outcome.ErrorCode), outcome.VisibleText, "[]", "",
			l.intentID, string(audit), 1, outcome.ForceVoice, now,
		).Error; err != nil {
			return err
		}
		return tx.Exec("UPDATE tool_call_results SET attempt_id = ?, turn_id = ?, execution_id = ?, input_hash = ? WHERE id = ?",
			l.attemptID, l.turnID, l.executionID, l.inputHash, resultID).Error
	})
}

func agentToolOutcomeRequiresReconciliation(outcome toolExecOutcome) bool {
	status := strings.ToUpper(strings.TrimSpace(outcome.Status))
	switch status {
	case "UNKNOWN", "UNCERTAIN", "INDETERMINATE", "TIMED_OUT", "TIMEOUT":
		return true
	}
	code := strings.ToUpper(strings.TrimSpace(outcome.ErrorCode))
	for _, segment := range []string{"TIMEOUT", "TIMED_OUT", "UNCERTAIN", "INDETERMINATE", "CONNECTION_LOST", "NETWORK_LOST", "DEVICE_OFFLINE", "UNKNOWN_RESULT"} {
		if strings.Contains(code, segment) {
			return true
		}
	}
	if outcome.HasError {
		message := strings.ToLower(strings.TrimSpace(outcome.ErrorMessage))
		for _, segment := range []string{
			"context deadline exceeded", "i/o timeout", "connection reset by peer",
			"connection closed", "connection lost", "transport closed",
			"device offline", "device disconnected", "websocket: close",
			"network is unreachable", "network unreachable", "unexpected eof",
		} {
			if strings.Contains(message, segment) {
				return true
			}
		}
	}
	return false
}

func (l *agentToolLedger) indeterminate(ctx context.Context, outcome toolExecOutcome) error {
	if !l.enabled {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	res := l.db.WithContext(ctx).Exec(
		"UPDATE tool_call_intents SET status = ?, error_class = ?, updated_at = ? WHERE id = ? AND status = 'PENDING'",
		"INDETERMINATE", outcome.ErrorCode, time.Now().UTC().Format(time.RFC3339Nano), l.intentID,
	)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errAgentToolIndeterminate
	}
	return nil
}

func (l *agentToolLedger) execution(ctx context.Context, run func() agentToolExecution) agentToolExecution {
	if err := l.verifyParentLease(ctx); err != nil {
		return agentToolExecution{Outcome: toolExecOutcome{
			Status: "UNKNOWN", ErrorCode: "TOOL_REQUIRES_RECONCILIATION",
			ErrorMessage: err.Error(), HasError: true, Found: true,
		}}
	}
	previous, err := l.begin(ctx)
	if err != nil {
		return agentToolExecution{Outcome: toolExecOutcome{
			Status: "UNKNOWN", ErrorCode: "TOOL_REQUIRES_RECONCILIATION",
			ErrorMessage: err.Error(), HasError: true, Found: true,
		}}
	}
	if previous != nil {
		return agentToolExecution{Outcome: *previous}
	}
	if err := l.verifyParentLease(ctx); err != nil {
		return agentToolExecution{Outcome: toolExecOutcome{
			Status: "UNKNOWN", ErrorCode: "TOOL_REQUIRES_RECONCILIATION",
			ErrorMessage: err.Error(), HasError: true, Found: true,
		}}
	}
	execution := run()
	if agentToolOutcomeRequiresReconciliation(execution.Outcome) {
		original := execution.Outcome
		if err := l.indeterminate(ctx, original); err != nil {
			original.ErrorMessage = fmt.Sprintf("%s (ledger reconciliation failed: %v)", original.ErrorMessage, err)
		}
		execution.Outcome = toolExecOutcome{
			Status: "UNKNOWN", ErrorCode: "TOOL_REQUIRES_RECONCILIATION",
			ErrorMessage: fmt.Sprintf("tool %s returned uncertain status=%s code=%s; %s", l.call.ID, original.Status, original.ErrorCode, original.ErrorMessage),
			HasError:     true, Found: true,
		}
		return execution
	}
	if err := l.finish(ctx, execution.Outcome); err != nil {
		execution.Outcome = toolExecOutcome{
			Status: "UNKNOWN", ErrorCode: "TOOL_RESULT_NOT_DURABLE",
			ErrorMessage: err.Error(), HasError: true, Found: true,
		}
	}
	return execution
}
