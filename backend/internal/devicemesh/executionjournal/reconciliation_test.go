package executionjournal_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/execution"
)

func irreversibleRequest(scope executionjournal.LedgerScope, operation string, sideEffect execution.SideEffectClass) executionjournal.IrreversibleRequest {
	return executionjournal.IrreversibleRequest{
		Scope:        scope,
		OperationID:  operation,
		Trace:        execution.InvocationTrace{RootExecutionID: "root-1", TraceID: "trace-1", InvocationID: "inv-" + operation},
		CapabilityID: "channel.message.send",
		ProviderID:   "plugins-code/channel/im",
		SideEffect:   sideEffect,
		PayloadHash:  "payload-" + operation,
	}
}

func openLedgerAt(t *testing.T, path string, conns int) (*sql.DB, *executionjournal.InvocationLedger) {
	t.Helper()
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
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

func TestReconcileLostAckIsNotResent(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	ctx := context.Background()
	scope := ledgerScope("space-a")
	request := irreversibleRequest(scope, "message-send-1", execution.SideEffectClassExternal)

	dispatches := 0
	record, err := ledger.DispatchIrreversible(ctx, request, func() error {
		dispatches++
		return fmt.Errorf("消息已发出但回执丢失: %w", context.DeadlineExceeded)
	})
	if err == nil || !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, executionjournal.ErrReconciliationPending) {
		t.Fatalf("回执丢失必须同时保留原始错误与待核对哨兵: %v", err)
	}
	if record.State != executionjournal.ReconciliationNeedsCheck || !record.NeedsHumanCheck() {
		t.Fatalf("回执丢失后状态必须是待核对: %s", record.State)
	}

	_, retryErr := ledger.DispatchIrreversible(ctx, request, func() error {
		dispatches++
		return nil
	})
	if !errors.Is(retryErr, executionjournal.ErrReconciliationPending) {
		t.Fatalf("未核对前必须拒绝再次发送: %v", retryErr)
	}
	if dispatches != 1 {
		t.Fatalf("已发送但回执丢失的消息不得重发: dispatches=%d", dispatches)
	}
	fmt.Printf("m04-s05 reconcile ack-lost dispatches=%d state=%s retryBlocked=true\n", dispatches, record.State)
}

func TestReconcileCaseSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restart.db")
	ctx := context.Background()
	scope := ledgerScope("space-a")
	request := irreversibleRequest(scope, "message-restart", execution.SideEffectClassExternal)

	db, ledger := openLedgerAt(t, path, 2)
	if _, err := ledger.DispatchIrreversible(ctx, request, func() error {
		return fmt.Errorf("回执丢失: %w", execution.ErrInvocationTransportLost)
	}); err == nil {
		t.Fatal("需要构造未知结果")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	_, reopened := openLedgerAt(t, path, 2)
	record, found, err := reopened.Reconciliation(ctx, scope, request.OperationID)
	if err != nil || !found {
		t.Fatalf("重启后必须仍可查到待核对记录: found=%t err=%v", found, err)
	}
	if record.State != executionjournal.ReconciliationNeedsCheck || record.Attempts != 1 || record.LastError == "" {
		t.Fatalf("重启后状态与原因必须原样保留: state=%s attempts=%d hasError=%t", record.State, record.Attempts, record.LastError != "")
	}
	open, err := reopened.OpenReconciliations(ctx, nil)
	if err != nil || len(open) != 1 || open[0].OperationID != request.OperationID {
		t.Fatalf("重启后待核对清单必须可查: open=%d err=%v", len(open), err)
	}
	fmt.Printf("m04-s05 reconcile restart visible=%t state=%s attempts=%d error=true\n", found, record.State, record.Attempts)
}

func TestReconcileRecoversInterruptedAttempt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crash.db")
	ctx := context.Background()
	scope := ledgerScope("space-a")
	request := irreversibleRequest(scope, "message-crash", execution.SideEffectClassWrite)

	db, ledger := openLedgerAt(t, path, 2)
	if _, err := ledger.ReserveIrreversible(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	_, reopened := openLedgerAt(t, path, 2)
	recovered, err := reopened.RecoverInterrupted(ctx)
	if err != nil || recovered != 1 {
		t.Fatalf("崩溃恢复必须把执行中的记录转成待核对: recovered=%d err=%v", recovered, err)
	}
	record, found, err := reopened.Reconciliation(ctx, scope, request.OperationID)
	if err != nil || !found {
		t.Fatalf("恢复后记录仍须存在: found=%t err=%v", found, err)
	}
	if record.State != executionjournal.ReconciliationNeedsCheck {
		t.Fatalf("中断记录必须进入待核对: %s", record.State)
	}
	fmt.Printf("m04-s05 reconcile crash-recovery recovered=%d state=%s\n", recovered, record.State)
}

func TestReconcileHumanDeliveredBlocksResend(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	ctx := context.Background()
	scope := ledgerScope("space-a")
	request := irreversibleRequest(scope, "message-delivered", execution.SideEffectClassExternal)

	dispatches := 0
	if _, err := ledger.DispatchIrreversible(ctx, request, func() error {
		dispatches++
		return fmt.Errorf("回执丢失: %w", context.DeadlineExceeded)
	}); err == nil {
		t.Fatal("需要构造未知结果")
	}
	resolved, err := ledger.ResolveReconciliation(ctx, scope, request.OperationID, executionjournal.VerdictDelivered, "operator-a", "在对方会话里看到了这条消息")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.State != executionjournal.ReconciliationDelivered || resolved.ResolvedBy != "operator-a" || resolved.ResolvedAt.IsZero() {
		t.Fatalf("人工结论必须落库: state=%s by=%s resolvedAt=%s", resolved.State, resolved.ResolvedBy, resolved.ResolvedAt)
	}
	_, retryErr := ledger.DispatchIrreversible(ctx, request, func() error {
		dispatches++
		return nil
	})
	if !errors.Is(retryErr, executionjournal.ErrReconciliationDelivered) {
		t.Fatalf("核实已送达后必须禁止再次发送: %v", retryErr)
	}
	if dispatches != 1 {
		t.Fatalf("核实已送达后不得重发: dispatches=%d", dispatches)
	}
	fmt.Printf("m04-s05 reconcile resolved-delivered resolved=true retryBlocked=true dispatches=%d\n", dispatches)
}

func TestReconcileHumanNotSentAllowsRetry(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	ctx := context.Background()
	scope := ledgerScope("space-a")
	request := irreversibleRequest(scope, "message-not-sent", execution.SideEffectClassExternal)

	dispatches := 0
	if _, err := ledger.DispatchIrreversible(ctx, request, func() error {
		dispatches++
		return fmt.Errorf("回执丢失: %w", context.DeadlineExceeded)
	}); err == nil {
		t.Fatal("需要构造未知结果")
	}
	if _, err := ledger.ResolveReconciliation(ctx, scope, request.OperationID, executionjournal.VerdictNotSent, "operator-b", "对方确认没有收到"); err != nil {
		t.Fatal(err)
	}
	record, err := ledger.DispatchIrreversible(ctx, request, func() error {
		dispatches++
		return nil
	})
	if err != nil {
		t.Fatalf("核实未送达后必须允许重发: %v", err)
	}
	if dispatches != 2 || record.Attempts != 2 || record.State != executionjournal.ReconciliationDelivered {
		t.Fatalf("重发必须被计数并最终核实送达: dispatches=%d attempts=%d state=%s", dispatches, record.Attempts, record.State)
	}
	if record.ResolvedBy != "" || record.Resolution != "" {
		t.Fatalf("重新发送后必须清空上一次人工结论: by=%q note=%q", record.ResolvedBy, record.Resolution)
	}
	fmt.Printf("m04-s05 reconcile resolved-not-sent retryAllowed=true attempts=%d state=%s\n", record.Attempts, record.State)
}

func TestReconcileDoubleResolveIsRejected(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	ctx := context.Background()
	scope := ledgerScope("space-a")
	request := irreversibleRequest(scope, "message-double", execution.SideEffectClassExternal)
	if _, err := ledger.DispatchIrreversible(ctx, request, func() error {
		return fmt.Errorf("回执丢失: %w", context.DeadlineExceeded)
	}); err == nil {
		t.Fatal("需要构造未知结果")
	}
	if _, err := ledger.ResolveReconciliation(ctx, scope, request.OperationID, executionjournal.VerdictDelivered, "operator-a", ""); err != nil {
		t.Fatal(err)
	}
	record, err := ledger.ResolveReconciliation(ctx, scope, request.OperationID, executionjournal.VerdictNotSent, "operator-b", "改口")
	if !errors.Is(err, executionjournal.ErrReconciliationResolved) {
		t.Fatalf("同一记录不得改写第二次结论: %v", err)
	}
	if record.State != executionjournal.ReconciliationDelivered || record.ResolvedBy != "operator-a" {
		t.Fatalf("被拒绝的改写不得污染原结论: state=%s by=%s", record.State, record.ResolvedBy)
	}
	fmt.Printf("m04-s05 reconcile double-resolve rejected=true state=%s\n", record.State)
}

func TestReconcileResolveRequiresOpenCase(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	ctx := context.Background()
	scope := ledgerScope("space-a")
	request := irreversibleRequest(scope, "message-inflight", execution.SideEffectClassWrite)
	if _, err := ledger.ReserveIrreversible(ctx, request); err != nil {
		t.Fatal(err)
	}
	record, err := ledger.ResolveReconciliation(ctx, scope, request.OperationID, executionjournal.VerdictDelivered, "operator-a", "")
	if !errors.Is(err, executionjournal.ErrReconciliationResolved) || record.State != executionjournal.ReconciliationAttempting {
		t.Fatalf("仍在执行中的记录不得被人工结论覆盖: state=%s err=%v", record.State, err)
	}
	_, missingErr := ledger.ResolveReconciliation(ctx, scope, "unknown-operation", executionjournal.VerdictDelivered, "operator-a", "")
	if !errors.Is(missingErr, executionjournal.ErrReconciliationNotFound) {
		t.Fatalf("未知记录必须报找不到: %v", missingErr)
	}
	fmt.Printf("m04-s05 reconcile resolve-guard inflightBlocked=true missingReported=true\n")
}

func TestReconcileDefiniteFailureAllowsRetry(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	ctx := context.Background()
	scope := ledgerScope("space-a")
	request := irreversibleRequest(scope, "message-definite", execution.SideEffectClassExternal)

	dispatches := 0
	record, err := ledger.DispatchIrreversible(ctx, request, func() error {
		dispatches++
		return errors.New("渠道返回 400 参数非法")
	})
	if err == nil || strings.Contains(err.Error(), "未知") {
		t.Fatalf("确定失败必须原样返回原始错误: %v", err)
	}
	if record.State != executionjournal.ReconciliationNotSent || !record.RetryAllowed() {
		t.Fatalf("确定失败必须允许重发: state=%s", record.State)
	}
	retried, err := ledger.DispatchIrreversible(ctx, request, func() error {
		dispatches++
		return nil
	})
	if err != nil || dispatches != 2 || retried.State != executionjournal.ReconciliationDelivered || retried.Attempts != 2 {
		t.Fatalf("确定失败后的重发必须成功并计数: dispatches=%d state=%s err=%v", dispatches, retried.State, err)
	}
	fmt.Printf("m04-s05 reconcile definite-failure state=not_delivered retryAllowed=true dispatches=%d\n", dispatches)
}

func TestReconcileOpenListIsQueryable(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	ctx := context.Background()
	spaceA := ledgerScope("space-a")
	spaceB := ledgerScope("space-b")

	for _, request := range []executionjournal.IrreversibleRequest{
		irreversibleRequest(spaceA, "open-a", execution.SideEffectClassExternal),
		irreversibleRequest(spaceA, "delivered-a", execution.SideEffectClassExternal),
		irreversibleRequest(spaceB, "open-b", execution.SideEffectClassWrite),
	} {
		request := request
		if _, err := ledger.DispatchIrreversible(ctx, request, func() error {
			if request.OperationID == "delivered-a" {
				return nil
			}
			return fmt.Errorf("回执丢失: %w", context.DeadlineExceeded)
		}); request.OperationID != "delivered-a" && err == nil {
			t.Fatal("需要构造未知结果")
		}
	}

	all, err := ledger.OpenReconciliations(ctx, nil)
	if err != nil || len(all) != 2 {
		t.Fatalf("全局待核对清单必须列出两条: open=%d err=%v", len(all), err)
	}
	scoped, err := ledger.OpenReconciliations(ctx, &spaceA)
	if err != nil || len(scoped) != 1 || scoped[0].OperationID != "open-a" {
		t.Fatalf("按作用域过滤必须只含本空间: scoped=%d err=%v", len(scoped), err)
	}
	if all[0].CreatedAt.After(all[1].CreatedAt) {
		t.Fatalf("待核对清单必须按创建时间稳定排序")
	}
	fmt.Printf("m04-s05 reconcile open-list open=%d scoped=%d\n", len(all), len(scoped))
}

func TestReconcileConcurrentDispatchSendsOnce(t *testing.T) {
	_, ledger := ledgerHarness(t, 8)
	ctx := context.Background()
	scope := ledgerScope("space-a")
	request := irreversibleRequest(scope, "message-concurrent", execution.SideEffectClassExternal)

	var dispatches int64
	leaders := int64(0)
	blocked := int64(0)
	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := ledger.DispatchIrreversible(ctx, request, func() error {
				atomic.AddInt64(&dispatches, 1)
				time.Sleep(5 * time.Millisecond)
				return nil
			})
			if err == nil {
				atomic.AddInt64(&leaders, 1)
				return
			}
			if errors.Is(err, executionjournal.ErrReconciliationPending) {
				atomic.AddInt64(&blocked, 1)
			}
		}()
	}
	wait.Wait()
	if dispatches != 1 || leaders != 1 || blocked != 7 {
		t.Fatalf("同一操作并发只允许一次真正发送: dispatches=%d leaders=%d blocked=%d", dispatches, leaders, blocked)
	}
	fmt.Printf("m04-s05 reconcile concurrent workers=8 dispatches=%d leaders=%d blocked=%d\n", dispatches, leaders, blocked)
}

func TestReconcileDoesNotTouchKeyedLedger(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	ctx := context.Background()
	scope := ledgerScope("space-a")
	request := irreversibleRequest(scope, "message-isolation", execution.SideEffectClassExternal)
	if _, err := ledger.DispatchIrreversible(ctx, request, func() error {
		return fmt.Errorf("回执丢失: %w", context.DeadlineExceeded)
	}); err == nil {
		t.Fatal("需要构造未知结果")
	}
	counts := make([]int, 3)
	queries := []string{
		`SELECT COUNT(*) FROM kernel_invocation_ledger`,
		`SELECT COUNT(*) FROM kernel_invocation_effect`,
		`SELECT COUNT(*) FROM kernel_irreversible_reconciliation`,
	}
	for index, query := range queries {
		if err := db.QueryRow(query).Scan(&counts[index]); err != nil {
			t.Fatal(err)
		}
	}
	if counts[0] != 0 || counts[1] != 0 || counts[2] != 1 {
		t.Fatalf("非幂等写不得写入幂等账本与效果凭据表: %v", counts)
	}
	fmt.Printf("m04-s05 reconcile isolation ledger_rows=%d effect_rows=%d reconciliation_rows=%d\n", counts[0], counts[1], counts[2])
}

func TestReconcileKeepsErrorTextVerbatim(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	ctx := context.Background()
	scope := ledgerScope("space-a")
	request := irreversibleRequest(scope, "message-verbatim", execution.SideEffectClassExternal)

	cause := fmt.Errorf("渠道网关返回\nstatus=502\tbody={\"error\":\"bad gateway\",\"retry\":false}\n原始堆栈 line 1\nline 2: %w", execution.ErrInvocationTransportLost)
	record, err := ledger.DispatchIrreversible(ctx, request, func() error { return cause })
	if err == nil {
		t.Fatal("需要构造未知结果")
	}
	if record.LastError != cause.Error() {
		t.Fatalf("错误文本必须原样保留: got=%q want=%q", record.LastError, cause.Error())
	}
	if !strings.Contains(record.LastError, "\n") || !strings.Contains(record.LastError, "line 2") {
		t.Fatalf("错误文本不得被折叠或截断: %q", record.LastError)
	}
	fmt.Printf("m04-s05 reconcile error-verbatim bytes=%d preserved=true\n", len(record.LastError))
}

func TestReconcileRejectsInvalidRequests(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	ctx := context.Background()
	scope := ledgerScope("space-a")
	base := irreversibleRequest(scope, "message-invalid", execution.SideEffectClassExternal)
	cases := []struct {
		name    string
		request executionjournal.IrreversibleRequest
	}{
		{name: "empty-space", request: func() executionjournal.IrreversibleRequest {
			value := base
			value.Scope.SpaceID = " "
			return value
		}()},
		{name: "empty-operation", request: func() executionjournal.IrreversibleRequest {
			value := base
			value.OperationID = " "
			return value
		}()},
		{name: "long-operation", request: func() executionjournal.IrreversibleRequest {
			value := base
			value.OperationID = strings.Repeat("k", 513)
			return value
		}()},
		{name: "separator-operation", request: func() executionjournal.IrreversibleRequest {
			value := base
			value.OperationID = "message\x1finvalid"
			return value
		}()},
		{name: "empty-invocation", request: func() executionjournal.IrreversibleRequest {
			value := base
			value.Trace.InvocationID = " "
			return value
		}()},
		{name: "empty-capability", request: func() executionjournal.IrreversibleRequest {
			value := base
			value.CapabilityID = " "
			return value
		}()},
		{name: "empty-provider", request: func() executionjournal.IrreversibleRequest {
			value := base
			value.ProviderID = " "
			return value
		}()},
		{name: "read-side-effect", request: func() executionjournal.IrreversibleRequest {
			value := base
			value.SideEffect = execution.SideEffectClassRead
			return value
		}()},
		{name: "empty-payload", request: func() executionjournal.IrreversibleRequest {
			value := base
			value.PayloadHash = " "
			return value
		}()},
	}
	rejected := 0
	for _, testCase := range cases {
		dispatched := false
		if _, err := ledger.DispatchIrreversible(ctx, testCase.request, func() error {
			dispatched = true
			return nil
		}); !errors.Is(err, executionjournal.ErrReconciliationKeyInvalid) {
			t.Fatalf("%s 必须被拒绝: %v", testCase.name, err)
		}
		if dispatched {
			t.Fatalf("%s 被拒绝时不得派发", testCase.name)
		}
		rejected++
	}
	valid := irreversibleRequest(scope, "message-valid", execution.SideEffectClassExternal)
	if _, err := ledger.DispatchIrreversible(ctx, valid, func() error {
		return fmt.Errorf("回执丢失: %w", context.DeadlineExceeded)
	}); err == nil {
		t.Fatal("需要构造未知结果")
	}
	badVerdicts := []struct {
		verdict  executionjournal.ReconciliationVerdict
		operator string
	}{
		{verdict: "done", operator: "operator-a"},
		{verdict: executionjournal.VerdictDelivered, operator: " "},
		{verdict: executionjournal.VerdictDelivered, operator: strings.Repeat("o", 129)},
	}
	for _, testCase := range badVerdicts {
		if _, err := ledger.ResolveReconciliation(ctx, scope, valid.OperationID, testCase.verdict, testCase.operator, ""); !errors.Is(err, executionjournal.ErrReconciliationVerdictInvalid) {
			t.Fatalf("非法结论必须被拒绝: verdict=%q operator=%d err=%v", testCase.verdict, len(testCase.operator), err)
		}
	}
	record, found, err := ledger.Reconciliation(ctx, scope, valid.OperationID)
	if err != nil || !found || record.State != executionjournal.ReconciliationNeedsCheck {
		t.Fatalf("非法结论不得改动状态: found=%t state=%s err=%v", found, record.State, err)
	}
	fmt.Printf("m04-s05 reconcile invalid-requests rejected=%d dispatched=0 verdicts=%d\n", rejected, len(badVerdicts))
}
