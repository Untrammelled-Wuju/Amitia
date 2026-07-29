package event_bus

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEventBusSchema(t *testing.T) {
	b := NewDefaultBus()
	if err := b.RegisterSchema(context.Background(), Schema{
		EventType: "test.event",
		Version:   1,
		Schema:    json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("RegisterSchema: %v", err)
	}
	if err := b.RegisterSchema(context.Background(), Schema{
		EventType: "test.event",
		Version:   1,
	}); err == nil {
		t.Errorf("expected conflict")
	}
}

func TestEventBusPublishNoSubscribers(t *testing.T) {
	b := NewDefaultBus()
	_ = b.RegisterSchema(context.Background(), Schema{EventType: "test.event", Version: 1})
	result, err := b.Publish(context.Background(), Event{
		EventID: "evt-1",
		Type:    "test.event",
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if result.AcceptCount != 0 {
		t.Errorf("expected 0 accept, got %d", result.AcceptCount)
	}
}

func TestEventBusPublishSchemaNotFound(t *testing.T) {
	b := NewDefaultBus()
	_, err := b.Publish(context.Background(), Event{
		EventID: "evt-1",
		Type:    "missing.event",
	})
	if !errors.Is(err, ErrSchemaNotFound) {
		t.Errorf("expected schema not found, got %v", err)
	}
}

func TestEventBusSubscribeAndPublish(t *testing.T) {
	b := NewDefaultBus()
	_ = b.RegisterSchema(context.Background(), Schema{EventType: "test.event", Version: 1})
	var received []Event
	var mu sync.Mutex
	_, err := b.Subscribe(context.Background(), Subscription{
		Subscriber: "sub-1",
		EventType:  "test.event",
		Handler: func(_ context.Context, e Event) error {
			mu.Lock()
			defer mu.Unlock()
			received = append(received, e)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	result, err := b.Publish(context.Background(), Event{
		EventID: "evt-1",
		Type:    "test.event",
		Payload: json.RawMessage(`{"hello":"world"}`),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if result.AcceptCount != 1 {
		t.Errorf("expected 1 accept, got %d", result.AcceptCount)
	}
	if len(received) != 1 {
		t.Errorf("expected 1 received, got %d", len(received))
	}
}

func TestEventBusMultipleSubscribers(t *testing.T) {
	b := NewDefaultBus()
	_ = b.RegisterSchema(context.Background(), Schema{EventType: "test.event", Version: 1})
	var counter int32
	for i := 0; i < 3; i++ {
		_, _ = b.Subscribe(context.Background(), Subscription{
			Subscriber: "sub",
			EventType:  "test.event",
			Handler: func(_ context.Context, _ Event) error {
				atomic.AddInt32(&counter, 1)
				return nil
			},
		})
	}
	result, _ := b.Publish(context.Background(), Event{
		EventID: "evt-1",
		Type:    "test.event",
	})
	if result.AcceptCount != 3 {
		t.Errorf("expected 3 accepts, got %d", result.AcceptCount)
	}
}

func TestEventBusRetry(t *testing.T) {
	b := NewDefaultBus()
	_ = b.RegisterSchema(context.Background(), Schema{EventType: "test.event", Version: 1})
	var attempts int32
	_, _ = b.Subscribe(context.Background(), Subscription{
		Subscriber: "sub-1",
		EventType:  "test.event",
		RetryLimit: 2,
		Timeout:    100 * time.Millisecond,
		Handler: func(_ context.Context, _ Event) error {
			atomic.AddInt32(&attempts, 1)
			return errors.New("transient")
		},
	})
	result, _ := b.Publish(context.Background(), Event{
		EventID: "evt-1",
		Type:    "test.event",
	})
	if result.AcceptCount != 0 {
		t.Errorf("expected 0 accept, got %d", result.AcceptCount)
	}
	if result.RejectCount != 1 {
		t.Errorf("expected 1 reject, got %d", result.RejectCount)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestEventBusUnsubscribe(t *testing.T) {
	b := NewDefaultBus()
	_ = b.RegisterSchema(context.Background(), Schema{EventType: "test.event", Version: 1})
	sub, _ := b.Subscribe(context.Background(), Subscription{
		Subscriber: "sub-1",
		EventType:  "test.event",
		Handler:    func(context.Context, Event) error { return nil },
	})
	if err := b.Unsubscribe(context.Background(), sub.SubscriptionID); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	if err := b.Unsubscribe(context.Background(), sub.SubscriptionID); err == nil {
		t.Errorf("expected not found")
	}
}

func TestEventBusPriorityOrder(t *testing.T) {
	b := NewDefaultBus()
	_ = b.RegisterSchema(context.Background(), Schema{EventType: "test.event", Version: 1})
	var order []string
	var mu sync.Mutex
	_, _ = b.Subscribe(context.Background(), Subscription{
		Subscriber: "low",
		EventType:  "test.event",
		Priority:   1,
		Handler: func(_ context.Context, _ Event) error {
			mu.Lock()
			order = append(order, "low")
			mu.Unlock()
			return nil
		},
	})
	_, _ = b.Subscribe(context.Background(), Subscription{
		Subscriber: "high",
		EventType:  "test.event",
		Priority:   10,
		Handler: func(_ context.Context, _ Event) error {
			mu.Lock()
			order = append(order, "high")
			mu.Unlock()
			return nil
		},
	})
	_, _ = b.Publish(context.Background(), Event{
		EventID: "evt-1",
		Type:    "test.event",
	})
	if len(order) != 2 || order[0] != "high" {
		t.Errorf("expected high first, got %v", order)
	}
}

func TestHookPipelineRegister(t *testing.T) {
	p := NewDefaultPipeline()
	if err := p.Register(context.Background(), HookRegistration{
		Point:   "host.tool.invoke",
		Phase:   HookPhaseBefore,
		Handler: func(context.Context, *HookContext) error { return nil },
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	hooks := p.List(context.Background(), "host.tool.invoke")
	if len(hooks) != 1 {
		t.Errorf("expected 1 hook, got %d", len(hooks))
	}
}

func TestHookPipelineExecuteBefore(t *testing.T) {
	p := NewDefaultPipeline()
	var called bool
	_ = p.Register(context.Background(), HookRegistration{
		Point: "host.tool.invoke",
		Phase: HookPhaseBefore,
		Handler: func(_ context.Context, _ *HookContext) error {
			called = true
			return nil
		},
	})
	result := p.Execute(context.Background(), "host.tool.invoke", &HookContext{
		Operation: "test",
	})
	if !called {
		t.Errorf("expected hook called")
	}
	if result.Aborted {
		t.Errorf("expected not aborted")
	}
	if len(result.ExecutedHooks) != 1 {
		t.Errorf("expected 1 executed, got %d", len(result.ExecutedHooks))
	}
}

func TestHookPipelineAbort(t *testing.T) {
	p := NewDefaultPipeline()
	_ = p.Register(context.Background(), HookRegistration{
		Point: "host.tool.invoke",
		Phase: HookPhaseBefore,
		Handler: func(_ context.Context, ctx *HookContext) error {
			ctx.Abort("denied by policy")
			return nil
		},
	})
	var afterCalled bool
	_ = p.Register(context.Background(), HookRegistration{
		Point: "host.tool.invoke",
		Phase: HookPhaseAfter,
		Handler: func(_ context.Context, _ *HookContext) error {
			afterCalled = true
			return nil
		},
	})
	result := p.Execute(context.Background(), "host.tool.invoke", &HookContext{
		Operation: "test",
	})
	if !result.Aborted {
		t.Errorf("expected aborted")
	}
	if afterCalled {
		t.Errorf("expected after hook not called after abort")
	}
}

func TestHookPipelineRequiredFailure(t *testing.T) {
	p := NewDefaultPipeline()
	_ = p.Register(context.Background(), HookRegistration{
		Point:    "host.tool.invoke",
		Phase:    HookPhaseBefore,
		Required: true,
		Handler:  func(context.Context, *HookContext) error { return errors.New("validation failed") },
	})
	result := p.Execute(context.Background(), "host.tool.invoke", &HookContext{
		Operation: "test",
	})
	if !result.Aborted {
		t.Errorf("expected aborted")
	}
}

func TestHookPipelineFailureContinue(t *testing.T) {
	p := NewDefaultPipeline()
	var nextCalled bool
	_ = p.Register(context.Background(), HookRegistration{
		Point:         "host.tool.invoke",
		Phase:         HookPhaseBefore,
		FailurePolicy: FailureContinue,
		Handler:       func(context.Context, *HookContext) error { return errors.New("ignore") },
	})
	_ = p.Register(context.Background(), HookRegistration{
		Point: "host.tool.invoke",
		Phase: HookPhaseAfter,
		Handler: func(_ context.Context, _ *HookContext) error {
			nextCalled = true
			return nil
		},
	})
	result := p.Execute(context.Background(), "host.tool.invoke", &HookContext{
		Operation: "test",
	})
	if result.Aborted {
		t.Errorf("expected not aborted with continue policy")
	}
	if !nextCalled {
		t.Errorf("expected next phase called")
	}
}

func TestHookPipelineTransform(t *testing.T) {
	p := NewDefaultPipeline()
	_ = p.Register(context.Background(), HookRegistration{
		Point: "host.tool.invoke",
		Phase: HookPhaseTransform,
		Handler: func(_ context.Context, ctx *HookContext) error {
			ctx.Output = map[string]any{"modified": true}
			ctx.MarkTransformed()
			return nil
		},
	})
	result := p.Execute(context.Background(), "host.tool.invoke", &HookContext{
		Operation: "test",
	})
	if !result.Transformed {
		t.Errorf("expected transformed")
	}
}

func TestHookPipelineOrder(t *testing.T) {
	p := NewDefaultPipeline()
	var order []string
	var mu sync.Mutex
	_ = p.Register(context.Background(), HookRegistration{
		Point:    "host.tool.invoke",
		Phase:    HookPhaseBefore,
		Priority: 1,
		Handler: func(_ context.Context, _ *HookContext) error {
			mu.Lock()
			order = append(order, "low")
			mu.Unlock()
			return nil
		},
	})
	_ = p.Register(context.Background(), HookRegistration{
		Point:    "host.tool.invoke",
		Phase:    HookPhaseBefore,
		Priority: 10,
		Handler: func(_ context.Context, _ *HookContext) error {
			mu.Lock()
			order = append(order, "high")
			mu.Unlock()
			return nil
		},
	})
	p.Execute(context.Background(), "host.tool.invoke", &HookContext{Operation: "test"})
	if len(order) != 2 || order[0] != "high" {
		t.Errorf("expected high first, got %v", order)
	}
}

func TestHookPipelineUnregister(t *testing.T) {
	p := NewDefaultPipeline()
	_ = p.Register(context.Background(), HookRegistration{
		Point:   "host.tool.invoke",
		Phase:   HookPhaseBefore,
		Handler: func(context.Context, *HookContext) error { return nil },
	})
	hooks := p.List(context.Background(), "host.tool.invoke")
	if err := p.Unregister(context.Background(), hooks[0].HookID); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	if len(p.List(context.Background(), "host.tool.invoke")) != 0 {
		t.Errorf("expected 0 after unregister")
	}
}

func TestHookPipelineQuarantine(t *testing.T) {
	p := NewDefaultPipeline()
	_ = p.Register(context.Background(), HookRegistration{
		Point:         "host.tool.invoke",
		Phase:         HookPhaseBefore,
		FailurePolicy: FailureQuarantine,
		Handler:       func(context.Context, *HookContext) error { return errors.New("bad") },
	})
	result := p.Execute(context.Background(), "host.tool.invoke", &HookContext{Operation: "test"})
	if result.Aborted {
		t.Errorf("expected not aborted")
	}
	hooks := p.List(context.Background(), "host.tool.invoke")
	if hooks[0].Active {
		t.Errorf("expected hook quarantined (inactive)")
	}
}
