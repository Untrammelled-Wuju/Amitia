package executionjournal_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/execution"
)

func ledgerHarness(t *testing.T, conns int) (*sql.DB, *executionjournal.InvocationLedger) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "ledger.db") + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(conns)
	t.Cleanup(func() { db.Close() })
	ledger, err := executionjournal.NewLedger(db)
	if err != nil {
		t.Fatal(err)
	}
	return db, ledger
}

func ledgerScope(space string) executionjournal.LedgerScope {
	return executionjournal.LedgerScope{SpaceID: space, RoleID: "role-main", DeviceID: "device-a", Surface: "desktop"}
}

func ledgerRequest(scope executionjournal.LedgerScope, key string, sideEffect execution.SideEffectClass) executionjournal.LedgerRequest {
	return executionjournal.LedgerRequest{
		Scope:           scope,
		IdempotencyKey:  key,
		Trace:           execution.InvocationTrace{RootExecutionID: "root-1", TraceID: "trace-1", InvocationID: "inv-1"},
		CapabilityID:    "host.tool.execute",
		BindingRevision: "rev-7",
		PayloadHash:     "payload-hash-1",
		SideEffect:      sideEffect,
	}
}

func ledgerResult(value string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"value":%q}`, value))
}

func TestLedgerRejectsInvalidKeys(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	base := ledgerRequest(ledgerScope("space-a"), "k", execution.SideEffectClassWrite)
	revisionless := base
	revisionless.BindingRevision = " "
	payloadless := base
	payloadless.PayloadHash = " "
	cases := []struct {
		name    string
		request executionjournal.LedgerRequest
	}{
		{"empty-space", ledgerRequest(executionjournal.LedgerScope{SpaceID: "  "}, "k", execution.SideEffectClassWrite)},
		{"empty-key", ledgerRequest(ledgerScope("space-a"), "   ", execution.SideEffectClassWrite)},
		{"long-key", ledgerRequest(ledgerScope("space-a"), strings.Repeat("k", 513), execution.SideEffectClassWrite)},
		{"empty-revision", revisionless},
		{"invalid-effect", ledgerRequest(ledgerScope("space-a"), "k", execution.SideEffectClass("mutate"))},
		{"empty-payload", payloadless},
		{"segment-too-big", executionjournal.LedgerRequest{Scope: executionjournal.LedgerScope{SpaceID: "space-a", RoleID: strings.Repeat("r", 257)}, IdempotencyKey: "k", Trace: execution.InvocationTrace{RootExecutionID: "root-1", TraceID: "trace-1", InvocationID: "inv-1"}, CapabilityID: "host.tool.execute", BindingRevision: "rev-7", PayloadHash: "payload-hash-1", SideEffect: execution.SideEffectClassWrite}},
	}
	var calls atomic.Int32
	run := func() (json.RawMessage, error) {
		calls.Add(1)
		return ledgerResult("x"), nil
	}
	for _, item := range cases {
		if _, _, err := ledger.Commit(t.Context(), item.request, run); !errors.Is(err, executionjournal.ErrLedgerKeyInvalid) {
			t.Fatalf("%s: expected ErrLedgerKeyInvalid, got %v", item.name, err)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid requests executed %d times", calls.Load())
	}
	fmt.Printf("m04-s02 ledger invalid-keys rejected=%d executed=0\n", len(cases))
}

func TestLedgerFirstCommitRunsOnceAndReplayReusesResult(t *testing.T) {
	_, ledger := ledgerHarness(t, 4)
	request := ledgerRequest(ledgerScope("space-a"), "idem-1", execution.SideEffectClassWrite)
	var calls atomic.Int32
	run := func() (json.RawMessage, error) {
		calls.Add(1)
		return ledgerResult("once"), nil
	}
	first, ran, err := ledger.Commit(t.Context(), request, run)
	if err != nil || !ran || first.State != executionjournal.EffectCommitted {
		t.Fatalf("first commit: entry=%+v ran=%v err=%v", first, ran, err)
	}
	second, ran, err := ledger.Commit(t.Context(), request, run)
	if err != nil || ran || second.State != executionjournal.EffectCommitted {
		t.Fatalf("replay: entry=%+v ran=%v err=%v", second, ran, err)
	}
	if string(first.Result) != string(second.Result) || string(second.Result) != `{"value":"once"}` {
		t.Fatalf("replay result mismatch: %s vs %s", first.Result, second.Result)
	}
	if calls.Load() != 1 {
		t.Fatalf("side effect executed %d times", calls.Load())
	}
	fmt.Printf("m04-s02 ledger replay-runs-once executions=%d state=%s result=%s\n", calls.Load(), second.State, second.Result)
}

func TestLedgerDefiniteFailureReleasesReservation(t *testing.T) {
	_, ledger := ledgerHarness(t, 4)
	request := ledgerRequest(ledgerScope("space-a"), "idem-retry", execution.SideEffectClassRead)
	boom := errors.New("runtime refused the call")
	var calls atomic.Int32
	failing := func() (json.RawMessage, error) {
		calls.Add(1)
		return nil, boom
	}
	if _, ran, err := ledger.Commit(t.Context(), request, failing); err == nil || !ran || !errors.Is(err, boom) {
		t.Fatalf("first attempt: ran=%v err=%v", ran, err)
	}
	if _, found, err := ledger.Lookup(t.Context(), request.Scope, request.IdempotencyKey); err != nil || found {
		t.Fatalf("reservation survived definite failure: found=%v err=%v", found, err)
	}
	recovered, ran, err := ledger.Commit(t.Context(), request, func() (json.RawMessage, error) {
		calls.Add(1)
		return ledgerResult("second"), nil
	})
	if err != nil || !ran || recovered.State != executionjournal.EffectCommitted {
		t.Fatalf("retry after definite failure: entry=%+v ran=%v err=%v", recovered, ran, err)
	}
	if calls.Load() != 2 {
		t.Fatalf("attempts = %d", calls.Load())
	}
	fmt.Printf("m04-s02 ledger retry-after-failure attempts=%d state=%s\n", calls.Load(), recovered.State)
}

func TestLedgerConcurrentSameKeyRunsOnce(t *testing.T) {
	_, ledger := ledgerHarness(t, 12)
	request := ledgerRequest(ledgerScope("space-a"), "idem-concurrent", execution.SideEffectClassWrite)
	var calls atomic.Int32
	run := func() (json.RawMessage, error) {
		calls.Add(1)
		return ledgerResult("shared"), nil
	}
	const workers = 16
	var wait sync.WaitGroup
	results := make([]string, workers)
	errs := make([]error, workers)
	ranFlags := make([]bool, workers)
	wait.Add(workers)
	for index := 0; index < workers; index++ {
		go func(slot int) {
			defer wait.Done()
			entry, ran, err := ledger.Commit(context.Background(), request, run)
			results[slot] = string(entry.Result)
			ranFlags[slot] = ran
			errs[slot] = err
		}(index)
	}
	wait.Wait()
	for slot := 0; slot < workers; slot++ {
		if errs[slot] != nil {
			t.Fatalf("worker %d: %v", slot, errs[slot])
		}
		if results[slot] != `{"value":"shared"}` {
			t.Fatalf("worker %d result %q", slot, results[slot])
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("concurrent commits executed side effect %d times", calls.Load())
	}
	owners := 0
	for _, ran := range ranFlags {
		if ran {
			owners++
		}
	}
	if owners != 1 {
		t.Fatalf("expected exactly one executing caller, got %d", owners)
	}
	fmt.Printf("m04-s02 ledger concurrent workers=%d executions=%d leaders=%d\n", workers, calls.Load(), owners)
}

func TestLedgerSameKeyDifferentScopeIsIndependent(t *testing.T) {
	db, ledger := ledgerHarness(t, 4)
	var calls atomic.Int32
	run := func() (json.RawMessage, error) {
		calls.Add(1)
		return ledgerResult("scoped"), nil
	}
	first := ledgerRequest(ledgerScope("space-a"), "idem-shared", execution.SideEffectClassWrite)
	second := ledgerRequest(ledgerScope("space-b"), "idem-shared", execution.SideEffectClassWrite)
	if _, ran, err := ledger.Commit(t.Context(), first, run); err != nil || !ran {
		t.Fatalf("scope a: ran=%v err=%v", ran, err)
	}
	if _, ran, err := ledger.Commit(t.Context(), second, run); err != nil || !ran {
		t.Fatalf("scope b: ran=%v err=%v", ran, err)
	}
	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_invocation_ledger`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || rows != 2 {
		t.Fatalf("executions=%d rows=%d", calls.Load(), rows)
	}
	fmt.Printf("m04-s02 ledger cross-scope executions=%d rows=%d\n", calls.Load(), rows)
}

func TestLedgerPayloadChangeRejected(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	request := ledgerRequest(ledgerScope("space-a"), "idem-payload", execution.SideEffectClassWrite)
	var calls atomic.Int32
	run := func() (json.RawMessage, error) {
		calls.Add(1)
		return ledgerResult("first"), nil
	}
	if _, _, err := ledger.Commit(t.Context(), request, run); err != nil {
		t.Fatal(err)
	}
	changed := request
	changed.PayloadHash = "payload-hash-2"
	if _, _, err := ledger.Commit(t.Context(), changed, run); !errors.Is(err, executionjournal.ErrLedgerPayloadConflict) {
		t.Fatalf("expected payload conflict, got %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("executions=%d", calls.Load())
	}
	fmt.Printf("m04-s02 ledger payload-conflict executions=%d conflict=%v\n", calls.Load(), executionjournal.ErrLedgerPayloadConflict)
}

func TestLedgerBindingRevisionFrozen(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	request := ledgerRequest(ledgerScope("space-a"), "idem-revision", execution.SideEffectClassWrite)
	var calls atomic.Int32
	run := func() (json.RawMessage, error) {
		calls.Add(1)
		return ledgerResult("frozen"), nil
	}
	entry, _, err := ledger.Commit(t.Context(), request, run)
	if err != nil {
		t.Fatal(err)
	}
	upgraded := request
	upgraded.BindingRevision = "rev-8"
	if _, _, err := ledger.Commit(t.Context(), upgraded, run); !errors.Is(err, executionjournal.ErrLedgerBindingFrozen) {
		t.Fatalf("expected frozen revision error, got %v", err)
	}
	stored, found, err := ledger.Lookup(t.Context(), request.Scope, request.IdempotencyKey)
	if err != nil || !found || stored.BindingRevision != "rev-7" || entry.BindingRevision != "rev-7" {
		t.Fatalf("frozen revision drifted: stored=%+v entry=%+v err=%v", stored, entry, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("executions=%d", calls.Load())
	}
	fmt.Printf("m04-s02 ledger binding-frozen revision=%s executions=%d\n", stored.BindingRevision, calls.Load())
}

func TestLedgerUnknownOutcomeBlocksRetry(t *testing.T) {
	_, ledger := ledgerHarness(t, 4)
	request := ledgerRequest(ledgerScope("space-a"), "idem-unknown", execution.SideEffectClassWrite)
	var calls atomic.Int32
	timeout := func() (json.RawMessage, error) {
		calls.Add(1)
		return nil, context.DeadlineExceeded
	}
	entry, ran, err := ledger.Commit(t.Context(), request, timeout)
	if !ran || !errors.Is(err, execution.ErrInvocationOutcomeUnknown) || entry.State != executionjournal.EffectUnknown {
		t.Fatalf("unknown outcome: entry=%+v ran=%v err=%v", entry, ran, err)
	}
	var unknown *execution.UnknownInvocationOutcomeError
	if !errors.As(err, &unknown) || unknown.RetrySafe() {
		t.Fatalf("unknown error must not be retry safe: %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unknown error lost its cause: %v", err)
	}
	if _, ran, err := ledger.Commit(t.Context(), request, timeout); !errors.Is(err, executionjournal.ErrLedgerEffectUnknown) || ran {
		t.Fatalf("retry after unknown: ran=%v err=%v", ran, err)
	}
	stored, found, lookupErr := ledger.Lookup(t.Context(), request.Scope, request.IdempotencyKey)
	if lookupErr != nil || !found || stored.State != executionjournal.EffectUnknown || calls.Load() != 1 {
		t.Fatalf("unknown state not persisted: stored=%+v found=%v executions=%d err=%v", stored, found, calls.Load(), lookupErr)
	}
	fmt.Printf("m04-s02 ledger unknown-blocks-retry executions=%d state=%s retrySafe=%v\n", calls.Load(), stored.State, unknown.RetrySafe())
}

func TestLedgerCommittedSurvivesRestart(t *testing.T) {
	db, ledger := ledgerHarness(t, 4)
	request := ledgerRequest(ledgerScope("space-a"), "idem-restart", execution.SideEffectClassWrite)
	var calls atomic.Int32
	run := func() (json.RawMessage, error) {
		calls.Add(1)
		return ledgerResult("durable"), nil
	}
	if _, _, err := ledger.Commit(t.Context(), request, run); err != nil {
		t.Fatal(err)
	}
	restarted, err := executionjournal.NewLedger(db)
	if err != nil {
		t.Fatal(err)
	}
	entry, ran, err := restarted.Commit(t.Context(), request, run)
	if err != nil || ran || entry.State != executionjournal.EffectCommitted || string(entry.Result) != `{"value":"durable"}` {
		t.Fatalf("restart replay: entry=%+v ran=%v err=%v", entry, ran, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("executions=%d", calls.Load())
	}
	fmt.Printf("m04-s02 ledger restart-replay executions=%d state=%s\n", calls.Load(), entry.State)
}
