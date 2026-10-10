package chat

import (
	"context"
	"strings"
	"testing"
)

func TestAssistantTurnRecoveryPreservesExecutionIdentityAndIsolation(t *testing.T) {
	db := testAgentRecoveryDB(t)
	ctx := context.Background()
	first := newAssistantTurnRecorder(db, "conversation-a", "char", "user-a", "req-a")
	if err := first.Start(ctx); err != nil {
		t.Fatal(err)
	}
	resumed := newAssistantTurnRecorder(db, "conversation-a", "char", "user-a", "req-a", first.TurnID)
	if resumed.ExecutionID == first.ExecutionID {
		t.Fatal("test precondition: generated execution identities should differ")
	}
	if err := resumed.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if resumed.ExecutionID != first.ExecutionID {
		t.Fatalf("execution identity changed during restart: %q vs %q", resumed.ExecutionID, first.ExecutionID)
	}
	for _, tc := range []struct{ name, conversation, character, user, request string }{
		{"wrong conversation", "conversation-b", "char", "user-a", "req-a"},
		{"wrong character", "conversation-a", "other-char", "user-a", "req-a"},
		{"wrong user", "conversation-a", "char", "other-user", "req-a"},
		{"wrong request", "conversation-a", "char", "user-a", "req-b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			foreign := newAssistantTurnRecorder(db, tc.conversation, tc.character, tc.user, tc.request, first.TurnID)
			if err := foreign.Start(ctx); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
				t.Fatalf("cross-request checkpoint access was not rejected: %v", err)
			}
		})
	}
	if err := db.Model(&AssistantTurn{}).Where("id = ?", first.TurnID).Update("status", assistantTurnStatusCompleted).Error; err != nil {
		t.Fatal(err)
	}
	finished := newAssistantTurnRecorder(db, "conversation-a", "char", "user-a", "req-a", first.TurnID)
	if err := finished.Start(ctx); err == nil || !strings.Contains(err.Error(), "completed") {
		t.Fatalf("completed turn must not restart: %v", err)
	}
}
