package chat

import (
	"context"
	"testing"
)

func TestLiveAgentEventsPersistReplayCheckpointBeforeTurnCommit(t *testing.T) {
	db := testAgentRecoveryDB(t)
	ctx := context.Background()
	recorder := newAssistantTurnRecorder(db, "live-checkpoint", "char", "user", "req")
	if err := recorder.Start(ctx); err != nil {
		t.Fatal(err)
	}
	projector := newModelEventProjector(recorder)
	if err := projector.Emit(ctx, ModelEvent{Type: ModelEventTextDelta, TextDelta: "partial stream"}); err != nil {
		t.Fatal(err)
	}
	if err := projector.checkpoint(ctx, projector.text); err != nil {
		t.Fatal(err)
	}
	var textItem AssistantTurnItem
	if err := db.Where("turn_id = ? AND item_type = ?", recorder.TurnID, assistantTurnItemText).First(&textItem).Error; err != nil {
		t.Fatalf("live partial text not journaled: %v", err)
	}
	if textItem.Content != "partial stream" || textItem.Status != assistantTurnStatusRunning {
		t.Fatalf("live text checkpoint lost delta: %+v", textItem)
	}
	if err := projector.Emit(ctx, ModelEvent{Type: ModelEventToolCallStarted, ToolCallID: "live-tool", ToolName: "read_file"}); err != nil {
		t.Fatal(err)
	}
	if err := projector.Emit(ctx, ModelEvent{Type: ModelEventToolCallArgumentsDelta, ToolCallID: "live-tool", ToolName: "read_file", ArgumentsDelta: `{"path":"a.go"}`}); err != nil {
		t.Fatal(err)
	}
	if err := projector.Emit(ctx, ModelEvent{Type: ModelEventToolCallDone, ToolCallID: "live-tool", ToolName: "read_file"}); err != nil {
		t.Fatal(err)
	}
	var toolItem AssistantTurnItem
	if err := db.Where("turn_id = ? AND call_id = ? AND item_type = ?", recorder.TurnID, "live-tool", assistantTurnItemToolCall).First(&toolItem).Error; err != nil {
		t.Fatalf("tool invocation not persisted before provider completion: %v", err)
	}
	if toolItem.ArgumentsJSON != `{"path":"a.go"}` || toolItem.Status != assistantTurnStatusRunning {
		t.Fatalf("in-flight tool arguments not stored: %+v", toolItem)
	}
	if err := projector.Complete(ctx, assistantTurnStatusInterrupted); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("id = ?", toolItem.ID).Take(&toolItem).Error; err != nil {
		t.Fatal(err)
	}
	if toolItem.Status != assistantTurnStatusInterrupted {
		t.Fatalf("interrupted event did not persist terminal state: %+v", toolItem)
	}
}

func TestStreamingToolIDsCannotReplaceAlreadyJournaledCall(t *testing.T) {
	db := testAgentRecoveryDB(t)
	ctx := context.Background()
	recorder := newAssistantTurnRecorder(db, "unique-call", "char", "user", "request")
	if err := recorder.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := recorder.AddToolCall(ctx, "original", "write_file", `{"path":"a.go"}`, assistantTurnStatusRunning); err != nil {
		t.Fatal(err)
	}
	projector := newModelEventProjector(recorder)
	if err := projector.Emit(ctx, ModelEvent{Type: ModelEventToolCallStarted, ToolCallID: "original", ToolName: "write_file"}); err == nil {
		t.Fatal("streamed tool must not replace an existing durable call ID")
	}
	var count int64
	if err := db.Model(&AssistantTurnItem{}).Where("turn_id = ? AND call_id = ?", recorder.TurnID, "original").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("duplicate tool ID created %d journal items", count)
	}
}
