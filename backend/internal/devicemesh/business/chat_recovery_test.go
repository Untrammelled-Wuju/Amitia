package business

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedRecoveryConfirmsSavedReplyWithoutCallingModel(t *testing.T) {
	engine, _, _, model := engineHarness(t)
	response, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "conversation", RequestID: "recovery", Message: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	calls := model.calls.Load()
	result, err := engine.ReconcileOwnedTurn(t.Context(), response.Scope, response.ConversationID)
	if err != nil || result.Status != "completed" || result.Response == nil || model.calls.Load() != calls {
		t.Fatalf("result=%+v err=%v calls=%d", result, err, model.calls.Load())
	}
	foreign := response.Scope
	foreign.ResourceOwnerID = "other"
	if _, err := engine.ReconcileOwnedTurn(t.Context(), foreign, response.ConversationID); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("foreign owner accepted: %v", err)
	}
}

func TestOwnedRecoveryMarksUnfinishedCheckpointWithoutReplayingTools(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	model.generate = func(context.Context, Inference) (Generation, error) {
		return Generation{}, errors.New("process stopped after tool dispatch")
	}
	_, _ = engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "conversation", RequestID: "unfinished", Message: "perform action"})
	store := coordination.NewOwnershipStore(db, "a")
	row, err := store.Get(t.Context(), "checkpoint", "turn/unfinished")
	if err != nil || row == nil {
		t.Fatalf("checkpoint=%+v err=%v", row, err)
	}
	var saved checkpoint
	if err := json.Unmarshal(row.Body, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Scope == nil {
		t.Fatal("missing execution scope")
	}
	result, err := engine.ReconcileOwnedTurn(t.Context(), *saved.Scope, "conversation")
	if err != nil || result.Status != "needs_reconciliation" || model.calls.Load() != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	row, _ = store.Get(t.Context(), "checkpoint", "turn/unfinished")
	_ = json.Unmarshal(row.Body, &saved)
	if saved.Status != "needs_reconciliation" {
		t.Fatalf("status=%s", saved.Status)
	}
}

func TestQueuedChatRestartsFromOwnerCheckpointAndExecutesOnlyOnce(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	initial, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "conversation", RequestID: "initial", Message: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	scope := initial.Scope
	scope.RequestID, scope.TurnID, scope.ExecutionID = "queued-after-restart", "queue-turn", "queue-execution"
	request := Request{SpaceID: "core", DeviceID: "a", TargetDeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "conversation", RequestID: scope.RequestID, Message: "queued input", ExpectedScope: &scope}
	actor := &auth.ActorContext{SpaceID: "core", DeviceID: "a", PrincipalType: auth.PrincipalTrustedDevice}
	document := queuedChatDocument{Actor: actor, Hash: "queue-hash", Request: request, Scope: scope, Status: "queued"}
	if err := engine.coordination.RegisterQueuedRequest(t.Context(), scope, document.Hash, request.ConversationID); err != nil {
		t.Fatal(err)
	}
	store := coordination.NewOwnershipStore(db, "a")
	_, err = store.Apply(t.Context(), coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: queuedChatResourceID(scope, scope.RequestID), RoleID: "role", Body: body(document)}}})
	if err != nil {
		t.Fatal(err)
	}
	restarted := NewEngine(engine.coordination, engine.data, model)
	ctx := auth.WithActor(t.Context(), actor)
	if _, err := restarted.ResumeQueuedChat(ctx, scope, scope.RequestID, "wrong-conversation"); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("foreign conversation accepted: %v", err)
	}
	if err := restarted.ResumePendingQueuedChats(t.Context()); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	for {
		row, err := store.Get(t.Context(), "checkpoint", queuedChatResourceID(scope, scope.RequestID))
		if err != nil {
			t.Fatal(err)
		}
		if json.Unmarshal(row.Body, &document) != nil {
			t.Fatal("invalid queued document")
		}
		if document.Status == "completed" {
			break
		}
		if document.Status == "failed" || document.Status == "uncertain" {
			t.Fatalf("queue=%+v", document)
		}
		select {
		case <-deadline:
			t.Fatal("queued request never resumed")
		case <-time.After(10 * time.Millisecond):
		}
	}
	if _, err := restarted.ResumeQueuedChat(ctx, scope, scope.RequestID, "conversation"); err != nil {
		t.Fatal(err)
	}
	if model.calls.Load() != 2 {
		t.Fatalf("model replayed: %d calls", model.calls.Load())
	}
}

func TestOwnedRuntimePersistsToolJournalAndRetryLoadsItWithoutReplay(t *testing.T) {
	engine, db, _, model := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "journal-conversation", RequestID: "journal-original", Message: "run task"}
	model.generate = func(ctx context.Context, inference Inference) (Generation, error) {
		for _, eventType := range []string{"tool-started", "tool-completed"} {
			err := inference.Emit(Event{Type: eventType, Progress: body(map[string]any{"toolCallId": "saved-call", "toolName": "write_file", "arguments": "{}", "result": "{\"saved\":true}"})})
			if err != nil {
				return Generation{}, err
			}
		}
		return Generation{}, errors.New("model disconnected after confirmed tool result")
	}
	_, err := engine.Run(t.Context(), request)
	if err == nil {
		t.Fatal("expected model disconnect")
	}
	row, err := coordination.NewOwnershipStore(db, "a").Get(t.Context(), "checkpoint", "turn/"+request.RequestID)
	var saved checkpoint
	if err != nil || row == nil || json.Unmarshal(row.Body, &saved) != nil || saved.Runtime == nil || len(saved.Runtime.Blocks) != 2 || saved.Runtime.Blocks[0].Status != "completed" || saved.Runtime.Blocks[1].Type != "error" || saved.Runtime.Blocks[1].Content != "model disconnected after confirmed tool result" {
		t.Fatalf("journal not durable: row=%+v saved=%+v err=%v", row, saved, err)
	}
	model.generate = func(ctx context.Context, inference Inference) (Generation, error) {
		if inference.RecoveredRuntime == nil || len(inference.RecoveredRuntime.Blocks) != 1 || inference.RecoveredRuntime.Blocks[0].CallID != "saved-call" {
			t.Fatal("confirmed journal not provided to model continuation")
		}
		return Generation{Text: "continued from saved result"}, nil
	}
	request.RetryRequestID, request.RequestID = request.RequestID, "journal-retry"
	response, err := engine.Run(t.Context(), request)
	if err != nil || !response.Saved || response.Text != "continued from saved result" {
		t.Fatalf("retry=%+v err=%v", response, err)
	}
}

func TestOwnedRetryRejectsUnconfirmedToolStartBeforeCallingModel(t *testing.T) {
	engine, _, _, model := engineHarness(t)
	request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "uncertain-conversation", RequestID: "uncertain-original", Message: "perform task"}
	model.generate = func(ctx context.Context, inference Inference) (Generation, error) {
		if err := inference.Emit(Event{Type: "tool-started", Progress: body(map[string]any{"toolCallId": "uncertain-call", "toolName": "send_message", "arguments": "{}"})}); err != nil {
			return Generation{}, err
		}
		return Generation{}, errors.New("process died after dispatch")
	}
	_, _ = engine.Run(t.Context(), request)
	request.RetryRequestID, request.RequestID = request.RequestID, "uncertain-retry"
	if _, err := engine.Run(t.Context(), request); !errors.Is(err, ErrUncertainExecution) {
		t.Fatalf("uncertain side effect resumed: %v", err)
	}
	if model.calls.Load() != 1 {
		t.Fatalf("unconfirmed side effect reached model again: %d", model.calls.Load())
	}
}
