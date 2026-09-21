package chat

import (
	"context"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAssistantTurnRecorderPreservesItemOrderAndFinalMessageLink(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "assistant-turn.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&AssistantTurn{}, &AssistantTurnItem{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	recorder := newAssistantTurnRecorder(db, "conv-1", "char-1", "user-1", "request-1")
	if err := recorder.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	projector := newModelEventProjector(recorder)
	if err := projector.Emit(context.Background(), ModelEvent{Type: ModelEventReasoningSummaryDelta, TextDelta: "分析问题"}); err != nil {
		t.Fatal(err)
	}
	if err := projector.Emit(context.Background(), ModelEvent{Type: ModelEventReasoningSummaryDone}); err != nil {
		t.Fatal(err)
	}
	if err := recorder.AddToolCall(context.Background(), "call-1", "read_file", `{"path":"a.go"}`, assistantTurnStatusRunning); err != nil {
		t.Fatal(err)
	}
	if err := recorder.AddToolResult(context.Background(), "call-1", "read_file", `{"content":"ok"}`, assistantTurnStatusCompleted, "", 42); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return completeAssistantTurnTx(tx, recorder.TurnID, "最终回复", "message-1")
	}); err != nil {
		t.Fatal(err)
	}

	var items []AssistantTurnItem
	if err := db.Where("turn_id = ?", recorder.TurnID).Order("sequence ASC").Find(&items).Error; err != nil {
		t.Fatal(err)
	}
	want := []string{assistantTurnItemReasoning, assistantTurnItemToolCall, assistantTurnItemToolResult, assistantTurnItemText}
	if len(items) != len(want) {
		t.Fatalf("item count = %d, want %d: %#v", len(items), len(want), items)
	}
	for i, itemType := range want {
		if items[i].ItemType != itemType {
			t.Fatalf("item[%d] = %q, want %q", i, items[i].ItemType, itemType)
		}
	}
	if items[1].Status != assistantTurnStatusCompleted || items[1].DurationMS != 42 {
		t.Fatalf("tool call status not reconciled: %+v", items[1])
	}
	if items[3].IsFinal != 1 || items[3].MessageID != "message-1" || items[3].Content != "最终回复" {
		t.Fatalf("final text linkage missing: %+v", items[3])
	}
	var turn AssistantTurn
	if err := db.Where("id = ?", recorder.TurnID).Take(&turn).Error; err != nil {
		t.Fatal(err)
	}
	if turn.Status != assistantTurnStatusCompleted || turn.ExecutionID == "" {
		t.Fatalf("unexpected final turn: %+v", turn)
	}
}
