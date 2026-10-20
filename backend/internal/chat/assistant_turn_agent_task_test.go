package chat

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPhase16RetrievalAgentTaskDurableAcrossReplay(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "retrieval-turn.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&AssistantTurn{}, &AssistantTurnItem{}); err != nil {
		t.Fatal(err)
	}
	recorder := newAssistantTurnRecorder(db, "conv-16", "char-16", "user-16", "request-16")
	ctx := context.Background()
	if err := recorder.Start(ctx); err != nil {
		t.Fatal(err)
	}
	blockID, startedAt, err := recorder.BeginAgentTask(ctx, "retrieval.started", "检索知识证据", map[string]any{"status": "running"})
	if err != nil || blockID == "" {
		t.Fatalf("start agent task: %v, block=%s", err, blockID)
	}
	if err := recorder.ProgressAgentTask(ctx, blockID, "investigation.round", map[string]any{"round": 1}); err != nil {
		t.Fatal(err)
	}
	if err := recorder.CompleteAgentTask(ctx, blockID, "retrieval.completed", assistantTurnStatusCompleted, map[string]any{"status": "done", "evidenceCount": 2}, startedAt); err != nil {
		t.Fatal(err)
	}
	var saved AssistantTurnItem
	if err := db.Where("id = ?", blockID).Take(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if saved.ItemType != assistantTurnItemAgentTask || saved.Status != assistantTurnStatusCompleted || saved.Revision < 3 {
		t.Fatalf("invalid task checkpoint: %+v", saved)
	}
	var content map[string]any
	if err := json.Unmarshal([]byte(saved.ResultJSON), &content); err != nil || content["status"] != "done" {
		t.Fatalf("checkpoint payload: %v %#v", err, content)
	}
	var persisted []AssistantTurnItem
	if err := db.Where("turn_id = ?", recorder.TurnID).Find(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 1 || persisted[0].ID != blockID {
		t.Fatalf("replay missing task block: %+v", persisted)
	}
}
