package main

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/interaction"
)

type harnessTestWorker struct {
	tracker           *interaction.InMemoryTracker
	started           []interaction.WorkerRunRequest
	finishImmediately bool
}

func (w *harnessTestWorker) StartWorker(ctx context.Context, req interaction.WorkerRunRequest) (string, error) {
	w.started = append(w.started, req)
	scope := interaction.InteractionScope{
		SpaceID: req.SpaceID, CharacterID: req.CharacterID, ConversationID: req.ConversationID,
		RequestID: "ma-" + req.AssignmentID, Source: "multi_agent", Channel: "web",
	}
	rec := interaction.NewInteractionRecord(scope)
	rec.Status = interaction.InteractionStatusProcessing
	if w.finishImmediately {
		rec.Status = interaction.InteractionStatusCompleted
		rec.ResultRef = "completed: " + req.Objective
	}
	if err := w.tracker.Create(ctx, rec); err != nil {
		return "", err
	}
	return rec.ID, nil
}

func testMultiAgentModelRuntime(t *testing.T) (*chatToolRuntimeAdapter, *harnessTestWorker, chat.SkillScope, *interaction.InMemoryTracker) {
	t.Helper()
	ctx := context.Background()
	tracker := interaction.NewInMemoryTracker()
	parent := interaction.NewInteractionRecord(interaction.InteractionScope{
		SpaceID: "user-space", ConversationID: "user-conversation", CharacterID: "character",
		RequestID: "request-a", Source: "web", Channel: "web",
	})
	parent.Status = interaction.InteractionStatusProcessing
	if err := tracker.Create(ctx, parent); err != nil {
		t.Fatal(err)
	}
	workers := &harnessTestWorker{tracker: tracker}
	goals := decision.NewGoalRegistry()
	coordinator := interaction.NewMultiAgentCoordinator(tracker, goals, nil, workers, nil, interaction.DefaultMultiAgentPolicy())
	adapter := &chatToolRuntimeAdapter{}
	adapter.ConfigureMultiAgent(coordinator, tracker, goals)
	scope := chat.SkillScope{
		SpaceID: "user-space", ConversationID: "user-conversation", CharacterID: "character",
		RequestID:      "request-a",
		PermissionMode: "request_approval",
		ExecContext: &execution.ExecutionContext{WorkspaceID: "workspace-a", Metadata: map[string]any{
			"workspaceDeviceId": "device-a", "workspaceName": "coding", "workspaceRootUri": "file:///source",
		}},
	}
	return adapter, workers, scope, tracker
}

func TestChatHarnessMultiAgentToolsHonorWorkspaceAuthorization(t *testing.T) {
	a, _, scope, _ := testMultiAgentModelRuntime(t)
	if len(a.appendMultiAgentTools(context.Background(), nil, scope)) != 3 {
		t.Fatal("delegation and status tools were not exposed")
	}
	scope.ExecContext.WorkspaceID = ""
	if len(a.appendMultiAgentTools(context.Background(), nil, scope)) != 0 {
		t.Fatal("delegation exposed outside workspace")
	}
	scope.ExecContext.WorkspaceID = "workspace-a"
	scope.IsInternal = true
	if len(a.appendMultiAgentTools(context.Background(), nil, scope)) != 0 {
		t.Fatal("delegation exposed to internal worker")
	}
	got := a.executeMultiAgentTool(context.Background(), multiAgentDelegateTool, json.RawMessage(`{"objectives":["Inspect the api endpoints","Add durable worker tests"]}`), scope)
	if got.Error == nil || got.Error.Code != "MULTI_AGENT_NOT_AUTHORIZED" {
		t.Fatalf("unauthorized delegation accepted: %+v", got)
	}
}

func TestChatHarnessMultiAgentModelDelegatesAndReconciles(t *testing.T) {
	ctx := context.Background()
	a, workers, scope, tracker := testMultiAgentModelRuntime(t)
	input := json.RawMessage(`{"objectives":["Review the file runtime","Test restart recovery"],"strategy":"sequential"}`)
	result := a.executeMultiAgentTool(ctx, multiAgentDelegateTool, input, scope)
	if result.Error != nil || result.Status != "SUCCESS" {
		t.Fatalf("cannot delegate: %+v", result)
	}
	var payload struct {
		CoordinationID string `json:"coordinationId"`
		Status         string `json:"status"`
	}
	if err := json.Unmarshal(result.Output, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.CoordinationID == "" || payload.Status != "running" || len(workers.started) != 1 {
		t.Fatalf("delegation must be running after first worker: %+v starts=%d", payload, len(workers.started))
	}
	if workers.started[0].SpaceID != "user-space" || workers.started[0].ConversationID != "user-conversation" || workers.started[0].WorkspaceID != "workspace-a" || workers.started[0].WorkspaceDeviceID != "device-a" || workers.started[0].PermissionMode != "request_approval" {
		t.Fatalf("child worker lost parent identity: %+v", workers.started[0])
	}
	again := a.executeMultiAgentTool(ctx, multiAgentDelegateTool, input, scope)
	if again.Error != nil || len(workers.started) != 1 {
		t.Fatalf("duplicate delegation spawned more workers: %+v count=%d", again, len(workers.started))
	}
	lookup, _ := json.Marshal(map[string]string{"coordination_id": payload.CoordinationID})
	status := a.executeMultiAgentTool(ctx, multiAgentStatusTool, lookup, scope)
	if status.Error != nil {
		t.Fatalf("status tool failed: %+v", status)
	}
	var snapshot interaction.CoordinationSnapshot
	if err := json.Unmarshal(status.Output, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Assignments) != 2 || snapshot.Assignments[0].Status != interaction.AssignmentRunning {
		t.Fatalf("worker state not grounded in tracker: %+v", snapshot)
	}
	first, found, err := tracker.Get(ctx, snapshot.Assignments[0].ChildInteractionID)
	if err != nil || !found {
		t.Fatalf("missing first child: %v", err)
	}
	if _, err := tracker.Complete(ctx, first.ID, first.StatusVersion, "verified:first"); err != nil {
		t.Fatal(err)
	}
	status = a.executeMultiAgentTool(ctx, multiAgentStatusTool, lookup, scope)
	if status.Error != nil {
		t.Fatal(status.Error)
	}
	if len(workers.started) != 2 {
		t.Fatalf("sequential worker was not advanced: %d", len(workers.started))
	}
	if err := json.Unmarshal(status.Output, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Assignments[0].ResultRef != "verified:first" || snapshot.Assignments[1].Status != interaction.AssignmentRunning {
		t.Fatalf("child evidence and sequential progress missing: %+v", snapshot)
	}
	second, found, err := tracker.Get(ctx, snapshot.Assignments[1].ChildInteractionID)
	if err != nil || !found {
		t.Fatalf("missing second worker: %v", err)
	}
	if _, err := tracker.Complete(ctx, second.ID, second.StatusVersion, "verified:second"); err != nil {
		t.Fatal(err)
	}
	status = a.executeMultiAgentTool(ctx, multiAgentStatusTool, lookup, scope)
	if status.Error != nil {
		t.Fatal(status.Error)
	}
	if err := json.Unmarshal(status.Output, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != interaction.CoordinationSucceeded || snapshot.Assignments[1].ResultRef != "verified:second" {
		t.Fatalf("parent did not receive durable child outcomes: %+v", snapshot)
	}
}

func TestChatHarnessMultiAgentRejectsCrossConversationResultRead(t *testing.T) {
	ctx := context.Background()
	a, _, scope, _ := testMultiAgentModelRuntime(t)
	result := a.executeMultiAgentTool(ctx, multiAgentDelegateTool, json.RawMessage(`{"objectives":["Review long task","Test interrupted tool"]}`), scope)
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	var response struct {
		CoordinationID string `json:"coordinationId"`
	}
	_ = json.Unmarshal(result.Output, &response)
	input, _ := json.Marshal(map[string]string{"coordination_id": response.CoordinationID})
	scope.ConversationID = "attacker-conversation"
	got := a.executeMultiAgentTool(ctx, multiAgentStatusTool, input, scope)
	if got.Error == nil || !strings.Contains(got.Error.Code, "NOT_AUTHORIZED") {
		t.Fatalf("cross-conversation status was exposed: %+v", got)
	}
}

func TestChatHarnessMultiAgentConcurrencyDoesNotDuplicateWorkers(t *testing.T) {
	a, workers, scope, _ := testMultiAgentModelRuntime(t)
	data := json.RawMessage(`{"objectives":["Inspect source api","Write durable tests"],"strategy":"sequential"}`)
	var wg sync.WaitGroup
	var failed atomic.Bool
	ids := make(chan string, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := a.executeMultiAgentTool(context.Background(), multiAgentDelegateTool, data, scope)
			if res.Error != nil {
				failed.Store(true)
				return
			}
			var payload struct {
				CoordinationID string `json:"coordinationId"`
			}
			if json.Unmarshal(res.Output, &payload) != nil {
				failed.Store(true)
				return
			}
			ids <- payload.CoordinationID
		}()
	}
	wg.Wait()
	close(ids)
	var first string
	for id := range ids {
		if first == "" {
			first = id
		}
		if id != first {
			t.Fatalf("concurrent dispatch created different coordinations: %q and %q", first, id)
		}
	}
	if failed.Load() || first == "" || len(workers.started) != 1 {
		t.Fatalf("concurrent delegation duplicated child executions: errors=%v count=%d", failed.Load(), len(workers.started))
	}
}

func TestChatHarnessMultiAgentRestoresGoalOnRestart(t *testing.T) {
	a, workers, scope, tracker := testMultiAgentModelRuntime(t)
	res := a.executeMultiAgentTool(context.Background(), multiAgentDelegateTool,
		json.RawMessage(`{"objectives":["Inspect first module","Inspect second module"],"strategy":"sequential"}`), scope)
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	var payload struct {
		CoordinationID string `json:"coordinationId"`
	}
	if err := json.Unmarshal(res.Output, &payload); err != nil {
		t.Fatal(err)
	}
	newGoals := decision.NewGoalRegistry()
	next := interaction.NewMultiAgentCoordinator(tracker, newGoals, nil, workers, nil, interaction.DefaultMultiAgentPolicy())
	if err := next.ReconcilePendingCoordinations(context.Background()); err != nil {
		t.Fatal(err)
	}
	snapshot, err := next.Snapshot(context.Background(), payload.CoordinationID)
	if err != nil || len(snapshot.Assignments) != 2 || len(workers.started) != 1 {
		t.Fatalf("restart lost parent identity or repeated execution: %+v err=%v started=%d", snapshot, err, len(workers.started))
	}
}

func TestChatHarnessMultiAgentSynchronousWorkersReturnTerminalResults(t *testing.T) {
	adapter, workers, scope, tracker := testMultiAgentModelRuntime(t)
	workers.finishImmediately = true
	res := adapter.executeMultiAgentTool(context.Background(), multiAgentDelegateTool,
		json.RawMessage(`{"objectives":["Inspect the existing source", "Validate the completed implementation"],"strategy":"sequential"}`), scope)
	if res.Error != nil {
		t.Fatalf("coordinator returned tool failure: %+v", res.Error)
	}
	var response struct {
		CoordinationID string                                       `json:"coordinationId"`
		Status         interaction.CoordinationStatus               `json:"status"`
		Assignments    []interaction.CoordinationAssignmentSnapshot `json:"assignments"`
		Verification   string                                       `json:"verification"`
	}
	if err := json.Unmarshal(res.Output, &response); err != nil {
		t.Fatal(err)
	}
	if response.Status != interaction.CoordinationSucceeded || len(response.Assignments) != 2 || len(workers.started) != 2 {
		t.Fatalf("synchronous workers failed to drive the coordinator to terminal: %+v, workers=%d", response, len(workers.started))
	}
	for _, item := range response.Assignments {
		if item.Status != interaction.AssignmentSucceeded || !strings.HasPrefix(item.ResultRef, "completed: ") {
			t.Fatalf("worker result absent from parent call: %+v", item)
		}
	}
	if response.Verification != "requires_parent_review" {
		t.Fatalf("child completion must not be accepted as final parent acceptance: %q", response.Verification)
	}
	parent, ok, err := tracker.GetByRequestID(context.Background(), scope.SpaceID, scope.RequestID)
	if err != nil || !ok || parent.RecoveryDescriptor == nil || parent.RecoveryDescriptor.MultiAgent == nil {
		t.Fatalf("terminal snapshot not persisted: %+v err=%v", parent, err)
	}
	if parent.RecoveryDescriptor.MultiAgent.Status != string(interaction.CoordinationSucceeded) {
		t.Fatalf("durable parent status lagged synchronous children: %s", parent.RecoveryDescriptor.MultiAgent.Status)
	}
}

func TestChatHarnessMultiAgentStatusRestoresTerminalHistoryAfterRestart(t *testing.T) {
	ctx := context.Background()
	adapter, workers, scope, tracker := testMultiAgentModelRuntime(t)
	workers.finishImmediately = true
	issued := adapter.executeMultiAgentTool(ctx, multiAgentDelegateTool,
		json.RawMessage(`{"objectives":["Implement change safely","Validate the parent build"],"strategy":"sequential"}`), scope)
	if issued.Error != nil {
		t.Fatal(issued.Error)
	}
	var response struct {
		CoordinationID string `json:"coordinationId"`
		Status         string `json:"status"`
	}
	if err := json.Unmarshal(issued.Output, &response); err != nil {
		t.Fatal(err)
	}
	if response.Status != "succeeded" || response.CoordinationID == "" {
		t.Fatalf("expected completed task: %+v", response)
	}
	freshGoals := decision.NewGoalRegistry()
	adapter.ConfigureMultiAgent(interaction.NewMultiAgentCoordinator(tracker, freshGoals, nil, workers, nil, interaction.DefaultMultiAgentPolicy()), tracker, freshGoals)
	arg, _ := json.Marshal(map[string]string{"coordination_id": response.CoordinationID})
	queried := adapter.executeMultiAgentTool(ctx, multiAgentStatusTool, arg, scope)
	if queried.Error != nil {
		t.Fatalf("completed task unavailable after restart: %+v", queried.Error)
	}
	var snapshot interaction.CoordinationSnapshot
	if err := json.Unmarshal(queried.Output, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != interaction.CoordinationSucceeded || len(snapshot.Assignments) != 2 ||
		snapshot.Assignments[0].ResultRef == "" || snapshot.Assignments[1].ResultRef == "" || len(workers.started) != 2 {
		t.Fatalf("terminal history lost or worker replayed: %+v started=%d", snapshot, len(workers.started))
	}
	scope.ConversationID = "unrelated-conversation"
	cross := adapter.executeMultiAgentTool(ctx, multiAgentStatusTool, arg, scope)
	if cross.Error == nil {
		t.Fatalf("other conversation read recovered results: %+v", cross)
	}
}

func TestChatHarnessMultiAgentWaitResumesParentWhenChildrenComplete(t *testing.T) {
	ctx := context.Background()
	a, workers, scope, tracker := testMultiAgentModelRuntime(t)
	result := a.executeMultiAgentTool(ctx, multiAgentDelegateTool,
		json.RawMessage(`{"objectives":["Review code in parallel","Run focused verification"],"strategy":"parallel"}`), scope)
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	var payload struct {
		CoordinationID string `json:"coordinationId"`
	}
	if err := json.Unmarshal(result.Output, &payload); err != nil {
		t.Fatal(err)
	}
	if len(workers.started) != 2 {
		t.Fatalf("parallel workers not started: %d", len(workers.started))
	}
	go func() {
		time.Sleep(120 * time.Millisecond)
		for _, started := range workers.started {
			child, ok, err := tracker.GetByRequestID(ctx, scope.SpaceID, "ma-"+started.AssignmentID)
			if err != nil || !ok {
				continue
			}
			_, _ = tracker.Complete(ctx, child.ID, child.StatusVersion, "verified:"+started.AssignmentID)
		}
	}()
	args, _ := json.Marshal(map[string]any{"coordination_id": payload.CoordinationID, "wait_seconds": 2})
	waited := a.executeMultiAgentTool(ctx, multiAgentWaitTool, args, scope)
	if waited.Error != nil {
		t.Fatalf("waiting for children failed: %+v", waited.Error)
	}
	var snapshot interaction.CoordinationSnapshot
	if err := json.Unmarshal(waited.Output, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != interaction.CoordinationSucceeded || len(snapshot.Assignments) != 2 {
		t.Fatalf("wait did not resume parent on actual terminal results: %+v", snapshot)
	}
	for _, item := range snapshot.Assignments {
		if item.ResultRef == "" {
			t.Fatalf("missing worker result reference: %+v", item)
		}
	}
	scope.ConversationID = "other-conversation"
	cross := a.executeMultiAgentTool(ctx, multiAgentWaitTool, args, scope)
	if cross.Error == nil {
		t.Fatal("other conversation accessed wait results")
	}
}

func TestChatHarnessMultiAgentWaitIsBoundedAndCannotFabricateCompletion(t *testing.T) {
	ctx := context.Background()
	a, _, scope, _ := testMultiAgentModelRuntime(t)
	result := a.executeMultiAgentTool(ctx, multiAgentDelegateTool,
		json.RawMessage(`{"objectives":["Inspect the network runtime","Inspect the worker lifecycle"]}`), scope)
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	var payload struct {
		CoordinationID string `json:"coordinationId"`
	}
	_ = json.Unmarshal(result.Output, &payload)
	args, _ := json.Marshal(map[string]any{"coordination_id": payload.CoordinationID, "wait_seconds": 1})
	waited := a.executeMultiAgentTool(ctx, multiAgentWaitTool, args, scope)
	if waited.Error != nil {
		t.Fatalf("pending workers should return status not fake completion: %+v", waited.Error)
	}
	var status struct {
		Status       string `json:"status"`
		WaitTimedOut bool   `json:"waitTimedOut"`
	}
	if err := json.Unmarshal(waited.Output, &status); err != nil {
		t.Fatal(err)
	}
	if !status.WaitTimedOut || status.Status == "succeeded" {
		t.Fatalf("pending workers were marked complete: %+v", status)
	}
}

func TestChatHarnessMultiAgentRejectsSpoofedInternalWorkerSource(t *testing.T) {
	a, _, scope, tracker := testMultiAgentModelRuntime(t)
	child := interaction.NewInteractionRecord(interaction.InteractionScope{
		SpaceID:        scope.SpaceID,
		CharacterID:    scope.CharacterID,
		ConversationID: "subagent-conversation",
		Channel:        "web",
		Source:         "multi_agent",
		RequestID:      "subagent-request",
	})
	child.Status = interaction.InteractionStatusProcessing
	if err := tracker.Create(context.Background(), child); err != nil {
		t.Fatal(err)
	}
	scope.ConversationID = child.Scope.ConversationID
	scope.RequestID = child.Scope.RequestID
	scope.IsInternal = false
	scope.Source = ""
	if len(a.appendMultiAgentTools(context.Background(), nil, scope)) != 0 {
		t.Fatal("internal child was allowed to discover recursive delegation despite missing source flags")
	}
	result := a.executeMultiAgentTool(context.Background(), multiAgentDelegateTool,
		json.RawMessage(`{"objectives":["Try recursive dispatch","Try scope escalation"]}`), scope)
	if result.Error == nil || result.Error.Code != "MULTI_AGENT_NOT_AUTHORIZED" {
		t.Fatalf("internal child bypassed durable tracker authorization: %+v", result)
	}
	scope.ConversationID = "other-conversation"
	if len(a.appendMultiAgentTools(context.Background(), nil, scope)) != 0 {
		t.Fatal("mismatched conversation discovered delegation")
	}
}
