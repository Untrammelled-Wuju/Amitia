package interaction

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/decision"
)

func TestMultiAgentDependencyIndexDispatchAndRecovery(t *testing.T) {
	ctx := context.Background()
	c := newTestCoordinator()
	workers := &fakeWorkerRunner{}
	c.starter = workers
	result, err := c.Start(ctx, StartCoordinationRequest{
		ParentGoalID: "goal-1", ParentGoalRevision: 1,
		WorkerRefs: []AgentWorkerRef{{WorkerID: "worker-1", CharacterID: "character-1"}},
		Objectives: []AssignmentObjective{
			{WorkerIndex: 0, Objective: "inspect source"},
			{WorkerIndex: 0, Objective: "build patch", Dependencies: []string{"index:0"}},
			{WorkerIndex: 0, Objective: "verify patch", Dependencies: []string{"index:1"}},
		},
		CompletionPlan: CoordinationRequireAll,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(workers.started) != 1 {
		t.Fatalf("expected only first worker, got %d", len(workers.started))
	}
	ac := c.coordinations[string(result.CoordinationID)]
	for i := 1; i < 3; i++ {
		if len(ac.assignments[i].Dependencies) != 1 || ac.assignments[i].Dependencies[0] != result.AssignmentIDs[i-1] {
			t.Fatalf("unstable dependency resolution: %+v", ac.assignments[i].Dependencies)
		}
	}
	first := ac.assignments[0]
	if err := c.OnAssignmentTerminal(ctx, first.ID, AgentAssignmentResult{AssignmentID: first.ID, ChildInteractionID: first.ChildInteractionID, Status: AssignmentSucceeded}); err != nil {
		t.Fatal(err)
	}
	if len(workers.started) != 2 || workers.started[1].AssignmentID != result.AssignmentIDs[1] {
		t.Fatalf("second worker was not dispatched after prerequisite: %+v", workers.started)
	}
	second := ac.assignments[1]
	if err := c.OnAssignmentTerminal(ctx, second.ID, AgentAssignmentResult{AssignmentID: second.ID, ChildInteractionID: second.ChildInteractionID, Status: AssignmentSucceeded}); err != nil {
		t.Fatal(err)
	}
	if len(workers.started) != 3 || workers.started[2].AssignmentID != result.AssignmentIDs[2] {
		t.Fatalf("third worker was not dispatched after prerequisite: %+v", workers.started)
	}
}

func TestMultiAgentDependencyFailureClosesPendingDescendants(t *testing.T) {
	c := newTestCoordinator()
	workers := &fakeWorkerRunner{}
	c.starter = workers
	res, err := c.Start(context.Background(), StartCoordinationRequest{
		ParentGoalID: "goal-1", ParentGoalRevision: 1,
		WorkerRefs: []AgentWorkerRef{{WorkerID: "worker-1"}},
		Objectives: []AssignmentObjective{
			{WorkerIndex: 0, Objective: "check dependency"},
			{WorkerIndex: 0, Objective: "uses check", Dependencies: []string{"index:0"}},
			{WorkerIndex: 0, Objective: "uses output", Dependencies: []string{"index:1"}},
		}, CompletionPlan: CoordinationRequireAll,
	})
	if err != nil {
		t.Fatal(err)
	}
	ac := c.coordinations[string(res.CoordinationID)]
	first := ac.assignments[0]
	if err := c.OnAssignmentTerminal(context.Background(), first.ID, AgentAssignmentResult{
		AssignmentID: first.ID, ChildInteractionID: first.ChildInteractionID, Status: AssignmentFailed,
		Error: &AssignmentError{Code: "TEST", Message: "prerequisite failed"},
	}); err != nil {
		t.Fatal(err)
	}
	if len(workers.started) != 1 {
		t.Fatal("dependent worker was launched after failed prerequisite")
	}
	for i := 1; i < 3; i++ {
		if a := ac.assignments[i]; a.Status != AssignmentFailed || !strings.Contains(a.Error, "dependency_failed") {
			t.Fatalf("dependent assignment did not converge: %+v", a)
		}
	}
	if ac.status != CoordinationFailed {
		t.Fatalf("parent coordination did not finish after cascade failure: %s", ac.status)
	}
}

func TestMultiAgentStartRejectsInvalidGraphBeforeDispatch(t *testing.T) {
	cases := []struct {
		name         string
		deps0, deps1 []string
	}{
		{"self dependency", []string{"index:0"}, nil},
		{"cycle", []string{"index:1"}, []string{"index:0"}},
		{"unknown dependency", []string{"missing"}, nil},
		{"bad index", []string{"index:99"}, nil},
		{"negative index", []string{"index:-1"}, nil},
		{"duplicate edge", nil, []string{"index:0", "index:0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCoordinator()
			workers := &fakeWorkerRunner{}
			c.starter = workers
			_, err := c.Start(context.Background(), StartCoordinationRequest{
				ParentGoalID: "goal-1", ParentGoalRevision: 1,
				WorkerRefs: []AgentWorkerRef{{WorkerID: "worker-1"}},
				Objectives: []AssignmentObjective{
					{WorkerIndex: 0, Objective: "first", Dependencies: tc.deps0},
					{WorkerIndex: 0, Objective: "second", Dependencies: tc.deps1},
				},
			})
			if err == nil {
				t.Fatal("invalid dependency graph accepted")
			}
			if len(workers.started) != 0 {
				t.Fatal("worker started before graph validation")
			}
			if len(c.coordinations) != 0 {
				t.Fatal("invalid coordination registered")
			}
		})
	}
}

func TestMultiAgentBestEffortWaitsForOtherWorkers(t *testing.T) {
	c := newTestCoordinator()
	res, err := c.Start(context.Background(), StartCoordinationRequest{
		ParentGoalID: "goal-1", ParentGoalRevision: 1,
		WorkerRefs:     []AgentWorkerRef{{WorkerID: "worker-1"}},
		Objectives:     []AssignmentObjective{{WorkerIndex: 0, Objective: "first"}, {WorkerIndex: 0, Objective: "second"}},
		CompletionPlan: CoordinationBestEffort,
	})
	if err != nil {
		t.Fatal(err)
	}
	ac := c.coordinations[string(res.CoordinationID)]
	first := ac.assignments[0]
	if err := c.OnAssignmentTerminal(context.Background(), first.ID, AgentAssignmentResult{AssignmentID: first.ID, ChildInteractionID: first.ChildInteractionID, Status: AssignmentFailed}); err != nil {
		t.Fatal(err)
	}
	if ac.status != CoordinationRunning {
		t.Fatalf("best effort prematurely aggregated on first failure: %s", ac.status)
	}
	second := ac.assignments[1]
	if err := c.OnAssignmentTerminal(context.Background(), second.ID, AgentAssignmentResult{AssignmentID: second.ID, ChildInteractionID: second.ChildInteractionID, Status: AssignmentSucceeded}); err != nil {
		t.Fatal(err)
	}
	if ac.status != CoordinationSucceeded {
		t.Fatalf("best effort lost other worker success: %s", ac.status)
	}
}

func TestMultiAgentTerminalCallbackRequiresStartedWorker(t *testing.T) {
	target := &AgentAssignment{ID: "pending", Status: AssignmentPending}
	result := AgentAssignmentResult{AssignmentID: "pending", Status: AssignmentSucceeded}
	if err := validateTerminalAssignmentResult(target, target.ID, result); err == nil {
		t.Fatal("pending child was accepted as completed")
	}
}

func TestMultiAgentPlanEdgesSurviveDurableRestart(t *testing.T) {
	ctx := context.Background()
	tracker := newFakeTracker()
	if err := tracker.Create(ctx, &InteractionRecord{ID: "parent-dag", Status: InteractionStatusProcessing,
		Scope: InteractionScope{SpaceID: "space-dag", ConversationID: "conversation-dag", CharacterID: "character-dag"},
	}); err != nil {
		t.Fatal(err)
	}
	goals := decision.NewGoalRegistry()
	if err := goals.Register(decision.Goal{ID: "goal-dag", Revision: 1, Status: decision.GoalStatusActive,
		SpaceID: "space-dag", ConversationID: "conversation-dag", CharacterID: "character-dag",
	}); err != nil {
		t.Fatal(err)
	}
	c := NewMultiAgentCoordinator(tracker, goals, nil, &fakeWorkerRunner{}, nil, DefaultMultiAgentPolicy())
	started, err := c.Start(ctx, StartCoordinationRequest{ParentInteractionID: "parent-dag", ParentGoalID: "goal-dag", ParentGoalRevision: 1,
		WorkerRefs: []AgentWorkerRef{{WorkerID: "worker-dag", CharacterID: "worker-character"}},
		Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "gather evidence"}, {WorkerIndex: 0, Objective: "review evidence", Dependencies: []string{"index:0"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	record, found, err := tracker.Get(ctx, "parent-dag")
	if err != nil || !found || record.RecoveryDescriptor == nil || record.RecoveryDescriptor.MultiAgent == nil {
		t.Fatalf("durable parent checkpoint missing: %v", err)
	}
	refs := record.RecoveryDescriptor.MultiAgent.AssignmentRefs
	if len(refs) != 2 || len(refs[1].Dependencies) != 1 || refs[1].Dependencies[0] != started.AssignmentIDs[0] {
		t.Fatalf("non-durable dependency edge: %+v", refs)
	}
	recovered := NewMultiAgentCoordinator(tracker, goals, nil, &fakeWorkerRunner{}, nil, DefaultMultiAgentPolicy())
	if err := recovered.restoreCoordinationFromRecord(record); err != nil {
		t.Fatal(err)
	}
	ac := recovered.coordinations[string(started.CoordinationID)]
	if ac == nil || len(ac.assignments[1].Dependencies) != 1 || ac.assignments[1].Dependencies[0] != started.AssignmentIDs[0] {
		t.Fatal("restart changed predecessor identity")
	}
}
