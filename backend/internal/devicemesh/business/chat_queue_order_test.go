package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestQueuedChatSnapshotPreservesMessageAndEnqueueOrder(t *testing.T) {
	engine, _, _, model := engineHarness(t)
	initial, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "queue-order", RequestID: "initial-order", Message: "initial"})
	if err != nil {
		t.Fatal(err)
	}
	actor := &auth.ActorContext{SpaceID: "core", DeviceID: "a", PrincipalType: auth.PrincipalTrustedDevice}
	ctx := auth.WithActor(context.Background(), actor)
	unlock := engine.lock("core\x00a\x00queue-order")
	locked := true
	defer func() {
		if locked {
			unlock()
		}
	}()
	t.Cleanup(func() {
		deadline := time.Now().Add(5 * time.Second)
		for {
			active := false
			activeQueuedChats.Range(func(key, _ any) bool {
				if key.(queuedChatKey).Engine == engine {
					active = true
					return false
				}
				return true
			})
			if !active {
				return
			}
			if time.Now().After(deadline) {
				t.Error("queued workers did not finish")
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
	for _, item := range []struct{ id, text string }{{"first", "first message"}, {"second", "second message"}, {"third", "deleted message"}} {
		_, err := engine.QueueChat(ctx, initial.Scope, Request{ConversationID: "queue-order", RequestID: item.id, Message: item.text})
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", TargetDeviceID: "a", RoleID: "role", ConversationID: "queue-order", ExpectedScope: &initial.Scope}
	items, err := engine.ResumeConversationQueuedChats(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].RequestID != "first" || items[0].Message != "first message" || items[1].RequestID != "second" || items[1].Message != "second message" || items[2].RequestID != "third" || items[0].Position >= items[1].Position {
		t.Fatalf("unexpected queue: %+v", items)
	}
	if _, err := engine.ReorderQueuedChats(ctx, request, []string{"second", "first", "third"}); err != nil {
		t.Fatal(err)
	}
	if err := engine.DeleteQueuedChat(ctx, request, "third"); err != nil {
		t.Fatal(err)
	}
	seen := make(chan string, 2)
	model.generate = func(_ context.Context, inference Inference) (Generation, error) {
		seen <- inference.Message
		return Generation{Text: "reply"}, nil
	}
	unlock()
	locked = false
	for _, expected := range []string{"second message", "first message"} {
		select {
		case actual := <-seen:
			if actual != expected {
				t.Fatalf("expected %q, got %q", expected, actual)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("queued message %q did not execute", expected)
		}
	}
	select {
	case actual := <-seen:
		t.Fatalf("deleted message executed: %q", actual)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSteeredQueuedChatIsConsumedWithoutStartingNewTurn(t *testing.T) {
	engine, _, _, model := engineHarness(t)
	initial, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "queue-steer", RequestID: "initial-steer", Message: "initial"})
	if err != nil {
		t.Fatal(err)
	}
	actor := &auth.ActorContext{SpaceID: "core", DeviceID: "a", PrincipalType: auth.PrincipalTrustedDevice}
	ctx := auth.WithActor(context.Background(), actor)
	unlock := engine.lock("core\x00a\x00queue-steer")
	defer unlock()
	_, err = engine.QueueChat(ctx, initial.Scope, Request{ConversationID: "queue-steer", RequestID: "to-steer", Message: "change direction"})
	if err != nil {
		t.Fatal(err)
	}
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", TargetDeviceID: "a", RoleID: "role", ConversationID: "queue-steer", ExpectedScope: &initial.Scope}
	key := RuntimeKey(initial.Scope, request.ConversationID)
	_, _, ok := ownedRuntimeManager.BeginExecution(key, "active-turn")
	if !ok {
		t.Fatal("could not start active turn")
	}
	defer ownedRuntimeManager.ClearExecution(key, "active-turn")
	if _, err := engine.SteerQueuedChat(ctx, request, "to-steer", "wrong-turn"); err == nil {
		t.Fatal("accepted steering for a different turn")
	}
	items, err := engine.ResumeConversationQueuedChats(ctx, request)
	if err != nil || len(items) != 1 {
		t.Fatalf("failed steering removed message: items=%+v err=%v", items, err)
	}
	content, err := engine.SteerQueuedChat(ctx, request, "to-steer", "active-turn")
	if err != nil || content != "change direction" {
		t.Fatalf("steer: content=%q err=%v", content, err)
	}
	steers := ownedRuntimeManager.ConsumeSteer(key, "active-turn")
	if len(steers) != 1 || steers[0] != content {
		t.Fatalf("unexpected steering input: %+v", steers)
	}
	items, err = engine.ResumeConversationQueuedChats(ctx, request)
	if err != nil || len(items) != 0 || model.calls.Load() != 1 {
		t.Fatalf("steered message was not removed: items=%+v calls=%d err=%v", items, model.calls.Load(), err)
	}
}

func TestDuplicateQueuedChatRequestIDRejectsChangedContent(t *testing.T) {
	engine, _, _, _ := engineHarness(t)
	initial, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "queue-duplicate", RequestID: "initial-duplicate", Message: "initial"})
	if err != nil {
		t.Fatal(err)
	}
	actor := &auth.ActorContext{SpaceID: "core", DeviceID: "a", PrincipalType: auth.PrincipalTrustedDevice}
	ctx := auth.WithActor(context.Background(), actor)
	unlock := engine.lock("core\x00a\x00queue-duplicate")
	defer unlock()
	if _, err := engine.QueueChat(ctx, initial.Scope, Request{ConversationID: "queue-duplicate", RequestID: "duplicate", Message: "first"}); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.QueueChat(ctx, initial.Scope, Request{ConversationID: "queue-duplicate", RequestID: "duplicate", Message: "changed"}); !errors.Is(err, coordination.ErrRequestConflict) {
		t.Fatalf("changed duplicate accepted: %v", err)
	}
}

func TestQueuedChatsAreIsolatedByConversationAndActorDevice(t *testing.T) {
	engine, _, _, _ := engineHarness(t)
	first, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "queue-isolation-a", RequestID: "initial-isolation-a", Message: "a"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "queue-isolation-b", RequestID: "initial-isolation-b", Message: "b"})
	if err != nil {
		t.Fatal(err)
	}
	actor := &auth.ActorContext{SpaceID: "core", DeviceID: "a", PrincipalType: auth.PrincipalTrustedDevice}
	ctx := auth.WithActor(context.Background(), actor)
	unlockFirst := engine.lock("core\x00a\x00queue-isolation-a")
	unlockSecond := engine.lock("core\x00a\x00queue-isolation-b")
	defer unlockSecond()
	defer unlockFirst()
	if _, err := engine.QueueChat(ctx, first.Scope, Request{ConversationID: "queue-isolation-a", RequestID: "isolated-a", Message: "only a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.QueueChat(ctx, second.Scope, Request{ConversationID: "queue-isolation-b", RequestID: "isolated-b", Message: "only b"}); err != nil {
		t.Fatal(err)
	}
	firstItems, err := engine.ResumeConversationQueuedChats(ctx, Request{SpaceID: "core", DeviceID: "a", CoreID: "core", TargetDeviceID: "a", RoleID: "role", ConversationID: "queue-isolation-a", ExpectedScope: &first.Scope})
	if err != nil || len(firstItems) != 1 || firstItems[0].RequestID != "isolated-a" {
		t.Fatalf("first conversation=%+v err=%v", firstItems, err)
	}
	secondItems, err := engine.ResumeConversationQueuedChats(ctx, Request{SpaceID: "core", DeviceID: "a", CoreID: "core", TargetDeviceID: "a", RoleID: "role", ConversationID: "queue-isolation-b", ExpectedScope: &second.Scope})
	if err != nil || len(secondItems) != 1 || secondItems[0].RequestID != "isolated-b" {
		t.Fatalf("second conversation=%+v err=%v", secondItems, err)
	}
	foreignActor := &auth.ActorContext{SpaceID: "core", DeviceID: "b", PrincipalType: auth.PrincipalTrustedDevice}
	foreignCtx := auth.WithActor(context.Background(), foreignActor)
	if _, err := engine.QueueChat(foreignCtx, first.Scope, Request{ConversationID: "queue-isolation-a", RequestID: "foreign", Message: "foreign"}); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("foreign device queued into owner conversation: %v", err)
	}
	if err := engine.DeleteQueuedChat(ctx, Request{SpaceID: "core", DeviceID: "a", CoreID: "core", TargetDeviceID: "a", RoleID: "role", ConversationID: "queue-isolation-a", ExpectedScope: &first.Scope}, "isolated-a"); err != nil {
		t.Fatal(err)
	}
	if err := engine.DeleteQueuedChat(ctx, Request{SpaceID: "core", DeviceID: "a", CoreID: "core", TargetDeviceID: "a", RoleID: "role", ConversationID: "queue-isolation-b", ExpectedScope: &second.Scope}, "isolated-b"); err != nil {
		t.Fatal(err)
	}
}

func TestQueuedChatAcceptsValidatedQuoteAndRejectsChangedSource(t *testing.T) {
	engine, db, _, _ := engineHarness(t)
	initial, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "queue-quote", RequestID: "initial-quote", Message: "原始问题"})
	if err != nil {
		t.Fatal(err)
	}
	message, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "message", initial.RequestID+"/assistant")
	if err != nil || message == nil {
		t.Fatalf("quote source=%+v err=%v", message, err)
	}
	var document struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(message.Body, &document); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(document.Content))
	expected := initial.Scope
	quote := &QuoteReference{
		OwnerID:          "a",
		RoleID:           "role",
		ConversationID:   "queue-quote",
		MessageID:        message.ID,
		ExpectedRevision: message.Revision,
		ContentHash:      hex.EncodeToString(digest[:]),
		ExpectedScope:    &expected,
	}
	actor := &auth.ActorContext{SpaceID: "core", DeviceID: "a", PrincipalType: auth.PrincipalTrustedDevice}
	ctx := auth.WithActor(context.Background(), actor)
	unlock := engine.lock("core\x00a\x00queue-quote")
	defer unlock()
	if _, err := engine.QueueChat(ctx, initial.Scope, Request{ConversationID: "queue-quote", RequestID: "quote-valid", Message: "按引用继续", Quote: quote}); err != nil {
		t.Fatal(err)
	}
	items, err := engine.ResumeConversationQueuedChats(ctx, Request{SpaceID: "core", DeviceID: "a", CoreID: "core", TargetDeviceID: "a", RoleID: "role", ConversationID: "queue-quote", ExpectedScope: &initial.Scope})
	if err != nil || len(items) != 1 || items[0].Quote == nil || items[0].Quote.MessageID != message.ID || items[0].Quote.ExpectedRevision != message.Revision {
		t.Fatalf("quoted queue=%+v err=%v", items, err)
	}
	changed := *quote
	changed.MessageID = "missing"
	if _, err := engine.QueueChat(ctx, initial.Scope, Request{ConversationID: "queue-quote", RequestID: "quote-missing", Message: "引用失效", Quote: &changed}); !errors.Is(err, coordination.ErrResourceVersion) && !strings.Contains(err.Error(), "引用消息不存在") {
		t.Fatalf("missing quote accepted: %v", err)
	}
	if err := engine.DeleteQueuedChat(ctx, Request{SpaceID: "core", DeviceID: "a", CoreID: "core", TargetDeviceID: "a", RoleID: "role", ConversationID: "queue-quote", ExpectedScope: &initial.Scope}, "quote-valid"); err != nil {
		t.Fatal(err)
	}
}

func TestInterruptedSteeringBecomesVisibleUncertainWithoutRedelivery(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	initial, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "queue-steer-recovery", RequestID: "initial-steer-recovery", Message: "initial"})
	if err != nil {
		t.Fatal(err)
	}
	actor := &auth.ActorContext{SpaceID: "core", DeviceID: "a", PrincipalType: auth.PrincipalTrustedDevice}
	ctx := auth.WithActor(context.Background(), actor)
	unlock := engine.lock("core\x00a\x00queue-steer-recovery")
	if _, err := engine.QueueChat(ctx, initial.Scope, Request{ConversationID: "queue-steer-recovery", RequestID: "steer-recovery", Message: "change direction"}); err != nil {
		unlock()
		t.Fatal(err)
	}
	store := coordination.NewOwnershipStore(db, "a")
	row, err := store.Get(t.Context(), "checkpoint", queuedChatResourceID(initial.Scope, "steer-recovery"))
	if err != nil || row == nil {
		unlock()
		t.Fatalf("queue row=%+v err=%v", row, err)
	}
	var document queuedChatDocument
	if err := json.Unmarshal(row.Body, &document); err != nil {
		unlock()
		t.Fatal(err)
	}
	document.Status = "steering"
	document.SteerTurnID = "turn-before-restart"
	commitScope := document.Scope
	commitScope.RequestID = "test-steering-recovery"
	if _, err := store.Apply(t.Context(), coordination.Commit{Scope: commitScope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: row.ID, RoleID: row.RoleID, ExpectedRevision: row.Revision, Body: body(document)}}}); err != nil {
		unlock()
		t.Fatal(err)
	}
	unlock()
	status := waitQueuedStatus(t, engine, ctx, initial.Scope, "queue-steer-recovery", "steer-recovery")
	if status.Status != "uncertain" || !strings.Contains(status.Error, "turn-before-restart") || !strings.Contains(status.Error, "不会再次投递") {
		t.Fatalf("interrupted steering not visible: %+v", status)
	}
	if model.calls.Load() != 1 {
		t.Fatalf("steering recovery started another turn: %d", model.calls.Load())
	}
}

func TestRunningCheckpointBecomesUncertainWithoutReplayingModel(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	initial, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "queue-running-recovery", RequestID: "initial-running-recovery", Message: "initial"})
	if err != nil {
		t.Fatal(err)
	}
	actor := &auth.ActorContext{SpaceID: "core", DeviceID: "a", PrincipalType: auth.PrincipalTrustedDevice}
	ctx := auth.WithActor(context.Background(), actor)
	unlock := engine.lock("core\x00a\x00queue-running-recovery")
	_, err = engine.QueueChat(ctx, initial.Scope, Request{ConversationID: "queue-running-recovery", RequestID: "running-recovery", Message: "perform action"})
	if err != nil {
		unlock()
		t.Fatal(err)
	}
	store := coordination.NewOwnershipStore(db, "a")
	row, err := store.Get(t.Context(), "checkpoint", queuedChatResourceID(initial.Scope, "running-recovery"))
	if err != nil || row == nil {
		unlock()
		t.Fatalf("queue row=%+v err=%v", row, err)
	}
	var document queuedChatDocument
	if err := json.Unmarshal(row.Body, &document); err != nil {
		unlock()
		t.Fatal(err)
	}
	document.Status = "running"
	commitScope := document.Scope
	commitScope.RequestID = "test-running-recovery"
	if _, err := store.Apply(t.Context(), coordination.Commit{Scope: commitScope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: row.ID, RoleID: row.RoleID, ExpectedRevision: row.Revision, Body: body(document)}}}); err != nil {
		unlock()
		t.Fatal(err)
	}
	runningCheckpoint := checkpoint{Hash: "running-hash", Status: "running", ConversationID: "queue-running-recovery", Scope: &document.Scope}
	checkpointScope := document.Scope
	checkpointScope.RequestID = "test-running-checkpoint"
	if _, err := store.Apply(t.Context(), coordination.Commit{Scope: checkpointScope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: "turn/running-recovery", RoleID: "role", Body: body(runningCheckpoint)}}}); err != nil {
		unlock()
		t.Fatal(err)
	}
	unlock()
	status := waitQueuedStatus(t, engine, ctx, initial.Scope, "queue-running-recovery", "running-recovery")
	if status.Status != "uncertain" || !strings.Contains(status.Error, "checkpoint status=running") {
		t.Fatalf("running checkpoint not visible: %+v", status)
	}
	if model.calls.Load() != 1 {
		t.Fatalf("running checkpoint was replayed: %d", model.calls.Load())
	}
}

func TestCompletedTurnRecoversQueueResultWithoutCallingModel(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	initial, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "queue-completed-recovery", RequestID: "initial-completed-recovery", Message: "initial"})
	if err != nil {
		t.Fatal(err)
	}
	actor := &auth.ActorContext{SpaceID: "core", DeviceID: "a", PrincipalType: auth.PrincipalTrustedDevice}
	ctx := auth.WithActor(context.Background(), actor)
	unlock := engine.lock("core\x00a\x00queue-completed-recovery")
	if _, err := engine.QueueChat(ctx, initial.Scope, Request{ConversationID: "queue-completed-recovery", RequestID: "completed-recovery", Message: "finish once"}); err != nil {
		unlock()
		t.Fatal(err)
	}
	unlock()
	status := waitQueuedStatus(t, engine, ctx, initial.Scope, "queue-completed-recovery", "completed-recovery")
	if status.Status != "completed" || model.calls.Load() != 2 {
		t.Fatalf("queued task did not finish once: %+v calls=%d", status, model.calls.Load())
	}
	store := coordination.NewOwnershipStore(db, "a")
	row, err := store.Get(t.Context(), "checkpoint", queuedChatResourceID(initial.Scope, "completed-recovery"))
	if err != nil || row == nil {
		t.Fatalf("queue row=%+v err=%v", row, err)
	}
	var document queuedChatDocument
	if err := json.Unmarshal(row.Body, &document); err != nil {
		t.Fatal(err)
	}
	document.Status = "running"
	document.Error = ""
	commitScope := document.Scope
	commitScope.RequestID = "test-completed-recovery"
	if _, err := store.Apply(t.Context(), coordination.Commit{Scope: commitScope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: row.ID, RoleID: row.RoleID, ExpectedRevision: row.Revision, Body: body(document)}}}); err != nil {
		t.Fatal(err)
	}
	restarted := NewEngine(engine.coordination, engine.data, model)
	if _, err := restarted.ResumeQueuedChat(ctx, initial.Scope, "completed-recovery", "queue-completed-recovery"); err != nil {
		t.Fatal(err)
	}
	status = waitQueuedStatus(t, restarted, ctx, initial.Scope, "queue-completed-recovery", "completed-recovery")
	if status.Status != "completed" || model.calls.Load() != 2 {
		t.Fatalf("completed checkpoint replayed: %+v calls=%d", status, model.calls.Load())
	}
}

func TestQueuedChatRestartRestoresPersistedResourceOwnerAndSurfacesFailure(t *testing.T) {
	engine, db, service, model := engineHarness(t)
	initial, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "queue-owner-recovery", RequestID: "initial-owner-recovery", Message: "initial"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.InitializeCoordinated(t.Context(), "core", "a", 1, "role"); err != nil {
		t.Fatal(err)
	}
	_, scope, finish, err := service.BeginWithOwners(t.Context(), "core", "a", "a", "core", "role", "owner-recovery-view", "a", "core")
	if err != nil {
		t.Fatal(err)
	}
	finish()
	scope.RoleRevision, scope.MemoryRoleID, scope.MemoryViewAll = initial.Scope.RoleRevision, initial.Scope.MemoryRoleID, initial.Scope.MemoryViewAll
	scope.RequestID, scope.TurnID, scope.ExecutionID = "owner-recovery", "owner-recovery-turn", "owner-recovery-execution"
	scope.ContinuityRoleID = ""
	request := Request{ConversationID: initial.ConversationID, RequestID: scope.RequestID, Message: "survive restart with original owner", ExpectedScope: &scope, RoleID: scope.RoleID, TargetDeviceID: scope.TargetDeviceID}
	actor := &auth.ActorContext{SpaceID: "core", DeviceID: "a", PrincipalType: auth.PrincipalTrustedDevice}
	hash := "owner-recovery-hash"
	if err := engine.coordination.RegisterQueuedRequest(t.Context(), scope, hash, initial.ConversationID); err != nil {
		t.Fatal(err)
	}
	document := queuedChatDocument{Actor: actor, Hash: hash, Request: request, Scope: scope, Status: "queued", Position: 1}
	store := coordination.NewOwnershipStore(db, "a")
	if _, err := store.Apply(t.Context(), coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: queuedChatResourceID(scope, scope.RequestID), RoleID: "role", Body: body(document)}}}); err != nil {
		t.Fatal(err)
	}
	restarted := NewEngine(service, engine.data, model)
	if err := restarted.ResumePendingQueuedChats(t.Context()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		row, err := store.Get(t.Context(), "checkpoint", queuedChatResourceID(scope, scope.RequestID))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(row.Body, &document); err != nil {
			t.Fatal(err)
		}
		if document.Status == "failed" || document.Status == "uncertain" {
			if !strings.Contains(document.Error, "设备专属数据") {
				t.Fatalf("owner recovery error missing: %+v", document)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("owner recovery did not settle: %+v", document)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitQueuedStatus(t *testing.T, engine *Engine, ctx context.Context, scope coordination.ExecutionScope, conversationID, requestID string) QueuedChatResponse {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		response, err := engine.ResumeQueuedChat(ctx, scope, requestID, conversationID)
		if err != nil {
			t.Fatal(err)
		}
		if response.Status == "completed" || response.Status == "failed" || response.Status == "uncertain" {
			return response
		}
		if time.Now().After(deadline) {
			t.Fatalf("queue did not settle: %+v", response)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
