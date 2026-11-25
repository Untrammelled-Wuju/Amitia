package trusted_service

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type breakerClock struct {
	mu sync.Mutex
	at time.Time
}

func newBreakerClock() *breakerClock {
	return &breakerClock{at: time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)}
}

func (c *breakerClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *breakerClock) Advance(step time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(step)
}

func breakerConfig(threshold int, recovery time.Duration, probes int) CircuitConfig {
	return CircuitConfig{
		FailureThreshold:  threshold,
		RecoveryAfter:     recovery,
		HalfOpenAttempts:  probes,
		ResetAfterHealthy: 2 * time.Minute,
	}
}

func scopedFixture(t *testing.T, config CircuitConfig) (*ScopedBreakers, *breakerClock) {
	t.Helper()
	clock := newBreakerClock()
	breakers := newScopedBreakers(config, clock.Now)
	if breakers == nil {
		t.Fatal("expected a scoped breaker registry")
	}
	return breakers, clock
}

func breakerKey(scope, service, provider string) BreakerKey {
	return BreakerKey{ScopeKey: scope, ServiceID: service, ProviderID: provider}
}

func trip(t *testing.T, breakers *ScopedBreakers, key BreakerKey, failures int) {
	t.Helper()
	for i := 0; i < failures; i++ {
		if err := breakers.Failure(key); err != nil {
			t.Fatalf("failure %d on %v: %v", i, key, err)
		}
	}
}

func requireState(t *testing.T, breakers *ScopedBreakers, key BreakerKey, want CircuitState) {
	t.Helper()
	state, err := breakers.State(key)
	if err != nil {
		t.Fatalf("state %v: %v", key, err)
	}
	if state != want {
		t.Fatalf("state %v = %s want %s", key, state, want)
	}
}

func TestScopedBreakerRejectsInvalidKeys(t *testing.T) {
	breakers, _ := scopedFixture(t, breakerConfig(3, 30*time.Second, 1))
	invalid := []BreakerKey{
		{},
		{ScopeKey: "scope"},
		{ScopeKey: "scope", ServiceID: "chat"},
		{ScopeKey: "   ", ServiceID: "chat", ProviderID: "p1"},
		{ScopeKey: "sc\x1eope", ServiceID: "chat", ProviderID: "p1"},
		{ScopeKey: "scope", ServiceID: "ch\x1eat", ProviderID: "p1"},
		{ScopeKey: "scope", ServiceID: "chat", ProviderID: string(make([]byte, 257))},
	}
	for index, key := range invalid {
		if err := breakers.Acquire(key); !errors.Is(err, ErrBreakerKeyInvalid) {
			t.Fatalf("case %d: expected ErrBreakerKeyInvalid, got %v", index, err)
		}
	}
	normalized, err := breakerKey("  scope  ", " chat ", " p1 ").Normalize()
	if err != nil {
		t.Fatalf("normalize valid key: %v", err)
	}
	if normalized.ScopeKey != "scope" || normalized.ServiceID != "chat" || normalized.ProviderID != "p1" {
		t.Fatalf("normalized key = %+v", normalized)
	}
	if breakers.Len() != 0 {
		t.Fatalf("invalid keys created %d breakers", breakers.Len())
	}
	fmt.Printf("m04-s06 breaker invalid-keys invalid=%d accepted=1 trimmed=true registered=%d\n", len(invalid), breakers.Len())
}

func TestScopedBreakerTripStaysInScope(t *testing.T) {
	breakers, _ := scopedFixture(t, breakerConfig(3, 30*time.Second, 1))
	tripped := breakerKey("scope-a", "chat", "plugin-a")
	trip(t, breakers, tripped, 3)
	requireState(t, breakers, tripped, CircuitOpen)
	requireState(t, breakers, breakerKey("scope-b", "chat", "plugin-a"), CircuitClosed)
	requireState(t, breakers, breakerKey("scope-a", "memory", "plugin-a"), CircuitClosed)
	requireState(t, breakers, breakerKey("scope-a", "chat", "plugin-b"), CircuitClosed)
	fmt.Printf("m04-s06 breaker scope-isolation tripped=open otherScope=closed otherService=closed otherProvider=closed\n")
}

func TestScopedBreakerThresholdBoundary(t *testing.T) {
	breakers, _ := scopedFixture(t, breakerConfig(3, 30*time.Second, 1))
	key := breakerKey("scope-a", "chat", "plugin-a")
	trip(t, breakers, key, 2)
	requireState(t, breakers, key, CircuitClosed)
	trip(t, breakers, key, 1)
	requireState(t, breakers, key, CircuitOpen)
	trip(t, breakers, key, 1)
	requireState(t, breakers, key, CircuitOpen)
	failures, err := breakers.Failures(key)
	if err != nil {
		t.Fatalf("failures: %v", err)
	}
	if failures != 4 {
		t.Fatalf("failures = %d want 4", failures)
	}
	fmt.Printf("m04-s06 breaker threshold threshold=3 closedAt=2 openAt=3 openAfterExtra=true failures=%d\n", failures)
}

func TestScopedBreakerHalfOpenProbeCap(t *testing.T) {
	breakers, clock := scopedFixture(t, breakerConfig(2, 10*time.Second, 2))
	key := breakerKey("scope-a", "chat", "plugin-a")
	trip(t, breakers, key, 2)
	clock.Advance(10 * time.Second)
	for i := 0; i < 2; i++ {
		if err := breakers.Acquire(key); err != nil {
			t.Fatalf("probe %d: %v", i, err)
		}
	}
	denied := 0
	for i := 0; i < 1; i++ {
		if err := breakers.Acquire(key); !errors.Is(err, ErrCircuitHalfOpenExhausted) {
			t.Fatalf("probe beyond cap: expected ErrCircuitHalfOpenExhausted, got %v", err)
		}
		denied++
	}
	probes, err := breakers.Probes(key)
	if err != nil {
		t.Fatalf("probes: %v", err)
	}
	if probes != 2 {
		t.Fatalf("probes = %d want 2", probes)
	}
	requireState(t, breakers, key, CircuitHalfOpen)
	fmt.Printf("m04-s06 breaker probe-cap probes=%d denied=%d state=half_open\n", probes, denied)
}

func TestScopedBreakerHalfOpenSuccessCloses(t *testing.T) {
	breakers, clock := scopedFixture(t, breakerConfig(2, 10*time.Second, 2))
	key := breakerKey("scope-a", "chat", "plugin-a")
	trip(t, breakers, key, 2)
	clock.Advance(10 * time.Second)
	for i := 0; i < 2; i++ {
		if err := breakers.Acquire(key); err != nil {
			t.Fatalf("probe %d: %v", i, err)
		}
		if err := breakers.Success(key); err != nil {
			t.Fatalf("success %d: %v", i, err)
		}
	}
	requireState(t, breakers, key, CircuitClosed)
	failures, err := breakers.Failures(key)
	if err != nil {
		t.Fatalf("failures: %v", err)
	}
	probes, err := breakers.Probes(key)
	if err != nil {
		t.Fatalf("probes: %v", err)
	}
	if failures != 0 || probes != 0 {
		t.Fatalf("failures=%d probes=%d want 0/0", failures, probes)
	}
	fmt.Printf("m04-s06 breaker probe-success successes=2 state=closed failures=0 probes=0\n")
}

func TestScopedBreakerHalfOpenFailureReopens(t *testing.T) {
	breakers, clock := scopedFixture(t, breakerConfig(2, 10*time.Second, 2))
	key := breakerKey("scope-a", "chat", "plugin-a")
	trip(t, breakers, key, 2)
	clock.Advance(10 * time.Second)
	if err := breakers.Acquire(key); err != nil {
		t.Fatalf("probe: %v", err)
	}
	if err := breakers.Failure(key); err != nil {
		t.Fatalf("failure: %v", err)
	}
	requireState(t, breakers, key, CircuitOpen)
	probes, err := breakers.Probes(key)
	if err != nil {
		t.Fatalf("probes: %v", err)
	}
	if probes != 0 {
		t.Fatalf("probes = %d want 0 after reopen", probes)
	}
	if err := breakers.Acquire(key); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen after reopen, got %v", err)
	}
	fmt.Printf("m04-s06 breaker probe-failure reopened=true probes=0 denied=1 state=open\n")
}

func TestScopedBreakerAbortReleasesProbe(t *testing.T) {
	breakers, clock := scopedFixture(t, breakerConfig(2, 10*time.Second, 1))
	key := breakerKey("scope-a", "chat", "plugin-a")
	trip(t, breakers, key, 2)
	clock.Advance(10 * time.Second)
	for i := 0; i < 3; i++ {
		if err := breakers.Acquire(key); err != nil {
			t.Fatalf("probe %d after abort: %v", i, err)
		}
		if err := breakers.Abort(key); err != nil {
			t.Fatalf("abort %d: %v", i, err)
		}
	}
	probes, err := breakers.Probes(key)
	if err != nil {
		t.Fatalf("probes: %v", err)
	}
	if probes != 0 {
		t.Fatalf("probes = %d want 0", probes)
	}
	requireState(t, breakers, key, CircuitHalfOpen)
	fmt.Printf("m04-s06 breaker probe-abort granted=3 aborted=3 probes=0 state=half_open\n")
}

func TestScopedBreakerConcurrentProbesSingle(t *testing.T) {
	breakers, clock := scopedFixture(t, breakerConfig(2, 10*time.Second, 1))
	key := breakerKey("scope-a", "chat", "plugin-a")
	trip(t, breakers, key, 2)
	clock.Advance(10 * time.Second)
	var granted atomic.Int64
	var denied atomic.Int64
	var other atomic.Int64
	var group sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 16; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			switch err := breakers.Acquire(key); {
			case err == nil:
				granted.Add(1)
			case errors.Is(err, ErrCircuitHalfOpenExhausted):
				denied.Add(1)
			default:
				other.Add(1)
			}
		}()
	}
	close(start)
	group.Wait()
	if granted.Load() != 1 || denied.Load() != 15 || other.Load() != 0 {
		t.Fatalf("granted=%d denied=%d other=%d", granted.Load(), denied.Load(), other.Load())
	}
	fmt.Printf("m04-s06 breaker concurrent-probe workers=16 granted=1 denied=15 other=0\n")
}

func TestScopedBreakerConcurrentKeysIsolated(t *testing.T) {
	breakers, _ := scopedFixture(t, breakerConfig(4, 30*time.Second, 1))
	keys := make([]BreakerKey, 0, 8)
	for _, scope := range []string{"scope-a", "scope-b"} {
		for _, service := range []string{"chat", "memory"} {
			for _, provider := range []string{"plugin-a", "plugin-b"} {
				keys = append(keys, breakerKey(scope, service, provider))
			}
		}
	}
	var group sync.WaitGroup
	for _, key := range keys {
		group.Add(1)
		go func(key BreakerKey) {
			defer group.Done()
			for i := 0; i < 4; i++ {
				if err := breakers.Failure(key); err != nil {
					t.Errorf("failure %v: %v", key, err)
					return
				}
			}
		}(key)
	}
	group.Wait()
	openCount := 0
	for _, key := range keys {
		requireState(t, breakers, key, CircuitOpen)
		failures, err := breakers.Failures(key)
		if err != nil || failures != 4 {
			t.Fatalf("failures %v = %d err=%v", key, failures, err)
		}
		openCount++
	}
	requireState(t, breakers, breakerKey("scope-a", "chat", "plugin-c"), CircuitClosed)
	fmt.Printf("m04-s06 breaker concurrent-keys keys=%d open=%d failures=4 untouched=closed\n", len(keys), openCount)
}

func TestScopedBreakerOpenInScopeLists(t *testing.T) {
	breakers, _ := scopedFixture(t, breakerConfig(2, 30*time.Second, 1))
	trip(t, breakers, breakerKey("scope-a", "chat", "plugin-b"), 2)
	trip(t, breakers, breakerKey("scope-a", "memory", "plugin-a"), 2)
	trip(t, breakers, breakerKey("scope-b", "chat", "plugin-a"), 2)
	if err := breakers.Success(breakerKey("scope-a", "chat", "plugin-c")); err != nil {
		t.Fatalf("success: %v", err)
	}
	scopeA, err := breakers.OpenInScope(" scope-a ")
	if err != nil {
		t.Fatalf("open scope-a: %v", err)
	}
	scopeB, err := breakers.OpenInScope("scope-b")
	if err != nil {
		t.Fatalf("open scope-b: %v", err)
	}
	missing, err := breakers.OpenInScope("scope-c")
	if err != nil {
		t.Fatalf("open scope-c: %v", err)
	}
	if len(scopeA) != 2 || len(scopeB) != 1 || len(missing) != 0 {
		t.Fatalf("scope-a=%d scope-b=%d scope-c=%d", len(scopeA), len(scopeB), len(missing))
	}
	if scopeA[0].String() >= scopeA[1].String() {
		t.Fatalf("open list is not ordered: %v", scopeA)
	}
	if _, err := breakers.OpenInScope("  "); !errors.Is(err, ErrBreakerKeyInvalid) {
		t.Fatalf("expected ErrBreakerKeyInvalid for empty scope, got %v", err)
	}
	fmt.Printf("m04-s06 breaker open-list scope-a=2 scope-b=1 missing=0 order=ascending rejected=true\n")
}

func TestScopedBreakerResetIsLocal(t *testing.T) {
	breakers, _ := scopedFixture(t, breakerConfig(3, 30*time.Second, 1))
	first := breakerKey("scope-a", "chat", "plugin-a")
	second := breakerKey("scope-a", "chat", "plugin-b")
	trip(t, breakers, first, 3)
	trip(t, breakers, second, 3)
	if err := breakers.Reset(first); err != nil {
		t.Fatalf("reset: %v", err)
	}
	requireState(t, breakers, first, CircuitClosed)
	requireState(t, breakers, second, CircuitOpen)
	failures, err := breakers.Failures(second)
	if err != nil || failures != 3 {
		t.Fatalf("peer failures = %d err=%v", failures, err)
	}
	fmt.Printf("m04-s06 breaker reset-local reset=closed peer=open peerFailures=3\n")
}

func TestScopedBreakerSnapshotOrderStable(t *testing.T) {
	breakers, _ := scopedFixture(t, breakerConfig(2, 30*time.Second, 1))
	keys := []BreakerKey{
		breakerKey("scope-b", "memory", "plugin-z"),
		breakerKey("scope-a", "memory", "plugin-a"),
		breakerKey("scope-b", "chat", "plugin-a"),
		breakerKey("scope-a", "chat", "plugin-a"),
	}
	for _, key := range keys {
		if err := breakers.Acquire(key); err != nil {
			t.Fatalf("acquire %v: %v", key, err)
		}
	}
	trip(t, breakers, breakerKey("scope-a", "chat", "plugin-a"), 2)
	first := breakers.Snapshot()
	second := breakers.Snapshot()
	if len(first) != 4 {
		t.Fatalf("snapshot entries = %d want 4", len(first))
	}
	for i := 1; i < len(first); i++ {
		if first[i-1].Key.String() >= first[i].Key.String() {
			t.Fatalf("snapshot is not sorted at %d", i)
		}
	}
	if fmt.Sprint(first) != fmt.Sprint(second) {
		t.Fatalf("snapshot is not stable: %v vs %v", first, second)
	}
	if first[0].State != CircuitOpen {
		t.Fatalf("first entry state = %s want open", first[0].State)
	}
	fmt.Printf("m04-s06 breaker snapshot entries=4 sorted=true stable=true open=1\n")
}

func TestScopedBreakerConfigDefaults(t *testing.T) {
	breakers, _ := scopedFixture(t, CircuitConfig{})
	key := breakerKey("scope-a", "chat", "plugin-a")
	trip(t, breakers, key, 4)
	requireState(t, breakers, key, CircuitClosed)
	trip(t, breakers, key, 1)
	requireState(t, breakers, key, CircuitOpen)
	failures, err := breakers.Failures(key)
	if err != nil || failures != 5 {
		t.Fatalf("failures = %d err=%v want 5", failures, err)
	}
	fmt.Printf("m04-s06 breaker config-defaults threshold=5 openAt=5 failures=%d\n", failures)
}

func TestScopedBreakerClosedStateDoesNotConsumeProbes(t *testing.T) {
	breakers, clock := scopedFixture(t, breakerConfig(2, 10*time.Second, 1))
	key := breakerKey("scope-a", "chat", "plugin-a")
	for i := 0; i < 5; i++ {
		if err := breakers.Acquire(key); err != nil {
			t.Fatalf("closed acquire %d: %v", i, err)
		}
	}
	probes, err := breakers.Probes(key)
	if err != nil || probes != 0 {
		t.Fatalf("probes = %d err=%v want 0", probes, err)
	}
	trip(t, breakers, key, 2)
	clock.Advance(10 * time.Second)
	requireState(t, breakers, key, CircuitHalfOpen)
	probes, err = breakers.Probes(key)
	if err != nil || probes != 0 {
		t.Fatalf("half-open probes = %d err=%v want 0", probes, err)
	}
	fmt.Printf("m04-s06 breaker closed-no-probe closedAcquires=5 probes=0 halfOpenStart=0\n")
}
