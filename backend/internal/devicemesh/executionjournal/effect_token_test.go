package executionjournal_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/execution"
)

type fakeAuthority struct {
	mu      sync.Mutex
	records map[string]executionjournal.EffectRecord
	fail    error
	lookups int
}

func newFakeAuthority() *fakeAuthority {
	return &fakeAuthority{records: map[string]executionjournal.EffectRecord{}}
}

func (f *fakeAuthority) LookupEffect(_ context.Context, businessKey string) (executionjournal.EffectRecord, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lookups++
	if f.fail != nil {
		return executionjournal.EffectRecord{}, false, f.fail
	}
	record, ok := f.records[businessKey]
	return record, ok, nil
}

func (f *fakeAuthority) put(t *testing.T, record executionjournal.EffectRecord) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records[record.BusinessKey] = record
}

func (f *fakeAuthority) lookupsCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lookups
}

func writeRequest(scope executionjournal.LedgerScope, key string, capability string) executionjournal.LedgerRequest {
	request := ledgerRequest(scope, key, execution.SideEffectClassWrite)
	request.CapabilityID = capability
	return request
}

func businessKey(t *testing.T, scope executionjournal.LedgerScope, key string) string {
	t.Helper()
	value, err := executionjournal.BusinessKey(scope, key)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestEffectTokenBindsRequest(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	ctx := context.Background()
	scope := ledgerScope("space-a")
	request := writeRequest(scope, "token-key", "plugin.write.memory")

	token, err := ledger.IssueEffectToken(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.VerifyEffectToken(ctx, request, token); err != nil {
		t.Fatalf("签发的凭据必须可用: %v", err)
	}
	tampered := executionjournal.EffectToken{Value: token.Value[:len(token.Value)-1] + "0", Attempt: token.Attempt}
	if tampered.Value == token.Value {
		tampered.Value = token.Value[:len(token.Value)-1] + "1"
	}
	errTampered := ledger.VerifyEffectToken(ctx, request, tampered)
	foreign, err := ledger.IssueEffectToken(ctx, writeRequest(scope, "token-key-2", "plugin.write.memory"))
	if err != nil {
		t.Fatal(err)
	}
	errForeign := ledger.VerifyEffectToken(ctx, request, foreign)
	mismatched := executionjournal.EffectToken{Value: token.Value, Attempt: token.Attempt + 1}
	errMismatched := ledger.VerifyEffectToken(ctx, request, mismatched)
	readToken, errRead := ledger.IssueEffectToken(ctx, ledgerRequest(scope, "token-read", execution.SideEffectClassRead))
	if errRead == nil {
		t.Fatalf("只读请求不应签发效果凭据: %v", readToken)
	}

	problems := 0
	for name, err := range map[string]error{"tampered": errTampered, "foreign": errForeign, "mismatched": errMismatched, "read": errRead} {
		if !errors.Is(err, executionjournal.ErrEffectTokenInvalid) {
			t.Errorf("%s 应以 ErrEffectTokenInvalid 拒绝，实际: %v", name, err)
			problems++
		}
	}
	fmt.Printf("m04-s04 effect token-binds attempts=2 verified=1 rejected=%d problems=%d\n", 4-problems, problems)
	if problems > 0 {
		t.FailNow()
	}
}

func TestEffectTokenRequiresMutatingRequest(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	ctx := context.Background()
	scope := ledgerScope("space-a")

	accepted, errAccepted := ledger.IssueEffectToken(ctx, writeRequest(scope, "mutating-write", "plugin.write.memory"))
	external := ledgerRequest(scope, "mutating-external", execution.SideEffectClassExternal)
	acceptedExternal, errExternal := ledger.IssueEffectToken(ctx, external)
	_, errRead := ledger.IssueEffectToken(ctx, ledgerRequest(scope, "read-only", execution.SideEffectClassRead))
	_, errNone := ledger.IssueEffectToken(ctx, ledgerRequest(scope, "none", execution.SideEffectClassNone))

	fmt.Printf("m04-s04 effect token-mutating write=%d external=%d readRejected=%t noneRejected=%t\n",
		1, 1, errors.Is(errRead, executionjournal.ErrEffectTokenInvalid), errors.Is(errNone, executionjournal.ErrEffectTokenInvalid))
	if errAccepted != nil || errExternal != nil {
		t.Fatalf("写与外部副作用都必须能签发凭据: write=%v external=%v", errAccepted, errExternal)
	}
	if accepted.Attempt != 1 || acceptedExternal.Attempt != 1 {
		t.Fatalf("不同幂等键的尝试序号应各自从 1 开始: %d/%d", accepted.Attempt, acceptedExternal.Attempt)
	}
	if !errors.Is(errRead, executionjournal.ErrEffectTokenInvalid) || !errors.Is(errNone, executionjournal.ErrEffectTokenInvalid) {
		t.Fatalf("只读与无副作用必须拒绝签发: read=%v none=%v", errRead, errNone)
	}
}

func TestEffectTimeoutAfterWriteIsNotReplayed(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	authority := newFakeAuthority()
	scope := ledgerScope("space-a")
	request := writeRequest(scope, "timeout-key", "plugin.write.memory")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executions := 0
	var firstToken string
	_, executed, err := ledger.CommitWithEffect(ctx, request, authority, func(token executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		firstToken = token.Value
		authority.put(t, executionjournal.EffectRecord{
			BusinessKey: businessKey(t, scope, request.IdempotencyKey),
			PayloadHash: request.PayloadHash,
			Token:       token.Value,
			Result:      ledgerResult("landed"),
		})
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if !errors.Is(err, execution.ErrInvocationOutcomeUnknown) {
		t.Fatalf("写已发出但超时必须判为未知，实际: %v", err)
	}
	if !executed {
		t.Fatal("回调确实执行过，executed 应为 true")
	}

	entry, found, lookupErr := ledger.Lookup(context.Background(), scope, request.IdempotencyKey)
	if lookupErr != nil || !found {
		t.Fatalf("未知结果必须留痕: found=%t err=%v", found, lookupErr)
	}
	if entry.State != executionjournal.EffectUnknown {
		t.Fatalf("超时后状态应为 unknown，实际 %s", entry.State)
	}

	replayed, secondExecuted, secondErr := ledger.CommitWithEffect(context.Background(), request, authority, func(executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		return ledgerResult("replayed"), nil
	})
	if secondErr != nil {
		t.Fatalf("权威记录已确认时应返回既有结果: %v", secondErr)
	}
	if secondExecuted {
		t.Fatal("已确认成功的写绝不能再次执行")
	}
	if string(replayed.Result) != string(ledgerResult("landed")) {
		t.Fatalf("应返回权威记录中的结果，实际 %s", replayed.Result)
	}
	if replayed.State != executionjournal.EffectCommitted {
		t.Fatalf("核对后状态应为 committed，实际 %s", replayed.State)
	}
	if executions != 1 {
		t.Fatalf("执行次数应为 1，实际 %d", executions)
	}
	if !strings.HasPrefix(firstToken, "v1.1.") {
		t.Fatalf("首次尝试的凭据应以 v1.1 开头，实际 %s", firstToken)
	}
	fmt.Printf("m04-s04 effect timeout-after-write executions=%d replays=%t state=%s result=%s\n",
		executions, secondExecuted, replayed.State, replayed.Result)
}

func TestEffectSameKeyReturnsRecordedAcrossProviders(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	authority := newFakeAuthority()
	ctx := context.Background()
	scope := ledgerScope("space-a")
	plugin := writeRequest(scope, "shared-key", "plugin.write.memory")
	core := writeRequest(scope, "shared-key", "builtin.core.write.memory")
	if businessKey(t, scope, plugin.IdempotencyKey) != businessKey(t, scope, core.IdempotencyKey) {
		t.Fatal("备用实现必须与插件共享同一权威键空间")
	}

	executions := 0
	first, executed, err := ledger.CommitWithEffect(ctx, plugin, authority, func(token executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		authority.put(t, executionjournal.EffectRecord{BusinessKey: businessKey(t, scope, plugin.IdempotencyKey), PayloadHash: plugin.PayloadHash, Token: token.Value, Result: ledgerResult("once")})
		return ledgerResult("once"), nil
	})
	if err != nil || !executed {
		t.Fatalf("首次写应执行: executed=%t err=%v", executed, err)
	}
	second, secondExecuted, err := ledger.CommitWithEffect(ctx, core, authority, func(executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		return ledgerResult("again"), nil
	})
	if err != nil {
		t.Fatalf("同键重试应返回既有结果: %v", err)
	}
	if secondExecuted || executions != 1 {
		t.Fatalf("同键重试不得再执行: executed=%t executions=%d", secondExecuted, executions)
	}
	if string(first.Result) != string(second.Result) || string(second.Result) != string(ledgerResult("once")) {
		t.Fatalf("两次返回结果必须一致: %s vs %s", first.Result, second.Result)
	}
	fmt.Printf("m04-s04 effect same-key executions=%d providers=2 result=%s\n", executions, second.Result)
}

func TestEffectPayloadConflictRejectsDifferentContent(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	authority := newFakeAuthority()
	ctx := context.Background()
	scope := ledgerScope("space-a")
	request := writeRequest(scope, "payload-key", "plugin.write.memory")

	executions := 0
	if _, _, err := ledger.CommitWithEffect(ctx, request, authority, func(token executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		authority.put(t, executionjournal.EffectRecord{BusinessKey: businessKey(t, scope, request.IdempotencyKey), PayloadHash: request.PayloadHash, Token: token.Value, Result: ledgerResult("first")})
		return ledgerResult("first"), nil
	}); err != nil {
		t.Fatal(err)
	}
	changed := request
	changed.PayloadHash = "payload-hash-2"
	_, executed, err := ledger.CommitWithEffect(ctx, changed, authority, func(executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		return nil, nil
	})
	if !errors.Is(err, executionjournal.ErrLedgerPayloadConflict) {
		t.Fatalf("同键不同内容必须拒绝，实际: %v", err)
	}
	if executed || executions != 1 {
		t.Fatalf("冲突请求不得执行: executed=%t executions=%d", executed, executions)
	}
	fmt.Printf("m04-s04 effect payload-conflict executions=%d conflict=%t\n", executions, errors.Is(err, executionjournal.ErrLedgerPayloadConflict))
}

func TestEffectAuthorityPayloadMismatchIsConflict(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	authority := newFakeAuthority()
	scope := ledgerScope("space-a")
	request := writeRequest(scope, "mismatch-key", "plugin.write.memory")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	executions := 0
	if _, _, err := ledger.CommitWithEffect(ctx, request, authority, func(token executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		authority.put(t, executionjournal.EffectRecord{BusinessKey: businessKey(t, scope, request.IdempotencyKey), PayloadHash: "other-payload", Token: token.Value, Result: ledgerResult("wrong")})
		go func() { time.Sleep(20 * time.Millisecond); cancel() }()
		<-ctx.Done()
		return nil, ctx.Err()
	}); !errors.Is(err, execution.ErrInvocationOutcomeUnknown) {
		t.Fatalf("需要先构造未知结果: %v", err)
	}
	_, replayed, err := ledger.CommitWithEffect(context.Background(), request, authority, func(executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		return ledgerResult("replay"), nil
	})
	if !errors.Is(err, executionjournal.ErrEffectRecordConflict) {
		t.Fatalf("权威记录的请求内容不一致必须拒绝复用，实际: %v", err)
	}
	if replayed || executions != 1 {
		t.Fatalf("冲突记录不得触发重放: replayed=%t executions=%d", replayed, executions)
	}
	fmt.Printf("m04-s04 effect authority-payload-mismatch executions=%d conflict=%t\n", executions, errors.Is(err, executionjournal.ErrEffectRecordConflict))
}

func TestEffectAuthorityTokenMustBeIssued(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	authority := newFakeAuthority()
	scope := ledgerScope("space-a")
	request := writeRequest(scope, "forged-key", "plugin.write.memory")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	executions := 0
	if _, _, err := ledger.CommitWithEffect(ctx, request, authority, func(executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		authority.put(t, executionjournal.EffectRecord{
			BusinessKey: businessKey(t, scope, request.IdempotencyKey),
			PayloadHash: request.PayloadHash,
			Token:       "v1.1." + strings.Repeat("ab", 32),
			Result:      ledgerResult("forged"),
		})
		go func() { time.Sleep(20 * time.Millisecond); cancel() }()
		<-ctx.Done()
		return nil, ctx.Err()
	}); !errors.Is(err, execution.ErrInvocationOutcomeUnknown) {
		t.Fatalf("需要先构造未知结果: %v", err)
	}
	_, replayed, err := ledger.CommitWithEffect(context.Background(), request, authority, func(executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		return ledgerResult("replay"), nil
	})
	if !errors.Is(err, executionjournal.ErrEffectRecordConflict) {
		t.Fatalf("伪造的效果凭据必须拒绝，实际: %v", err)
	}
	if replayed || executions != 1 {
		t.Fatalf("伪造凭据不得触发重放: replayed=%t executions=%d", replayed, executions)
	}
	fmt.Printf("m04-s04 effect authority-token-forged executions=%d rejected=%t\n", executions, errors.Is(err, executionjournal.ErrEffectRecordConflict))
}

func TestEffectAuthorityUnavailableBlocksRetry(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	authority := newFakeAuthority()
	scope := ledgerScope("space-a")
	request := writeRequest(scope, "down-key", "plugin.write.memory")
	ctx, cancel := context.WithCancel(context.Background())
	executions := 0
	if _, _, err := ledger.CommitWithEffect(ctx, request, authority, func(token executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		authority.put(t, executionjournal.EffectRecord{BusinessKey: businessKey(t, scope, request.IdempotencyKey), PayloadHash: request.PayloadHash, Token: token.Value, Result: ledgerResult("landed")})
		go func() { time.Sleep(20 * time.Millisecond); cancel() }()
		<-ctx.Done()
		return nil, ctx.Err()
	}); !errors.Is(err, execution.ErrInvocationOutcomeUnknown) {
		t.Fatalf("需要先构造未知结果: %v", err)
	}
	cancel()
	outage := errors.New("authority down")
	authority.mu.Lock()
	authority.fail = outage
	authority.mu.Unlock()
	_, replayed, err := ledger.CommitWithEffect(context.Background(), request, authority, func(executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		return ledgerResult("replay"), nil
	})
	if !errors.Is(err, executionjournal.ErrLedgerEffectUnknown) || !errors.Is(err, outage) {
		t.Fatalf("核对失败必须判为未知并保留原始错误，实际: %v", err)
	}
	if replayed || executions != 1 {
		t.Fatalf("核对失败不得重放: replayed=%t executions=%d", replayed, executions)
	}
	fmt.Printf("m04-s04 effect authority-unavailable executions=%d blocked=true cause=%v\n", executions, errors.Is(err, outage))
}

func TestEffectAuthorityAbsentAllowsRetry(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	authority := newFakeAuthority()
	scope := ledgerScope("space-a")
	request := writeRequest(scope, "absent-key", "plugin.write.memory")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	executions := 0
	if _, _, err := ledger.CommitWithEffect(ctx, request, authority, func(executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		go func() { time.Sleep(20 * time.Millisecond); cancel() }()
		<-ctx.Done()
		return nil, ctx.Err()
	}); !errors.Is(err, execution.ErrInvocationOutcomeUnknown) {
		t.Fatalf("需要先构造未知结果: %v", err)
	}
	entry, executed, err := ledger.CommitWithEffect(context.Background(), request, authority, func(token executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		authority.put(t, executionjournal.EffectRecord{BusinessKey: businessKey(t, scope, request.IdempotencyKey), PayloadHash: request.PayloadHash, Token: token.Value, Result: ledgerResult("second")})
		return ledgerResult("second"), nil
	})
	if err != nil || !executed {
		t.Fatalf("权威证明确未生效后才允许重试: executed=%t err=%v", executed, err)
	}
	if executions != 2 || entry.State != executionjournal.EffectCommitted {
		t.Fatalf("重试应执行且提交: executions=%d state=%s", executions, entry.State)
	}
	fmt.Printf("m04-s04 effect authority-absent executions=%d retrySafe=true state=%s\n", executions, entry.State)
}

func TestEffectCancelledBeforeRunIssuesNothing(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	authority := newFakeAuthority()
	scope := ledgerScope("space-a")
	request := writeRequest(scope, "cancel-key", "plugin.write.memory")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	executions := 0
	_, executed, err := ledger.CommitWithEffect(ctx, request, authority, func(executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		return ledgerResult("never"), nil
	})
	if err == nil {
		t.Fatal("已取消的上下文必须失败")
	}
	if executed || executions != 0 {
		t.Fatalf("取消后不得执行回调: executed=%t executions=%d", executed, executions)
	}
	if entry, found, lookupErr := ledger.Lookup(context.Background(), scope, request.IdempotencyKey); lookupErr != nil || found {
		t.Fatalf("取消后不得留下账本记录: found=%t state=%s err=%v", found, entry.State, lookupErr)
	}
	fmt.Printf("m04-s04 effect cancelled executions=0 recorded=false err=%v\n", errors.Is(err, context.Canceled))
}

func TestEffectRestartKeepsConfirmedResult(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	authority := newFakeAuthority()
	scope := ledgerScope("space-a")
	request := writeRequest(scope, "restart-key", "plugin.write.memory")
	executions := 0
	if _, _, err := ledger.CommitWithEffect(context.Background(), request, authority, func(token executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		authority.put(t, executionjournal.EffectRecord{BusinessKey: businessKey(t, scope, request.IdempotencyKey), PayloadHash: request.PayloadHash, Token: token.Value, Result: ledgerResult("persisted")})
		return ledgerResult("persisted"), nil
	}); err != nil {
		t.Fatal(err)
	}
	restarted, err := executionjournal.NewLedger(db)
	if err != nil {
		t.Fatal(err)
	}
	entry, executed, err := restarted.CommitWithEffect(context.Background(), request, authority, func(executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		return ledgerResult("after-restart"), nil
	})
	if err != nil {
		t.Fatalf("重启后应能读到既有结果: %v", err)
	}
	if executed || executions != 1 {
		t.Fatalf("重启后不得再执行: executed=%t executions=%d", executed, executions)
	}
	if string(entry.Result) != string(ledgerResult("persisted")) {
		t.Fatalf("重启后应返回持久化的结果，实际 %s", entry.Result)
	}
	fmt.Printf("m04-s04 effect restart executions=%d state=%s result=%s\n", executions, entry.State, entry.Result)
}

func TestEffectConcurrentSingleExecution(t *testing.T) {
	_, ledger := ledgerHarness(t, 8)
	authority := newFakeAuthority()
	scope := ledgerScope("space-a")
	request := writeRequest(scope, "concurrent-key", "plugin.write.memory")

	var executions int32
	var wg sync.WaitGroup
	results := make([]string, 8)
	errs := make([]error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			entry, _, err := ledger.CommitWithEffect(context.Background(), request, authority, func(token executionjournal.EffectToken) (json.RawMessage, error) {
				atomic.AddInt32(&executions, 1)
				authority.put(t, executionjournal.EffectRecord{BusinessKey: businessKey(t, scope, request.IdempotencyKey), PayloadHash: request.PayloadHash, Token: token.Value, Result: ledgerResult("once")})
				return ledgerResult("once"), nil
			})
			errs[index] = err
			results[index] = string(entry.Result)
		}(i)
	}
	wg.Wait()
	for index, err := range errs {
		if err != nil {
			t.Fatalf("并发调用 %d 失败: %v", index, err)
		}
		if results[index] != string(ledgerResult("once")) {
			t.Fatalf("并发调用 %d 结果不一致: %s", index, results[index])
		}
	}
	if got := atomic.LoadInt32(&executions); got != 1 {
		t.Fatalf("并发下只允许一次执行，实际 %d", got)
	}
	fmt.Printf("m04-s04 effect concurrent workers=8 executions=%d leaders=%d\n", executions, 1)
}

func TestEffectReconcileRejectsNilAuthority(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	ctx := context.Background()
	request := writeRequest(ledgerScope("space-a"), "nil-authority", "plugin.write.memory")
	if _, err := ledger.Reconcile(ctx, request, nil); !errors.Is(err, executionjournal.ErrEffectTokenInvalid) {
		t.Fatalf("缺少权威来源时必须拒绝核对，实际: %v", err)
	}
	executions := 0
	_, executed, err := ledger.CommitWithEffect(ctx, request, nil, func(executionjournal.EffectToken) (json.RawMessage, error) {
		executions++
		return ledgerResult("unsafe"), nil
	})
	if err == nil || executed || executions != 0 {
		t.Fatalf("没有权威来源时不得执行写: executed=%t executions=%d err=%v", executed, executions, err)
	}
	fmt.Printf("m04-s04 effect nil-authority executions=0 rejected=%t\n", errors.Is(err, executionjournal.ErrEffectTokenInvalid))
}

func TestEffectRetrySafeOnlyWhenAbsentOrRejected(t *testing.T) {
	cases := []struct {
		resolution executionjournal.EffectResolution
		want       bool
	}{
		{executionjournal.EffectResolvedAbsent, true},
		{executionjournal.EffectResolvedRejected, true},
		{executionjournal.EffectResolvedCommitted, false},
		{executionjournal.EffectResolvedUndetermined, false},
	}
	allowed := 0
	for _, testCase := range cases {
		if got := (executionjournal.Reconciliation{Resolution: testCase.resolution}).RetrySafe(); got != testCase.want {
			t.Fatalf("resolution=%s retrySafe=%t want %t", testCase.resolution, got, testCase.want)
		}
		if testCase.want {
			allowed++
		}
	}
	fmt.Printf("m04-s04 effect retry-safe resolutions=%d allowed=%d\n", len(cases), allowed)
}
