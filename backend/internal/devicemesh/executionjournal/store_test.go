package executionjournal_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

func harness(t *testing.T) (*sql.DB, *executionjournal.Store) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "journal.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if err := kernelsqlite.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	return db, executionjournal.NewStore(db)
}

func invocation() protocol.RuntimeInvokePayload {
	return protocol.RuntimeInvokePayload{InvocationID: "invoke", SpaceID: "core-b", DeviceID: "device-a", Handler: "device.action", IdempotencyKey: "action", Input: json.RawMessage(`{"text":"hello"}`)}
}

func TestCompletedActionSurvivesRestartAndCannotBeReplayedFromAnotherCore(t *testing.T) {
	db, store := harness(t)
	var calls atomic.Int32
	run := func() (*protocol.RuntimeResultPayload, error) {
		calls.Add(1)
		return &protocol.RuntimeResultPayload{Status: "completed", Result: json.RawMessage(`{"ok":true}`)}, nil
	}
	invoke := invocation()
	if _, err := store.Execute(t.Context(), invoke, run); err != nil {
		t.Fatal(err)
	}
	store = executionjournal.NewStore(db)
	if result, err := store.Execute(t.Context(), invoke, run); err != nil || result.Status != "completed" {
		t.Fatalf("completed action: %+v %v", result, err)
	}
	invoke.SpaceID = "core-c"
	if _, err := store.Execute(t.Context(), invoke, run); !errors.Is(err, executionjournal.ErrConflict) {
		t.Fatalf("provider replay accepted: %v", err)
	}
	invoke = invocation()
	invoke.Input = json.RawMessage(`{"text":"different"}`)
	if _, err := store.Execute(t.Context(), invoke, run); !errors.Is(err, executionjournal.ErrConflict) {
		t.Fatalf("changed action accepted: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("action executed %d times", calls.Load())
	}
}

func TestUncertainActionCannotBeRetriedAfterProcessRestart(t *testing.T) {
	db, store := harness(t)
	invoke := invocation()
	var calls int
	run := func() (*protocol.RuntimeResultPayload, error) {
		calls++
		return nil, errors.New("external action may have succeeded before disconnect")
	}
	if _, err := store.Execute(t.Context(), invoke, run); err == nil {
		t.Fatal("failed action accepted")
	}
	if _, err := executionjournal.NewStore(db).Execute(t.Context(), invoke, run); !errors.Is(err, executionjournal.ErrUncertain) {
		t.Fatalf("unknown action retried: %v", err)
	}
	if calls != 1 {
		t.Fatal("uncertain action repeated")
	}
}

func TestOwnedActionScopeCannotReusePreviousRoleOrPermissionResult(t *testing.T) {
	_, store := harness(t)
	invoke := invocation()
	invoke.OwnedExecutionScope = json.RawMessage(`{"coreId":"core-b","roleId":"role-a","permissionRevision":1}`)
	calls := 0
	run := func() (*protocol.RuntimeResultPayload, error) {
		calls++
		return &protocol.RuntimeResultPayload{Status: "success", Result: json.RawMessage(`{"ok":true}`)}, nil
	}
	if _, err := store.Execute(t.Context(), invoke, run); err != nil {
		t.Fatal(err)
	}
	invoke.AuthorityCallID = "new-authority"
	if _, err := store.Execute(t.Context(), invoke, run); err != nil || calls != 1 {
		t.Fatalf("same action repeated: %d %v", calls, err)
	}
	for _, scope := range []string{`{"coreId":"core-b","roleId":"role-b","permissionRevision":1}`, `{"coreId":"core-b","roleId":"role-a","permissionRevision":2}`} {
		invoke.OwnedExecutionScope = json.RawMessage(scope)
		if _, err := store.Execute(t.Context(), invoke, run); !errors.Is(err, executionjournal.ErrConflict) || calls != 1 {
			t.Fatalf("scope collision returned stale result or executed again: %d %v", calls, err)
		}
	}
}

func TestPersistedFenceRejectsObsoleteAttemptAndSupersededCompletion(t *testing.T) {
	db, store := harness(t)
	invoke := invocation()
	invoke.WorkflowRunID = "workflow"
	invoke.WorkflowNodeID = "node"
	invoke.FencingToken = 2
	run := func() (*protocol.RuntimeResultPayload, error) {
		return &protocol.RuntimeResultPayload{Result: json.RawMessage(`{}`)}, nil
	}
	if _, err := store.Execute(t.Context(), invoke, run); err != nil {
		t.Fatal(err)
	}
	old := invoke
	old.IdempotencyKey = "old-attempt"
	old.FencingToken = 1
	if _, err := executionjournal.NewStore(db).Execute(t.Context(), old, run); !errors.Is(err, executionjournal.ErrFence) {
		t.Fatalf("old lease executed: %v", err)
	}
	newer := invoke
	newer.IdempotencyKey = "new-attempt"
	newer.FencingToken = 3
	if _, err := store.Execute(t.Context(), newer, func() (*protocol.RuntimeResultPayload, error) {
		if _, err := db.Exec(`UPDATE kernel_device_execution_fences SET token=4`); err != nil {
			t.Fatal(err)
		}
		return run()
	}); !errors.Is(err, executionjournal.ErrFence) {
		t.Fatalf("superseded result committed: %v", err)
	}
	if _, err := executionjournal.NewStore(db).Execute(t.Context(), newer, run); !errors.Is(err, executionjournal.ErrUncertain) {
		t.Fatalf("superseded action retried: %v", err)
	}
	if _, err := executionjournal.NewStore(db).Execute(t.Context(), invoke, run); !errors.Is(err, executionjournal.ErrFence) {
		t.Fatalf("obsolete cached result accepted: %v", err)
	}
}

func TestCoordinatedResultRemainsOnlyInMemoryAndCannotBeReexecutedAfterRestart(t *testing.T) {
	db, store := harness(t)
	invoke := invocation()
	scope := coordination.ExecutionScope{CoreID: "core-b", SpaceID: "core-b", ResourceOwnerID: "core-b", TargetDeviceID: "device-a", Coordinated: true}
	invoke.OwnedExecutionScope, _ = json.Marshal(scope)
	ctx := coordination.WithScope(t.Context(), scope)
	var calls atomic.Int32
	run := func() (*protocol.RuntimeResultPayload, error) {
		calls.Add(1)
		return &protocol.RuntimeResultPayload{Status: "completed", Result: json.RawMessage(`{"private":"core-only"}`)}, nil
	}
	result, err := store.Execute(ctx, invoke, run)
	if err != nil || result == nil || string(result.Result) != `{"private":"core-only"}` {
		t.Fatalf("live result lost: %+v %v", result, err)
	}
	var status string
	var body []byte
	if err := db.QueryRow(`SELECT status,result FROM kernel_device_execution_journal WHERE device_id=? AND action_id=?`, "device-a", "action").Scan(&status, &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 0 || status != "performed" {
		t.Fatalf("Core-owned body stored on device: %q %s", body, status)
	}
	if _, err := executionjournal.NewStore(db).Execute(ctx, invoke, run); !errors.Is(err, executionjournal.ErrUncertain) || calls.Load() != 1 {
		t.Fatalf("unconfirmed result replayed after restart: %d %v", calls.Load(), err)
	}
}
