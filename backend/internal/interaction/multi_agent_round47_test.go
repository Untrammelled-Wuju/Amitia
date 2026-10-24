package interaction

import (
	"context"
	"strings"
	"testing"
)

func round47RecoveryReplica(t *testing.T) (*MultiAgentCoordinator, *MultiAgentCoordinator, *InMemoryTracker, string) {
	t.Helper()
	original, tracker, runner, id := round46RecoverySetup(t)
	parent, found, err := tracker.Get(context.Background(), "round46-parent")
	if err != nil || !found {
		t.Fatalf("parent checkpoint: %v", err)
	}
	replica := NewMultiAgentCoordinator(tracker, original.goals, nil, runner, nil, DefaultMultiAgentPolicy())
	if err := replica.restoreCoordinationFromRecord(parent); err != nil {
		t.Fatal(err)
	}
	return original, replica, tracker, id
}

func TestRound47SnapshotReadsLatestDurableStateFromAnotherCore(t *testing.T) {
	ctx := context.Background()
	original, replica, _, id := round47RecoveryReplica(t)
	live := original.coordinations[id]
	live.status = CoordinationWaiting
	live.assignments[0].Status = AssignmentWaiting
	live.assignments[0].Error = "needs external reconciliation"
	if err := original.persistCoordinationSnapshot(ctx, live); err != nil {
		t.Fatal(err)
	}
	got, err := replica.Snapshot(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != CoordinationWaiting || got.Assignments[0].Status != AssignmentWaiting || got.Assignments[0].Error != "needs external reconciliation" {
		t.Fatalf("returned stale Core-local snapshot: %+v", got)
	}
}

func TestRound47SnapshotDoesNotExposeUncommittedTerminalResult(t *testing.T) {
	ctx := context.Background()
	original, _, _, id := round47RecoveryReplica(t)
	live := original.coordinations[id]
	live.status = CoordinationSucceeded
	live.assignments[0].Status = AssignmentSucceeded
	got, err := original.Snapshot(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != CoordinationRunning || got.Assignments[0].Status != AssignmentRunning {
		t.Fatalf("snapshot published uncommitted local completion: %+v", got)
	}
}

func TestRound47SnapshotRejectsMissingTerminalChildEvidence(t *testing.T) {
	ctx := context.Background()
	original, replica, _, id := round47RecoveryReplica(t)
	live := original.coordinations[id]
	live.status = CoordinationSucceeded
	live.assignments[0].Status = AssignmentSucceeded
	if err := original.persistCoordinationSnapshot(ctx, live); err != nil {
		t.Fatal(err)
	}
	if _, err := replica.Snapshot(ctx, id); err == nil || !strings.Contains(err.Error(), "lacks durable child result") {
		t.Fatalf("missing child evidence was reported as success: %v", err)
	}
}

func TestRound47SnapshotIncludesVerifiedTerminalChildResult(t *testing.T) {
	ctx := context.Background()
	original, replica, tracker, id := round47RecoveryReplica(t)
	live := original.coordinations[id]
	childID := live.assignments[0].ChildInteractionID
	if err := tracker.Create(ctx, &InteractionRecord{ID: childID, Status: InteractionStatusCompleted, ResultRef: "verified-result"}); err != nil {
		t.Fatal(err)
	}
	live.assignments[0].Status = AssignmentSucceeded
	live.status = CoordinationSucceeded
	if err := original.persistCoordinationSnapshot(ctx, live); err != nil {
		t.Fatal(err)
	}
	got, err := replica.Snapshot(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != CoordinationSucceeded || got.Assignments[0].Status != AssignmentSucceeded || got.Assignments[0].ResultRef != "verified-result" {
		t.Fatalf("valid durable result not visible: %+v", got)
	}
}

func TestRound47SnapshotRejectsCorruptedDurableFingerprint(t *testing.T) {
	ctx := context.Background()
	_, replica, tracker, id := round47RecoveryReplica(t)
	tracker.mu.Lock()
	tracker.records["round46-parent"].RecoveryDescriptor.MultiAgent.Status = string(CoordinationSucceeded)
	tracker.mu.Unlock()
	if _, err := replica.Snapshot(ctx, id); err == nil || !strings.Contains(err.Error(), "fingerprint") {
		t.Fatalf("tampered coordination checkpoint accepted: %v", err)
	}
}
