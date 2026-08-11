package interaction

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/decision"
)

type fakeClock struct {
	now time.Time
}

func (f fakeClock) Now() time.Time { return f.now }

type fakeWorkerRunner struct {
	started []WorkerRunRequest
}

func (r *fakeWorkerRunner) StartWorker(ctx context.Context, req WorkerRunRequest) (string, error) {
	r.started = append(r.started, req)
	return "child-" + req.AssignmentID, nil
}

type fakeTracker struct {
	records     map[string]*InteractionRecord
	rangeCalled int
}

func newFakeTracker() *fakeTracker {
	return &fakeTracker{records: make(map[string]*InteractionRecord)}
}

func (t *fakeTracker) Create(ctx context.Context, record *InteractionRecord) error {
	t.records[record.ID] = record
	return nil
}

func (t *fakeTracker) Get(ctx context.Context, id string) (*InteractionRecord, bool, error) {
	r, ok := t.records[id]
	return r, ok, nil
}

func (t *fakeTracker) GetByRequestID(ctx context.Context, userID string, requestID string) (*InteractionRecord, bool, error) {
	for _, r := range t.records {
		if r.Scope.UserID == userID && r.Scope.ConversationID != "" {
			return r, true, nil
		}
	}
	return nil, false, nil
}

func (t *fakeTracker) ListActive(ctx context.Context, scope InteractionScope) ([]*InteractionRecord, error) {
	result := make([]*InteractionRecord, 0)
	for _, r := range t.records {
		result = append(result, r)
	}
	return result, nil
}

func (t *fakeTracker) ListByScope(ctx context.Context, scope InteractionScope) ([]*InteractionRecord, error) {
	return t.ListActive(ctx, scope)
}

func (t *fakeTracker) UpdateMetadata(ctx context.Context, id string, update InteractionMetadataUpdate) (*InteractionRecord, error) {
	if r, ok := t.records[id]; ok {
		if update.RecoveryDescriptor != nil {
			r.RecoveryDescriptor = update.RecoveryDescriptor
		}
		r.UpdatedAt = time.Now().UTC()
		return r, nil
	}
	return nil, ErrInteractionNotFound
}

func (t *fakeTracker) TransitionCAS(ctx context.Context, id string, expectedVersion int64, target InteractionStatus) (*InteractionRecord, error) {
	r, ok := t.records[id]
	if !ok {
		return nil, ErrInteractionNotFound
	}
	r.Status = target
	r.StatusVersion++
	r.UpdatedAt = time.Now().UTC()
	return r, nil
}

func (t *fakeTracker) RequestCancel(ctx context.Context, id string, reason string) error {
	return nil
}

func (t *fakeTracker) MarkSuperseded(ctx context.Context, targetID string, supersededByID string) error {
	return nil
}

func (t *fakeTracker) Complete(ctx context.Context, id string, expectedVersion int64, resultRef string) (*InteractionRecord, error) {
	r, ok := t.records[id]
	if !ok {
		return nil, ErrInteractionNotFound
	}
	r.Status = InteractionStatusCompleted
	r.StatusVersion++
	r.ResultRef = resultRef
	r.CompletedAt = time.Now().UTC()
	r.UpdatedAt = time.Now().UTC()
	return r, nil
}

func (t *fakeTracker) Fail(ctx context.Context, id string, expectedVersion int64, code string, message string) (*InteractionRecord, error) {
	r, ok := t.records[id]
	if !ok {
		return nil, ErrInteractionNotFound
	}
	r.Status = InteractionStatusFailed
	r.StatusVersion++
	r.ErrorCode = code
	r.ErrorMessage = message
	r.UpdatedAt = time.Now().UTC()
	return r, nil
}

func (t *fakeTracker) Archive(ctx context.Context, id string, expectedVersion int64) error {
	r, ok := t.records[id]
	if !ok {
		return ErrInteractionNotFound
	}
	r.Status = InteractionStatusArchived
	r.StatusVersion++
	r.UpdatedAt = time.Now().UTC()
	return nil
}

func (t *fakeTracker) AcquireCommitToken(ctx context.Context, id string, expectedVersion int64) (*CommitToken, error) {
	return nil, nil
}

func (t *fakeTracker) Range(ctx context.Context, fn func(record *InteractionRecord) bool) error {
	t.rangeCalled++
	for _, r := range t.records {
		if !fn(r) {
			break
		}
	}
	return nil
}

func TestBuildAggregateObservationID_Deterministic(t *testing.T) {
	id1 := BuildAggregateObservationID("coord-1", 5)
	id2 := BuildAggregateObservationID("coord-1", 5)
	id3 := BuildAggregateObservationID("coord-2", 5)
	id4 := BuildAggregateObservationID("coord-1", 6)

	if id1 != id2 {
		t.Fatalf("expected deterministic id, got %s != %s", id1, id2)
	}
	if id1 == id3 {
		t.Fatalf("expected different id for different coordination, got %s", id1)
	}
	if id1 == id4 {
		t.Fatalf("expected different id for different revision, got %s", id1)
	}
	if len(id1) == 0 {
		t.Fatal("empty id")
	}
}

func TestBuildCoordinationTriggerID_Deterministic(t *testing.T) {
	id1 := BuildCoordinationTriggerID("coord-xyz")
	id2 := BuildCoordinationTriggerID("coord-xyz")
	id3 := BuildCoordinationTriggerID("coord-abc")

	if id1 != id2 {
		t.Fatalf("expected deterministic id, got %s != %s", id1, id2)
	}
	if id1 == id3 {
		t.Fatalf("expected different id for different coordination")
	}
}

func TestCoordinationStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		status   CoordinationStatus
		terminal bool
	}{
		{CoordinationSucceeded, true},
		{CoordinationFailed, true},
		{CoordinationCancelled, true},
		{CoordinationPlanning, false},
		{CoordinationRunning, false},
		{CoordinationWaiting, false},
		{CoordinationAggregating, false},
		{CoordinationPaused, false},
	}
	for _, tt := range tests {
		if got := tt.status.IsTerminal(); got != tt.terminal {
			t.Errorf("CoordinationStatus(%s).IsTerminal() = %v, want %v", tt.status, got, tt.terminal)
		}
	}
}

func TestAgentAssignmentStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		status   AgentAssignmentStatus
		terminal bool
	}{
		{AssignmentSucceeded, true},
		{AssignmentFailed, true},
		{AssignmentCancelled, true},
		{AssignmentPending, false},
		{AssignmentRunning, false},
		{AssignmentWaiting, false},
		{AssignmentPaused, false},
	}
	for _, tt := range tests {
		if got := tt.status.IsTerminal(); got != tt.terminal {
			t.Errorf("AgentAssignmentStatus(%s).IsTerminal() = %v, want %v", tt.status, got, tt.terminal)
		}
	}
}

func TestMultiAgentCoordinator_Start_Validation(t *testing.T) {
	c := NewMultiAgentCoordinator(nil, nil, nil, nil, nil, DefaultMultiAgentPolicy())
	ctx := context.Background()

	_, err := c.Start(ctx, StartCoordinationRequest{})
	if err == nil {
		t.Fatal("expected error for empty objectives")
	}

	objectives := make([]AssignmentObjective, 0)
	for i := 0; i < c.policy.MaxAssignments+1; i++ {
		objectives = append(objectives, AssignmentObjective{
			WorkerIndex: 0,
			Objective:   "task",
		})
	}
	_, err = c.Start(ctx, StartCoordinationRequest{Objectives: objectives})
	if err == nil {
		t.Fatal("expected error for exceeding MaxAssignments")
	}
}

func TestMultiAgentCoordinator_Start_DepthLimit(t *testing.T) {
	c := NewMultiAgentCoordinator(nil, nil, nil, nil, nil, DefaultMultiAgentPolicy())
	ctx := context.Background()

	_, err := c.Start(ctx, StartCoordinationRequest{
		Objectives: []AssignmentObjective{{WorkerIndex: 0, Objective: "x"}},
		Depth:      c.policy.MaxDepth + 1,
	})
	if err == nil {
		t.Fatal("expected error for exceeding MaxDepth")
	}
}

func TestMultiAgentCoordinator_Start_And_Aggregate(t *testing.T) {
	tracker := newFakeTracker()
	_ = tracker.Create(context.Background(), &InteractionRecord{ID: "parent-1", Status: InteractionStatusProcessing})
	goals := decision.NewGoalRegistry()

	goal := decision.Goal{
		ID:        "goal-1",
		UserID:    "user-1",
		Status:    decision.GoalStatusActive,
		Revision:  1,
		CreatedAt: time.Now().UTC(),
	}
	if err := goals.Register(goal); err != nil {
		t.Fatal(err)
	}

	workers := &fakeWorkerRunner{}
	policy := DefaultMultiAgentPolicy()
	clock := fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	c := NewMultiAgentCoordinator(tracker, goals, nil, workers, nil, policy)
	c.clock = clock

	ctx := context.Background()
	res, err := c.Start(ctx, StartCoordinationRequest{
		ParentInteractionID: "parent-1",
		ParentGoalID:        "goal-1",
		ParentGoalRevision:  1,
		WorkerRefs: []AgentWorkerRef{
			{WorkerID: "w1", CharacterID: "c1", Role: "worker"},
			{WorkerID: "w2", CharacterID: "c2", Role: "worker"},
		},
		Objectives: []AssignmentObjective{
			{WorkerIndex: 0, Objective: "analyze data", ExpectedOutcome: "analysis complete"},
			{WorkerIndex: 1, Objective: "summarize findings", ExpectedOutcome: "summary ready", Dependencies: []string{}},
		},
		Strategy:       CoordinationParallel,
		CompletionPlan: CoordinationRequireAll,
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if res.CoordinationID == "" {
		t.Fatal("empty coordination id")
	}
	if len(res.AssignmentIDs) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(res.AssignmentIDs))
	}

	c.mu.Lock()
	ac := c.coordinations[string(res.CoordinationID)]
	c.mu.Unlock()
	if ac == nil {
		t.Fatal("coordination not stored")
	}
	if ac.status != CoordinationRunning {
		t.Fatalf("expected running, got %s", ac.status)
	}
	if len(workers.started) != 2 {
		t.Fatalf("expected 2 workers started, got %d", len(workers.started))
	}
}

func TestMultiAgentCoordinator_OnAssignmentTerminal_RequireAll(t *testing.T) {
	c := newTestCoordinator()

	workers := &fakeWorkerRunner{}
	c.starter = workers

	res, err := c.Start(context.Background(), StartCoordinationRequest{
		ParentGoalID:       "goal-1",
		ParentGoalRevision: 1,
		WorkerRefs:         []AgentWorkerRef{{WorkerID: "w1", CharacterID: "c1"}},
		Objectives: []AssignmentObjective{
			{WorkerIndex: 0, Objective: "a"},
			{WorkerIndex: 0, Objective: "b"},
		},
		CompletionPlan: CoordinationRequireAll,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	c.mu.Lock()
	ac := c.coordinations[string(res.CoordinationID)]
	c.mu.Unlock()

	id1 := ac.assignments[0].ID
	id2 := ac.assignments[1].ID

	_ = c.OnAssignmentTerminal(context.Background(), id1, AgentAssignmentResult{
		AssignmentID: id1,
		Status:       AssignmentSucceeded,
	})
	if ac.status == CoordinationAggregating {
		t.Fatal("should not aggregate yet - second assignment still pending")
	}

	_ = c.OnAssignmentTerminal(context.Background(), id2, AgentAssignmentResult{
		AssignmentID: id2,
		Status:       AssignmentSucceeded,
	})
	if ac.status != CoordinationSucceeded {
		t.Fatalf("expected succeeded, got %s", ac.status)
	}
}

func TestMultiAgentCoordinator_OnAssignmentTerminal_FirstSuccess(t *testing.T) {
	c := newTestCoordinator()

	res, err := c.Start(context.Background(), StartCoordinationRequest{
		ParentGoalID:       "goal-1",
		ParentGoalRevision: 1,
		WorkerRefs:         []AgentWorkerRef{{WorkerID: "w1", CharacterID: "c1"}},
		Objectives: []AssignmentObjective{
			{WorkerIndex: 0, Objective: "a"},
			{WorkerIndex: 0, Objective: "b"},
			{WorkerIndex: 0, Objective: "c"},
		},
		CompletionPlan: CoordinationFirstSuccess,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	c.mu.Lock()
	ac := c.coordinations[string(res.CoordinationID)]
	c.mu.Unlock()

	id1 := ac.assignments[0].ID

	_ = c.OnAssignmentTerminal(context.Background(), id1, AgentAssignmentResult{
		AssignmentID: id1,
		Status:       AssignmentSucceeded,
	})

	if ac.status != CoordinationSucceeded {
		t.Fatalf("expected succeeded after first success policy, got %s", ac.status)
	}
}

func TestMultiAgentCoordinator_Cancel(t *testing.T) {
	c := newTestCoordinator()

	res, err := c.Start(context.Background(), StartCoordinationRequest{
		ParentGoalID:       "goal-1",
		ParentGoalRevision: 1,
		WorkerRefs:         []AgentWorkerRef{{WorkerID: "w1", CharacterID: "c1"}},
		Objectives:         []AssignmentObjective{{WorkerIndex: 0, Objective: "a"}},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	err = c.Cancel(context.Background(), string(res.CoordinationID))
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	c.mu.Lock()
	ac := c.coordinations[string(res.CoordinationID)]
	c.mu.Unlock()
	if ac.status != CoordinationCancelled {
		t.Fatalf("expected cancelled, got %s", ac.status)
	}
	if ac.assignments[0].Status != AssignmentCancelled {
		t.Fatalf("expected assignment cancelled, got %s", ac.assignments[0].Status)
	}
}

func TestMultiAgentCoordinator_Cancel_Idempotent(t *testing.T) {
	c := newTestCoordinator()

	res, err := c.Start(context.Background(), StartCoordinationRequest{
		ParentGoalID:       "goal-1",
		ParentGoalRevision: 1,
		WorkerRefs:         []AgentWorkerRef{{WorkerID: "w1", CharacterID: "c1"}},
		Objectives:         []AssignmentObjective{{WorkerIndex: 0, Objective: "a"}},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	_ = c.Cancel(context.Background(), string(res.CoordinationID))
	err = c.Cancel(context.Background(), string(res.CoordinationID))
	if err != nil {
		t.Fatalf("second cancel should succeed: %v", err)
	}
}

func TestMultiAgentCoordinator_Resume_StaleRevision(t *testing.T) {
	goals := decision.NewGoalRegistry()
	_ = goals.Register(decision.Goal{ID: "goal-1", Status: decision.GoalStatusActive, Revision: 1})
	clock := fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	c := NewMultiAgentCoordinator(nil, goals, nil, &fakeWorkerRunner{}, nil, DefaultMultiAgentPolicy())
	c.clock = clock

	res, err := c.Start(context.Background(), StartCoordinationRequest{
		ParentGoalID:       "goal-1",
		ParentGoalRevision: 1,
		WorkerRefs:         []AgentWorkerRef{{WorkerID: "w1", CharacterID: "c1"}},
		Objectives:         []AssignmentObjective{{WorkerIndex: 0, Objective: "a"}},
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	c.mu.Lock()
	ac := c.coordinations[string(res.CoordinationID)]
	c.mu.Unlock()

	ac.status = CoordinationPaused

	_ = goals.UpdateStatus("goal-1", decision.GoalStatusActive, 0.9)
	_ = goals.UpdateStatusAt("goal-1", decision.GoalStatusActive, 0.9, time.Now().UTC())

	err = c.Resume(context.Background(), string(ac.id))
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if ac.status != CoordinationCancelled {
		t.Fatalf("expected cancelled after stale revision, got %s", ac.status)
	}
}

func TestMultiAgentCoordinator_DetectConflicts_DuplicateGoal(t *testing.T) {
	ac := &activeCoordination{
		id:       "coord-1",
		status:   CoordinationRunning,
		strategy: CoordinationParallel,
		assignments: []*AgentAssignment{
			{
				ID:           "a1",
				Status:       AssignmentSucceeded,
				WorkerRef:    AgentWorkerRef{WorkerID: "w1"},
				ChildGoalID:  "child-goal-x",
				Objective:    "obj",
			},
			{
				ID:           "a2",
				Status:       AssignmentSucceeded,
				WorkerRef:    AgentWorkerRef{WorkerID: "w1"},
				ChildGoalID:  "child-goal-x",
				Objective:    "different",
			},
		},
		completionPlan: CoordinationRequireAll,
	}

	c := NewMultiAgentCoordinator(nil, nil, nil, nil, nil, DefaultMultiAgentPolicy())
	conflicts := c.DetectConflicts(ac)

	found := false
	for _, cf := range conflicts {
		if cf.Kind == ConflictDuplicateAssignment {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected duplicate assignment conflict, got %v", conflicts)
	}
}

func TestMultiAgentCoordinator_DetectConflicts_Nil(t *testing.T) {
	c := NewMultiAgentCoordinator(nil, nil, nil, nil, nil, DefaultMultiAgentPolicy())
	conflicts := c.DetectConflicts(nil)
	if conflicts != nil {
		t.Fatalf("expected nil for nil coordination, got %v", conflicts)
	}
}

func TestMultiAgentCoordinator_Reconcile_UnknownID(t *testing.T) {
	c := NewMultiAgentCoordinator(nil, nil, nil, nil, nil, DefaultMultiAgentPolicy())
	err := c.Reconcile(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("reconcile unknown should be no-op: %v", err)
	}
}

func TestObjectiveFingerprint_Stability(t *testing.T) {
	a := objectiveFingerprint("hello world")
	b := objectiveFingerprint("hello world")
	c := objectiveFingerprint("different")
	if a != b {
		t.Fatal("should be stable")
	}
	if a == c {
		t.Fatal("different objectives should have different fingerprints")
	}
}

func TestTruncateObjective(t *testing.T) {
	short := "short"
	if got := truncateObjective(short); got != short {
		t.Fatalf("short objective should not be truncated")
	}
	long := make([]byte, 10000)
	for i := range long {
		long[i] = 'a'
	}
	got := truncateObjective(string(long))
	if len(got) != 8192 {
		t.Fatalf("expected truncation to 8192, got %d", len(got))
	}
}

func TestDefaultMultiAgentPolicy(t *testing.T) {
	p := DefaultMultiAgentPolicy()
	if p.MaxWorkers <= 0 {
		t.Fatal("MaxWorkers should be positive")
	}
	if p.MaxAssignments <= 0 {
		t.Fatal("MaxAssignments should be positive")
	}
	if p.MaxDepth <= 0 {
		t.Fatal("MaxDepth should be positive")
	}
}

func newTestCoordinator() *MultiAgentCoordinator {
	tracker := newFakeTracker()
	goals := decision.NewGoalRegistry()
	_ = goals.Register(decision.Goal{ID: "goal-1", Status: decision.GoalStatusActive, Revision: 1})
	clock := fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	c := NewMultiAgentCoordinator(tracker, goals, nil, &fakeWorkerRunner{}, nil, DefaultMultiAgentPolicy())
	c.clock = clock
	return c
}
