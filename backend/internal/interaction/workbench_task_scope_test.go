package interaction

import (
	"context"
	"testing"
	"time"
)

func TestWorkbenchTaskReaderFiltersRoleBeforeApplyingLimit(t *testing.T) {
	tracker := newTestSQLiteInteractionTracker(t)
	for _, role := range []string{"one", "two"} {
		row := InteractionRecordModel{ID: "task-" + role, SpaceID: "space", CharacterID: role, ConversationID: "same-id", Source: "web", RecoveryDescriptorJSON: "{}", UpdatedAt: time.Now()}
		if err := tracker.db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	rows, err := tracker.ListRecentWorkbenchTaskRecords(t.Context(), InteractionScope{SpaceID: "space", CharacterID: "one", ConversationID: "same-id"}, 1)
	if err != nil || len(rows) != 1 || rows[0].Scope.CharacterID != "one" {
		t.Fatalf("wrong role projection: %+v %v", rows, err)
	}
	rows, err = tracker.ListRecentWorkbenchTaskRecords(t.Context(), InteractionScope{SpaceID: "space", CharacterID: "foreign", ConversationID: "same-id"}, 121)
	if err != nil || len(rows) != 0 {
		t.Fatalf("foreign role projection: %+v %v", rows, err)
	}
}

func TestWorkbenchTaskOwnerBindingSurvivesRecovery(t *testing.T) {
	original, tracker, _, id := newReconciliationCoordinator(t, CoordinationSequential)
	active := original.coordinations[id]
	active.executionCoreID, active.resourceOwnerID = "core", "owner-a"
	if err := original.persistCoordinationSnapshot(context.Background(), active); err != nil {
		t.Fatal(err)
	}
	parent := tracker.records["parent-reconcile"]
	restored := NewMultiAgentCoordinator(tracker, original.goals, nil, &fakeWorkerRunner{}, nil, DefaultMultiAgentPolicy())
	if err := restored.restoreCoordinationFromRecord(parent); err != nil {
		t.Fatal(err)
	}
	if got := restored.coordinations[id]; got == nil || got.executionCoreID != "core" || got.resourceOwnerID != "owner-a" {
		t.Fatalf("lost owner binding: %+v", got)
	}
	parent.RecoveryDescriptor.MultiAgent.ResourceOwnerID = "forged-owner"
	if err := restored.restoreCoordinationFromRecord(parent); err == nil {
		t.Fatal("tampered owner binding accepted")
	}
}
