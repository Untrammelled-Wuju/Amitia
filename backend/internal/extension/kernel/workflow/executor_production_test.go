package workflow

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type countingHandler struct {
	mu    sync.Mutex
	calls int
}

type failingHandler struct{}

type blockingNonInterruptibleHandler struct {
	mu      sync.Mutex
	calls   int
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (failingHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	return nil, ErrHandlerNotFound
}

func (h *countingHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	h.mu.Lock()
	h.calls++
	h.mu.Unlock()
	return input, nil
}

func (h *blockingNonInterruptibleHandler) Execute(_ context.Context, _ WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	h.mu.Lock()
	h.calls++
	h.mu.Unlock()
	h.once.Do(func() { close(h.started) })
	<-h.release
	return input, nil
}

type memoryRunStore struct {
	mu    sync.Mutex
	runs  map[string]WorkflowRun
	steps map[string]StepRun
}

func newMemoryRunStore() *memoryRunStore {
	return &memoryRunStore{runs: map[string]WorkflowRun{}, steps: map[string]StepRun{}}
}

func (s *memoryRunStore) Start(ctx context.Context, run WorkflowRun) (*WorkflowRun, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.runs[run.ExecutionID]; ok {
		return &existing, false, nil
	}
	s.runs[run.ExecutionID] = run
	return &run, true, nil
}

func (s *memoryRunStore) SaveStep(ctx context.Context, step StepRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steps[step.ExecutionID+":"+step.NodeID] = step
	return nil
}

func (s *memoryRunStore) GetStep(ctx context.Context, executionID, nodeID string) (*StepRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	step, ok := s.steps[executionID+":"+nodeID]
	if !ok {
		return nil, nil
	}
	copy := step
	copy.Input = append(json.RawMessage(nil), step.Input...)
	copy.Output = append(json.RawMessage(nil), step.Output...)
	return &copy, nil
}

func (s *memoryRunStore) Finish(ctx context.Context, run WorkflowRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ExecutionID] = run
	return nil
}

func (s *memoryRunStore) Get(ctx context.Context, executionID string) (*WorkflowRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[executionID]
	if !ok {
		return nil, ErrWorkflowRunNotFound
	}
	return &run, nil
}

func (s *memoryRunStore) ListRecoverable(ctx context.Context, limit int) ([]WorkflowRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]WorkflowRun, 0)
	for _, run := range s.runs {
		if run.Status == RunStatusRunning || run.Status == RunStatusCompensating {
			result = append(result, run)
		}
	}
	return result, nil
}

func (s *memoryRunStore) UpdateStateCAS(ctx context.Context, run WorkflowRun, expectedStatus RunStatus) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.runs[run.ExecutionID]
	if !ok {
		return false, ErrWorkflowRunNotFound
	}
	if existing.Status != expectedStatus {
		return false, nil
	}
	s.runs[run.ExecutionID] = run
	return true, nil
}

func TestWorkflowExecutorPersistsAndDeduplicatesRun(t *testing.T) {
	registry := NewWorkflowRegistry()
	if err := registry.Register(WorkflowDefinition{ID: "wf", Name: "workflow", Enabled: true, Nodes: []WorkflowNode{{ID: "step", Type: "tool"}}}); err != nil {
		t.Fatal(err)
	}
	handler := &countingHandler{}
	store := newMemoryRunStore()
	executor := NewWorkflowExecutor(registry)
	executor.RegisterHandler("tool", handler)
	executor.SetRunStore(store)
	req := ExecuteRequest{WorkflowID: "wf", Input: json.RawMessage(`{"ok":true}`), Context: ExecutionContext{InvocationID: "run-1", IdempotencyKey: "trigger-1"}}
	first, err := executor.Execute(context.Background(), req)
	if err != nil || !first.Success {
		t.Fatalf("first execution failed: result=%+v err=%v", first, err)
	}
	second, err := executor.Execute(context.Background(), req)
	if err != nil || !second.Success {
		t.Fatalf("duplicate execution failed: result=%+v err=%v", second, err)
	}
	handler.mu.Lock()
	calls := handler.calls
	handler.mu.Unlock()
	if calls != 1 {
		t.Fatalf("expected one handler call, got %d", calls)
	}
	run, err := store.Get(context.Background(), "run-1")
	if err != nil || run.Status != RunStatusSucceeded || len(run.Steps) != 1 {
		t.Fatalf("unexpected persisted run: %+v err=%v", run, err)
	}
	if _, ok := store.steps["run-1:step"]; !ok {
		t.Fatal("step run was not persisted")
	}
}

func TestWorkflowExecutorBindsRevisionForEveryNewRun(t *testing.T) {
	registry := NewWorkflowRegistry()
	if err := registry.Register(WorkflowDefinition{
		ID: "wf-revision-bound", Name: "revision-bound", Enabled: true,
		Metadata: map[string]any{"ownerUserId": "user-a"},
		Nodes:    []WorkflowNode{{ID: "step", Type: "tool"}},
	}); err != nil {
		t.Fatal(err)
	}
	store := newMemoryRunStore()
	executor := NewWorkflowExecutor(registry)
	executor.RegisterHandler("tool", &countingHandler{})
	executor.SetRunStore(store)
	bindCalls := 0
	executor.SetRevisionBinder(func(_ context.Context, userID string, def WorkflowDefinition) (string, error) {
		bindCalls++
		if userID != "user-a" || def.ID != "wf-revision-bound" {
			t.Fatalf("unexpected revision bind request: user=%q workflow=%q", userID, def.ID)
		}
		return "wfrev-12", nil
	})

	result, err := executor.Execute(context.Background(), ExecuteRequest{
		WorkflowID: "wf-revision-bound",
		Context:    ExecutionContext{InvocationID: "revision-bound-run"},
	})
	if err != nil || result == nil || !result.Success {
		t.Fatalf("execution failed: result=%+v err=%v", result, err)
	}
	if bindCalls != 1 {
		t.Fatalf("expected exactly one revision bind, got %d", bindCalls)
	}
	run, err := store.Get(context.Background(), "revision-bound-run")
	if err != nil {
		t.Fatal(err)
	}
	if run.Context.RevisionID != "wfrev-12" {
		t.Fatalf("revision id not persisted on run: %+v", run.Context)
	}
	if run.Context.DefinitionHash == "" || len(run.Context.DefinitionSnapshot) == 0 {
		t.Fatalf("immutable definition evidence missing: %+v", run.Context)
	}
}

func TestWorkflowSecurityGuardFailsClosed(t *testing.T) {
	guard := &SecurityGuard{}
	err := guard.Check(context.Background(), WorkflowDefinition{ExtensionID: "ext", Permissions: []string{"network"}}, WorkflowNode{}, ExecutionContext{ExtensionID: "ext"})
	if err == nil {
		t.Fatal("expected permission failure")
	}
	err = guard.Check(context.Background(), WorkflowDefinition{ExtensionID: "ext", Scope: "character"}, WorkflowNode{}, ExecutionContext{ExtensionID: "ext"})
	if err == nil {
		t.Fatal("expected scope failure")
	}
	err = guard.Check(context.Background(), WorkflowDefinition{ExtensionID: "ext"}, WorkflowNode{}, ExecutionContext{ExtensionID: "ext", Generation: 2})
	if err == nil {
		t.Fatal("expected generation failure")
	}
}

func TestWorkflowDefinitionHashCoversRuntimePermissionAndScope(t *testing.T) {
	base := WorkflowDefinition{ID: "wf", Name: "workflow", Permissions: []string{"read"}, Scope: "character", Nodes: []WorkflowNode{{ID: "one", Type: "wasm", TargetID: "module-a"}}}
	first := ComputeDefinitionHash(base)
	base.Nodes[0].TargetID = "module-b"
	second := ComputeDefinitionHash(base)
	base.Nodes[0].TargetID = "module-a"
	base.Permissions = []string{"write"}
	third := ComputeDefinitionHash(base)
	base.Permissions = []string{"read"}
	base.Scope = "conversation"
	fourth := ComputeDefinitionHash(base)
	if first == second || first == third || first == fourth {
		t.Fatal("definition hash did not cover runtime, permission, and scope")
	}
}

func TestWaitHandlerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (WaitHandler{}).Execute(ctx, WorkflowNode{}, json.RawMessage(`{"durationMs":1000}`))
	if err == nil {
		t.Fatal("expected cancellation")
	}
}

func waitForRunStatus(t *testing.T, store *memoryRunStore, executionID string, status RunStatus, timeout time.Duration) *WorkflowRun {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		run, err := store.Get(context.Background(), executionID)
		if err == nil && run != nil && run.Status == status {
			return run
		}
		time.Sleep(5 * time.Millisecond)
	}
	run, _ := store.Get(context.Background(), executionID)
	t.Fatalf("run %s did not reach status %s before timeout; last=%+v", executionID, status, run)
	return nil
}

func TestWorkflowDurablePausePersistsWaitRemainingAndResumesAfterRestart(t *testing.T) {
	registry := NewWorkflowRegistry()
	if err := registry.Register(WorkflowDefinition{
		ID: "pause-wait", Name: "pause wait", Enabled: true,
		Nodes: []WorkflowNode{{ID: "wait", Type: "wait", Runtime: structRuntimeMetadata(500)}},
	}); err != nil {
		t.Fatal(err)
	}
	store := newMemoryRunStore()
	executor := NewWorkflowExecutor(registry)
	executor.RegisterHandler("wait", WaitHandler{})
	executor.SetRunStore(store)

	done := make(chan *ExecuteResult, 1)
	go func() {
		result, _ := executor.Execute(context.Background(), ExecuteRequest{
			WorkflowID: "pause-wait",
			Context:    ExecutionContext{InvocationID: "pause-wait-run"},
		})
		done <- result
	}()

	waitForRunStatus(t, store, "pause-wait-run", RunStatusRunning, time.Second)
	time.Sleep(40 * time.Millisecond)
	if _, err := executor.Pause(context.Background(), "pause-wait-run", "test"); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		if result == nil || result.Status != RunStatusPaused || !result.Accepted {
			t.Fatalf("unexpected paused execution result: %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("pausing wait workflow timed out")
	}
	waitForRunStatus(t, store, "pause-wait-run", RunStatusPaused, time.Second)

	step, err := store.GetStep(context.Background(), "pause-wait-run", "wait")
	if err != nil || step == nil || step.Status != "paused" {
		t.Fatalf("paused wait progress not persisted: step=%+v err=%v", step, err)
	}
	var progress waitPauseProgress
	if err := json.Unmarshal(step.Output, &progress); err != nil {
		t.Fatalf("decode paused wait progress: %v", err)
	}
	if progress.RemainingMS <= 0 || progress.RemainingMS >= 500 {
		t.Fatalf("unexpected remaining wait duration: %dms", progress.RemainingMS)
	}

	// Simulate a backend restart: use a fresh executor with the same durable
	// run store. Resume must reconstruct the wait from remainingMs rather than
	// starting the original 500ms duration again.
	restarted := NewWorkflowExecutor(registry)
	restarted.RegisterHandler("wait", WaitHandler{})
	restarted.SetRunStore(store)
	resumeStarted := time.Now()
	if _, err := restarted.Resume(context.Background(), "pause-wait-run"); err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, store, "pause-wait-run", RunStatusSucceeded, 2*time.Second)
	if elapsed := time.Since(resumeStarted); elapsed > 480*time.Millisecond {
		t.Fatalf("resume restarted the full wait instead of remaining duration: %s", elapsed)
	}
}

func TestWorkflowPausePersistsCompletedParallelSiblingBeforeStopping(t *testing.T) {
	registry := NewWorkflowRegistry()
	if err := registry.Register(WorkflowDefinition{
		ID: "pause-parallel", Name: "pause parallel", Enabled: true,
		Nodes: []WorkflowNode{
			{ID: "wait", Type: "wait", Runtime: structRuntimeMetadata(1000)},
			{ID: "side", Type: "side"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	store := newMemoryRunStore()
	checkpoints := NewMemoryCheckpointStore()
	blocking := &blockingNonInterruptibleHandler{started: make(chan struct{}), release: make(chan struct{})}
	executor := NewWorkflowExecutor(registry)
	executor.RegisterHandler("wait", WaitHandler{})
	executor.RegisterHandler("side", blocking)
	executor.SetRunStore(store)
	executor.SetCheckpointStore(checkpoints)

	done := make(chan *ExecuteResult, 1)
	go func() {
		result, _ := executor.Execute(context.Background(), ExecuteRequest{
			WorkflowID: "pause-parallel",
			Context:    ExecutionContext{InvocationID: "pause-parallel-run"},
		})
		done <- result
	}()

	select {
	case <-blocking.started:
	case <-time.After(time.Second):
		t.Fatal("parallel sibling did not start")
	}
	if _, err := executor.Pause(context.Background(), "pause-parallel-run", "test"); err != nil {
		t.Fatal(err)
	}
	// Give WaitHandler a deterministic opportunity to publish its paused result
	// before the non-interruptible sibling completes.
	time.Sleep(20 * time.Millisecond)
	close(blocking.release)

	select {
	case result := <-done:
		if result == nil || result.Status != RunStatusPaused {
			t.Fatalf("expected paused result, got %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("parallel pause did not settle")
	}

	if cp, err := checkpoints.Load(context.Background(), "pause-parallel-run", "side"); err != nil || cp == nil {
		t.Fatalf("completed sibling checkpoint was lost at pause boundary: cp=%+v err=%v", cp, err)
	}
	if step, _ := store.GetStep(context.Background(), "pause-parallel-run", "side"); step == nil || step.Status != "succeeded" {
		t.Fatalf("completed sibling step was not durably recorded: %+v", step)
	}

	if _, err := executor.Resume(context.Background(), "pause-parallel-run"); err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, store, "pause-parallel-run", RunStatusSucceeded, 2*time.Second)
	blocking.mu.Lock()
	calls := blocking.calls
	blocking.mu.Unlock()
	if calls != 1 {
		t.Fatalf("completed sibling replayed after resume: calls=%d", calls)
	}
}

func TestWorkflowPauseAfterNonInterruptibleFinalLevelDoesNotReportSuccess(t *testing.T) {
	registry := NewWorkflowRegistry()
	if err := registry.Register(WorkflowDefinition{
		ID: "pause-final", Name: "pause final", Enabled: true,
		Nodes: []WorkflowNode{{ID: "side", Type: "side"}},
	}); err != nil {
		t.Fatal(err)
	}
	store := newMemoryRunStore()
	checkpoints := NewMemoryCheckpointStore()
	blocking := &blockingNonInterruptibleHandler{started: make(chan struct{}), release: make(chan struct{})}
	executor := NewWorkflowExecutor(registry)
	executor.RegisterHandler("side", blocking)
	executor.SetRunStore(store)
	executor.SetCheckpointStore(checkpoints)

	done := make(chan *ExecuteResult, 1)
	go func() {
		result, _ := executor.Execute(context.Background(), ExecuteRequest{
			WorkflowID: "pause-final",
			Context:    ExecutionContext{InvocationID: "pause-final-run"},
		})
		done <- result
	}()
	select {
	case <-blocking.started:
	case <-time.After(time.Second):
		t.Fatal("final node did not start")
	}
	if _, err := executor.Pause(context.Background(), "pause-final-run", "test"); err != nil {
		t.Fatal(err)
	}
	close(blocking.release)

	select {
	case result := <-done:
		if result == nil || result.Status != RunStatusPaused || result.Success {
			t.Fatalf("pause request was crossed by final level: %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("final-level pause did not settle")
	}

	if _, err := executor.Resume(context.Background(), "pause-final-run"); err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, store, "pause-final-run", RunStatusSucceeded, time.Second)
	blocking.mu.Lock()
	calls := blocking.calls
	blocking.mu.Unlock()
	if calls != 1 {
		t.Fatalf("final-level node replayed after resume: calls=%d", calls)
	}
}

func TestWorkflowExecutorCancel(t *testing.T) {
	registry := NewWorkflowRegistry()
	_ = registry.Register(WorkflowDefinition{ID: "wait-workflow", Name: "wait", Enabled: true, Nodes: []WorkflowNode{{ID: "wait", Type: "wait", Runtime: structRuntimeMetadata(5000)}}})
	executor := NewWorkflowExecutor(registry)
	executor.RegisterHandler("wait", WaitHandler{})
	done := make(chan *ExecuteResult, 1)
	go func() {
		result, _ := executor.Execute(context.Background(), ExecuteRequest{WorkflowID: "wait-workflow", Context: ExecutionContext{InvocationID: "cancel-run"}})
		done <- result
	}()
	deadline := time.Now().Add(time.Second)
	for !executor.Cancel("cancel-run") && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	select {
	case result := <-done:
		if result == nil || result.Success || result.Status != RunStatusCancelled {
			t.Fatalf("unexpected cancelled result: %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("workflow cancellation timed out")
	}
}

func TestNestedWorkflowDepthLimit(t *testing.T) {
	registry := NewWorkflowRegistry()
	_ = registry.Register(WorkflowDefinition{ID: "parent", Name: "parent", Enabled: true, Limits: WorkflowLimits{MaxSkillCallDepth: 1}, Nodes: []WorkflowNode{{ID: "nested", Type: "nested_workflow", TargetID: "child"}}})
	_ = registry.Register(WorkflowDefinition{ID: "child", Name: "child", Enabled: true, Limits: WorkflowLimits{MaxSkillCallDepth: 1}, Nodes: []WorkflowNode{{ID: "wait", Type: "wait", Runtime: structRuntimeMetadata(0)}}})
	executor := NewWorkflowExecutor(registry)
	executor.RegisterHandler("nested_workflow", NestedWorkflowHandler{Executor: executor})
	executor.RegisterHandler("wait", WaitHandler{})
	result, err := executor.Execute(context.Background(), ExecuteRequest{WorkflowID: "parent", Context: ExecutionContext{InvocationID: "nested-run", Depth: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatal("expected nested depth failure")
	}
}

func TestWorkflowCompensationRunsAndPersists(t *testing.T) {
	registry := NewWorkflowRegistry()
	_ = registry.Register(WorkflowDefinition{ID: "compensate", Name: "compensate", Enabled: true, Nodes: []WorkflowNode{{ID: "create", Type: "ok"}, {ID: "fail", Type: "fail", DependsOn: []string{"create"}}}})
	executor := NewWorkflowExecutor(registry)
	executor.RegisterHandler("ok", &countingHandler{})
	executor.RegisterHandler("fail", failingHandler{})
	compensated := false
	manager := NewCompensationManager()
	manager.Register("create", CompensationAction{NodeID: "create", Handler: func(ctx context.Context, output json.RawMessage) error {
		compensated = true
		return nil
	}})
	executor.SetCompensationManager(manager)
	store := newMemoryRunStore()
	executor.SetRunStore(store)
	result, err := executor.Execute(context.Background(), ExecuteRequest{WorkflowID: "compensate", Context: ExecutionContext{InvocationID: "compensate-run"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || !compensated || result.Status != RunStatusCompensated || len(result.CompensationResults) != 1 || result.CompensationResults[0].Status != "succeeded" {
		t.Fatalf("unexpected compensation result: %+v compensated=%v", result, compensated)
	}
	run, _ := store.Get(context.Background(), "compensate-run")
	if len(run.CompensationResults) != 1 {
		t.Fatalf("compensation state was not persisted: %+v", run)
	}
}

func TestWorkflowRecoveryUsesCheckpoint(t *testing.T) {
	registry := NewWorkflowRegistry()
	_ = registry.Register(WorkflowDefinition{ID: "recover", Name: "recover", Enabled: true, Nodes: []WorkflowNode{{ID: "step", Type: "count"}}})
	handler := &countingHandler{}
	store := newMemoryRunStore()
	store.runs["recover-run"] = WorkflowRun{ExecutionID: "recover-run", WorkflowID: "recover", Status: RunStatusRunning, Input: json.RawMessage(`{"input":true}`), Context: ExecutionContext{InvocationID: "recover-run"}, StartedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	checkpoints := NewMemoryCheckpointStore()
	_ = checkpoints.Save(context.Background(), Checkpoint{WorkflowID: "recover", ExecutionID: "recover-run", NodeID: "step", Output: json.RawMessage(`{"restored":true}`), CompletedAt: time.Now().UTC()})
	executor := NewWorkflowExecutor(registry)
	executor.RegisterHandler("count", handler)
	executor.SetRunStore(store)
	executor.SetCheckpointStore(checkpoints)
	if err := executor.Recover(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	handler.mu.Lock()
	calls := handler.calls
	handler.mu.Unlock()
	if calls != 0 {
		t.Fatalf("checkpointed step executed again: %d", calls)
	}
	run, _ := store.Get(context.Background(), "recover-run")
	if run.Status != RunStatusSucceeded {
		t.Fatalf("workflow recovery did not finish: %+v", run)
	}
}

func structRuntimeMetadata(duration int64) capability.RuntimeBinding {
	return capability.RuntimeBinding{Metadata: map[string]any{"durationMs": float64(duration)}}
}

func TestNestedWorkflowRejectsCrossUserTarget(t *testing.T) {
	registry := NewWorkflowRegistry()
	_ = registry.Register(WorkflowDefinition{
		ID: "parent-user-a", Name: "parent", Enabled: true, Source: "user",
		Metadata: map[string]any{"ownerUserId": "user-a"},
		Nodes:    []WorkflowNode{{ID: "nested", Type: "nested_workflow", TargetID: "child-user-b"}},
	})
	_ = registry.Register(WorkflowDefinition{
		ID: "child-user-b", Name: "child", Enabled: true, Source: "user",
		Metadata: map[string]any{"ownerUserId": "user-b"},
		Nodes:    []WorkflowNode{{ID: "wait", Type: "wait", Runtime: structRuntimeMetadata(0)}},
	})
	executor := NewWorkflowExecutor(registry)
	executor.RegisterHandler("nested_workflow", NestedWorkflowHandler{Executor: executor})
	executor.RegisterHandler("wait", WaitHandler{})
	result, err := executor.Execute(context.Background(), ExecuteRequest{
		WorkflowID: "parent-user-a",
		Context:    ExecutionContext{InvocationID: "cross-user-nested", UserID: "user-a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || !strings.Contains(result.Error, "owner mismatch") {
		t.Fatalf("expected cross-user nested workflow rejection, got %+v", result)
	}
}

func TestNestedWorkflowRejectsRecursiveCycle(t *testing.T) {
	registry := NewWorkflowRegistry()
	_ = registry.Register(WorkflowDefinition{ID: "cycle-a", Name: "a", Enabled: true, Nodes: []WorkflowNode{{ID: "to-b", Type: "nested_workflow", TargetID: "cycle-b"}}})
	_ = registry.Register(WorkflowDefinition{ID: "cycle-b", Name: "b", Enabled: true, Nodes: []WorkflowNode{{ID: "to-a", Type: "nested_workflow", TargetID: "cycle-a"}}})
	executor := NewWorkflowExecutor(registry)
	executor.RegisterHandler("nested_workflow", NestedWorkflowHandler{Executor: executor})
	result, err := executor.Execute(context.Background(), ExecuteRequest{WorkflowID: "cycle-a", Context: ExecutionContext{InvocationID: "nested-cycle"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || !strings.Contains(result.Error, "nested workflow cycle detected") {
		t.Fatalf("expected nested workflow cycle rejection, got %+v", result)
	}
}

func TestWorkflowRunStoresSavedDefinitionHashBeforeEdgeMaterialization(t *testing.T) {
	condition := json.RawMessage(`{"op":"eq","left":{"value":true},"right":{"value":true}}`)
	def := WorkflowDefinition{
		SchemaVersion: UserWorkflowSchemaVersion,
		ID:            "hash-edge-workflow", Name: "hash edge", Enabled: true,
		Nodes: []WorkflowNode{{ID: "first", Type: "tool"}, {ID: "second", Type: "tool"}},
		Edges: []WorkflowEdge{{ID: "edge", Source: "first", Target: "second", Condition: condition}},
	}
	registry := NewWorkflowRegistry()
	if err := registry.Register(def); err != nil {
		t.Fatal(err)
	}
	executor := NewWorkflowExecutor(registry)
	executor.RegisterHandler("tool", &countingHandler{})
	store := newMemoryRunStore()
	executor.SetRunStore(store)
	result, err := executor.Execute(context.Background(), ExecuteRequest{WorkflowID: def.ID, Context: ExecutionContext{InvocationID: "hash-edge-run"}})
	if err != nil || result == nil || !result.Success {
		t.Fatalf("execute: result=%+v err=%v", result, err)
	}
	run, err := store.Get(context.Background(), "hash-edge-run")
	if err != nil {
		t.Fatal(err)
	}
	if want := ComputeDefinitionHash(def); run.Context.DefinitionHash != want {
		t.Fatalf("definition hash = %q, want saved definition hash %q", run.Context.DefinitionHash, want)
	}
}

func TestWorkflowExecutorExecutionModesAreDurableAndFailClosed(t *testing.T) {
	registry := NewWorkflowRegistry()
	if err := registry.Register(WorkflowDefinition{
		ID: "execution-modes", Name: "execution modes", Enabled: true,
		Nodes: []WorkflowNode{{ID: "effect", Type: "tool", TargetID: "send_message"}},
	}); err != nil {
		t.Fatal(err)
	}
	handler := &countingHandler{}
	store := newMemoryRunStore()
	executor := NewWorkflowExecutor(registry)
	executor.RegisterHandler("tool", handler)
	executor.SetRunStore(store)

	t.Run("dry run never calls handler and is persisted", func(t *testing.T) {
		result, err := executor.Execute(context.Background(), ExecuteRequest{
			WorkflowID: "execution-modes",
			Context:    ExecutionContext{InvocationID: "mode-dry-run"},
			Options:    ExecutionOptions{Mode: ExecutionModeDryRun},
		})
		if err != nil || result == nil || !result.Success || result.ExecutionMode != ExecutionModeDryRun {
			t.Fatalf("dry run failed: result=%+v err=%v", result, err)
		}
		handler.mu.Lock()
		calls := handler.calls
		handler.mu.Unlock()
		if calls != 0 {
			t.Fatalf("dry run called real handler %d times", calls)
		}
		run, err := store.Get(context.Background(), "mode-dry-run")
		if err != nil || run.Status != RunStatusSucceeded || run.Context.ExecutionOptions.Mode != ExecutionModeDryRun {
			t.Fatalf("dry run not durably persisted: run=%+v err=%v", run, err)
		}
	})

	t.Run("mocked side effect without mock fails closed", func(t *testing.T) {
		result, err := executor.Execute(context.Background(), ExecuteRequest{
			WorkflowID: "execution-modes",
			Context:    ExecutionContext{InvocationID: "mode-mocked-blocked"},
			Options:    ExecutionOptions{Mode: ExecutionModeMocked},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result == nil || result.Success || !strings.Contains(result.Error, "explicit mock") {
			t.Fatalf("mocked side effect should fail closed, got %+v", result)
		}
		handler.mu.Lock()
		calls := handler.calls
		handler.mu.Unlock()
		if calls != 0 {
			t.Fatalf("blocked mocked run called real handler %d times", calls)
		}
	})

	t.Run("controlled live waits durably then resumes same run", func(t *testing.T) {
		result, err := executor.Execute(context.Background(), ExecuteRequest{
			WorkflowID: "execution-modes",
			Context:    ExecutionContext{InvocationID: "mode-controlled"},
			Options:    ExecutionOptions{Mode: ExecutionModeControlled},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result == nil || result.Status != RunStatusWaitingConfirmation || len(result.RequiredConfirmations) != 1 || result.RequiredConfirmations[0] != "effect" {
			t.Fatalf("unexpected controlled-live wait result: %+v", result)
		}
		run, err := store.Get(context.Background(), "mode-controlled")
		if err != nil || run.Status != RunStatusWaitingConfirmation {
			t.Fatalf("controlled run not durably waiting: run=%+v err=%v", run, err)
		}
		missing := MissingControlledApprovalsForRun(run)
		if len(missing) != 1 || missing[0] != "effect" {
			t.Fatalf("missing approvals = %v, want [effect]", missing)
		}
		confirmed, remaining, err := executor.ConfirmControlledRun(context.Background(), "mode-controlled", []string{"effect"})
		if err != nil || len(remaining) != 0 || confirmed == nil || confirmed.ExecutionID != "mode-controlled" {
			t.Fatalf("confirm failed: run=%+v remaining=%v err=%v", confirmed, remaining, err)
		}
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			current, getErr := store.Get(context.Background(), "mode-controlled")
			if getErr == nil && current.Status.IsTerminal() {
				if current.Status != RunStatusSucceeded {
					t.Fatalf("confirmed controlled run ended %s: %+v", current.Status, current)
				}
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		current, _ := store.Get(context.Background(), "mode-controlled")
		if current.Status != RunStatusSucceeded {
			t.Fatalf("controlled run did not finish: %+v", current)
		}
		handler.mu.Lock()
		calls := handler.calls
		handler.mu.Unlock()
		if calls != 1 {
			t.Fatalf("controlled handler calls = %d, want 1", calls)
		}
	})
}

func TestExecutionOptionsForRerunRequiresFreshControlledApproval(t *testing.T) {
	run := &WorkflowRun{Context: ExecutionContext{ExecutionOptions: ExecutionOptions{
		Mode: ExecutionModeControlled, ApprovedSideEffects: []string{"effect"},
		Mocks: []MockBehavior{{NodeID: "effect", Output: json.RawMessage(`{"ok":true}`)}},
	}}}
	opts := ExecutionOptionsForRerun(run)
	if opts.Mode != ExecutionModeControlled {
		t.Fatalf("mode = %q, want controlled_live", opts.Mode)
	}
	if len(opts.ApprovedSideEffects) != 0 {
		t.Fatalf("rerun inherited approvals: %v", opts.ApprovedSideEffects)
	}
	if len(opts.Mocks) != 1 {
		t.Fatalf("rerun should preserve mocks, got %v", opts.Mocks)
	}
}
