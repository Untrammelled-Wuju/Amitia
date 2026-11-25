package executionjournal_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

const (
	matrixReadCap  = "memory.search"
	matrixReadPerm = "memory.read/low"
	matrixPluginID = "plugins-code/emote/memory"
	matrixCoreID   = "builtin.core/core/memory"
)

type faultRow struct {
	Name       string
	State      string
	Fallback   string
	Executions int
	Chain      string
}

var faultTruthTable = []faultRow{
	{Name: "read-panic", State: "read", Fallback: "core", Executions: 2, Chain: "panic"},
	{Name: "read-permission", State: "read", Fallback: "none", Executions: 1, Chain: "permission"},
	{Name: "read-timeout", State: "read", Fallback: "core", Executions: 2, Chain: "deadline"},
	{Name: "write-panic", State: "unknown", Fallback: "none", Executions: 1, Chain: "panic"},
	{Name: "write-timeout", State: "unknown", Fallback: "none", Executions: 1, Chain: "deadline"},
	{Name: "lost-ack", State: "needs_reconciliation", Fallback: "none", Executions: 1, Chain: "deadline"},
	{Name: "crash-after-commit", State: "committed", Fallback: "none", Executions: 1, Chain: "none"},
	{Name: "db-busy", State: "absent", Fallback: "none", Executions: 0, Chain: "busy"},
}

func faultTruthRow(name string) (faultRow, bool) {
	for _, row := range faultTruthTable {
		if row.Name == name {
			return row, true
		}
	}
	return faultRow{}, false
}

func emitFaultRow(t *testing.T, actual faultRow) {
	t.Helper()
	want, found := faultTruthRow(actual.Name)
	if !found {
		t.Fatalf("故障真值表缺少 case=%s", actual.Name)
	}
	if actual != want {
		t.Fatalf("故障真值表不匹配 case=%s got=%+v want=%+v", actual.Name, actual, want)
	}
	fmt.Printf("m04-s07 matrix %s state=%s fallback=%s executions=%d chain=%s\n",
		actual.Name, actual.State, actual.Fallback, actual.Executions, actual.Chain)
}

func matrixOpen(t *testing.T, path, busyTimeout string) (*sql.DB, *executionjournal.InvocationLedger) {
	t.Helper()
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(" + busyTimeout + ")"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	ledger, err := executionjournal.NewLedger(db)
	if err != nil {
		t.Fatalf("new ledger: %v", err)
	}
	return db, ledger
}

func matrixLedger(t *testing.T, busyTimeout string) (string, *sql.DB, *executionjournal.InvocationLedger) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "matrix.db")
	db, ledger := matrixOpen(t, path, busyTimeout)
	return path, db, ledger
}

func matrixScope() executionjournal.LedgerScope {
	return executionjournal.LedgerScope{SpaceID: "space-a", RoleID: "role-main", DeviceID: "device-a", Surface: "desktop"}
}

func matrixTrace() execution.InvocationTrace {
	return execution.InvocationTrace{RootExecutionID: "root-s07", TraceID: "trace-s07", InvocationID: "inv-s07"}
}

func matrixWriteRequest(t *testing.T, key string) executionjournal.LedgerRequest {
	t.Helper()
	request := executionjournal.LedgerRequest{
		Scope:           matrixScope(),
		IdempotencyKey:  key,
		Trace:           matrixTrace(),
		CapabilityID:    "host.tool.execute",
		BindingRevision: "rev-7",
		PayloadHash:     "payload-s07",
		SideEffect:      execution.SideEffectClassWrite,
	}
	if err := request.Trace.Validate(); err != nil {
		t.Fatalf("trace: %v", err)
	}
	return request
}

func matrixReadRequest(t *testing.T) plugincontract.ReadFallbackRequest {
	t.Helper()
	contract, err := execution.NewInvocationContract(matrixTrace(), matrixReadCap, execution.SideEffectClassRead, "")
	if err != nil {
		t.Fatalf("contract: %v", err)
	}
	return plugincontract.ReadFallbackRequest{
		Contract:   contract,
		Permission: matrixReadPerm,
		Scope:      plugincontract.ScopeRequirement{plugincontract.ScopeDimensionSpace: "space-a"},
	}
}

func matrixCandidate(id string, core bool, invoke func(context.Context) (json.RawMessage, error)) plugincontract.ReadFallbackCandidate {
	return plugincontract.ReadFallbackCandidate{
		ProviderID:         id,
		Core:               core,
		GrantedPermissions: []string{matrixReadPerm},
		Claims:             plugincontract.ScopeClaims{plugincontract.ScopeDimensionSpace: "space-a"},
		Invoke:             invoke,
	}
}

func TestFaultMatrixReadPanic(t *testing.T) {
	request := matrixReadRequest(t)
	var pluginCalls, coreCalls int32
	candidates := []plugincontract.ReadFallbackCandidate{
		matrixCandidate(matrixPluginID, false, func(context.Context) (json.RawMessage, error) {
			atomic.AddInt32(&pluginCalls, 1)
			panic("plugin process crashed: SIGSEGV at 0x7f3a")
		}),
		matrixCandidate(matrixCoreID, true, func(context.Context) (json.RawMessage, error) {
			atomic.AddInt32(&coreCalls, 1)
			return json.RawMessage(`{"items":["core-result"],"source":"builtin.core"}`), nil
		}),
	}
	result, err := plugincontract.InvokeReadWithFallback(t.Context(), request, candidates, plugincontract.ReadFallbackOptions{})
	if err != nil {
		t.Fatalf("read panic must fall back to core: %v", err)
	}
	if !result.Fallback || !result.Core || result.ProviderID != matrixCoreID {
		t.Fatalf("fallback=%v core=%v provider=%q", result.Fallback, result.Core, result.ProviderID)
	}
	if string(result.Output) != `{"items":["core-result"],"source":"builtin.core"}` {
		t.Fatalf("core output=%s", result.Output)
	}
	var panicErr *plugincontract.ReadFallbackPanicError
	if !errors.As(result.Failure(), &panicErr) {
		t.Fatalf("panic error missing from attempts: %v", result.Failure())
	}
	if !strings.Contains(panicErr.Value, "SIGSEGV") || !strings.Contains(panicErr.Stack, "goroutine ") {
		t.Fatalf("panic detail lost: value=%q stack=%q", panicErr.Value, panicErr.Stack)
	}
	if !strings.Contains(panicErr.Error(), "plugin process crashed") {
		t.Fatalf("panic text rewritten: %s", panicErr.Error())
	}
	emitFaultRow(t, faultRow{Name: "read-panic", State: "read", Fallback: "core", Executions: int(pluginCalls + coreCalls), Chain: "panic"})
}

func TestFaultMatrixReadPermissionDenied(t *testing.T) {
	request := matrixReadRequest(t)
	var pluginCalls, coreCalls int32
	denied := fmt.Errorf("插件拒绝 capability=%q: %w", matrixReadCap, plugincontract.ErrReadFallbackPermissionDenied)
	candidates := []plugincontract.ReadFallbackCandidate{
		matrixCandidate(matrixPluginID, false, func(context.Context) (json.RawMessage, error) {
			atomic.AddInt32(&pluginCalls, 1)
			return nil, denied
		}),
		matrixCandidate(matrixCoreID, true, func(context.Context) (json.RawMessage, error) {
			atomic.AddInt32(&coreCalls, 1)
			return json.RawMessage(`{"items":["core-result"]}`), nil
		}),
	}
	result, err := plugincontract.InvokeReadWithFallback(t.Context(), request, candidates, plugincontract.ReadFallbackOptions{})
	if err == nil {
		t.Fatalf("permission denial must abort the fallback chain")
	}
	if !errors.Is(err, plugincontract.ErrReadFallbackPermissionDenied) {
		t.Fatalf("permission sentinel lost: %v", err)
	}
	if !strings.Contains(err.Error(), denied.Error()) {
		t.Fatalf("original denial text rewritten: %s", err.Error())
	}
	if coreCalls != 0 {
		t.Fatalf("core executed after permission denial: %d", coreCalls)
	}
	if len(result.Attempts) != 1 || !result.Attempts[0].Executed {
		t.Fatalf("attempts=%+v", result.Attempts)
	}
	emitFaultRow(t, faultRow{Name: "read-permission", State: "read", Fallback: "none", Executions: int(pluginCalls + coreCalls), Chain: "permission"})
}

func TestFaultMatrixReadTimeout(t *testing.T) {
	request := matrixReadRequest(t)
	var pluginCalls, coreCalls int32
	deadline := fmt.Errorf("插件读超时: %w", context.DeadlineExceeded)
	candidates := []plugincontract.ReadFallbackCandidate{
		matrixCandidate(matrixPluginID, false, func(context.Context) (json.RawMessage, error) {
			atomic.AddInt32(&pluginCalls, 1)
			return nil, deadline
		}),
		matrixCandidate(matrixCoreID, true, func(context.Context) (json.RawMessage, error) {
			atomic.AddInt32(&coreCalls, 1)
			return json.RawMessage(`{"items":["core-result"]}`), nil
		}),
	}
	result, err := plugincontract.InvokeReadWithFallback(t.Context(), request, candidates, plugincontract.ReadFallbackOptions{})
	if err != nil {
		t.Fatalf("read timeout must fall back to core: %v", err)
	}
	if !result.Fallback || !result.Core || coreCalls != 1 {
		t.Fatalf("fallback=%v core=%v coreCalls=%d", result.Fallback, result.Core, coreCalls)
	}
	failure := result.Failure()
	if !errors.Is(failure, context.DeadlineExceeded) || !strings.Contains(failure.Error(), "插件读超时") {
		t.Fatalf("deadline evidence lost: %v", failure)
	}
	emitFaultRow(t, faultRow{Name: "read-timeout", State: "read", Fallback: "core", Executions: int(pluginCalls + coreCalls), Chain: "deadline"})
}

func TestFaultMatrixWritePanic(t *testing.T) {
	_, _, ledger := matrixLedger(t, "5000")
	request := matrixWriteRequest(t, "fault-write-panic")
	var executions int32
	run := func() (json.RawMessage, error) {
		atomic.AddInt32(&executions, 1)
		panic("plugin process crashed: runtime error: invalid memory address")
	}
	entry, _, err := ledger.Commit(t.Context(), request, run)
	if err == nil {
		t.Fatalf("panic must surface as an error")
	}
	if entry.State != executionjournal.EffectUnknown {
		t.Fatalf("state=%s", entry.State)
	}
	if !errors.Is(err, execution.ErrInvocationOutcomeUnknown) {
		t.Fatalf("panic must classify as unknown: %v", err)
	}
	var panicErr *executionjournal.RunPanicError
	if !errors.As(err, &panicErr) {
		t.Fatalf("panic detail lost from error chain: %v", err)
	}
	if !strings.Contains(panicErr.Value, "invalid memory address") || !strings.Contains(panicErr.Stack, "goroutine ") {
		t.Fatalf("panic detail truncated: value=%q stack=%q", panicErr.Value, panicErr.Stack)
	}
	if !strings.Contains(err.Error(), "plugin process crashed") {
		t.Fatalf("panic text rewritten: %s", err.Error())
	}
	_, _, replayErr := ledger.Commit(t.Context(), request, run)
	if replayErr == nil {
		t.Fatalf("unknown outcome must block replay")
	}
	if !errors.Is(replayErr, executionjournal.ErrLedgerEffectUnknown) {
		t.Fatalf("replay error=%v", replayErr)
	}
	if executions != 1 {
		t.Fatalf("side effect replayed after panic: %d", executions)
	}
	stored, found, lookupErr := ledger.Lookup(t.Context(), matrixScope(), request.IdempotencyKey)
	if lookupErr != nil || !found || stored.State != executionjournal.EffectUnknown || stored.ErrorCode != "unknown" {
		t.Fatalf("stored entry=%+v found=%v err=%v", stored, found, lookupErr)
	}
	emitFaultRow(t, faultRow{Name: "write-panic", State: "unknown", Fallback: "none", Executions: 1, Chain: "panic"})
}

func TestFaultMatrixWriteTimeout(t *testing.T) {
	_, _, ledger := matrixLedger(t, "5000")
	request := matrixWriteRequest(t, "fault-write-timeout")
	var executions int32
	timeout := fmt.Errorf("插件调用超时: %w", context.DeadlineExceeded)
	run := func() (json.RawMessage, error) {
		atomic.AddInt32(&executions, 1)
		return nil, timeout
	}
	entry, _, err := ledger.Commit(t.Context(), request, run)
	if err == nil {
		t.Fatalf("timeout must surface as an error")
	}
	if entry.State != executionjournal.EffectUnknown {
		t.Fatalf("state=%s", entry.State)
	}
	var unknown *execution.UnknownInvocationOutcomeError
	if !errors.As(err, &unknown) || unknown.RetrySafe() {
		t.Fatalf("timeout must be non-retryable: %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), timeout.Error()) {
		t.Fatalf("original deadline error lost: %v", err)
	}
	_, _, replayErr := ledger.Commit(t.Context(), request, run)
	if replayErr == nil || executions != 1 {
		t.Fatalf("timeout replay executed=%d err=%v", executions, replayErr)
	}
	emitFaultRow(t, faultRow{Name: "write-timeout", State: "unknown", Fallback: "none", Executions: 1, Chain: "deadline"})
}

func TestFaultMatrixLostAck(t *testing.T) {
	_, _, ledger := matrixLedger(t, "5000")
	request := executionjournal.IrreversibleRequest{
		Scope:        matrixScope(),
		OperationID:  "op-s07-ack",
		Trace:        matrixTrace(),
		CapabilityID: "im.send",
		ProviderID:   "plugins-im/wechat/main",
		SideEffect:   execution.SideEffectClassExternal,
		PayloadHash:  "payload-s07",
	}
	var dispatches int32
	lost := fmt.Errorf("消息已发送但回执丢失: %w", context.DeadlineExceeded)
	dispatch := func() error {
		atomic.AddInt32(&dispatches, 1)
		return lost
	}
	record, err := ledger.DispatchIrreversible(t.Context(), request, dispatch)
	if err == nil {
		t.Fatalf("lost ack must surface as an error")
	}
	if record.State != executionjournal.ReconciliationNeedsCheck || !record.NeedsHumanCheck() {
		t.Fatalf("state=%s", record.State)
	}
	if record.LastError != lost.Error() {
		t.Fatalf("last error rewritten: %q", record.LastError)
	}
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, executionjournal.ErrReconciliationPending) {
		t.Fatalf("error chain=%v", err)
	}
	second, secondErr := ledger.DispatchIrreversible(t.Context(), request, dispatch)
	if secondErr == nil || !errors.Is(secondErr, executionjournal.ErrReconciliationPending) {
		t.Fatalf("second dispatch must be blocked: %v", secondErr)
	}
	if second.State != executionjournal.ReconciliationNeedsCheck || dispatches != 1 {
		t.Fatalf("state=%s dispatches=%d", second.State, dispatches)
	}
	open, openErr := ledger.OpenReconciliations(t.Context(), nil)
	if openErr != nil || len(open) != 1 || open[0].OperationID != request.OperationID {
		t.Fatalf("open=%+v err=%v", open, openErr)
	}
	emitFaultRow(t, faultRow{Name: "lost-ack", State: "needs_reconciliation", Fallback: "none", Executions: int(dispatches), Chain: "deadline"})
}

func TestFaultMatrixCrashAfterCommit(t *testing.T) {
	path, _, ledger := matrixLedger(t, "5000")
	request := matrixWriteRequest(t, "fault-crash-commit")
	var executions int32
	entry, ran, err := ledger.Commit(t.Context(), request, func() (json.RawMessage, error) {
		atomic.AddInt32(&executions, 1)
		return json.RawMessage(`{"value":"committed-before-crash"}`), nil
	})
	if err != nil || !ran {
		t.Fatalf("commit ran=%v err=%v", ran, err)
	}
	if entry.State != executionjournal.EffectCommitted || string(entry.Result) != `{"value":"committed-before-crash"}` {
		t.Fatalf("entry=%+v", entry)
	}
	_, restarted := matrixOpen(t, path, "5000")
	replayed, ranAgain, replayErr := restarted.Commit(t.Context(), request, func() (json.RawMessage, error) {
		t.Fatalf("crash replay must not execute the side effect again")
		return nil, nil
	})
	if replayErr != nil || ranAgain {
		t.Fatalf("replay ran=%v err=%v", ranAgain, replayErr)
	}
	if replayed.State != executionjournal.EffectCommitted || string(replayed.Result) != `{"value":"committed-before-crash"}` {
		t.Fatalf("replayed=%+v", replayed)
	}
	if executions != 1 {
		t.Fatalf("executions=%d", executions)
	}
	emitFaultRow(t, faultRow{Name: "crash-after-commit", State: "committed", Fallback: "none", Executions: 1, Chain: "none"})
}

func TestFaultMatrixDBBusy(t *testing.T) {
	path, db, ledger := matrixLedger(t, "0")
	const probe = `INSERT INTO kernel_invocation_ledger(scope_key,idempotency_key,invocation_id,capability_id,binding_revision,payload_hash,effect_state,result,error_code,updated_at) VALUES('probe','probe','i','c','r','p','pending',NULL,'','2026-11-24T00:00:00Z')`
	locker, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(0)")
	if err != nil {
		t.Fatalf("locker open: %v", err)
	}
	locker.SetMaxOpenConns(1)
	t.Cleanup(func() { locker.Close() })
	tx, err := locker.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("locker begin: %v", err)
	}
	t.Cleanup(func() { tx.Rollback() })
	if _, err := tx.ExecContext(t.Context(), probe); err != nil {
		t.Fatalf("lock holder insert: %v", err)
	}
	prober, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(0)")
	if err != nil {
		t.Fatalf("probe open: %v", err)
	}
	prober.SetMaxOpenConns(1)
	t.Cleanup(func() { prober.Close() })
	if _, probeErr := prober.ExecContext(t.Context(), probe); probeErr == nil {
		t.Fatalf("probe insert should be blocked by the write lock")
	} else if canonical := probeErr.Error(); canonical == "" {
		t.Fatalf("probe insert produced an empty error")
	} else {
		request := matrixWriteRequest(t, "fault-db-busy")
		var executions int32
		entry, ran, commitErr := ledger.Commit(t.Context(), request, func() (json.RawMessage, error) {
			atomic.AddInt32(&executions, 1)
			return json.RawMessage(`{"value":"must-not-run"}`), nil
		})
		if commitErr == nil {
			t.Fatalf("busy write must fail")
		}
		if ran || executions != 0 {
			t.Fatalf("side effect executed under db busy: ran=%v executions=%d", ran, executions)
		}
		if entry.State != "" {
			t.Fatalf("entry=%+v", entry)
		}
		if !strings.Contains(commitErr.Error(), canonical) {
			t.Fatalf("busy error rewritten: got=%q want substring=%q", commitErr.Error(), canonical)
		}
		var rows int
		if err := db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM kernel_invocation_ledger`).Scan(&rows); err != nil {
			t.Fatalf("count rows: %v", err)
		}
		if rows != 0 {
			t.Fatalf("ledger rows=%d", rows)
		}
	}
	emitFaultRow(t, faultRow{Name: "db-busy", State: "absent", Fallback: "none", Executions: 0, Chain: "busy"})
}

func TestFaultMatrixTruthTable(t *testing.T) {
	if len(faultTruthTable) != 8 {
		t.Fatalf("truth table rows=%d", len(faultTruthTable))
	}
	seen := make(map[string]bool, len(faultTruthTable))
	for _, row := range faultTruthTable {
		if row.Name == "" || row.State == "" || row.Fallback == "" || row.Chain == "" {
			t.Fatalf("incomplete row: %+v", row)
		}
		if seen[row.Name] {
			t.Fatalf("duplicate case: %s", row.Name)
		}
		seen[row.Name] = true
	}
	for _, name := range []string{"read-panic", "read-permission", "read-timeout", "write-panic", "write-timeout", "lost-ack", "crash-after-commit", "db-busy"} {
		if !seen[name] {
			t.Fatalf("truth table missing case: %s", name)
		}
	}
	fmt.Printf("m04-s07 matrix-truth cases=%d complete=true\n", len(faultTruthTable))
}
