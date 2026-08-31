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

func (failingHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	return nil, ErrHandlerNotFound
}

func (h *countingHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	h.mu.Lock()
	h.calls++
	h.mu.Unlock()
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
