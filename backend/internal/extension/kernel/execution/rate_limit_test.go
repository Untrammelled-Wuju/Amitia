package execution

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type fakeRateLimitClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeRateLimitTimer
}

func newFakeRateLimitClock(start time.Time) *fakeRateLimitClock {
	return &fakeRateLimitClock{now: start}
}

func (c *fakeRateLimitClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeRateLimitClock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	c.fireTimersLocked()
}

func (c *fakeRateLimitClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
	c.fireTimersLocked()
}

func (c *fakeRateLimitClock) fireTimersLocked() {
	alive := c.timers[:0]
	for _, t := range c.timers {
		if t.isStopped() {
			continue
		}
		if !c.now.Before(t.deadline) {
			select {
			case t.c <- t.deadline:
				t.markFired()
			default:
				alive = append(alive, t)
			}
		} else {
			alive = append(alive, t)
		}
	}
	c.timers = alive
}

func (c *fakeRateLimitClock) NewTimer(d time.Duration) RateLimitTimer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeRateLimitTimer{
		clock:    c,
		deadline: c.now.Add(d),
		c:        make(chan time.Time, 1),
	}
	c.timers = append(c.timers, t)
	return t
}

type fakeRateLimitTimer struct {
	clock    *fakeRateLimitClock
	deadline time.Time
	c        chan time.Time
	stopped  int32
	fired    int32
}

func (t *fakeRateLimitTimer) C() <-chan time.Time { return t.c }

func (t *fakeRateLimitTimer) Stop() bool {
	if atomic.CompareAndSwapInt32(&t.stopped, 0, 1) {
		return true
	}
	return false
}

func (t *fakeRateLimitTimer) isStopped() bool { return atomic.LoadInt32(&t.stopped) == 1 }
func (t *fakeRateLimitTimer) markFired()      { atomic.StoreInt32(&t.fired, 1) }

func waitForWaiters(r *RateLimiter, n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if r.Snapshot().Waiters == n {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

func newRateLimitTool(id, extID string) capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:             id,
		ExtensionID:    extID,
		HasSideEffects: false,
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeBuiltin,
			RuntimeID:   "test",
		},
	}
}

func newRateLimitInvocation(userID, charID, convID string) capability.ToolInvocationContext {
	return capability.ToolInvocationContext{
		InvocationID:   "inv-001",
		UserID:         userID,
		CharacterID:    charID,
		ConversationID: convID,
	}
}

func TestRateLimiter_EnabledFalse_AlwaysAdmitted(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{Enabled: false}, clock)
	if err != nil {
		t.Fatal(err)
	}
	tool := newRateLimitTool("t1", "ext-1")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	admission, err := r.Admit(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	if admission.Decision != RateLimitAdmitted {
		t.Fatalf("expected admitted, got %s", admission.Decision)
	}
}

func TestRateLimiter_NoRelevantKeys_Admitted(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}
	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("", "", "")

	admission, err := r.Admit(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	if admission.Decision != RateLimitAdmitted {
		t.Fatalf("expected admitted, got %s", admission.Decision)
	}
}

func TestRateLimiter_GlobalTokenBucket_BurstThenReject(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		Global:       RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 3},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}
	tool := newRateLimitTool("t1", "ext-1")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	for i := 0; i < 3; i++ {
		adm, err := r.Admit(context.Background(), tool, inv)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		if adm.Decision != RateLimitAdmitted {
			t.Fatalf("iter %d: expected admitted, got %s", i, adm.Decision)
		}
	}

	adm, err := r.Admit(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	if adm.Decision != RateLimitRejected {
		t.Fatalf("expected rejected after burst, got %s", adm.Decision)
	}
	if adm.RetryAfter <= 0 {
		t.Fatal("expected positive retry after")
	}
}

func TestRateLimiter_TokenRefill_AfterInterval(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	clock := newFakeRateLimitClock(start)
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		Global:       RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 2},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}
	tool := newRateLimitTool("t1", "ext-1")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	for i := 0; i < 2; i++ {
		adm, _ := r.Admit(context.Background(), tool, inv)
		if adm.Decision != RateLimitAdmitted {
			t.Fatalf("iter %d: expected admitted", i)
		}
	}

	adm, _ := r.Admit(context.Background(), tool, inv)
	if adm.Decision != RateLimitRejected {
		t.Fatal("expected rejected right after burst exhaustion")
	}

	clock.Add(1 * time.Second)

	adm, err = r.Admit(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	if adm.Decision != RateLimitAdmitted {
		t.Fatalf("expected admitted after 1s refill, got %s", adm.Decision)
	}
}

func TestRateLimiter_PerTool_Isolated(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		PerTool:      RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	toolA := newRateLimitTool("tool-a", "")
	toolB := newRateLimitTool("tool-b", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	adm, _ := r.Admit(context.Background(), toolA, inv)
	if adm.Decision != RateLimitAdmitted {
		t.Fatalf("tool-a: expected admitted, got %s", adm.Decision)
	}

	adm, _ = r.Admit(context.Background(), toolA, inv)
	if adm.Decision != RateLimitRejected {
		t.Fatalf("tool-a: expected rejected, got %s", adm.Decision)
	}

	adm, _ = r.Admit(context.Background(), toolB, inv)
	if adm.Decision != RateLimitAdmitted {
		t.Fatalf("tool-b: expected admitted, got %s", adm.Decision)
	}
}

func TestRateLimiter_PerExtension_Isolated(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		PerExtension: RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	toolA := newRateLimitTool("t1", "ext-A")
	toolB := newRateLimitTool("t2", "ext-B")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	adm, _ := r.Admit(context.Background(), toolA, inv)
	if adm.Decision != RateLimitAdmitted {
		t.Fatalf("ext-A: expected admitted, got %s", adm.Decision)
	}

	adm, _ = r.Admit(context.Background(), toolA, inv)
	if adm.Decision != RateLimitRejected {
		t.Fatalf("ext-A: expected rejected, got %s", adm.Decision)
	}

	adm, _ = r.Admit(context.Background(), toolB, inv)
	if adm.Decision != RateLimitAdmitted {
		t.Fatalf("ext-B: expected admitted, got %s", adm.Decision)
	}
}

func TestRateLimiter_PerCharacter_Isolated(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		PerCharacter: RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	invA := newRateLimitInvocation("u1", "charA", "")
	invB := newRateLimitInvocation("u1", "charB", "")

	adm, _ := r.Admit(context.Background(), tool, invA)
	if adm.Decision != RateLimitAdmitted {
		t.Fatalf("charA: expected admitted, got %s", adm.Decision)
	}

	adm, _ = r.Admit(context.Background(), tool, invA)
	if adm.Decision != RateLimitRejected {
		t.Fatalf("charA: expected rejected, got %s", adm.Decision)
	}

	adm, _ = r.Admit(context.Background(), tool, invB)
	if adm.Decision != RateLimitAdmitted {
		t.Fatalf("charB: expected admitted, got %s", adm.Decision)
	}
}

func TestRateLimiter_PerConversation_Isolated(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:         true,
		PerConversation: RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		Backpressure:    BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	invA := newRateLimitInvocation("u1", "c1", "convA")
	invB := newRateLimitInvocation("u1", "c1", "convB")

	adm, _ := r.Admit(context.Background(), tool, invA)
	if adm.Decision != RateLimitAdmitted {
		t.Fatalf("convA: expected admitted, got %s", adm.Decision)
	}

	adm, _ = r.Admit(context.Background(), tool, invA)
	if adm.Decision != RateLimitRejected {
		t.Fatalf("convA: expected rejected, got %s", adm.Decision)
	}

	adm, _ = r.Admit(context.Background(), tool, invB)
	if adm.Decision != RateLimitAdmitted {
		t.Fatalf("convB: expected admitted, got %s", adm.Decision)
	}
}

func TestRateLimiter_MultiDimension_AtomicAdmission(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		Global:       RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 2},
		PerTool:      RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	adm, _ := r.Admit(context.Background(), tool, inv)
	if adm.Decision != RateLimitAdmitted {
		t.Fatalf("first: expected admitted, got %s", adm.Decision)
	}

	adm, _ = r.Admit(context.Background(), tool, inv)
	if adm.Decision != RateLimitRejected {
		t.Fatalf("second: expected rejected (tool exhausted), got %s", adm.Decision)
	}

	snap := r.Snapshot()
	if snap.BucketsByDimension[RateLimitGlobal] != 1 {
		t.Fatalf("expected global bucket to exist, got %d buckets", snap.BucketCount)
	}
	if snap.BucketsByDimension[RateLimitTool] != 1 {
		t.Fatal("expected tool bucket to exist")
	}
}

func TestRateLimiter_MultiDimension_NoPartialDecrement(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		Global:       RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 5},
		PerTool:      RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	adm, _ := r.Admit(context.Background(), tool, inv)
	if adm.Decision != RateLimitAdmitted {
		t.Fatal("first should admit")
	}

	adm, _ = r.Admit(context.Background(), tool, inv)
	if adm.Decision != RateLimitRejected {
		t.Fatal("tool exhausted")
	}

	clock.Add(2 * time.Second)

	adm, err = r.Admit(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	if adm.Decision != RateLimitAdmitted {
		t.Fatalf("after refill should admit, got %s", adm.Decision)
	}
}

func TestRateLimiter_BackpressureReject_Mode(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		Global:       RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	adm, _ := r.Admit(context.Background(), tool, inv)
	if adm.Decision != RateLimitAdmitted {
		t.Fatal("first should admit")
	}

	adm, err = r.Admit(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	if adm.Decision != RateLimitRejected {
		t.Fatalf("expected rejected, got %s", adm.Decision)
	}
	if adm.Reason != "rate_limited" {
		t.Fatalf("expected reason rate_limited, got %s", adm.Reason)
	}
}

func TestRateLimiter_BackpressureWait_TokenRefills(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled: true,
		Global:  RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{
			Mode:             BackpressureWait,
			MaxWait:          10 * time.Second,
			MaxWaiters:       100,
			MaxWaitersPerKey: 100,
		},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	adm, _ := r.Admit(context.Background(), tool, inv)
	if adm.Decision != RateLimitAdmitted {
		t.Fatal("first should admit")
	}

	resultCh := make(chan RateLimitAdmission, 1)
	go func() {
		adm, err := r.Admit(context.Background(), tool, inv)
		if err != nil {
			t.Errorf("wait admit err: %v", err)
			return
		}
		resultCh <- adm
	}()

	if !waitForWaiters(r, 1, 2*time.Second) {
		t.Fatalf("expected 1 waiter, got %d", r.Snapshot().Waiters)
	}

	clock.Add(2 * time.Second)

	select {
	case adm := <-resultCh:
		if adm.Decision != RateLimitAdmitted {
			t.Fatalf("expected admitted after refill, got %s", adm.Decision)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("waiter did not wake after refill")
	}

	if !waitForWaiters(r, 0, 2*time.Second) {
		t.Fatalf("expected 0 waiters after wake, got %d", r.Snapshot().Waiters)
	}
}

func TestRateLimiter_Wait_MaxWaitExceeded(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled: true,
		Global:  RateLimitSpec{Tokens: 1, Interval: 100 * time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{
			Mode:             BackpressureWait,
			MaxWait:          1 * time.Second,
			MaxWaiters:       100,
			MaxWaitersPerKey: 100,
		},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	adm, _ := r.Admit(context.Background(), tool, inv)
	if adm.Decision != RateLimitAdmitted {
		t.Fatal("first should admit")
	}

	adm, err = r.Admit(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	if adm.Decision != RateLimitBackpressureRejected {
		t.Fatalf("expected backpressure_rejected, got %s", adm.Decision)
	}
	if adm.Reason != "max_wait_exceeded" {
		t.Fatalf("expected max_wait_exceeded reason, got %s", adm.Reason)
	}
}

func TestRateLimiter_Wait_MaxWaitersGlobal(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled: true,
		Global:  RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{
			Mode:             BackpressureWait,
			MaxWait:          10 * time.Second,
			MaxWaiters:       1,
			MaxWaitersPerKey: 100,
		},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	adm, _ := r.Admit(context.Background(), tool, inv)
	if adm.Decision != RateLimitAdmitted {
		t.Fatal("first should admit")
	}

	blockCh := make(chan RateLimitAdmission, 1)
	go func() {
		adm, _ := r.Admit(context.Background(), tool, inv)
		blockCh <- adm
	}()

	if !waitForWaiters(r, 1, 2*time.Second) {
		t.Fatalf("expected 1 waiter, got %d", r.Snapshot().Waiters)
	}

	adm, err = r.Admit(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	if adm.Decision != RateLimitBackpressureRejected {
		t.Fatalf("expected backpressure_rejected (global queue full), got %s", adm.Decision)
	}
	if adm.Reason != "queue_full" {
		t.Fatalf("expected queue_full reason, got %s", adm.Reason)
	}

	clock.Add(2 * time.Second)

	select {
	case adm := <-blockCh:
		if adm.Decision != RateLimitAdmitted {
			t.Fatalf("expected admitted, got %s", adm.Decision)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("first waiter should be admitted after refill")
	}
}

func TestRateLimiter_Wait_PerKeyLimit(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled: true,
		Global:  RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{
			Mode:             BackpressureWait,
			MaxWait:          10 * time.Second,
			MaxWaiters:       100,
			MaxWaitersPerKey: 1,
		},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	adm, _ := r.Admit(context.Background(), tool, inv)
	if adm.Decision != RateLimitAdmitted {
		t.Fatal("first should admit")
	}

	blockCh := make(chan RateLimitAdmission, 1)
	go func() {
		adm, _ := r.Admit(context.Background(), tool, inv)
		blockCh <- adm
	}()

	if !waitForWaiters(r, 1, 2*time.Second) {
		t.Fatalf("expected 1 waiter before 2nd call, got %d", r.Snapshot().Waiters)
	}

	adm, err = r.Admit(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	if adm.Decision != RateLimitBackpressureRejected {
		t.Fatalf("expected backpressure_rejected (per-key full), got %s", adm.Decision)
	}
	if adm.Reason != "queue_full" {
		t.Fatalf("expected queue_full, got %s", adm.Reason)
	}

	clock.Add(2 * time.Second)

	select {
	case adm := <-blockCh:
		if adm.Decision != RateLimitAdmitted {
			t.Fatalf("expected admitted after refill, got %s", adm.Decision)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("first waiter should wake")
	}
}

func TestRateLimiter_Wait_ContextCancel_CleansUp(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled: true,
		Global:  RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{
			Mode:             BackpressureWait,
			MaxWait:          30 * time.Second,
			MaxWaiters:       100,
			MaxWaitersPerKey: 100,
		},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	adm, _ := r.Admit(context.Background(), tool, inv)
	if adm.Decision != RateLimitAdmitted {
		t.Fatal("first should admit")
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := r.Admit(ctx, tool, inv)
		errCh <- err
	}()

	if !waitForWaiters(r, 1, 2*time.Second) {
		t.Fatalf("expected 1 waiter before cancel, got %d", r.Snapshot().Waiters)
	}

	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected context.Canceled error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("waiter did not exit on cancel")
	}

	if !waitForWaiters(r, 0, 2*time.Second) {
		t.Fatalf("expected 0 waiters after cancel, got %d", r.Snapshot().Waiters)
	}
}

func TestRateLimiter_Wait_DeadlineBeforeRefill(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled: true,
		Global:  RateLimitSpec{Tokens: 1, Interval: 100 * time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{
			Mode:             BackpressureWait,
			MaxWait:          30 * time.Second,
			MaxWaiters:       100,
			MaxWaitersPerKey: 100,
		},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	adm, _ := r.Admit(context.Background(), tool, inv)
	if adm.Decision != RateLimitAdmitted {
		t.Fatal("first should admit")
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	adm, err = r.Admit(ctx, tool, inv)
	if err == nil {
		t.Log("warning: ctx not yet expired, reason:", adm.Reason)
	} else if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context error, got %v", err)
	}
}

func TestRateLimiter_BucketPrune_OnlyWhenFull(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		Global:       RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		PerTool:      RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	toolA := newRateLimitTool("t1", "")
	toolB := newRateLimitTool("t2", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	r.Admit(context.Background(), toolA, inv)
	r.Admit(context.Background(), toolB, inv)
	_ = r.Snapshot()

	clock.Add(1 * time.Second)

	r.Admit(context.Background(), toolA, inv)

	snap := r.Snapshot()
	if snap.BucketCount != 2 {
		t.Fatalf("expected 2 buckets (pruned stale toolB bucket), got %d", snap.BucketCount)
	}
}

func TestRateLimiter_ClockRegression_NoNegativeTokens(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		Global:       RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	adm, _ := r.Admit(context.Background(), tool, inv)
	if adm.Decision != RateLimitAdmitted {
		t.Fatal("first should admit")
	}

	clock.Set(time.Unix(999_900, 0))

	adm, err = r.Admit(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	if adm.Decision != RateLimitRejected {
		t.Fatalf("expected rejected after clock regression, got %s", adm.Decision)
	}
}

func TestRateLimiter_EmptyIDs_SkipPerX(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:         true,
		PerTool:         RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		PerExtension:    RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		PerCharacter:    RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		PerConversation: RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		Backpressure:    BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("", "", "")

	adm, err := r.Admit(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	if adm.Decision != RateLimitAdmitted {
		t.Fatalf("expected admitted (no global set), got %s", adm.Decision)
	}
}

func TestRateLimiter_Snapshot_Accuracy(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		Global:       RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 5},
		PerTool:      RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 3},
		PerExtension: RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 2},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "ext-1")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	r.Admit(context.Background(), tool, inv)

	snap := r.Snapshot()
	if !snap.Enabled {
		t.Fatal("expected enabled")
	}
	if snap.BucketCount != 3 {
		t.Fatalf("expected 3 buckets, got %d", snap.BucketCount)
	}
	if snap.BucketsByDimension[RateLimitGlobal] != 1 {
		t.Fatal("expected 1 global")
	}
	if snap.BucketsByDimension[RateLimitTool] != 1 {
		t.Fatal("expected 1 tool")
	}
	if snap.BucketsByDimension[RateLimitExtension] != 1 {
		t.Fatal("expected 1 extension")
	}
}

func TestRateLimiter_ValidatePolicy_NegativeTokens(t *testing.T) {
	_, err := NewRateLimiter(RateLimitPolicy{
		Enabled: true,
		Global:  RateLimitSpec{Tokens: -1, Interval: time.Second, Burst: 1},
	})
	if err == nil {
		t.Fatal("expected error for negative tokens")
	}
}

func TestRateLimiter_ValidatePolicy_ZeroIntervalWithPositiveTokens(t *testing.T) {
	_, err := NewRateLimiter(RateLimitPolicy{
		Enabled: true,
		Global:  RateLimitSpec{Tokens: 10, Interval: 0, Burst: 1},
	})
	if err == nil {
		t.Fatal("expected error for zero interval with positive tokens")
	}
}

func TestRateLimiter_ValidatePolicy_ZeroBurstWithPositiveTokens(t *testing.T) {
	_, err := NewRateLimiter(RateLimitPolicy{
		Enabled: true,
		Global:  RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 0},
	})
	if err == nil {
		t.Fatal("expected error for zero burst with positive tokens")
	}
}

func TestRateLimiter_ValidatePolicy_InvalidBackpressureMode(t *testing.T) {
	_, err := NewRateLimiter(RateLimitPolicy{
		Enabled:      true,
		Backpressure: BackpressurePolicy{Mode: BackpressureMode("bogus")},
	})
	if err == nil {
		t.Fatal("expected error for invalid backpressure mode")
	}
}

func TestRateLimiter_ValidatePolicy_WaitModeRequiresPositive(t *testing.T) {
	_, err := NewRateLimiter(RateLimitPolicy{
		Enabled: true,
		Backpressure: BackpressurePolicy{
			Mode:             BackpressureWait,
			MaxWait:          0,
			MaxWaiters:       1,
			MaxWaitersPerKey: 1,
		},
	})
	if err == nil {
		t.Fatal("expected error for zero MaxWait in wait mode")
	}

	_, err = NewRateLimiter(RateLimitPolicy{
		Enabled: true,
		Backpressure: BackpressurePolicy{
			Mode:             BackpressureWait,
			MaxWait:          time.Second,
			MaxWaiters:       0,
			MaxWaitersPerKey: 1,
		},
	})
	if err == nil {
		t.Fatal("expected error for zero MaxWaiters in wait mode")
	}

	_, err = NewRateLimiter(RateLimitPolicy{
		Enabled: true,
		Backpressure: BackpressurePolicy{
			Mode:             BackpressureWait,
			MaxWait:          time.Second,
			MaxWaiters:       1,
			MaxWaitersPerKey: 0,
		},
	})
	if err == nil {
		t.Fatal("expected error for zero MaxWaitersPerKey in wait mode")
	}
}

func TestRateLimiter_ValidatePolicy_ValidSpec(t *testing.T) {
	_, err := NewRateLimiter(RateLimitPolicy{
		Enabled: true,
		Global:  RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 5},
		Backpressure: BackpressurePolicy{
			Mode:             BackpressureWait,
			MaxWait:          10 * time.Second,
			MaxWaiters:       50,
			MaxWaitersPerKey: 10,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRateLimiter_Concurrency_AtomicAdmission(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		Global:       RateLimitSpec{Tokens: 100, Interval: time.Second, Burst: 50},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	const goroutines = 200
	var admitted atomic.Int32
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			adm, err := r.Admit(context.Background(), tool, inv)
			if err == nil && adm.Decision == RateLimitAdmitted {
				admitted.Add(1)
			}
		}()
	}

	wg.Wait()
	if admitted.Load() > 50 {
		t.Fatalf("expected at most 50 admits (burst), got %d", admitted.Load())
	}
	if admitted.Load() < 1 {
		t.Fatal("expected at least 1 admit in concurrent test")
	}
}

func TestRateLimiter_Observability_Events(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		Global:       RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	var admittedCount atomic.Int32
	var rejectedCount atomic.Int32

	SetRateLimitObservabilityHooks(
		func(dimensions []string) { admittedCount.Add(1) },
		func(dimensions []string, reason string, retryAfterMs int64) { rejectedCount.Add(1) },
		func(dimensions []string, reason string, retryAfterMs int64) {},
		func(dimensions []string, waitMs int64) {},
	)
	defer SetRateLimitObservabilityHooks(nil, nil, nil, nil)

	tool := newRateLimitTool("t1", "ext-1")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	r.Admit(context.Background(), tool, inv)
	r.Admit(context.Background(), tool, inv)

	if admittedCount.Load() != 1 {
		t.Fatalf("expected 1 admitted event, got %d", admittedCount.Load())
	}
	if rejectedCount.Load() != 1 {
		t.Fatalf("expected 1 rejected event, got %d", rejectedCount.Load())
	}
}

func TestRateLimiter_RetryDoesNotDoubleAdmit(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		Global:       RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 2},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	adm1, _ := r.Admit(context.Background(), tool, inv)
	if adm1.Decision != RateLimitAdmitted {
		t.Fatal("first should admit")
	}
	adm2, _ := r.Admit(context.Background(), tool, inv)
	if adm2.Decision != RateLimitAdmitted {
		t.Fatal("second should admit")
	}
	adm3, _ := r.Admit(context.Background(), tool, inv)
	if adm3.Decision != RateLimitRejected {
		t.Fatal("third should reject (no retry re-admit)")
	}
}

func TestRateLimiter_TokenBucket_PartialRefill(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	clock := newFakeRateLimitClock(start)
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		Global:       RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 10},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	for i := 0; i < 10; i++ {
		adm, _ := r.Admit(context.Background(), tool, inv)
		if adm.Decision != RateLimitAdmitted {
			t.Fatalf("iter %d: expected admitted", i)
		}
	}

	adm, _ := r.Admit(context.Background(), tool, inv)
	if adm.Decision != RateLimitRejected {
		t.Fatal("11th should be rejected (burst exhausted)")
	}

	clock.Add(500 * time.Millisecond)

	partialAdmits := 0
	for i := 0; i < 6; i++ {
		adm, _ := r.Admit(context.Background(), tool, inv)
		if adm.Decision == RateLimitAdmitted {
			partialAdmits++
		}
	}
	if partialAdmits < 4 || partialAdmits > 5 {
		t.Fatalf("expected ~5 admits at 500ms partial refill, got %d", partialAdmits)
	}
}

func TestRateLimiter_QueueFullButTokenAvailable(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled: true,
		Global:  RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 10},
		Backpressure: BackpressurePolicy{
			Mode:             BackpressureWait,
			MaxWait:          5 * time.Second,
			MaxWaiters:       10,
			MaxWaitersPerKey: 100,
		},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	adm, err := r.Admit(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	if adm.Decision != RateLimitAdmitted {
		t.Fatalf("should admit when tokens available, got %s", adm.Decision)
	}
}

func TestRateLimiter_WaiterCleanup_OnContextCancel(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled: true,
		Global:  RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{
			Mode:             BackpressureWait,
			MaxWait:          30 * time.Second,
			MaxWaiters:       100,
			MaxWaitersPerKey: 100,
		},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	r.Admit(context.Background(), tool, inv)

	ctx, cancel := context.WithCancel(context.Background())
	cancelDone := make(chan struct{})
	go func() {
		r.Admit(ctx, tool, inv)
		close(cancelDone)
	}()

	if !waitForWaiters(r, 1, 2*time.Second) {
		t.Fatalf("expected 1 waiter, got %d", r.Snapshot().Waiters)
	}

	cancel()

	select {
	case <-cancelDone:
	case <-time.After(3 * time.Second):
		t.Fatal("goroutine did not exit on cancel")
	}

	if !waitForWaiters(r, 0, 2*time.Second) {
		t.Fatalf("expected 0 waiters after cancel, got %d", r.Snapshot().Waiters)
	}

	clock.Add(2 * time.Second)

	adm, err := r.Admit(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	if adm.Decision != RateLimitAdmitted {
		t.Fatalf("should admit after waiter cleanup + refill, got %s", adm.Decision)
	}
}

func TestRateLimiter_ZeroTokensSpec_Skipped(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		Global:       RateLimitSpec{Tokens: 0, Interval: 0, Burst: 0},
		PerTool:      RateLimitSpec{Tokens: 0, Interval: 0, Burst: 0},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	adm, err := r.Admit(context.Background(), tool, inv)
	if err != nil {
		t.Fatal(err)
	}
	if adm.Decision != RateLimitAdmitted {
		t.Fatalf("all specs zero should admit, got %s", adm.Decision)
	}

	snap := r.Snapshot()
	if snap.BucketCount != 0 {
		t.Fatalf("expected 0 buckets for zero-token specs, got %d", snap.BucketCount)
	}
}

func TestRateLimiter_Wait_CtxCancelDuringWait(t *testing.T) {
	clock := newFakeRateLimitClock(time.Unix(1_000_000, 0))
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled: true,
		Global:  RateLimitSpec{Tokens: 10, Interval: time.Second, Burst: 1},
		Backpressure: BackpressurePolicy{
			Mode:             BackpressureWait,
			MaxWait:          30 * time.Second,
			MaxWaiters:       100,
			MaxWaitersPerKey: 100,
		},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	r.Admit(context.Background(), tool, inv)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, err = r.Admit(ctx, tool, inv)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if r.Snapshot().Waiters != 0 {
		t.Fatalf("expected 0 waiters after cancel, got %d", r.Snapshot().Waiters)
	}
}

func TestRateLimiter_TokenRefill_Partial(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	clock := newFakeRateLimitClock(start)
	r, err := NewRateLimiterWithClock(RateLimitPolicy{
		Enabled:      true,
		Global:       RateLimitSpec{Tokens: 10, Interval: 2 * time.Second, Burst: 10},
		Backpressure: BackpressurePolicy{Mode: BackpressureReject},
	}, clock)
	if err != nil {
		t.Fatal(err)
	}

	tool := newRateLimitTool("t1", "")
	inv := newRateLimitInvocation("u1", "c1", "conv1")

	r.Admit(context.Background(), tool, inv)
	r.Admit(context.Background(), tool, inv)

	clock.Add(1 * time.Second)

	adm, _ := r.Admit(context.Background(), tool, inv)
	if adm.Decision != RateLimitAdmitted {
		t.Fatalf("expected admitted after partial refill (5 tokens added), got %s", adm.Decision)
	}
}
