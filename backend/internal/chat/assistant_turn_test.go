package chat

import (
	"context"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAssistantTurnRecorderPreservesItemOrder(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "assistant-turn.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&AssistantTurn{}, &AssistantTurnItem{}, &AssistantTurnEvent{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	var streamEvents []AssistantTurnStreamEvent
	SetAssistantTurnStreamPublisher(func(event AssistantTurnStreamEvent) {
		streamEvents = append(streamEvents, event)
	})
	t.Cleanup(func() {
		SetAssistantTurnStreamPublisher(nil)
	})
	recorder := newAssistantTurnRecorder(db, "conv-1", "char-1", "user-1", "request-1", "web")
	if err := recorder.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := recorder.AddThinking(context.Background(), "分析问题", 1200); err != nil {
		t.Fatal(err)
	}
	if err := recorder.AddToolCall(context.Background(), "call-1", "read_file", `{"path":"a.go"}`, "running"); err != nil {
		t.Fatal(err)
	}
	if err := recorder.AddToolResult(context.Background(), "call-1", "read_file", `{"content":"ok"}`, assistantTurnStatusCompleted, "", 42); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return completeAssistantTurnTx(tx, recorder.TurnID, "request-1", "最终回复", []string{"message-1"})
	}); err != nil {
		t.Fatal(err)
	}
	var items []AssistantTurnItem
	if err := db.Where("turn_id = ?", recorder.TurnID).Order("sequence ASC").Find(&items).Error; err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(items))
	for _, item := range items {
		got = append(got, item.ItemType)
	}
	want := []string{assistantTurnItemThinking, assistantTurnItemToolCall, assistantTurnItemToolResult, assistantTurnItemText}
	if len(got) != len(want) {
		t.Fatalf("item count = %d, want %d: %v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("item[%d] = %q, want %q", index, got[index], want[index])
		}
	}
	if items[1].Status != assistantTurnStatusCompleted || items[1].DurationMS != 42 {
		t.Fatalf("tool call status not reconciled: %+v", items[1])
	}
	if items[3].IsFinal != 1 || items[3].LegacyMessageID != "message-1" {
		t.Fatalf("final text linkage missing: %+v", items[3])
	}
	var startedToolEvent *AssistantTurnStreamEvent
	var updatedToolEvent *AssistantTurnStreamEvent
	for index := range streamEvents {
		event := &streamEvents[index]
		if event.EventType == "item.started" && event.Item != nil && event.Item.ItemType == assistantTurnItemToolCall {
			startedToolEvent = event
		}
		if event.EventType == "item.updated" && event.Item != nil && event.Item.ItemType == assistantTurnItemToolCall {
			updatedToolEvent = event
		}
	}
	if startedToolEvent == nil || startedToolEvent.Item.Sequence != 2 || startedToolEvent.Status != assistantTurnStatusRunning {
		t.Fatalf("tool start stream event missing sequence/running status: %+v", startedToolEvent)
	}
	if updatedToolEvent == nil || updatedToolEvent.Item.Sequence != 2 || updatedToolEvent.Item.ArgumentsJSON == "" || updatedToolEvent.Status != assistantTurnStatusRunning {
		t.Fatalf("tool update stream event missing original item fields: %+v", updatedToolEvent)
	}
}
