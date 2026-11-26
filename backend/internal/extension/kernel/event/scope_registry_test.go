//go:build rsi_eventing

package event

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func scopedTestHandler(calls *atomic.Int64) ScopedHandler {
	return ScopedHandlerFunc(func(ctx context.Context, event ScopedEvent) error {
		calls.Add(1)
		return nil
	})
}

func TestScopedSubscribeIsIdempotent(t *testing.T) {
	registry := NewScopedRegistry()
	var calls atomic.Int64
	registration := ScopedRegistration{
		Topic:   "chat.message",
		Scope:   "device:a",
		Owner:   "plugin-a",
		Filter:  ScopeFilter{"kind": "chat"},
		Handler: scopedTestHandler(&calls),
	}
	first, created, err := registry.Subscribe(context.Background(), registration)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if !created || first.Generation != 1 {
		t.Fatalf("first subscribe created=%v generation=%d", created, first.Generation)
	}
	second, created, err := registry.Subscribe(context.Background(), registration)
	if err != nil {
		t.Fatalf("duplicate subscribe: %v", err)
	}
	if created {
		t.Fatalf("duplicate subscribe reported created")
	}
	if second.Generation != 1 || second.Duplicates != 1 {
		t.Fatalf("duplicate generation=%d duplicates=%d", second.Generation, second.Duplicates)
	}
	if registry.Len() != 1 {
		t.Fatalf("subscriptions=%d", registry.Len())
	}
	fmt.Printf("m07-s02 subscribe created=true generation=1\n")
	fmt.Printf("m07-s02 duplicate created=false generation=1 subscriptions=1\n")
}

func TestScopedSubscribeConflict(t *testing.T) {
	registry := NewScopedRegistry()
	base := ScopedRegistration{
		Topic:   "chat.message",
		Scope:   "device:a",
		Owner:   "plugin-a",
		Filter:  ScopeFilter{"kind": "chat"},
		Handler: ScopedHandlerFunc(func(context.Context, ScopedEvent) error { return nil }),
	}
	if _, _, err := registry.Subscribe(context.Background(), base); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	changed := base
	changed.Filter = ScopeFilter{"kind": "system"}
	_, _, err := registry.Subscribe(context.Background(), changed)
	if !errors.Is(err, ErrScopedSubscriptionConflict) {
		t.Fatalf("conflict err=%v", err)
	}
	view, ok := registry.Get(base.key())
	if !ok || view.Filter != "kind=chat" || registry.Len() != 1 {
		t.Fatalf("after conflict view=%+v ok=%v len=%d", view, ok, registry.Len())
	}
	fmt.Printf("m07-s02 conflict rejected=true filter=%q\n", view.Filter)
}

func TestScopedSubscribeInvalid(t *testing.T) {
	registry := NewScopedRegistry()
	handler := ScopedHandlerFunc(func(context.Context, ScopedEvent) error { return nil })
	cases := []ScopedRegistration{
		{Topic: "", Scope: "device:a", Owner: "plugin-a", Handler: handler},
		{Topic: "chat.message", Scope: " ", Owner: "plugin-a", Handler: handler},
		{Topic: "chat.message", Scope: "device:a", Owner: "", Handler: handler},
		{Topic: "chat.message", Scope: "device:a", Owner: "plugin-a", Handler: nil},
	}
	rejected := 0
	for _, registration := range cases {
		if _, _, err := registry.Subscribe(context.Background(), registration); !errors.Is(err, ErrScopedSubscriptionInvalid) {
			t.Fatalf("invalid registration err=%v", err)
		}
		rejected++
	}
	if registry.Len() != 0 {
		t.Fatalf("subscriptions=%d", registry.Len())
	}
	fmt.Printf("m07-s02 reject invalid=%d subscriptions=0\n", rejected)
}

func TestScopedUnsubscribeIsIdempotent(t *testing.T) {
	registry := NewScopedRegistry()
	registration := ScopedRegistration{
		Topic:   "chat.message",
		Scope:   "device:a",
		Owner:   "plugin-a",
		Handler: ScopedHandlerFunc(func(context.Context, ScopedEvent) error { return nil }),
	}
	if _, _, err := registry.Subscribe(context.Background(), registration); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	key := registration.key()
	first, err := registry.Unsubscribe(context.Background(), key)
	if err != nil || !first {
		t.Fatalf("first unsubscribe removed=%v err=%v", first, err)
	}
	second, err := registry.Unsubscribe(context.Background(), key)
	if err != nil || second {
		t.Fatalf("second unsubscribe removed=%v err=%v", second, err)
	}
	if registry.Len() != 0 {
		t.Fatalf("subscriptions=%d", registry.Len())
	}
	fmt.Printf("m07-s02 unsubscribe first=true second=false subscriptions=0\n")
}

func TestScopedDeliverOncePerKey(t *testing.T) {
	registry := NewScopedRegistry()
	var calls atomic.Int64
	registration := ScopedRegistration{
		Topic:   "chat.message",
		Scope:   "device:a",
		Owner:   "plugin-a",
		Handler: scopedTestHandler(&calls),
	}
	for i := 0; i < 50; i++ {
		if _, _, err := registry.Subscribe(context.Background(), registration); err != nil {
			t.Fatalf("subscribe %d: %v", i, err)
		}
	}
	perEvent := map[string]int{}
	handler := ScopedHandlerFunc(func(ctx context.Context, event ScopedEvent) error {
		perEvent[event.ID]++
		calls.Add(1)
		return nil
	})
	key := registration.key()
	registry.mu.Lock()
	registry.entries[key].handler = handler
	registry.mu.Unlock()
	for i := 0; i < 10; i++ {
		event := ScopedEvent{ID: fmt.Sprintf("evt-%02d", i), Topic: "chat.message", Scope: "device:a"}
		if _, err := registry.Dispatch(context.Background(), event); err != nil {
			t.Fatalf("dispatch %d: %v", i, err)
		}
	}
	maxPerEvent := 0
	for _, count := range perEvent {
		if count > maxPerEvent {
			maxPerEvent = count
		}
	}
	if calls.Load() != 10 || len(perEvent) != 10 || maxPerEvent != 1 {
		t.Fatalf("calls=%d events=%d max=%d", calls.Load(), len(perEvent), maxPerEvent)
	}
	fmt.Printf("m07-s02 deliver events=10 calls=10 per_event=1\n")
}

func TestScopedScopeIsolation(t *testing.T) {
	registry := NewScopedRegistry()
	var mu sync.Mutex
	seen := map[string][]string{}
	record := func(name string) ScopedHandler {
		return ScopedHandlerFunc(func(ctx context.Context, event ScopedEvent) error {
			mu.Lock()
			seen[name] = append(seen[name], event.Scope)
			mu.Unlock()
			return nil
		})
	}
	for scope, name := range map[string]string{"device:a": "a", "device:b": "b", "*": "*"} {
		registration := ScopedRegistration{
			Topic:   "chat.message",
			Scope:   scope,
			Owner:   "plugin-" + name,
			Handler: record(name),
		}
		if _, _, err := registry.Subscribe(context.Background(), registration); err != nil {
			t.Fatalf("subscribe %s: %v", scope, err)
		}
	}
	for _, scope := range []string{"device:a", "device:b"} {
		event := ScopedEvent{ID: "evt-" + scope, Topic: "chat.message", Scope: scope}
		if _, err := registry.Dispatch(context.Background(), event); err != nil {
			t.Fatalf("dispatch %s: %v", scope, err)
		}
	}
	mu.Lock()
	aScopes := append([]string(nil), seen["a"]...)
	bScopes := append([]string(nil), seen["b"]...)
	wildcardScopes := append([]string(nil), seen["*"]...)
	mu.Unlock()
	cross := 0
	for _, scope := range aScopes {
		if scope != "device:a" {
			cross++
		}
	}
	for _, scope := range bScopes {
		if scope != "device:b" {
			cross++
		}
	}
	if len(aScopes) != 1 || len(bScopes) != 1 || len(wildcardScopes) != 2 || cross != 0 {
		t.Fatalf("a=%v b=%v wildcard=%v cross=%d", aScopes, bScopes, wildcardScopes, cross)
	}
	fmt.Printf("m07-s02 scope exact_a=1 exact_b=1 wildcard=2 cross=0\n")
}

func TestScopedColdSubscribeDoesNotWake(t *testing.T) {
	activator := &scopedTestActivator{}
	registry := NewScopedRegistry().WithActivator(activator)
	var calls atomic.Int64
	for i := 0; i < 5; i++ {
		registration := ScopedRegistration{
			Topic:   "chat.message",
			Scope:   "device:a",
			Owner:   fmt.Sprintf("plugin-%d", i),
			Handler: scopedTestHandler(&calls),
		}
		if _, _, err := registry.Subscribe(context.Background(), registration); err != nil {
			t.Fatalf("subscribe %d: %v", i, err)
		}
	}
	if activator.calls.Load() != 0 {
		t.Fatalf("wakes on subscribe=%d", activator.calls.Load())
	}
	event := ScopedEvent{ID: "evt-cold", Topic: "chat.message", Scope: "device:a"}
	report, err := registry.Dispatch(context.Background(), event)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if activator.calls.Load() != 0 || report.Wakes != 0 || calls.Load() != 5 {
		t.Fatalf("wakes=%d report=%d calls=%d", activator.calls.Load(), report.Wakes, calls.Load())
	}
	fmt.Printf("m07-s02 cold wakes_on_subscribe=0 wakes_on_dispatch=0 calls=5\n")
}

func TestScopedUnsubscribeStopsDispatch(t *testing.T) {
	activator := &scopedTestActivator{}
	registry := NewScopedRegistry().WithActivator(activator)
	var calls atomic.Int64
	registration := ScopedRegistration{
		Topic:   "chat.message",
		Scope:   "device:a",
		Owner:   "plugin-a",
		Wake:    true,
		Handler: scopedTestHandler(&calls),
	}
	key := registration.key()
	if _, _, err := registry.Subscribe(context.Background(), registration); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if removed, err := registry.Unsubscribe(context.Background(), key); err != nil || !removed {
		t.Fatalf("unsubscribe removed=%v err=%v", removed, err)
	}
	event := ScopedEvent{ID: "evt-after", Topic: "chat.message", Scope: "device:a"}
	if _, err := registry.Dispatch(context.Background(), event); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if activator.calls.Load() != 0 || calls.Load() != 0 {
		t.Fatalf("wakes=%d calls=%d", activator.calls.Load(), calls.Load())
	}
	fmt.Printf("m07-s02 unsubscribed wakes=0 calls=0\n")
}

type scopedTestActivator struct {
	mu    sync.Mutex
	calls atomic.Int64
	fail  map[string]error
}

func (a *scopedTestActivator) EnsureActive(ctx context.Context, owner string) error {
	a.calls.Add(1)
	a.mu.Lock()
	defer a.mu.Unlock()
	if err, ok := a.fail[owner]; ok {
		return err
	}
	return nil
}

func TestScopedWakeFailureSkipsHandler(t *testing.T) {
	sentinel := errors.New("cold start failed")
	activator := &scopedTestActivator{fail: map[string]error{"plugin-a": sentinel}}
	registry := NewScopedRegistry().WithActivator(activator)
	var callsA, callsB atomic.Int64
	if _, _, err := registry.Subscribe(context.Background(), ScopedRegistration{
		Topic: "chat.message", Scope: "device:a", Owner: "plugin-a", Wake: true, Handler: scopedTestHandler(&callsA),
	}); err != nil {
		t.Fatalf("subscribe a: %v", err)
	}
	if _, _, err := registry.Subscribe(context.Background(), ScopedRegistration{
		Topic: "chat.message", Scope: "device:a", Owner: "plugin-b", Wake: true, Handler: scopedTestHandler(&callsB),
	}); err != nil {
		t.Fatalf("subscribe b: %v", err)
	}
	report, err := registry.Dispatch(context.Background(), ScopedEvent{ID: "evt-wake", Topic: "chat.message", Scope: "device:a"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("dispatch err=%v", err)
	}
	if report.ActivationFailed != 1 || report.Delivered != 1 || report.Wakes != 2 || callsA.Load() != 0 || callsB.Load() != 1 {
		t.Fatalf("report=%+v callsA=%d callsB=%d", report, callsA.Load(), callsB.Load())
	}
	fmt.Printf("m07-s02 wake failed=1 delivered=1 wakes=2\n")
}

func TestScopedHandlerErrorIsolated(t *testing.T) {
	sentinel := errors.New("handler exploded")
	registry := NewScopedRegistry()
	var good atomic.Int64
	if _, _, err := registry.Subscribe(context.Background(), ScopedRegistration{
		Topic: "chat.message", Scope: "device:a", Owner: "plugin-a",
		Handler: ScopedHandlerFunc(func(context.Context, ScopedEvent) error { return sentinel }),
	}); err != nil {
		t.Fatalf("subscribe a: %v", err)
	}
	if _, _, err := registry.Subscribe(context.Background(), ScopedRegistration{
		Topic: "chat.message", Scope: "device:a", Owner: "plugin-b", Handler: scopedTestHandler(&good),
	}); err != nil {
		t.Fatalf("subscribe b: %v", err)
	}
	report, err := registry.Dispatch(context.Background(), ScopedEvent{ID: "evt-err", Topic: "chat.message", Scope: "device:a"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("dispatch err=%v", err)
	}
	if report.HandlerFailed != 1 || report.Delivered != 1 || good.Load() != 1 {
		t.Fatalf("report=%+v good=%d", report, good.Load())
	}
	fmt.Printf("m07-s02 handler errors_joined=true delivered=1 failed=1\n")
}

func TestScopedConcurrentSubscribe(t *testing.T) {
	registry := NewScopedRegistry()
	registration := ScopedRegistration{
		Topic:   "chat.message",
		Scope:   "device:a",
		Owner:   "plugin-a",
		Handler: ScopedHandlerFunc(func(context.Context, ScopedEvent) error { return nil }),
	}
	var created atomic.Int64
	var wait sync.WaitGroup
	wait.Add(32)
	for i := 0; i < 32; i++ {
		go func() {
			defer wait.Done()
			_, wasCreated, err := registry.Subscribe(context.Background(), registration)
			if err != nil {
				t.Errorf("subscribe: %v", err)
				return
			}
			if wasCreated {
				created.Add(1)
			}
		}()
	}
	wait.Wait()
	view, ok := registry.Get(registration.key())
	if !ok || created.Load() != 1 || registry.Len() != 1 || view.Duplicates != 31 || view.Generation != 1 {
		t.Fatalf("created=%d len=%d view=%+v ok=%v", created.Load(), registry.Len(), view, ok)
	}
	fmt.Printf("m07-s02 concurrent goroutines=32 created=1 duplicates=31\n")
}

func TestScopedCancel(t *testing.T) {
	registry := NewScopedRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	registration := ScopedRegistration{
		Topic:   "chat.message",
		Scope:   "device:a",
		Owner:   "plugin-a",
		Handler: ScopedHandlerFunc(func(context.Context, ScopedEvent) error { return nil }),
	}
	if _, _, err := registry.Subscribe(ctx, registration); !errors.Is(err, context.Canceled) {
		t.Fatalf("subscribe err=%v", err)
	}
	if registry.Len() != 0 {
		t.Fatalf("subscriptions=%d", registry.Len())
	}
	report, err := registry.Dispatch(ctx, ScopedEvent{ID: "evt-cancel", Topic: "chat.message", Scope: "device:a"})
	if !errors.Is(err, context.Canceled) || report.Delivered != 0 {
		t.Fatalf("dispatch err=%v report=%+v", err, report)
	}
	fmt.Printf("m07-s02 cancel subscribe=context.Canceled dispatch=context.Canceled calls=0\n")
}

func TestScopedResubscribeGeneration(t *testing.T) {
	registry := NewScopedRegistry()
	var calls atomic.Int64
	registration := ScopedRegistration{
		Topic:   "chat.message",
		Scope:   "device:a",
		Owner:   "plugin-a",
		Handler: scopedTestHandler(&calls),
	}
	first, created, err := registry.Subscribe(context.Background(), registration)
	if err != nil || !created || first.Generation != 1 {
		t.Fatalf("first created=%v generation=%d err=%v", created, first.Generation, err)
	}
	if removed, err := registry.Unsubscribe(context.Background(), registration.key()); err != nil || !removed {
		t.Fatalf("unsubscribe removed=%v err=%v", removed, err)
	}
	second, created, err := registry.Subscribe(context.Background(), registration)
	if err != nil || !created || second.Generation != 2 {
		t.Fatalf("second created=%v generation=%d err=%v", created, second.Generation, err)
	}
	for i := 0; i < 2; i++ {
		if _, err := registry.Dispatch(context.Background(), ScopedEvent{ID: fmt.Sprintf("evt-%d", i), Topic: "chat.message", Scope: "device:a"}); err != nil {
			t.Fatalf("dispatch %d: %v", i, err)
		}
	}
	view, ok := registry.Get(registration.key())
	if !ok || view.Generation != 2 || view.Deliveries != 2 || calls.Load() != 2 {
		t.Fatalf("view=%+v calls=%d", view, calls.Load())
	}
	fmt.Printf("m07-s02 resubscribe generation=2 deliveries=2\n")
}
