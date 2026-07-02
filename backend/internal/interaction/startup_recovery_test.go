package interaction

import (
	"context"
	"testing"
	"time"
)

func TestRecoverStaleInteractionsFailsStaleRuntimeRecords(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	ctx := context.Background()
	cutoff := time.Now().Add(-time.Minute)
	statuses := []InteractionStatus{
		InteractionStatusProcessing,
		InteractionStatusContextReady,
		InteractionStatusDecided,
		InteractionStatusGenerated,
	}
	for _, status := range statuses {
		record := NewInteractionRecord(InteractionScope{
			UserID:         "user-1",
			CharacterID:    "char-1",
			ConversationID: "conv-" + string(status),
			Channel:        "web",
			RequestID:      "req-" + string(status),
		})
		record.Status = status
		record.UpdatedAt = cutoff.Add(-time.Minute)
		if err := tracker.Create(ctx, record); err != nil {
			t.Fatal(err)
		}
	}

	result, err := RecoverStaleInteractions(ctx, tracker, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if result.Recovered != len(statuses) || result.Failed != 0 {
		t.Fatalf("unexpected recovery result: %#v", result)
	}
	for _, status := range statuses {
		got, ok, err := tracker.GetByRequestID(ctx, "user-1", "req-"+string(status))
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("record missing for status %s", status)
		}
		if got.Status != InteractionStatusFailed || got.ErrorCode != "startup_recovered" {
			t.Fatalf("record was not recovered: %#v", got)
		}
	}
}

func TestRecoverStaleInteractionsSkipsFreshAndTerminalRecords(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	ctx := context.Background()
	cutoff := time.Now().Add(-time.Minute)

	fresh := NewInteractionRecord(InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-fresh", Channel: "web", RequestID: "req-fresh"})
	fresh.Status = InteractionStatusProcessing
	fresh.UpdatedAt = cutoff.Add(time.Second)
	if err := tracker.Create(ctx, fresh); err != nil {
		t.Fatal(err)
	}
	terminal := NewInteractionRecord(InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-terminal", Channel: "web", RequestID: "req-terminal"})
	terminal.Status = InteractionStatusCompleted
	terminal.CompletedAt = cutoff.Add(-time.Minute)
	terminal.UpdatedAt = cutoff.Add(-time.Minute)
	if err := tracker.Create(ctx, terminal); err != nil {
		t.Fatal(err)
	}

	result, err := RecoverStaleInteractions(ctx, tracker, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if result.Recovered != 0 {
		t.Fatalf("unexpected recovery result: %#v", result)
	}
	gotFresh, ok, err := tracker.Get(ctx, fresh.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || gotFresh.Status != InteractionStatusProcessing {
		t.Fatalf("fresh record should remain processing: %#v", gotFresh)
	}
	gotTerminal, ok, err := tracker.Get(ctx, terminal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || gotTerminal.Status != InteractionStatusCompleted {
		t.Fatalf("terminal record should remain completed: %#v", gotTerminal)
	}
}
