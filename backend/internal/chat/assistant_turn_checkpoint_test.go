package chat

import (
	"context"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAssistantTurnPersistsToolCheckpointsBeforeFinalCommit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "checkpoint.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&AssistantTurn{}, &AssistantTurnItem{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	recorder := newAssistantTurnRecorder(db, "checkpoint-conv", "char", "user", "request")
	if err := recorder.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := recorder.AddToolCall(ctx, "file-call", "write_file", `{"path":"README.md"}`, assistantTurnStatusRunning); err != nil {
		t.Fatal(err)
	}
	var call AssistantTurnItem
	if err := db.Where("turn_id = ? AND call_id = ? AND item_type = ?", recorder.TurnID, "file-call", assistantTurnItemToolCall).Take(&call).Error; err != nil {
		t.Fatalf("in-progress tool call missing from durable journal: %v", err)
	}
	if call.Status != assistantTurnStatusRunning || call.ToolName != "write_file" {
		t.Fatalf("invalid running checkpoint: %+v", call)
	}
	if err := recorder.AddToolResult(ctx, "file-call", "write_file", `{"changed":true}`, assistantTurnStatusCompleted, "", 5); err != nil {
		t.Fatal(err)
	}
	var saved []AssistantTurnItem
	if err := db.Where("turn_id = ? AND call_id = ?", recorder.TurnID, "file-call").Order("sequence ASC").Find(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if len(saved) != 2 || saved[0].Status != assistantTurnStatusCompleted || saved[1].ItemType != assistantTurnItemToolResult {
		t.Fatalf("tool call/result checkpoints missing before final commit: %+v", saved)
	}
	restored := newAssistantTurnRecorder(db, "checkpoint-conv", "char", "user", "request", recorder.TurnID, recorder.ExecutionID)
	if err := restored.Start(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, ok := restored.toolCall("file-call")
	if !ok || recovered.Status != assistantTurnStatusCompleted {
		t.Fatalf("new recorder could not read persisted checkpoint: %+v, found=%v", recovered, ok)
	}
	if err := restored.AddToolCall(ctx, "file-call", "write_file", `{"path":"README.md"}`, assistantTurnStatusRunning); err == nil {
		t.Fatal("completed tool call must not be restarted with the same call ID")
	}
	if err := restored.AddToolResult(ctx, "file-call", "write_file", "duplicate", assistantTurnStatusCompleted, "", 1); err == nil {
		t.Fatal("completed tool result must not be persisted a second time")
	}
}
