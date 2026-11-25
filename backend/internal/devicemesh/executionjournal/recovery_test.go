package executionjournal_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/execution"
)

func recoveryLog(t *testing.T, line string) {
	t.Helper()
	t.Logf("m04-s12 %s", line)
}

func pendingRow(t *testing.T, db *sql.DB, request executionjournal.LedgerRequest, state executionjournal.EffectState) {
	t.Helper()
	scopeKey, err := request.Scope.Key()
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(
		`INSERT INTO kernel_invocation_ledger(scope_key,idempotency_key,invocation_id,capability_id,binding_revision,payload_hash,effect_state,result,error_code,updated_at)
		 VALUES(?,?,?,?,?,?,?,NULL,'',?)`,
		scopeKey, request.IdempotencyKey, request.Trace.InvocationID, request.CapabilityID,
		request.BindingRevision, request.PayloadHash, string(state), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
}

func recoveryCount(t *testing.T, db *sql.DB, scope executionjournal.LedgerScope, key string) int {
	t.Helper()
	scopeKey, err := scope.Key()
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(1) FROM kernel_invocation_recovery WHERE scope_key=? AND idempotency_key=?`, scopeKey, key).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestJournalRecoveryPendingBecomesReconcile(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-recovery")
	request := ledgerRequest(scope, "key-pending", execution.SideEffectClassWrite)
	pendingRow(t, db, request, executionjournal.EffectPending)

	report, err := ledger.RecoverInFlight(context.Background(), scope, 0)
	if err != nil {
		t.Fatalf("RecoverInFlight: %v", err)
	}
	if report.Scanned != 1 || len(report.Recovered) != 1 {
		t.Fatalf("扫描=%d 恢复=%d 期望各 1", report.Scanned, len(report.Recovered))
	}
	record := report.Recovered[0]
	if record.Decision != executionjournal.RecoveryReconcile {
		t.Fatalf("decision=%q", record.Decision)
	}
	if record.Reexecute() {
		t.Fatalf("恢复后仍允许重新执行")
	}
	if !report.ManualOnly() {
		t.Fatalf("恢复结论必须全部为人工核对")
	}
	stored, found, err := ledger.InspectRecovery(context.Background(), scope, "key-pending")
	if err != nil || !found {
		t.Fatalf("InspectRecovery found=%v err=%v", found, err)
	}
	if stored.StateBefore != executionjournal.EffectPending || stored.InvocationID != "inv-1" {
		t.Fatalf("落盘记录不符: %+v", stored)
	}
	recoveryLog(t, "scanned=1 recovered=1 decision=reconcile reexecute=false manual_only=true state_before=pending")
}

func TestJournalRecoveryUnknownAlsoReconciled(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-recovery")
	request := ledgerRequest(scope, "key-unknown", execution.SideEffectClassWrite)
	pendingRow(t, db, request, executionjournal.EffectUnknown)

	report, err := ledger.RecoverInFlight(context.Background(), scope, 0)
	if err != nil {
		t.Fatalf("RecoverInFlight: %v", err)
	}
	if len(report.Recovered) != 1 || report.Recovered[0].StateBefore != executionjournal.EffectUnknown {
		t.Fatalf("UNKNOWN 未被恢复: %+v", report.Recovered)
	}
	recoveryLog(t, "scanned=1 recovered=1 state_before=unknown decision=reconcile")
}

func TestJournalRecoverySkipsSettledRows(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-recovery")
	for _, key := range []string{"key-committed", "key-committed-2"} {
		request := ledgerRequest(scope, key, execution.SideEffectClassWrite)
		if _, _, err := ledger.Commit(context.Background(), request, func() (json.RawMessage, error) {
			return ledgerResult("ok"), nil
		}); err != nil {
			t.Fatalf("Commit %s: %v", key, err)
		}
	}

	report, err := ledger.RecoverInFlight(context.Background(), scope, 0)
	if err != nil {
		t.Fatalf("RecoverInFlight: %v", err)
	}
	if report.Scanned != 0 || len(report.Recovered) != 0 {
		t.Fatalf("已终结的记录被恢复: %+v", report)
	}
	entry, found, err := ledger.Lookup(context.Background(), scope, "key-committed")
	if err != nil || !found || entry.State != executionjournal.EffectCommitted {
		t.Fatalf("committed 记录被改动: %+v err=%v", entry, err)
	}
	recoveryLog(t, "settled=2 scanned=0 recovered=0 committed_state=committed")
}

func TestJournalRecoveryDuplicateWorkerConverges(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-recovery")
	request := ledgerRequest(scope, "key-shared", execution.SideEffectClassWrite)
	pendingRow(t, db, request, executionjournal.EffectPending)

	first, err := ledger.RecoverInFlight(context.Background(), scope, 0)
	if err != nil {
		t.Fatalf("首个恢复者: %v", err)
	}
	second, err := ledger.RecoverInFlight(context.Background(), scope, 0)
	if err != nil {
		t.Fatalf("第二个恢复者: %v", err)
	}
	if len(first.Recovered) != 1 || len(second.Recovered) != 0 || second.Previously != 1 {
		t.Fatalf("重复恢复未收敛: first=%+v second=%+v", first, second)
	}
	if rows := recoveryCount(t, db, scope, "key-shared"); rows != 1 {
		t.Fatalf("恢复记录 rows=%d 期望 1", rows)
	}
	recoveryLog(t, "workers=2 first_recovered=1 second_recovered=0 previously=1 rows=1")
}

func TestJournalRecoveryBlocksReexecute(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-recovery")
	request := ledgerRequest(scope, "key-crashed", execution.SideEffectClassWrite)
	pendingRow(t, db, request, executionjournal.EffectPending)
	if _, err := ledger.RecoverInFlight(context.Background(), scope, 0); err != nil {
		t.Fatalf("RecoverInFlight: %v", err)
	}

	var executions atomic.Int64
	_, replayed, err := ledger.Commit(context.Background(), request, func() (json.RawMessage, error) {
		executions.Add(1)
		return ledgerResult("dup"), nil
	})
	if !errors.Is(err, executionjournal.ErrRecoveryRequired) {
		t.Fatalf("被恢复的键仍可执行: err=%v", err)
	}
	if replayed {
		t.Fatalf("被恢复的键被报成重放成功")
	}
	if executions.Load() != 0 {
		t.Fatalf("外部副作用被重复执行 %d 次", executions.Load())
	}
	entry, found, err := ledger.Lookup(context.Background(), scope, "key-crashed")
	if err != nil || !found || entry.State != executionjournal.EffectPending {
		t.Fatalf("账本状态被改写: %+v err=%v", entry, err)
	}
	recoveryLog(t, "commit_after_recovery=blocked executions=0 state=pending replayed=false")
}

func TestJournalRecoverySurvivesRestart(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "ledger.db") + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(2)
	ledger, err := executionjournal.NewLedger(db)
	if err != nil {
		t.Fatal(err)
	}
	scope := ledgerScope("space-restart")
	request := ledgerRequest(scope, "key-restart", execution.SideEffectClassWrite)
	pendingRow(t, db, request, executionjournal.EffectPending)
	if _, err := ledger.RecoverInFlight(context.Background(), scope, 0); err != nil {
		t.Fatalf("RecoverInFlight: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("关闭: %v", err)
	}

	dbreopen, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	dbreopen.SetMaxOpenConns(2)
	t.Cleanup(func() { dbreopen.Close() })
	restarted, err := executionjournal.NewLedger(dbreopen)
	if err != nil {
		t.Fatalf("重启建账本: %v", err)
	}
	stored, found, err := restarted.InspectRecovery(context.Background(), scope, "key-restart")
	if err != nil || !found || stored.Decision != executionjournal.RecoveryReconcile {
		t.Fatalf("重启后恢复记录丢失: %+v err=%v", stored, err)
	}
	var executions atomic.Int64
	_, _, err = restarted.Commit(context.Background(), request, func() (json.RawMessage, error) {
		executions.Add(1)
		return ledgerResult("dup"), nil
	})
	if !errors.Is(err, executionjournal.ErrRecoveryRequired) {
		t.Fatalf("重启后仍可重复执行: err=%v", err)
	}
	if executions.Load() != 0 {
		t.Fatalf("重启后外部副作用被执行 %d 次", executions.Load())
	}
	again, err := restarted.RecoverInFlight(context.Background(), scope, 0)
	if err != nil {
		t.Fatalf("重启后再次恢复: %v", err)
	}
	if len(again.Recovered) != 0 || again.Previously != 1 {
		t.Fatalf("重启后重复登记: %+v", again)
	}
	recoveryLog(t, "restarted=true recovery_persisted=true commit=blocked executions=0 previously=1")
}

func TestJournalRecoveryRecordsImmutable(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-recovery")
	request := ledgerRequest(scope, "key-immutable", execution.SideEffectClassWrite)
	pendingRow(t, db, request, executionjournal.EffectPending)
	if _, err := ledger.RecoverInFlight(context.Background(), scope, 0); err != nil {
		t.Fatalf("RecoverInFlight: %v", err)
	}
	scopeKey, err := scope.Key()
	if err != nil {
		t.Fatal(err)
	}
	_, updateErr := db.Exec(`UPDATE kernel_invocation_recovery SET decision='reconcile', state_before='committed' WHERE scope_key=? AND idempotency_key=?`, scopeKey, "key-immutable")
	_, deleteErr := db.Exec(`DELETE FROM kernel_invocation_recovery WHERE scope_key=? AND idempotency_key=?`, scopeKey, "key-immutable")
	if !executionjournal.IsRecoveryImmutableError(updateErr) || !executionjournal.IsRecoveryImmutableError(deleteErr) {
		t.Fatalf("恢复记录可被改写: update=%v delete=%v", updateErr, deleteErr)
	}
	if rows := recoveryCount(t, db, scope, "key-immutable"); rows != 1 {
		t.Fatalf("恢复记录 rows=%d", rows)
	}
	recoveryLog(t, "update=blocked delete=blocked rows=1")
}

func TestJournalRecoveryRejectsInvalidScope(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	if _, err := ledger.RecoverInFlight(context.Background(), executionjournal.LedgerScope{}, 0); err == nil {
		t.Fatalf("空作用域未被拒绝")
	}
	if _, _, err := ledger.InspectRecovery(context.Background(), ledgerScope("space-recovery"), " "); !errors.Is(err, executionjournal.ErrLedgerKeyInvalid) {
		t.Fatalf("空幂等键未被拒绝: err=%v", err)
	}
	recoveryLog(t, "empty_scope=rejected empty_key=rejected")
}
