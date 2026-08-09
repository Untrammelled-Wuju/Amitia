package notification

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type errorSink struct {
	err error
}

func (e *errorSink) Publish(ctx context.Context, n Notification) error {
	return e.err
}

func TestMemorySink_Publish(t *testing.T) {
	sink := NewMemorySink()

	if err := sink.Publish(context.Background(), Notification{Method: "a"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := sink.Publish(context.Background(), Notification{Method: "b"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sink.Count() != 2 {
		t.Fatalf("expected count 2, got %d", sink.Count())
	}
}

func TestMemorySink_Snapshot(t *testing.T) {
	sink := NewMemorySink()
	_ = sink.Publish(context.Background(), Notification{Method: "original"})

	snap := sink.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("expected 1, got %d", len(snap))
	}

	sink.Clear()
	snap2 := sink.Snapshot()
	if len(snap2) != 0 {
		t.Errorf("after clear, snapshot should be empty, got %d", len(snap2))
	}

	if snap[0].Method != "original" {
		t.Error("original snapshot mutated")
	}
}

func TestMemorySink_Clear(t *testing.T) {
	sink := NewMemorySink()
	_ = sink.Publish(context.Background(), Notification{Method: "a"})
	_ = sink.Publish(context.Background(), Notification{Method: "b"})

	sink.Clear()
	if sink.Count() != 0 {
		t.Errorf("expected 0 after clear, got %d", sink.Count())
	}
}

func TestMemorySink_Concurrent(t *testing.T) {
	sink := NewMemorySink()
	const N = 100
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			_ = sink.Publish(context.Background(), Notification{Method: "evt"})
		}()
	}
	wg.Wait()
	if sink.Count() != N {
		t.Errorf("expected %d, got %d", N, sink.Count())
	}
}

func TestCompositeSink_AllSuccess(t *testing.T) {
	a := NewMemorySink()
	b := NewMemorySink()
	c := NewCompositeSink(a, b)

	if err := c.Publish(context.Background(), Notification{Method: "test"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Count() != 1 {
		t.Errorf("sink a expected 1, got %d", a.Count())
	}
	if b.Count() != 1 {
		t.Errorf("sink b expected 1, got %d", b.Count())
	}
}

func TestCompositeSink_PartialFailure(t *testing.T) {
	a := NewMemorySink()
	b := &errorSink{err: errBFailed}
	c := NewCompositeSink(a, b)

	err := c.Publish(context.Background(), Notification{Method: "test"})
	if err == nil {
		t.Fatal("expected composite error")
	}
	if a.Count() != 1 {
		t.Errorf("sink a should still publish, got %d", a.Count())
	}
}

var errBFailed = errors.New("b failed")

func TestCompositeSink_NoSinks(t *testing.T) {
	c := NewCompositeSink()
	if err := c.Publish(context.Background(), Notification{Method: "test"}); err != nil {
		t.Fatalf("expected no error for empty composite, got %v", err)
	}
}

func TestCompositeSink_NilSinkSkipped(t *testing.T) {
	a := NewMemorySink()
	c := NewCompositeSink(nil, a)

	if err := c.Publish(context.Background(), Notification{Method: "test"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Count() != 1 {
		t.Errorf("expected 1, got %d", a.Count())
	}
}

type countingObserver struct {
	mu      sync.Mutex
	total   int
	methods map[string]int
}

func (o *countingObserver) OnNotification(n Notification) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.total++
	o.methods[n.Method]++
}

func TestObserverSink_NotifiesInner(t *testing.T) {
	inner := NewMemorySink()
	obs := &countingObserver{methods: make(map[string]int)}
	sink := NewObserverSink(inner, obs)

	if err := sink.Publish(context.Background(), Notification{Method: "m1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := sink.Publish(context.Background(), Notification{Method: "m2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if inner.Count() != 2 {
		t.Errorf("inner expected 2, got %d", inner.Count())
	}
	if obs.total != 2 {
		t.Errorf("observer expected total 2, got %d", obs.total)
	}
	if obs.methods["m1"] != 1 {
		t.Errorf("observer expected m1 count 1, got %d", obs.methods["m1"])
	}
}

func TestObserverSink_InnerFailure(t *testing.T) {
	obs := &countingObserver{methods: make(map[string]int)}
	sink := NewObserverSink(&errorSink{err: errors.New("fail")}, obs)

	err := sink.Publish(context.Background(), Notification{Method: "m1"})
	if err == nil {
		t.Fatal("expected error from inner")
	}
	if obs.total != 1 {
		t.Errorf("observer should still notify, got %d", obs.total)
	}
}

func TestObserverSink_NilInner(t *testing.T) {
	sink := NewObserverSink(nil, nil)
	if err := sink.Publish(context.Background(), Notification{Method: "test"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCountingObserver_Methods(t *testing.T) {
	obs := NewCountingObserver()
	for i := 0; i < 3; i++ {
		obs.OnNotification(Notification{Method: "evt.a"})
	}
	obs.OnNotification(Notification{Method: "evt.b"})

	if obs.Total() != 4 {
		t.Errorf("total expected 4, got %d", obs.Total())
	}
	if obs.Count("evt.a") != 3 {
		t.Errorf("evt.a expected 3, got %d", obs.Count("evt.a"))
	}
	if obs.Count("evt.b") != 1 {
		t.Errorf("evt.b expected 1, got %d", obs.Count("evt.b"))
	}
	if obs.Count("nonexistent") != 0 {
		t.Errorf("nonexistent expected 0, got %d", obs.Count("nonexistent"))
	}
}

func TestObserverFunc(t *testing.T) {
	var n Notification
	fn := ObserverFunc(func(notif Notification) {
		n = notif
	})
	fn.OnNotification(Notification{Method: "test"})
	if n.Method != "test" {
		t.Errorf("expected method=test, got %q", n.Method)
	}
}
