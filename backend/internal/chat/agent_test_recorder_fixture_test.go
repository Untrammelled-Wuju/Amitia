package chat

import (
	"context"
	"testing"
)

func testAgentLoopRecorder(t *testing.T, conversationID, characterID, requestID string) *assistantTurnRecorder {
	t.Helper()
	db, _, _ := setupAgentToolLedger(t)
	if err := db.AutoMigrate(&AssistantTurn{}, &AssistantTurnItem{}); err != nil {
		t.Fatal(err)
	}
	recorder := newAssistantTurnRecorder(db, conversationID, characterID, "test-user", requestID)
	if err := recorder.Start(context.Background()); err != nil {
		t.Fatalf("create real durable agent turn fixture: %v", err)
	}
	return recorder
}
