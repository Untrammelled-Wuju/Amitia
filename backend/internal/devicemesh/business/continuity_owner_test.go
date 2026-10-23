package business

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/continuity"
)

func TestOwnedContinuityBindingObservationAndRuntimeSignal(t *testing.T) {
	engine, _, _, model := engineHarness(t)
	engine.data = continuitySignalPort{engine.data.(testDataPort)}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "create"}
	document, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{Action: "create", Title: "完成部署", Goal: "完成部署并验证", ExpectedRevision: 0})
	if err != nil {
		t.Fatal(err)
	}
	binding := continuity.NewThreadBinding(document.Thread.ID, "conversation", "chat-1", "context", "test", 1)
	request.RequestID = "bind"
	document, _, err = engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: document.Thread.Revision, Action: "bind", Binding: &binding})
	if err != nil || len(document.Bindings) != 1 {
		t.Fatalf("bind=%+v err=%v", document.Bindings, err)
	}
	payload := continuity.ObservePayload{Version: "v1", ThreadID: document.Thread.ID, SpaceID: "core", CharacterID: "role", ConversationID: "chat-1", RequestID: "turn", ExecutionID: "execution", UserMessage: "继续部署", AssistantReply: "部署正在进行。请确认是否继续迁移。", Source: "chat"}
	patch := continuity.BuildHeuristicThreadPatch(&document.Thread, payload)
	request.RequestID = "observe"
	document, _, err = engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: document.Thread.Revision, Action: "observe", Observation: &payload, ThreadPatch: &patch})
	if err != nil || document.Thread.CurrentState == "" || len(document.Waits) != 1 || document.Waits[0].WaitType != continuity.WaitTypeUser {
		t.Fatalf("observe=%+v err=%v", document, err)
	}
	request.RequestID = "wait"
	document, _, err = engine.Continuity(t.Context(), request, ContinuityMutation{
		ID: document.Thread.ID, ExpectedRevision: document.Thread.Revision, Action: "add_wait",
		Wait: &continuity.Wait{WaitType: continuity.WaitTypeDependency, ConditionJSON: `{"taskRunId":"task-1"}`, AutoResume: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Bindings) != 1 || len(document.Waits) < 2 {
		t.Fatalf("prepared document bindings=%d waits=%d", len(document.Bindings), len(document.Waits))
	}
	if !continuity.WaitMatchesSignal(document.Waits[len(document.Waits)-1], continuity.Signal{WaitType: continuity.WaitTypeDependency, Attributes: map[string]any{"taskRunId": "task-1"}}) {
		t.Fatal("prepared dependency wait does not match task signal")
	}
	listed, _, err := engine.ListContinuity(t.Context(), request, false)
	if err != nil || len(listed) != 1 || len(listed[0].Bindings) != 1 {
		t.Fatalf("listed=%+v err=%v", listed, err)
	}
	request.RequestID = "runtime-event"
	resolved, err := engine.ApplyContinuityRuntimeEvent(t.Context(), request, ContinuityRuntimeEvent{
		Bindings:       []continuity.BindingLookup{{Type: "conversation", ID: "chat-1"}},
		EventType:      "task.succeeded",
		SourceType:     "task_runtime",
		SourceID:       "task-1",
		ConversationID: "chat-1",
		ExecutionID:    "execution-1",
		PayloadJSON:    `{"taskRunId":"task-1","status":"succeeded"}`,
		CurrentState:   "任务执行已完成",
		Signal:         &continuity.Signal{WaitType: continuity.WaitTypeDependency, Source: "task_runtime", SourceID: "task-1", Attributes: map[string]any{"taskRunId": "task-1"}, OccurredAt: time.Now().UTC()},
	})
	if err != nil || len(resolved) != 1 {
		t.Fatalf("resolved=%v err=%v", resolved, err)
	}
	if err := engine.TickContinuity(t.Context()); err != nil {
		t.Fatal(err)
	}
	if model.calls.Load() != 1 {
		t.Fatalf("continuity wake calls=%d", model.calls.Load())
	}
}

func TestOwnedContinuityRuntimeSignalUsesCoordinatedOwner(t *testing.T) {
	engine, _, service, _ := engineHarness(t)
	engine.data = continuitySignalPort{engine.data.(testDataPort)}
	if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
		t.Fatal(err)
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", RequestID: "create-cloud"}
	document, _, err := engine.Continuity(t.Context(), request, ContinuityMutation{Action: "create", Title: "云端任务", Goal: "验证云端统筹", ExpectedRevision: 0})
	if err != nil || document.OwnerID != "core" {
		t.Fatalf("document=%+v err=%v", document, err)
	}
	binding := continuity.NewThreadBinding(document.Thread.ID, "conversation", "cloud-chat", "context", "test", 1)
	request.RequestID = "bind-cloud"
	document, _, err = engine.Continuity(t.Context(), request, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: document.Thread.Revision, Action: "bind", Binding: &binding})
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID = "runtime-cloud"
	if _, err := engine.ApplyContinuityRuntimeEvent(t.Context(), request, ContinuityRuntimeEvent{Bindings: []continuity.BindingLookup{{Type: "conversation", ID: "cloud-chat"}}, EventType: "workflow.succeeded", SourceType: "workflow", SourceID: "workflow-1", CurrentState: "云端完成"}); err != nil {
		t.Fatal(err)
	}
	documents, _, err := engine.ListContinuity(t.Context(), request, true)
	if err != nil || len(documents) != 1 || documents[0].OwnerID != "core" || documents[0].Thread.CurrentState != "云端完成" {
		t.Fatalf("documents=%+v err=%v", documents, err)
	}
}
