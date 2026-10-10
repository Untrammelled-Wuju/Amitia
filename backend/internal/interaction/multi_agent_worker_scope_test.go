package interaction

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/decision"
)

func TestMultiAgentChildInheritsParentSpaceAndConversation(t *testing.T) {
	ctx := context.Background()
	tracker := newFakeTracker()
	_ = tracker.Create(ctx, &InteractionRecord{
		ID: "parent-scope", Status: InteractionStatusProcessing,
		Scope: InteractionScope{SpaceID: "space-a", ConversationID: "conversation-a", CharacterID: "parent-char"},
	})
	goals := decision.NewGoalRegistry()
	if err := goals.Register(decision.Goal{ID: "goal-scope", SpaceID: "space-a", ConversationID: "conversation-a", Revision: 1, Status: decision.GoalStatusActive}); err != nil {
		t.Fatal(err)
	}
	workers := &fakeWorkerRunner{}
	coordinator := NewMultiAgentCoordinator(tracker, goals, nil, workers, nil, DefaultMultiAgentPolicy())
	_, err := coordinator.Start(ctx, StartCoordinationRequest{
		ParentInteractionID: "parent-scope", ParentGoalID: "goal-scope", ParentGoalRevision: 1,
		WorkspaceID: "workspace-1", WorkspaceDeviceID: "device-1",
		WorkspaceName: "Amitia", WorkspaceKind: "git", WorkspaceRootURI: "file:///workspace", PermissionMode: "request_approval",
		WorkerRefs: []AgentWorkerRef{{WorkerID: "worker-a", CharacterID: "worker-char"}},
		Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "run scoped coding task"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(workers.started) != 1 {
		t.Fatalf("expected one worker, got %d", len(workers.started))
	}
	started := workers.started[0]
	if started.SpaceID != "space-a" || started.ConversationID != "conversation-a" || started.CharacterID != "worker-char" {
		t.Fatalf("child scope was dropped: %+v", started)
	}
	if started.WorkspaceID != "workspace-1" || started.WorkspaceDeviceID != "device-1" ||
		started.WorkspaceKind != "git" || started.WorkspaceRootURI != "file:///workspace" || started.PermissionMode != "request_approval" {
		t.Fatalf("child coding workspace was dropped: %+v", started)
	}
	parent := tracker.records["parent-scope"]
	ref := parent.RecoveryDescriptor.MultiAgent
	if ref.WorkspaceID != "workspace-1" || ref.WorkspaceDeviceID != "device-1" {
		t.Fatalf("coding workspace did not survive durable checkpoint: %+v", ref)
	}
	restarted := NewMultiAgentCoordinator(tracker, goals, nil, &fakeWorkerRunner{}, nil, DefaultMultiAgentPolicy())
	if err := restarted.restoreCoordinationFromRecord(parent); err != nil {
		t.Fatal(err)
	}
	for _, recovered := range restarted.coordinations {
		if recovered.workspaceID != "workspace-1" || recovered.workspaceDeviceID != "device-1" || recovered.permissionMode != "request_approval" {
			t.Fatalf("restart lost coding workspace: %+v", recovered)
		}
	}
}

func TestMultiAgentRejectsCrossSpaceChildDispatch(t *testing.T) {
	ctx := context.Background()
	tracker := newFakeTracker()
	_ = tracker.Create(ctx, &InteractionRecord{
		ID: "parent-scope", Status: InteractionStatusProcessing,
		Scope: InteractionScope{SpaceID: "space-b", ConversationID: "conversation-a"},
	})
	goals := decision.NewGoalRegistry()
	_ = goals.Register(decision.Goal{ID: "goal-scope", SpaceID: "space-a", ConversationID: "conversation-a", Revision: 1, Status: decision.GoalStatusActive})
	workers := &fakeWorkerRunner{}
	coordinator := NewMultiAgentCoordinator(tracker, goals, nil, workers, nil, DefaultMultiAgentPolicy())
	_, err := coordinator.Start(ctx, StartCoordinationRequest{
		ParentInteractionID: "parent-scope", ParentGoalID: "goal-scope", ParentGoalRevision: 1,
		WorkerRefs: []AgentWorkerRef{{WorkerID: "worker-a", CharacterID: "worker-char"}},
		Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "do not cross the space boundary"}},
	})
	if err == nil || !strings.Contains(err.Error(), "space identities conflict") {
		t.Fatalf("cross-space dispatch must be refused: %v", err)
	}
	if len(workers.started) > 0 {
		t.Fatal("worker was dispatched across a mismatched space")
	}
}

func TestMultiAgentUnifiedWorkerRejectsMissingIdentity(t *testing.T) {
	goals := decision.NewGoalRegistry()
	_ = goals.Register(decision.Goal{ID: "goal", Revision: 1, Status: decision.GoalStatusActive})
	c := NewMultiAgentCoordinator(newFakeTracker(), goals, nil, NewUnifiedEntryWorkerRunner(nil), nil, DefaultMultiAgentPolicy())
	_, err := c.Start(context.Background(), StartCoordinationRequest{
		ParentGoalID: "goal", ParentGoalRevision: 1,
		WorkerRefs: []AgentWorkerRef{{WorkerID: "worker", CharacterID: "character"}},
		Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "do task"}},
	})
	if err == nil || !strings.Contains(err.Error(), "requires a durable space and conversation scope") {
		t.Fatalf("unscoped internal worker execution must fail closed: %v", err)
	}
}

func TestWorkerConversationIsDistinctAndDeterministic(t *testing.T) {
	parent := InteractionScope{SpaceID: "space", CharacterID: "character", ConversationID: "original", Channel: "web"}
	req := WorkerRunRequest{SpaceID: "space", CharacterID: "character", ConversationID: "original", AssignmentID: "assignment-a"}
	childID := workerConversationID(req)
	child := InteractionScope{SpaceID: req.SpaceID, CharacterID: req.CharacterID, ConversationID: childID, Channel: "web"}
	if childID == parent.ConversationID || sameSupersedeScope(parent, child) {
		t.Fatalf("sub-agent conversation %q would supersede the parent", childID)
	}
	if childID != workerConversationID(req) {
		t.Fatal("sub-agent conversation must survive retry and restart")
	}
	req.AssignmentID = "assignment-b"
	if childID == workerConversationID(req) {
		t.Fatal("different workers shared a conversation")
	}
}
