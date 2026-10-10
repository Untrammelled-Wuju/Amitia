package chat

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAgentToolLedger(t *testing.T) (*gorm.DB, *assistantTurnRecorder, agentToolCall) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "ledger.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	for _, statement := range []string{
		`CREATE TABLE tool_call_intents (id TEXT PRIMARY KEY, request_id TEXT, conversation_id TEXT, character_id TEXT, channel TEXT, tool_call_id TEXT, tool_name TEXT, args_json TEXT, idempotency_key TEXT, status TEXT, attempt_id TEXT, turn_id TEXT, execution_id TEXT, input_hash TEXT, owner_instance_id TEXT, result_ref TEXT, error_class TEXT, started_at TEXT, finished_at TEXT, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE tool_call_results (id TEXT PRIMARY KEY, intent_id TEXT, request_id TEXT, conversation_id TEXT, character_id TEXT, channel TEXT, tool_call_id TEXT, tool_name TEXT, status TEXT, content TEXT, error_code TEXT, visible_text TEXT, side_effects_json TEXT, external_operation_id TEXT, idempotency_key TEXT, audit_json TEXT, confidence REAL, force_voice INTEGER, attempt_id TEXT, turn_id TEXT, execution_id TEXT, input_hash TEXT, created_at TEXT)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	recorder := newAssistantTurnRecorder(db, "conversation", "character", "user", "request", "turn", "execution")
	recorder.enabled = true
	call := agentToolCall{
		ID: "call-1", Name: "execute_host_command", Arguments: `{"command":"test"}`,
		Scope: SkillScope{ConversationID: "conversation", CharacterID: "character",
			RequestID: "request", Channel: "web", ToolCallID: "call-1"},
	}
	return db, recorder, call
}

func TestAgentToolLedgerReusesDurableResultInsteadOfExecutingTwice(t *testing.T) {
	db, recorder, call := setupAgentToolLedger(t)
	var executed atomic.Int32
	run := func() agentToolExecution {
		executed.Add(1)
		return agentToolExecution{Outcome: toolExecOutcome{
			Status: "SUCCEEDED", VisibleText: "done", Output: []byte(`{"exitCode":0}`),
			Found: true,
		}}
	}
	first := newAgentToolLedger(recorder, call).execution(context.Background(), run)
	second := newAgentToolLedger(recorder, call).execution(context.Background(), run)
	if first.Outcome.HasError || second.Outcome.HasError || second.Outcome.VisibleText != "done" ||
		executed.Load() != 1 || string(second.Outcome.Output) != `{"exitCode":0}` {
		t.Fatalf("durable result was not reused: first=%+v second=%+v count=%d",
			first, second, executed.Load())
	}
	var intents, results int64
	if err := db.Table("tool_call_intents").Count(&intents).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("tool_call_results").Count(&results).Error; err != nil {
		t.Fatal(err)
	}
	if intents != 1 || results != 1 {
		t.Fatalf("expected one durable intent and one result: intents=%d results=%d", intents, results)
	}
}

func TestAgentToolLedgerBlocksIndeterminateCrashWindow(t *testing.T) {
	_, recorder, call := setupAgentToolLedger(t)
	first := newAgentToolLedger(recorder, call)
	if old, err := first.begin(context.Background()); err != nil || old != nil {
		t.Fatalf("first claim failed: prior=%+v err=%v", old, err)
	}
	var ran atomic.Int32
	result := newAgentToolLedger(recorder, call).execution(context.Background(), func() agentToolExecution {
		ran.Add(1)
		return agentToolExecution{}
	})
	if ran.Load() != 0 || result.Outcome.ErrorCode != "TOOL_REQUIRES_RECONCILIATION" {
		t.Fatalf("indeterminate action replayed: %+v executions=%d", result, ran.Load())
	}
}

func TestAgentToolLedgerRejectsMutatedArguments(t *testing.T) {
	_, recorder, call := setupAgentToolLedger(t)
	started := newAgentToolLedger(recorder, call)
	if _, err := started.begin(context.Background()); err != nil {
		t.Fatal(err)
	}
	call.Arguments = `{"command":"danger"}`
	outcome := newAgentToolLedger(recorder, call).execution(context.Background(), func() agentToolExecution {
		t.Fatal("mutated tool arguments executed")
		return agentToolExecution{}
	})
	if outcome.Outcome.ErrorCode != "TOOL_REQUIRES_RECONCILIATION" {
		t.Fatalf("mutated tool inputs were accepted: %+v", outcome)
	}
}

func TestAgentToolLedgerFailsClosedWhenPersistenceUnavailable(t *testing.T) {
	db, recorder, call := setupAgentToolLedger(t)
	if err := db.Exec("DROP TABLE tool_call_intents").Error; err != nil {
		t.Fatal(err)
	}
	var ran atomic.Int32
	result := newAgentToolLedger(recorder, call).execution(context.Background(), func() agentToolExecution {
		ran.Add(1)
		return agentToolExecution{}
	})
	if ran.Load() != 0 || result.Outcome.ErrorCode != "TOOL_REQUIRES_RECONCILIATION" {
		t.Fatalf("tool executed without durable intent: result=%+v executions=%d", result, ran.Load())
	}
}

func TestAgentToolLedgerRetainsFailureEvidence(t *testing.T) {
	_, recorder, call := setupAgentToolLedger(t)
	run := func() agentToolExecution {
		return agentToolExecution{Outcome: toolExecOutcome{
			Status: "FAILED", ErrorCode: "TOOL_NONZERO_EXIT", ErrorMessage: "compile failed",
			HasError: true, Found: true,
		}}
	}
	newAgentToolLedger(recorder, call).execution(context.Background(), run)
	result := newAgentToolLedger(recorder, call).execution(context.Background(), func() agentToolExecution {
		t.Fatal("failed attempt replayed without a new safe decision")
		return agentToolExecution{}
	})
	if !result.Outcome.HasError || result.Outcome.ErrorCode != "TOOL_NONZERO_EXIT" ||
		!strings.Contains(result.Outcome.ErrorMessage, "compile failed") {
		t.Fatalf("failed terminal attempt not recovered: %+v", result)
	}
}

func TestAgentToolLedgerReconcilesCrashAfterEffectBeforeTurnCheckpoint(t *testing.T) {
	db, recorder, call := setupAgentToolLedger(t)
	if err := db.AutoMigrate(&AssistantTurn{}, &AssistantTurnItem{}); err != nil {
		t.Fatal(err)
	}
	turn := AssistantTurn{
		ID: recorder.TurnID, ConversationID: recorder.ConversationID,
		CharacterID: recorder.CharacterID, RequestID: recorder.RequestID,
		ExecutionID: recorder.ExecutionID, Status: assistantTurnStatusWaitingTool,
	}
	if err := db.Create(&turn).Error; err != nil {
		t.Fatal(err)
	}
	callItem := AssistantTurnItem{
		ID: "call-block-1", TurnID: turn.ID, ConversationID: turn.ConversationID,
		Sequence: 1, ItemType: assistantTurnItemToolCall, Status: assistantTurnStatusRunning,
		CallID: call.ID, ToolName: call.Name, ArgumentsJSON: call.Arguments,
	}
	if err := db.Create(&callItem).Error; err != nil {
		t.Fatal(err)
	}
	first := newAgentToolLedger(recorder, call).execution(context.Background(), func() agentToolExecution {
		return agentToolExecution{Outcome: toolExecOutcome{
			Status: "SUCCEEDED", Output: []byte(`{"exitCode":0}`), Found: true,
		}}
	})
	if first.Outcome.HasError {
		t.Fatalf("tool outcome could not be persisted: %+v", first)
	}
	reconciled, err := ReconcileAgentToolCheckpoint(context.Background(), db, turn, []AssistantTurnItem{callItem})
	if err != nil {
		t.Fatal(err)
	}
	if len(reconciled) != 2 || reconciled[1].ItemType != assistantTurnItemToolResult ||
		reconciled[1].CallID != call.ID || reconciled[1].Status != assistantTurnStatusCompleted {
		t.Fatalf("persisted tool result was not restored: %+v", reconciled)
	}
	if second, err := ReconcileAgentToolCheckpoint(context.Background(), db, turn, reconciled); err != nil || len(second) != 2 {
		t.Fatalf("reconciliation was not idempotent: %+v %v", second, err)
	}
}

func TestAgentToolLedgerReconcileRefusesMissingOrUndispatchedEvidence(t *testing.T) {
	db, recorder, call := setupAgentToolLedger(t)
	if err := db.AutoMigrate(&AssistantTurn{}, &AssistantTurnItem{}); err != nil {
		t.Fatal(err)
	}
	turn := AssistantTurn{ID: recorder.TurnID, ConversationID: recorder.ConversationID,
		RequestID: recorder.RequestID, Status: assistantTurnStatusWaitingTool}
	if err := db.Create(&turn).Error; err != nil {
		t.Fatal(err)
	}
	item := AssistantTurnItem{ID: "call-block", TurnID: turn.ID, ConversationID: turn.ConversationID,
		ItemType: assistantTurnItemToolCall, Status: assistantTurnStatusRunning,
		CallID: call.ID, ToolName: call.Name, ArgumentsJSON: call.Arguments}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := newAgentToolLedger(recorder, call).begin(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := ReconcileAgentToolCheckpoint(context.Background(), db, turn, []AssistantTurnItem{item})
	if err != nil || len(got) != 1 {
		t.Fatalf("unproven side effect was marked complete: %+v %v", got, err)
	}
}

func TestAgentToolLedgerRejectsDisabledCheckpointStore(t *testing.T) {
	_, recorder, call := setupAgentToolLedger(t)
	recorder.enabled = false
	executed := false
	got := newAgentToolLedger(recorder, call).execution(context.Background(), func() agentToolExecution {
		executed = true
		return agentToolExecution{}
	})
	if executed || got.Outcome.ErrorCode != "TOOL_REQUIRES_RECONCILIATION" {
		t.Fatalf("disabled durable recorder ran untracked side effect: %+v", got)
	}
}

func TestAgentToolLedgerRecoversEmptySuccessfulResultAsValidJSON(t *testing.T) {
	db, recorder, call := setupAgentToolLedger(t)
	call.Name = "workspace.write"
	if err := db.AutoMigrate(&AssistantTurn{}, &AssistantTurnItem{}); err != nil {
		t.Fatal(err)
	}
	turn := AssistantTurn{ID: recorder.TurnID, ConversationID: recorder.ConversationID,
		CharacterID: recorder.CharacterID, RequestID: recorder.RequestID,
		ExecutionID: recorder.ExecutionID, Status: assistantTurnStatusWaitingTool}
	if err := db.Create(&turn).Error; err != nil {
		t.Fatal(err)
	}
	item := AssistantTurnItem{ID: "empty-result-call", TurnID: turn.ID,
		ConversationID: turn.ConversationID, Sequence: 1,
		ItemType: assistantTurnItemToolCall, Status: assistantTurnStatusRunning,
		CallID: call.ID, ToolName: call.Name, ArgumentsJSON: call.Arguments}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	result := newAgentToolLedger(recorder, call).execution(context.Background(), func() agentToolExecution {
		return agentToolExecution{Outcome: toolExecOutcome{Status: "SUCCEEDED", Found: true}}
	})
	if result.Outcome.HasError {
		t.Fatalf("empty output durable result failed: %+v", result)
	}
	restored, err := ReconcileAgentToolCheckpoint(context.Background(), db, turn, []AssistantTurnItem{item})
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != 2 || !json.Valid([]byte(restored[1].ResultJSON)) || restored[1].ResultJSON != `""` {
		t.Fatalf("successful empty result was not safely checkpointed: %+v", restored)
	}
}

func TestAgentToolLedgerStopsOnUnknownExternalOutcome(t *testing.T) {
	for _, uncertain := range []toolExecOutcome{
		{Status: "TIMED_OUT", ErrorCode: "TOOL_TIMED_OUT", HasError: true, Found: true},
		{Status: "UNKNOWN", ErrorCode: "DEVICE_OFFLINE", HasError: true, Found: true},
		{Status: "FAILED", ErrorCode: "NETWORK_CONNECTION_LOST", HasError: true, Found: true},
	} {
		t.Run(uncertain.ErrorCode, func(t *testing.T) {
			db, recorder, call := setupAgentToolLedger(t)
			var executed atomic.Int32
			first := newAgentToolLedger(recorder, call).execution(context.Background(), func() agentToolExecution {
				executed.Add(1)
				return agentToolExecution{Outcome: uncertain}
			})
			if first.Outcome.ErrorCode != "TOOL_REQUIRES_RECONCILIATION" {
				t.Fatalf("uncertain side effect was represented as safe failure: %+v", first)
			}
			second := newAgentToolLedger(recorder, call).execution(context.Background(), func() agentToolExecution {
				executed.Add(1)
				return agentToolExecution{}
			})
			if second.Outcome.ErrorCode != "TOOL_REQUIRES_RECONCILIATION" || executed.Load() != 1 {
				t.Fatalf("unconfirmed external effect was replayed: %+v count=%d", second, executed.Load())
			}
			var status string
			if err := db.Table("tool_call_intents").Select("status").Where("tool_call_id = ?", call.ID).Scan(&status).Error; err != nil || status != "INDETERMINATE" {
				t.Fatalf("tool accounting did not preserve unknown state: status=%q err=%v", status, err)
			}
		})
	}
}

func TestAgentToolLedgerRejectsMissingDurabilityStore(t *testing.T) {
	_, _, call := setupAgentToolLedger(t)
	for _, recorder := range []*assistantTurnRecorder{nil, {}} {
		executed := false
		result := newAgentToolLedger(recorder, call).execution(context.Background(), func() agentToolExecution {
			executed = true
			return agentToolExecution{Outcome: toolExecOutcome{Status: "SUCCEEDED", Found: true}}
		})
		if executed || result.Outcome.ErrorCode != "TOOL_REQUIRES_RECONCILIATION" {
			t.Fatalf("executed external operation without durable journal: %+v", result)
		}
	}
}

func TestAgentToolLedgerClassifiesTransportUncertainty(t *testing.T) {
	for _, message := range []string{
		"context deadline exceeded", "read tcp: i/o timeout",
		"connection reset by peer", "websocket: close 1006",
		"remote device disconnected", "unexpected EOF",
	} {
		if !agentToolOutcomeRequiresReconciliation(toolExecOutcome{
			Status: "FAILED", HasError: true, ErrorCode: "TOOL_STREAM_FAILED", ErrorMessage: message,
		}) {
			t.Fatalf("uncertain external effect treated as deterministic failure: %s", message)
		}
	}
	if agentToolOutcomeRequiresReconciliation(toolExecOutcome{
		Status: "FAILED", HasError: true, ErrorCode: "INVALID_ARGS", ErrorMessage: "invalid command parameters",
	}) {
		t.Fatal("deterministic validation error must not masquerade as an unknown external effect")
	}
}

func TestAgentToolLedgerRejectsIncompleteOrContradictoryResultProof(t *testing.T) {
	for _, scenario := range []string{
		"pending_intent", "missing_ref", "unmatched_result_ref", "wrong_attempt",
		"wrong_input_hash", "wrong_result_status", "contradictory_success",
	} {
		t.Run(scenario, func(t *testing.T) {
			db, recorder, call := setupAgentToolLedger(t)
			var count atomic.Int32
			first := newAgentToolLedger(recorder, call).execution(context.Background(), func() agentToolExecution {
				count.Add(1)
				return agentToolExecution{Outcome: toolExecOutcome{Status: "SUCCEEDED", Found: true, VisibleText: "confirmed"}}
			})
			if first.Outcome.HasError {
				t.Fatalf("initial result not durable: %+v", first)
			}
			intentID := agentToolLedgerIntentID(recorder.TurnID, call.Scope.ConversationID, call.Scope.RequestID, call.ID)
			var statement string
			switch scenario {
			case "pending_intent":
				statement = "UPDATE tool_call_intents SET status = 'PENDING' WHERE id = ?"
			case "missing_ref":
				statement = "UPDATE tool_call_intents SET result_ref = '' WHERE id = ?"
			case "unmatched_result_ref":
				statement = "UPDATE tool_call_intents SET result_ref = 'unknown-result' WHERE id = ?"
			case "wrong_attempt":
				statement = "UPDATE tool_call_results SET attempt_id = 'wrong-attempt' WHERE intent_id = ?"
			case "wrong_input_hash":
				statement = "UPDATE tool_call_results SET input_hash = 'wrong-hash' WHERE intent_id = ?"
			case "wrong_result_status":
				statement = "UPDATE tool_call_results SET status = 'FAILED' WHERE intent_id = ?"
			case "contradictory_success":
				statement = "UPDATE tool_call_results SET audit_json = '{\"status\":\"FAILED\",\"hasError\":true,\"found\":true}' WHERE intent_id = ?"
			}
			if err := db.Exec(statement, intentID).Error; err != nil {
				t.Fatal(err)
			}
			second := newAgentToolLedger(recorder, call).execution(context.Background(), func() agentToolExecution {
				count.Add(1)
				return agentToolExecution{}
			})
			if count.Load() != 1 || second.Outcome.ErrorCode != "TOOL_REQUIRES_RECONCILIATION" {
				t.Fatalf("corrupt ledger proof allowed effect replay: scenario=%s count=%d result=%+v", scenario, count.Load(), second)
			}
		})
	}
}

func TestReconcileAgentToolCheckpointRejectsUnlinkedResultProof(t *testing.T) {
	db, recorder, call := setupAgentToolLedger(t)
	if err := db.AutoMigrate(&AssistantTurn{}, &AssistantTurnItem{}); err != nil {
		t.Fatal(err)
	}
	turn := AssistantTurn{
		ID: recorder.TurnID, ConversationID: recorder.ConversationID,
		CharacterID: recorder.CharacterID, RequestID: recorder.RequestID,
		ExecutionID: recorder.ExecutionID, Status: assistantTurnStatusWaitingTool,
	}
	if err := db.Create(&turn).Error; err != nil {
		t.Fatal(err)
	}
	item := AssistantTurnItem{
		ID: "unlinked-tool", TurnID: turn.ID, ConversationID: turn.ConversationID,
		ItemType: assistantTurnItemToolCall, Status: assistantTurnStatusRunning,
		CallID: call.ID, ToolName: call.Name, ArgumentsJSON: call.Arguments,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	outcome := newAgentToolLedger(recorder, call).execution(context.Background(), func() agentToolExecution {
		return agentToolExecution{Outcome: toolExecOutcome{Status: "SUCCEEDED", Found: true}}
	})
	if outcome.Outcome.HasError {
		t.Fatal(outcome)
	}
	id := agentToolLedgerIntentID(turn.ID, turn.ConversationID, turn.RequestID, call.ID)
	if err := db.Exec("UPDATE tool_call_intents SET result_ref = 'forged-result' WHERE id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ReconcileAgentToolCheckpoint(context.Background(), db, turn, []AssistantTurnItem{item}); err == nil {
		t.Fatal("checkpoint reconciled without a matching durable result reference")
	}
	var results int64
	if err := db.Model(&AssistantTurnItem{}).Where("item_type = ?", assistantTurnItemToolResult).Count(&results).Error; err != nil {
		t.Fatal(err)
	}
	if results != 0 {
		t.Fatalf("unproven external effect created %d assistant result checkpoints", results)
	}
}
