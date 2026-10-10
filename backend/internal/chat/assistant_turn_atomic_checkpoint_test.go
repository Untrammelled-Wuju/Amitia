package chat

import (
	"context"
	"testing"
)

func TestAgentToolOutcomeCheckpointIsAtomicOnDatabaseFailure(t *testing.T) {
	db := testAgentRecoveryDB(t)
	ctx := context.Background()
	recorder := newAssistantTurnRecorder(db, "atomic-conv", "char", "user", "request")
	if err := recorder.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := recorder.AddToolCall(ctx, "call-write", "workspace.write", `{"path":"a.go"}`, assistantTurnStatusRunning); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TRIGGER force_tool_result_failure
		BEFORE INSERT ON assistant_turn_items
		WHEN NEW.item_type = 'tool_result'
		BEGIN SELECT RAISE(ABORT, 'injected tool result failure'); END;`).Error; err != nil {
		t.Fatal(err)
	}
	if err := recorder.AddToolResult(ctx, "call-write", "workspace.write", "changed", assistantTurnStatusCompleted, "", 4); err == nil {
		t.Fatal("expected injected database failure")
	}
	var saved AssistantTurnItem
	if err := db.Where("turn_id = ? AND call_id = ? AND item_type = ?", recorder.TurnID, "call-write", assistantTurnItemToolCall).Take(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != assistantTurnStatusRunning {
		t.Fatalf("partial transaction incorrectly marked call complete: %+v", saved)
	}
	var results int64
	if err := db.Model(&AssistantTurnItem{}).Where("turn_id = ? AND item_type = ?", recorder.TurnID, assistantTurnItemToolResult).Count(&results).Error; err != nil {
		t.Fatal(err)
	}
	if results != 0 {
		t.Fatalf("failed result transaction wrote %d inconsistent result rows", results)
	}
}
