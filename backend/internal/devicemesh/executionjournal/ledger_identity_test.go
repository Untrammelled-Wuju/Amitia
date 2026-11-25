package executionjournal_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	executionjournal "github.com/u-ai/backend/internal/devicemesh/executionjournal"
	execution "github.com/u-ai/backend/internal/execution"
)

func identityRequest(scope executionjournal.LedgerScope, invocation, key string) executionjournal.LedgerRequest {
	return executionjournal.LedgerRequest{
		Scope:           scope,
		IdempotencyKey:  key,
		Trace:           execution.InvocationTrace{RootExecutionID: "root-s10", TraceID: "trace-s10", InvocationID: invocation},
		CapabilityID:    "host.tool.execute",
		BindingRevision: "rev-s10",
		PayloadHash:     "payload-s10",
		SideEffect:      execution.SideEffectClassWrite,
	}
}

func identityLedger(t *testing.T, path string) (*sql.DB, *executionjournal.InvocationLedger) {
	t.Helper()
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("打开账本数据库失败: %v", err)
	}
	db.SetMaxOpenConns(1)
	ledger, err := executionjournal.NewLedger(db)
	if err != nil {
		db.Close()
		t.Fatalf("创建账本失败: %v", err)
	}
	return db, ledger
}

func identityScopeKey(t *testing.T, scope executionjournal.LedgerScope) string {
	t.Helper()
	key, err := scope.Key()
	if err != nil {
		t.Fatalf("作用域键非法: %v", err)
	}
	return key
}

func emitIdentity(t *testing.T, line string) {
	t.Helper()
	t.Logf("m04-s10 %s", line)
}

func TestJournalIdentityRejectsForeignInvocation(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s10-foreign")
	executed := 0
	entry, ran, err := ledger.Commit(t.Context(), identityRequest(scope, "inv-owner", "key-shared"), func() (json.RawMessage, error) {
		executed++
		return ledgerResult("first"), nil
	})
	if err != nil || !ran {
		t.Fatalf("首次提交必须执行: ran=%v err=%v", ran, err)
	}
	if entry.State != executionjournal.EffectCommitted {
		t.Fatalf("首次提交状态=%s want committed", entry.State)
	}
	intruderRuns := 0
	_, intruderRan, mismatchErr := ledger.Commit(t.Context(), identityRequest(scope, "inv-intruder", "key-shared"), func() (json.RawMessage, error) {
		intruderRuns++
		return ledgerResult("second"), nil
	})
	if !errors.Is(mismatchErr, executionjournal.ErrLedgerInvocationMismatch) {
		t.Fatalf("换调用复用同键必须报身份不符: %v", mismatchErr)
	}
	if intruderRan || intruderRuns != 0 {
		t.Fatalf("身份不符的调用不得执行副作用: ran=%v runs=%d", intruderRan, intruderRuns)
	}
	if !strings.Contains(mismatchErr.Error(), `invocation="inv-owner"`) || !strings.Contains(mismatchErr.Error(), `requestedInvocation="inv-intruder"`) {
		t.Fatalf("身份不符错误必须带两侧调用: %v", mismatchErr)
	}
	stored, found, err := ledger.Lookup(t.Context(), scope, "key-shared")
	if err != nil || !found {
		t.Fatalf("回读账本失败: found=%v err=%v", found, err)
	}
	if stored.InvocationID != "inv-owner" || string(stored.Result) != `{"value":"first"}` {
		t.Fatalf("账本行被改写: invocation=%s result=%s", stored.InvocationID, stored.Result)
	}
	_, verdict, err := ledger.InspectBinding(t.Context(), scope, "inv-intruder", "key-shared")
	if err != nil || verdict != executionjournal.BindingForeign {
		t.Fatalf("只读判定=%s err=%v want foreign", verdict, err)
	}
	emitIdentity(t, fmt.Sprintf("key=key-shared owner=inv-owner intruder=inv-intruder state=%s executions=%d rejected=%t verdict=%s", stored.State, executed, true, verdict))
}

func TestJournalIdentityReplayOfSameInvocation(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s10-replay")
	request := identityRequest(scope, "inv-same", "key-replay")
	executed := 0
	first, ran, err := ledger.Commit(t.Context(), request, func() (json.RawMessage, error) {
		executed++
		return ledgerResult("once"), nil
	})
	if err != nil || !ran {
		t.Fatalf("首次提交必须执行: ran=%v err=%v", ran, err)
	}
	replayRuns := 0
	second, replayed, replayErr := ledger.Commit(t.Context(), request, func() (json.RawMessage, error) {
		replayRuns++
		return ledgerResult("twice"), nil
	})
	if replayErr != nil || replayed {
		t.Fatalf("同调用重放必须直接命中终态: ran=%v err=%v", replayed, replayErr)
	}
	if replayRuns != 0 || executed != 1 {
		t.Fatalf("同调用重放不得执行副作用: executed=%d replayRuns=%d", executed, replayRuns)
	}
	if string(second.Result) != string(first.Result) || second.InvocationID != first.InvocationID {
		t.Fatalf("重放结果不一致: first=%s second=%s", first.Result, second.Result)
	}
	_, verdict, err := ledger.InspectBinding(t.Context(), scope, "inv-same", "key-replay")
	if err != nil || verdict != executionjournal.BindingOwned {
		t.Fatalf("只读判定=%s err=%v want owned", verdict, err)
	}
	emitIdentity(t, fmt.Sprintf("replay=owned executions=%d result=match state=%s verdict=%s", executed, second.State, verdict))
}

func TestJournalIdentityScopeParticipates(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	first := ledgerScope("space-s10-a")
	second := ledgerScope("space-s10-b")
	executed := 0
	for _, scope := range []executionjournal.LedgerScope{first, second} {
		_, ran, err := ledger.Commit(t.Context(), identityRequest(scope, "inv-shared", "key-shared"), func() (json.RawMessage, error) {
			executed++
			return ledgerResult("scoped"), nil
		})
		if err != nil || !ran {
			t.Fatalf("跨作用域同键必须各自执行: scope=%+v ran=%v err=%v", scope, ran, err)
		}
	}
	rows := 0
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_invocation_ledger`).Scan(&rows); err != nil {
		t.Fatalf("统计账本行失败: %v", err)
	}
	if executed != 2 || rows != 2 {
		t.Fatalf("跨作用域应各自成行: executions=%d rows=%d", executed, rows)
	}
	emitIdentity(t, fmt.Sprintf("scope_parts=true rows=%d executions=%d", rows, executed))
}

func TestJournalIdentityKeyParticipates(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s10-keys")
	executed := 0
	for _, key := range []string{"key-one", "key-two"} {
		_, ran, err := ledger.Commit(t.Context(), identityRequest(scope, "inv-shared", key), func() (json.RawMessage, error) {
			executed++
			return ledgerResult(key), nil
		})
		if err != nil || !ran {
			t.Fatalf("同调用不同键必须各自执行: key=%s ran=%v err=%v", key, ran, err)
		}
	}
	rows := 0
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_invocation_ledger`).Scan(&rows); err != nil {
		t.Fatalf("统计账本行失败: %v", err)
	}
	if executed != 2 || rows != 2 {
		t.Fatalf("不同键应各自成行: executions=%d rows=%d", executed, rows)
	}
	emitIdentity(t, fmt.Sprintf("keys=2 rows=%d executions=%d", rows, executed))
}

func TestJournalIdentityInspectBindingIsReadOnly(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s10-inspect")
	_, verdict, err := ledger.InspectBinding(t.Context(), scope, "inv-probe", "key-probe")
	if err != nil || verdict != executionjournal.BindingAbsent {
		t.Fatalf("空库判定=%s err=%v want absent", verdict, err)
	}
	rows := 0
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_invocation_ledger`).Scan(&rows); err != nil {
		t.Fatalf("统计账本行失败: %v", err)
	}
	if rows != 0 {
		t.Fatalf("只读判定不得写库: rows=%d", rows)
	}
	if _, _, err := ledger.Commit(t.Context(), identityRequest(scope, "inv-probe", "key-probe"), func() (json.RawMessage, error) {
		return ledgerResult("probe"), nil
	}); err != nil {
		t.Fatalf("准备账本行失败: %v", err)
	}
	verdicts := map[string]executionjournal.BindingVerdict{}
	for _, invocation := range []string{"inv-probe", "inv-other"} {
		_, owned, err := ledger.InspectBinding(t.Context(), scope, invocation, "key-probe")
		if err != nil {
			t.Fatalf("判定 %s 失败: %v", invocation, err)
		}
		verdicts[invocation] = owned
	}
	if verdicts["inv-probe"] != executionjournal.BindingOwned || verdicts["inv-other"] != executionjournal.BindingForeign {
		t.Fatalf("判定结果不符: %+v", verdicts)
	}
	after := 0
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_invocation_ledger`).Scan(&after); err != nil {
		t.Fatalf("统计账本行失败: %v", err)
	}
	if after != 1 {
		t.Fatalf("只读判定改动了行数: rows=%d", after)
	}
	emitIdentity(t, fmt.Sprintf("absent=%s owned=%s foreign=%s rows=%d writes=0", executionjournal.BindingAbsent, verdicts["inv-probe"], verdicts["inv-other"], after))
}

func TestJournalIdentitySurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.db")
	db, ledger := identityLedger(t, path)
	scope := ledgerScope("space-s10-restart")
	request := identityRequest(scope, "inv-restart", "key-restart")
	executed := 0
	first, ran, err := ledger.Commit(t.Context(), request, func() (json.RawMessage, error) {
		executed++
		return ledgerResult("persisted"), nil
	})
	if err != nil || !ran {
		t.Fatalf("首次提交必须执行: ran=%v err=%v", ran, err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("关闭数据库失败: %v", err)
	}
	reopened, restarted := identityLedger(t, path)
	t.Cleanup(func() { reopened.Close() })
	afterRuns := 0
	second, replayRan, replayErr := restarted.Commit(t.Context(), request, func() (json.RawMessage, error) {
		afterRuns++
		return ledgerResult("rewritten"), nil
	})
	if replayErr != nil || replayRan {
		t.Fatalf("重启后重放必须直接命中终态: ran=%v err=%v", replayRan, replayErr)
	}
	if afterRuns != 0 || string(second.Result) != string(first.Result) || second.State != executionjournal.EffectCommitted {
		t.Fatalf("重启后终态不一致: state=%s result=%s runs=%d", second.State, second.Result, afterRuns)
	}
	if _, verdict, err := restarted.InspectBinding(t.Context(), scope, "inv-restart", "key-restart"); err != nil || verdict != executionjournal.BindingOwned {
		t.Fatalf("重启后只读判定=%s err=%v want owned", verdict, err)
	}
	_, foreignRan, foreignErr := restarted.Commit(t.Context(), identityRequest(scope, "inv-late", "key-restart"), func() (json.RawMessage, error) {
		return ledgerResult("late"), nil
	})
	if !errors.Is(foreignErr, executionjournal.ErrLedgerInvocationMismatch) || foreignRan {
		t.Fatalf("重启后身份约束必须仍生效: ran=%v err=%v", foreignRan, foreignErr)
	}
	emitIdentity(t, fmt.Sprintf("restarted=true state=%s result=match executions_after_restart=%d verdict=owned foreign=rejected", second.State, afterRuns))
}

func TestJournalTokenIdentityIsImmutable(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s10-token")
	request := identityRequest(scope, "inv-token", "key-token")
	token, err := ledger.IssueEffectToken(t.Context(), request)
	if err != nil {
		t.Fatalf("签发效果凭据失败: %v", err)
	}
	scopeKey := identityScopeKey(t, scope)
	saltErr := tamperToken(t, db, scopeKey, "key-token", token.Attempt, `UPDATE kernel_invocation_effect SET salt='deadbeef' WHERE scope_key=? AND idempotency_key=? AND attempt=?`)
	tokenErr := tamperToken(t, db, scopeKey, "key-token", token.Attempt, `UPDATE kernel_invocation_effect SET token='v1.1.' || lower(hex(randomblob(32))) WHERE scope_key=? AND idempotency_key=? AND attempt=?`)
	if !executionjournal.IsEffectTokenImmutableError(saltErr) || !executionjournal.IsEffectTokenImmutableError(tokenErr) {
		t.Fatalf("篡改必须被数据库约束拦住: salt=%v token=%v", saltErr, tokenErr)
	}
	var storedSalt, storedToken string
	if err := db.QueryRow(`SELECT salt,token FROM kernel_invocation_effect WHERE scope_key=? AND idempotency_key=? AND attempt=?`, scopeKey, "key-token", token.Attempt).Scan(&storedSalt, &storedToken); err != nil {
		t.Fatalf("回读凭据失败: %v", err)
	}
	if storedSalt == "deadbeef" || storedToken != token.Value {
		t.Fatalf("凭据被改写: salt=%s token=%s", storedSalt, storedToken)
	}
	if err := ledger.VerifyEffectToken(t.Context(), request, token); err != nil {
		t.Fatalf("篡改失败后原凭据必须仍有效: %v", err)
	}
	emitIdentity(t, fmt.Sprintf("update_salt=blocked update_token=blocked salt_unchanged=%t verify=valid", storedSalt != "deadbeef"))
}

func TestJournalTokenRemovalIsBlocked(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s10-token-delete")
	request := identityRequest(scope, "inv-token", "key-token-delete")
	token, err := ledger.IssueEffectToken(t.Context(), request)
	if err != nil {
		t.Fatalf("签发效果凭据失败: %v", err)
	}
	scopeKey := identityScopeKey(t, scope)
	_, deleteErr := db.Exec(`DELETE FROM kernel_invocation_effect WHERE scope_key=? AND idempotency_key=? AND attempt=?`, scopeKey, "key-token-delete", token.Attempt)
	if !executionjournal.IsEffectTokenImmutableError(deleteErr) {
		t.Fatalf("删除签发记录必须被拦住: %v", deleteErr)
	}
	rows := 0
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_invocation_effect WHERE scope_key=? AND idempotency_key=?`, scopeKey, "key-token-delete").Scan(&rows); err != nil {
		t.Fatalf("统计凭据行失败: %v", err)
	}
	if rows != 1 {
		t.Fatalf("凭据行被删除: rows=%d", rows)
	}
	if err := ledger.VerifyEffectToken(t.Context(), request, token); err != nil {
		t.Fatalf("删除失败后原凭据必须仍有效: %v", err)
	}
	emitIdentity(t, fmt.Sprintf("delete=blocked rows=%d verify=valid", rows))
}

func TestJournalTokenStateTransitionAllowed(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s10-token-state")
	request := identityRequest(scope, "inv-token", "key-token-state")
	token, err := ledger.IssueEffectToken(t.Context(), request)
	if err != nil {
		t.Fatalf("签发效果凭据失败: %v", err)
	}
	scopeKey := identityScopeKey(t, scope)
	if _, err := db.Exec(`UPDATE kernel_invocation_effect SET state='redeemed',redeemed_at='2026-11-25T00:00:00Z' WHERE scope_key=? AND idempotency_key=? AND attempt=?`, scopeKey, "key-token-state", token.Attempt); err != nil {
		t.Fatalf("状态迁移必须放行: %v", err)
	}
	verifyErr := ledger.VerifyEffectToken(t.Context(), request, token)
	if !errors.Is(verifyErr, executionjournal.ErrEffectTokenInvalid) {
		t.Fatalf("已核销凭据必须不可再用: %v", verifyErr)
	}
	emitIdentity(t, fmt.Sprintf("state_transition=allowed verify_after_redeem=rejected attempt=%d", token.Attempt))
}

func TestJournalTokenImmutabilitySurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.db")
	db, ledger := identityLedger(t, path)
	scope := ledgerScope("space-s10-token-restart")
	request := identityRequest(scope, "inv-token", "key-token-restart")
	token, err := ledger.IssueEffectToken(t.Context(), request)
	if err != nil {
		t.Fatalf("签发效果凭据失败: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("关闭数据库失败: %v", err)
	}
	reopened, restarted := identityLedger(t, path)
	t.Cleanup(func() { reopened.Close() })
	scopeKey := identityScopeKey(t, scope)
	tamperErr := tamperToken(t, reopened, scopeKey, "key-token-restart", token.Attempt, `UPDATE kernel_invocation_effect SET token='v1.1.' || lower(hex(randomblob(32))) WHERE scope_key=? AND idempotency_key=? AND attempt=?`)
	if !executionjournal.IsEffectTokenImmutableError(tamperErr) {
		t.Fatalf("重启后篡改仍必须被拦住: %v", tamperErr)
	}
	if err := restarted.VerifyEffectToken(t.Context(), request, token); err != nil {
		t.Fatalf("重启后原凭据必须仍有效: %v", err)
	}
	rows := 0
	if err := reopened.QueryRow(`SELECT COUNT(*) FROM kernel_invocation_effect`).Scan(&rows); err != nil {
		t.Fatalf("统计凭据行失败: %v", err)
	}
	if rows != 1 {
		t.Fatalf("凭据行数不符: rows=%d", rows)
	}
	emitIdentity(t, fmt.Sprintf("restarted=true tamper=blocked verify=valid rows=%d", rows))
}

func tamperToken(t *testing.T, db *sql.DB, scopeKey, key string, attempt int, statement string) error {
	t.Helper()
	_, err := db.Exec(statement, scopeKey, key, attempt)
	return err
}
