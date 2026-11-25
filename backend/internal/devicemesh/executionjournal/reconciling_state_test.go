package executionjournal_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/execution"
)

func insertLedgerRow(t *testing.T, db *sql.DB, scopeKey, key string, state executionjournal.EffectState) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO kernel_invocation_ledger(scope_key,idempotency_key,invocation_id,capability_id,binding_revision,payload_hash,effect_state,result,error_code,updated_at) VALUES(?,?,?,?,?,?,?,NULL,'',?)`,
		scopeKey, key, "inv-1", "host.tool.execute", "rev-7", "payload-hash-1", string(state), now); err != nil {
		t.Fatalf("插入账本行失败: %v", err)
	}
}

func insertReconciliationRow(t *testing.T, db *sql.DB, scopeKey, operation string, state executionjournal.ReconciliationState) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`INSERT INTO kernel_irreversible_reconciliation(scope_key,operation_id,invocation_id,capability_id,provider_id,side_effect,payload_hash,state,attempts,reason,last_error,created_at,updated_at,resolved_by,resolution_note,resolved_at) VALUES(?,?,?,?,?,?,?,?,1,'','',?,?,'','','')`,
		scopeKey, operation, "inv-1", "host.tool.execute", "core.kernel", "write", "payload-hash-1", string(state), now, now); err != nil {
		t.Fatalf("插入核对行失败: %v", err)
	}
}

func mustScopeKey(t *testing.T, scope executionjournal.LedgerScope) string {
	t.Helper()
	key, err := scope.Key()
	if err != nil {
		t.Fatalf("计算作用域键失败: %v", err)
	}
	return key
}

func emitReconciling(t *testing.T, line string) {
	t.Helper()
	t.Logf("m04-s09 %s", line)
}

func TestReconcilingStateAbsent(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s09-absent")
	snapshot, err := ledger.DescribeReconciliation(t.Context(), scope, "key-missing", "op-missing")
	if err != nil {
		t.Fatalf("查询空账本失败: %v", err)
	}
	if snapshot.State != executionjournal.ReconcilingAbsent || snapshot.AutoRetry || snapshot.ManualReview || snapshot.Conflict {
		t.Fatalf("空账本状态错误: %+v", snapshot)
	}
	if snapshot.NextAction != "none" || snapshot.LedgerFound || snapshot.ReconciliationFound {
		t.Fatalf("空账本动作错误: %+v", snapshot)
	}
	emitReconciling(t, "state=absent ledger=none reconciliation=none auto=false next=none")
}

func TestReconcilingStateSettled(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s09-settled")
	scopeKey := mustScopeKey(t, scope)
	insertLedgerRow(t, db, scopeKey, "key-settled", executionjournal.EffectCommitted)
	insertReconciliationRow(t, db, scopeKey, "op-settled", executionjournal.ReconciliationDelivered)
	snapshot, err := ledger.DescribeReconciliation(t.Context(), scope, "key-settled", "op-settled")
	if err != nil {
		t.Fatalf("查询已结算记录失败: %v", err)
	}
	if snapshot.State != executionjournal.ReconcilingSettled || snapshot.AutoRetry || snapshot.ManualReview || snapshot.Conflict {
		t.Fatalf("已结算状态错误: %+v", snapshot)
	}
	if snapshot.NextAction != "none" || snapshot.Effect != executionjournal.EffectCommitted || snapshot.Reconciliation != executionjournal.ReconciliationDelivered {
		t.Fatalf("已结算动作错误: %+v", snapshot)
	}
	emitReconciling(t, "state=settled ledger=committed reconciliation=delivered auto=false next=none")
}

func TestReconcilingStateUnknownBlocksAutoRetry(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s09-unknown")
	scopeKey := mustScopeKey(t, scope)
	insertLedgerRow(t, db, scopeKey, "key-unknown", executionjournal.EffectUnknown)
	for index := 0; index < 20; index++ {
		snapshot, err := ledger.DescribeReconciliation(t.Context(), scope, "key-unknown", "")
		if err != nil {
			t.Fatalf("第 %d 次查询未知结果失败: %v", index, err)
		}
		if snapshot.State != executionjournal.ReconcilingAmbiguous || snapshot.AutoRetry || !snapshot.ManualReview {
			t.Fatalf("未知结果被误判为可自动补发: %+v", snapshot)
		}
	}
	var executed int
	entry, inserted, err := ledger.Commit(t.Context(), ledgerRequest(scope, "key-unknown", execution.SideEffectClassWrite), func() (json.RawMessage, error) {
		executed++
		return nil, nil
	})
	if !errors.Is(err, executionjournal.ErrLedgerEffectUnknown) {
		t.Fatalf("未知结果必须拒绝同键重放: entry=%+v inserted=%v err=%v", entry, inserted, err)
	}
	if executed != 0 {
		t.Fatalf("未知结果不得触发副作用: executed=%d", executed)
	}
	settled, err := ledger.DescribeReconciliation(t.Context(), scope, "key-unknown", "")
	if err != nil {
		t.Fatalf("重放被拒后查询失败: %v", err)
	}
	emitReconciling(t, "state="+string(settled.State)+" ledger="+string(settled.Effect)+" reconciliation=none auto=false next=manual_review resends=0")
}

func TestReconcilingStateNeedsReconciliation(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s09-needs")
	scopeKey := mustScopeKey(t, scope)
	insertReconciliationRow(t, db, scopeKey, "op-needs", executionjournal.ReconciliationNeedsCheck)
	snapshot, err := ledger.DescribeReconciliation(t.Context(), scope, "", "op-needs")
	if err != nil {
		t.Fatalf("查询待核对记录失败: %v", err)
	}
	if snapshot.State != executionjournal.ReconcilingAmbiguous || snapshot.AutoRetry || !snapshot.ManualReview {
		t.Fatalf("待核对状态错误: %+v", snapshot)
	}
	if snapshot.Attempts != 1 || snapshot.NextAction != "manual_review" || snapshot.LedgerFound {
		t.Fatalf("待核对字段错误: %+v", snapshot)
	}
	emitReconciling(t, "state=ambiguous ledger=none reconciliation=needs_reconciliation auto=false next=manual_review")
}

func TestReconcilingStateNotDeliveredRetryable(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s09-retry")
	scopeKey := mustScopeKey(t, scope)
	insertReconciliationRow(t, db, scopeKey, "op-retry", executionjournal.ReconciliationNotSent)
	snapshot, err := ledger.DescribeReconciliation(t.Context(), scope, "", "op-retry")
	if err != nil {
		t.Fatalf("查询未送达记录失败: %v", err)
	}
	if snapshot.State != executionjournal.ReconcilingRetryable || !snapshot.AutoRetry || snapshot.ManualReview {
		t.Fatalf("确定未送达状态错误: %+v", snapshot)
	}
	if snapshot.NextAction != "retry" {
		t.Fatalf("确定未送达动作错误: %+v", snapshot)
	}
	emitReconciling(t, "state=retryable ledger=none reconciliation=not_delivered auto=true next=retry")
}

func TestReconcilingStateConflictIsBlocked(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s09-conflict")
	scopeKey := mustScopeKey(t, scope)
	insertLedgerRow(t, db, scopeKey, "key-conflict", executionjournal.EffectCommitted)
	insertReconciliationRow(t, db, scopeKey, "op-conflict", executionjournal.ReconciliationNotSent)
	snapshot, err := ledger.DescribeReconciliation(t.Context(), scope, "key-conflict", "op-conflict")
	if err != nil {
		t.Fatalf("查询冲突记录失败: %v", err)
	}
	if snapshot.State != executionjournal.ReconcilingBlocked || snapshot.AutoRetry || !snapshot.ManualReview || !snapshot.Conflict {
		t.Fatalf("冲突状态错误: %+v", snapshot)
	}
	if !strings.Contains(snapshot.Reason, "conflict=") {
		t.Fatalf("冲突原因缺失: %q", snapshot.Reason)
	}
	emitReconciling(t, "state=blocked ledger=committed reconciliation=not_delivered conflict=true auto=false next=manual_review")
}

func TestReconcilingStatePendingIsInFlight(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s09-pending")
	scopeKey := mustScopeKey(t, scope)
	insertLedgerRow(t, db, scopeKey, "key-pending", executionjournal.EffectPending)
	snapshot, err := ledger.DescribeReconciliation(t.Context(), scope, "key-pending", "")
	if err != nil {
		t.Fatalf("查询执行中记录失败: %v", err)
	}
	if snapshot.State != executionjournal.ReconcilingInFlight || snapshot.AutoRetry || !snapshot.ManualReview {
		t.Fatalf("执行中状态错误: %+v", snapshot)
	}
	emitReconciling(t, "state=in_flight ledger=pending reconciliation=none auto=false next=manual_review")
}

func TestReconcilingStateReadOnlyQuery(t *testing.T) {
	db, ledger := ledgerHarness(t, 1)
	scope := ledgerScope("space-s09-readonly")
	scopeKey := mustScopeKey(t, scope)
	insertLedgerRow(t, db, scopeKey, "key-readonly", executionjournal.EffectUnknown)
	insertReconciliationRow(t, db, scopeKey, "op-readonly", executionjournal.ReconciliationNeedsCheck)
	digest := func() string {
		t.Helper()
		rows, err := db.Query(`SELECT scope_key,idempotency_key,effect_state,updated_at FROM kernel_invocation_ledger ORDER BY 1,2`)
		if err != nil {
			t.Fatalf("读取账本摘要失败: %v", err)
		}
		defer rows.Close()
		var parts []string
		for rows.Next() {
			var scopeKey, key, state, updated string
			if err := rows.Scan(&scopeKey, &key, &state, &updated); err != nil {
				t.Fatalf("扫描账本摘要失败: %v", err)
			}
			parts = append(parts, scopeKey+"|"+key+"|"+state+"|"+updated)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("遍历账本摘要失败: %v", err)
		}
		return strings.Join(parts, "\n")
	}
	before := digest()
	if _, err := db.Exec(`PRAGMA query_only=ON`); err != nil {
		t.Fatalf("开启只读模式失败: %v", err)
	}
	for index := 0; index < 50; index++ {
		snapshot, err := ledger.DescribeReconciliation(t.Context(), scope, "key-readonly", "op-readonly")
		if err != nil {
			t.Fatalf("只读查询第 %d 次失败: %v", index, err)
		}
		if snapshot.State != executionjournal.ReconcilingAmbiguous || snapshot.AutoRetry {
			t.Fatalf("只读查询状态错误: %+v", snapshot)
		}
	}
	after := digest()
	if before != after {
		t.Fatalf("只读查询改动了账本: before=%q after=%q", before, after)
	}
	_, err := ledger.ResolveReconciliation(t.Context(), scope, "op-readonly", executionjournal.VerdictDelivered, "ops", "只读校验")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "readonly") {
		t.Fatalf("只读连接上的写入必须失败: %v", err)
	}
	emitReconciling(t, "state=ambiguous readonly=true writes=0 unchanged=true rows=1")
}

func TestReconcilingStateInvalidAndCancelled(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s09-invalid")
	ctx := t.Context()
	invalid := 0
	if _, err := ledger.DescribeReconciliation(ctx, scope, "", ""); errors.Is(err, executionjournal.ErrReconcileQueryInvalid) {
		invalid++
	}
	if _, err := ledger.DescribeReconciliation(ctx, executionjournal.LedgerScope{}, "key", ""); errors.Is(err, executionjournal.ErrReconcileQueryInvalid) {
		invalid++
	}
	if _, err := ledger.DescribeReconciliation(ctx, scope, strings.Repeat("k", 513), ""); errors.Is(err, executionjournal.ErrReconcileQueryInvalid) {
		invalid++
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	cancelledOK := false
	if _, err := ledger.DescribeReconciliation(cancelled, scope, "key", ""); errors.Is(err, context.Canceled) {
		cancelledOK = true
	}
	if invalid != 3 || !cancelledOK {
		t.Fatalf("非法与取消输入未被拒绝: invalid=%d cancelled=%v", invalid, cancelledOK)
	}
	emitReconciling(t, "invalid=3 cancelled=true scope=rejected")
}
