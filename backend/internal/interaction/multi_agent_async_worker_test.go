package interaction

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/temporal"
)

type blockingAsyncWorkerProcessor struct {
	entered chan string
	release chan struct{}
	calls   atomic.Int32
}

func (p *blockingAsyncWorkerProcessor) ProcessMessageCtx(ctx context.Context, req *ProcessRequest) (*ProcessResponse, error) {
	p.calls.Add(1)
	select {
	case p.entered <- req.InteractionID:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case <-p.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &ProcessResponse{
		ConversationID: req.ConversationID, CharacterID: req.CharacterID,
		RequestID: req.RequestID, Reply: "verified " + req.Message, MessageIDs: []string{"worker-result"},
	}, nil
}

func newAsyncWorkerFixture(t *testing.T) (*asyncUnifiedEntryWorkerRunner, *InMemoryTracker, *blockingAsyncWorkerProcessor) {
	t.Helper()
	tracker := NewInMemoryTracker()
	processor := &blockingAsyncWorkerProcessor{entered: make(chan string, 8), release: make(chan struct{})}
	orch := NewOrchestratorWithStores(DefaultOrchestratorConfig(), processor, tracker, nil)
	orch.SetReady(true)
	entry := NewUnifiedEntry(orch, NewScopeResolver(nil), temporal.SystemClock{})
	runner, ok := NewAsyncUnifiedEntryWorkerRunner(entry, tracker).(*asyncUnifiedEntryWorkerRunner)
	if !ok {
		t.Fatal("async runner has unexpected type")
	}
	return runner, tracker, processor
}

func asyncReq(id string) WorkerRunRequest {
	return WorkerRunRequest{
		AssignmentID: id, CoordinationID: "coord-a", ParentInteractionID: "parent-a",
		ParentGoalID: "goal-a", ParentGoalRevision: 1, SpaceID: "space-a",
		CharacterID: "character-a", ConversationID: "parent-conversation",
		WorkspaceID: "workspace-a", PermissionMode: "request_approval",
		Source: "multi_agent", Objective: "Inspect and validate " + id,
	}
}

func TestAsyncUnifiedWorkersRunInParallelWithoutReplayingRequests(t *testing.T) {
	runner, tracker, processor := newAsyncWorkerFixture(t)
	defer func() { _ = runner.Shutdown(context.Background()) }()
	first := asyncReq("worker-a")
	second := asyncReq("worker-b")
	ctx := context.Background()
	start := time.Now()
	id1, err := runner.StartWorker(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := runner.StartWorker(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("worker dispatch blocked on the synchronous agent execution")
	}
	if id1 == "" || id2 == "" || id1 == id2 {
		t.Fatalf("invalid durable child IDs %q %q", id1, id2)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-processor.entered:
		case <-time.After(3 * time.Second):
			t.Fatal("parallel workers did not both enter execution")
		}
	}
	if processor.calls.Load() != 2 {
		t.Fatalf("expected two true concurrent processor calls, got %d", processor.calls.Load())
	}
	if repeated, err := runner.StartWorker(ctx, first); err != nil || repeated != id1 {
		t.Fatalf("duplicate reservation changed execution: %q, %v", repeated, err)
	}
	if processor.calls.Load() != 2 {
		t.Fatal("same assignment launched duplicate processor")
	}
	for _, pair := range []struct {
		id  string
		req WorkerRunRequest
	}{{id1, first}, {id2, second}} {
		rec, ok, err := tracker.GetByRequestID(ctx, pair.req.SpaceID, "ma-"+pair.req.AssignmentID)
		if err != nil || !ok || rec.ID != pair.id {
			t.Fatalf("child reservation does not match tracker: %+v err=%v", rec, err)
		}
		if rec.Scope.ConversationID == pair.req.ConversationID {
			t.Fatal("worker inherited parent conversation")
		}
	}
	close(processor.release)
	for _, id := range []string{id1, id2} {
		deadline := time.After(3 * time.Second)
		for {
			rec, ok, err := tracker.Get(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if ok && rec.IsTerminal() {
				break
			}
			select {
			case <-time.After(5 * time.Millisecond):
			case <-deadline:
				t.Fatalf("worker %s never completed", id)
			}
		}
	}
	if processor.calls.Load() != 2 {
		t.Fatalf("processor replayed an assignment: %d", processor.calls.Load())
	}
}

func TestAsyncWorkerResumesDurablyReservedReceivedInteraction(t *testing.T) {
	runner, tracker, processor := newAsyncWorkerFixture(t)
	defer func() { close(processor.release); _ = runner.Shutdown(context.Background()) }()
	req := asyncReq("reserved-worker")
	input := &UnifiedEntryRequest{Channel: "web", Source: req.Source, SpaceID: req.SpaceID, CharacterID: req.CharacterID,
		ConversationID: workerConversationID(req), RequestID: "ma-" + req.AssignmentID}
	scope, err := runner.entry.ResolveScope(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	record := NewInteractionRecord(scope.Scope)
	record.ID = runner.WorkerInteractionID(req)
	if err := tracker.Create(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	id, err := runner.StartWorker(context.Background(), req)
	if err != nil || id != record.ID {
		t.Fatalf("reserved execution could not resume: %q %v", id, err)
	}
	select {
	case <-processor.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("reserved worker never executed")
	}
	if processor.calls.Load() != 1 {
		t.Fatal("resumed worker was executed more than once")
	}
}

func TestAsyncWorkerShutdownCancelsActiveProcessing(t *testing.T) {
	runner, tracker, processor := newAsyncWorkerFixture(t)
	req := asyncReq("shutdown-worker")
	id, err := runner.StartWorker(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-processor.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := runner.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed to wait for worker cancellation: %v", err)
	}
	if _, err := runner.StartWorker(context.Background(), asyncReq("new-worker")); err == nil {
		t.Fatal("shutdown runner accepted new tasks")
	}
	rec, ok, err := tracker.Get(context.Background(), id)
	if err != nil || !ok {
		t.Fatalf("child record lost at shutdown: %v", err)
	}
	if !rec.IsTerminal() {
		t.Fatalf("worker remained in flight after shutdown: %s", rec.Status)
	}
}

func TestAsyncMultiAgentCoordinatorReallyDispatchesInParallelAndPreservesParent(t *testing.T) {
	ctx := context.Background()
	runner, tracker, processor := newAsyncWorkerFixture(t)
	defer func() { _ = runner.Shutdown(context.Background()) }()
	parent := NewInteractionRecord(InteractionScope{SpaceID: "space-a", CharacterID: "character-a", ConversationID: "parent-conversation", Channel: "web", RequestID: "parent-request"})
	parent.Status = InteractionStatusProcessing
	if err := tracker.Create(ctx, parent); err != nil {
		t.Fatal(err)
	}
	goals := decision.NewGoalRegistry()
	if err := goals.Register(decision.Goal{
		ID: "goal-async", SpaceID: "space-a", CharacterID: "character-a",
		ConversationID: "parent-conversation", Revision: 1, Status: decision.GoalStatusActive,
	}); err != nil {
		t.Fatal(err)
	}
	coordinator := NewMultiAgentCoordinator(tracker, goals, nil, runner, nil, DefaultMultiAgentPolicy())
	started, err := coordinator.Start(ctx, StartCoordinationRequest{
		ParentInteractionID: parent.ID, ParentGoalID: "goal-async", ParentGoalRevision: 1,
		Strategy: CoordinationParallel, CompletionPlan: CoordinationRequireAll,
		WorkspaceID: "workspace-a", PermissionMode: "request_approval",
		WorkerRefs: []AgentWorkerRef{{WorkerID: "w1", CharacterID: "character-a"}, {WorkerID: "w2", CharacterID: "character-a"}},
		Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "check build output"}, {WorkerIndex: 1, Objective: "review source code"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(started.AssignmentIDs) != 2 {
		t.Fatalf("expected two worker tasks: %+v", started)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-processor.entered:
		case <-time.After(3 * time.Second):
			t.Fatal("coordinator failed to start workers concurrently")
		}
	}
	parentNow, ok, err := tracker.Get(ctx, parent.ID)
	if err != nil || !ok || parentNow.Status != InteractionStatusProcessing {
		t.Fatalf("sub agents unexpectedly superseded parent turn: %+v %v", parentNow, err)
	}
	close(processor.release)
	deadline := time.After(3 * time.Second)
	for {
		if err := coordinator.Reconcile(ctx, string(started.CoordinationID)); err != nil {
			t.Fatal(err)
		}
		snapshot, err := coordinator.Snapshot(ctx, string(started.CoordinationID))
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Status == CoordinationSucceeded {
			if len(snapshot.Assignments) != 2 || snapshot.Assignments[0].ResultRef == "" || snapshot.Assignments[1].ResultRef == "" {
				t.Fatalf("missing successful worker references: %+v", snapshot)
			}
			break
		}
		select {
		case <-time.After(10 * time.Millisecond):
		case <-deadline:
			t.Fatalf("coordination remained %s after worker completion", snapshot.Status)
		}
	}
	if processor.calls.Load() != 2 {
		t.Fatalf("coordinator ran %d worker executions instead of 2", processor.calls.Load())
	}
}

func TestAsyncCoordinatorCanRecoverReservedWorkerWithoutRecord(t *testing.T) {
	ctx := context.Background()
	runner, tracker, processor := newAsyncWorkerFixture(t)
	defer func() { close(processor.release); _ = runner.Shutdown(context.Background()) }()
	parent := NewInteractionRecord(InteractionScope{SpaceID: "space-a", CharacterID: "character-a", ConversationID: "parent-conversation", Channel: "web", RequestID: "parent-for-recovery"})
	if err := tracker.Create(ctx, parent); err != nil {
		t.Fatal(err)
	}
	goals := decision.NewGoalRegistry()
	if err := goals.Register(decision.Goal{
		ID: "goal-recovery", SpaceID: "space-a", CharacterID: "character-a",
		ConversationID: "parent-conversation", Revision: 1, Status: decision.GoalStatusActive,
	}); err != nil {
		t.Fatal(err)
	}
	coordinator := NewMultiAgentCoordinator(tracker, goals, nil, runner, nil, DefaultMultiAgentPolicy())
	assignment := &AgentAssignment{
		ID: "reservation-a", CoordinationID: "coord-recover", ParentGoalID: "goal-recovery",
		ParentGoalRevision: 1, WorkerRef: AgentWorkerRef{WorkerID: "worker", CharacterID: "character-a"},
		Objective: "review reserved source", Status: AssignmentRunning,
	}
	assignment.ChildInteractionID = runner.WorkerInteractionID(asyncReq(assignment.ID))
	ac := &activeCoordination{
		id: CoordinationID("coord-recover"), parentInteractionID: parent.ID,
		parentGoalID: "goal-recovery", parentGoalRev: 1, status: CoordinationRunning,
		strategy: CoordinationSequential, completionPlan: CoordinationRequireAll,
		workspaceID: "workspace-a", permissionMode: "request_approval",
		assignments: []*AgentAssignment{assignment},
	}
	coordinator.coordinations["coord-recover"] = ac
	if err := coordinator.persistCoordinationSnapshot(ctx, ac); err != nil {
		t.Fatal(err)
	}
	if _, found, err := tracker.Get(ctx, assignment.ChildInteractionID); err != nil || found {
		t.Fatal("child should initially be only a durable dispatch intent")
	}
	if err := coordinator.Reconcile(ctx, "coord-recover"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-processor.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("reserved worker was never started from durable checkpoint")
	}
	rec, found, err := tracker.Get(ctx, assignment.ChildInteractionID)
	if err != nil || !found || rec.Scope.ConversationID == "parent-conversation" {
		t.Fatalf("recovered worker identity invalid: %+v %v", rec, err)
	}
}
