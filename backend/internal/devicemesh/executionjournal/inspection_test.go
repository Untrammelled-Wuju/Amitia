package executionjournal_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

func TestJournalInspectionShowsOnlyMatchingConfirmedResult(t *testing.T) {
	db, store := harness(t)
	invoke := invocation()
	if got, err := store.Inspect(t.Context(), invoke); err != nil || got.State != "absent" || got.Result != nil {
		t.Fatalf("absent inspection: %+v %v", got, err)
	}
	calls := 0
	run := func() (*protocol.RuntimeResultPayload, error) {
		calls++
		return &protocol.RuntimeResultPayload{Status: "completed", Result: json.RawMessage(`{"ok":true}`)}, nil
	}
	if _, err := store.Execute(t.Context(), invoke, run); err != nil {
		t.Fatal(err)
	}
	if got, err := executionjournal.NewStore(db).Inspect(t.Context(), invoke); err != nil || got.State != "completed" || got.Result == nil || got.Result.Status != "completed" || calls != 1 {
		t.Fatalf("stored result inspection: %+v %v calls=%d", got, err, calls)
	}
	changed := invoke
	changed.Input = json.RawMessage(`{"text":"other"}`)
	if _, err := store.Inspect(t.Context(), changed); !errors.Is(err, executionjournal.ErrConflict) {
		t.Fatalf("different operation payload could query old result: %v", err)
	}
	changed = invoke
	changed.SpaceID = "other-core"
	if _, err := store.Inspect(t.Context(), changed); !errors.Is(err, executionjournal.ErrConflict) {
		t.Fatalf("another core could query old result: %v", err)
	}
}

func TestJournalInspectionKeepsUnknownAndOwnedResultsPrivate(t *testing.T) {
	db, store := harness(t)
	invoke := invocation()
	if _, err := store.Execute(t.Context(), invoke, func() (*protocol.RuntimeResultPayload, error) {
		return nil, errors.New("network response lost")
	}); err == nil {
		t.Fatal("unconfirmed operation accepted")
	}
	if got, err := executionjournal.NewStore(db).Inspect(t.Context(), invoke); err != nil || got.State != "started" || got.Result != nil {
		t.Fatalf("unknown operation result leaked: %+v %v", got, err)
	}
	owned := invocation()
	owned.IdempotencyKey = "private-owned-operation"
	scope := coordination.ExecutionScope{CoreID: "core-b", SpaceID: "core-b", ResourceOwnerID: "core-b", TargetDeviceID: "device-a", Coordinated: true}
	owned.OwnedExecutionScope, _ = json.Marshal(scope)
	ctx := coordination.WithScope(t.Context(), scope)
	if _, err := store.Execute(ctx, owned, func() (*protocol.RuntimeResultPayload, error) {
		return &protocol.RuntimeResultPayload{Status: "completed", Result: json.RawMessage(`{"secret":"not-on-device"}`)}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if got, err := executionjournal.NewStore(db).Inspect(ctx, owned); err != nil || got.State != "performed" || got.Result != nil {
		t.Fatalf("owner-private result was disclosed or certified: %+v %v", got, err)
	}
	if _, err := store.Inspect(t.Context(), owned); !errors.Is(err, executionjournal.ErrConflict) {
		t.Fatalf("unowned caller queried coordinated result: %v", err)
	}
}

func TestJournalInspectionHonorsPersistedFencing(t *testing.T) {
	_, store := harness(t)
	invoke := invocation()
	invoke.WorkflowRunID, invoke.WorkflowNodeID, invoke.FencingToken = "w1", "n1", 7
	if _, err := store.Execute(t.Context(), invoke, func() (*protocol.RuntimeResultPayload, error) {
		return &protocol.RuntimeResultPayload{Status: "completed"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	stale := invoke
	stale.FencingToken = 6
	if _, err := store.Inspect(t.Context(), stale); !errors.Is(err, executionjournal.ErrFence) {
		t.Fatalf("stale lease inspected previous effect: %v", err)
	}
	if got, err := store.Inspect(t.Context(), invoke); err != nil || got.State != "completed" {
		t.Fatalf("valid fence lost result: %+v %v", got, err)
	}
}
