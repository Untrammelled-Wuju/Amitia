package interaction

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/decision"
)

func TestMultiAgentAbandonedProcessingChildRequiresReconciliationNotReplay(t *testing.T) {
	ctx := context.Background()
	runner, tracker, processor := newAsyncWorkerFixture(t)
	defer func() { close(processor.release); _ = runner.Shutdown(context.Background()) }()
	parent := NewInteractionRecord(InteractionScope{
		SpaceID: "space-a", CharacterID: "character-a",
		ConversationID: "parent-conversation", RequestID: "crash-parent", Channel: "web",
	})
	parent.Status = InteractionStatusProcessing
	if err := tracker.Create(ctx, parent); err != nil {
		t.Fatal(err)
	}
	goals := decision.NewGoalRegistry()
	if err := goals.Register(decision.Goal{
		ID: "crash-goal", SpaceID: "space-a", CharacterID: "character-a",
		ConversationID: "parent-conversation", Status: decision.GoalStatusActive, Revision: 1,
	}); err != nil {
		t.Fatal(err)
	}
	req := asyncReq("crashed-child")
	child := NewInteractionRecord(InteractionScope{
		SpaceID: req.SpaceID, CharacterID: req.CharacterID,
		ConversationID: workerConversationID(req), RequestID: "ma-" + req.AssignmentID,
		Source: "multi_agent", Channel: "web",
	})
	child.ID = runner.WorkerInteractionID(req)
	child.Status = InteractionStatusProcessing
	if err := tracker.Create(ctx, child); err != nil {
		t.Fatal(err)
	}
	assignment := &AgentAssignment{
		ID: req.AssignmentID, CoordinationID: "crashed-coordination",
		ParentGoalID: "crash-goal", ParentGoalRevision: 1,
		WorkerRef: AgentWorkerRef{WorkerID: "worker-a", CharacterID: "character-a"},
		Objective: req.Objective, Status: AssignmentRunning, ChildInteractionID: child.ID,
	}
	ac := &activeCoordination{
		id: CoordinationID("crashed-coordination"), parentInteractionID: parent.ID,
		parentGoalID: "crash-goal", parentGoalRev: 1,
		status: CoordinationRunning, strategy: CoordinationSequential,
		completionPlan: CoordinationRequireAll, workspaceID: "workspace-a",
		assignments: []*AgentAssignment{assignment},
	}
	coord := NewMultiAgentCoordinator(tracker, goals, nil, runner, nil, DefaultMultiAgentPolicy())
	coord.coordinations[string(ac.id)] = ac
	if err := coord.persistCoordinationSnapshot(ctx, ac); err != nil {
		t.Fatal(err)
	}
	if err := coord.Reconcile(ctx, string(ac.id)); err != nil {
		t.Fatal(err)
	}
	snapshot, err := coord.Snapshot(ctx, string(ac.id))
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != CoordinationWaiting || snapshot.Assignments[0].Status != AssignmentWaiting ||
		!strings.Contains(snapshot.Assignments[0].Error, "manual reconciliation") {
		t.Fatalf("orphaned side effects were treated as running or safe to replay: %+v", snapshot)
	}
	if processor.calls.Load() != 0 {
		t.Fatal("abandoned worker was replayed")
	}
	persisted, found, err := tracker.Get(ctx, parent.ID)
	if err != nil || !found {
		t.Fatalf("parent checkpoint missing: %v", err)
	}
	restored := NewMultiAgentCoordinator(tracker, goals, nil, runner, nil, DefaultMultiAgentPolicy())
	if err := restored.RecoverCoordination(ctx, persisted); err != nil {
		t.Fatal(err)
	}
	snapshot, err = restored.Snapshot(ctx, string(ac.id))
	if err != nil || snapshot.Status != CoordinationWaiting ||
		!strings.Contains(snapshot.Assignments[0].Error, "manual reconciliation") {
		t.Fatalf("crash recovery discarded indeterminate worker evidence: %+v %v", snapshot, err)
	}
	latest, _, err := tracker.Get(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tracker.Complete(ctx, child.ID, latest.StatusVersion, "verified-existing-worker"); err != nil {
		t.Fatal(err)
	}
	if err := restored.Reconcile(ctx, string(ac.id)); err != nil {
		t.Fatal(err)
	}
	finished, err := restored.Snapshot(ctx, string(ac.id))
	if err != nil {
		t.Fatal(err)
	}
	if finished.Status != CoordinationSucceeded ||
		finished.Assignments[0].ResultRef != "verified-existing-worker" ||
		processor.calls.Load() != 0 {
		t.Fatalf("actual terminal event failed to resolve indeterminate work safely: %+v", finished)
	}
}
