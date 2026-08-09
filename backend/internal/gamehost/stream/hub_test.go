package stream

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/state"
)

type fakeEventPublisher struct {
	mu     sync.Mutex
	events []EventEnvelope
	err    error
}

func (f *fakeEventPublisher) PublishEvent(ctx context.Context, ev EventEnvelope, opts ...PublishEventOption) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.events = append(f.events, ev)
	return nil
}

func (f *fakeEventPublisher) snapshot() []EventEnvelope {
	f.mu.Lock()
	defer f.mu.Unlock()
	res := make([]EventEnvelope, len(f.events))
	copy(res, f.events)
	return res
}

func newTestEventEnvelope(method string) EventEnvelope {
	return EventEnvelope{
		ID:        "evt-1",
		TypeID:    "test.event",
		Version:   1,
		PluginID:  "plugin-x",
		RuntimeID: "runtime-x",
		ServiceID: "svc-x",
		Method:    method,
		Payload:   json.RawMessage(`{"test":true}`),
		Metadata:  map[string]json.RawMessage{"trace": jsonRawFake("abc")},
	}
}

func jsonRawFake(s string) json.RawMessage {
	return json.RawMessage(`"` + s + `"`)
}

func TestGameHub_PublishEvent(t *testing.T) {
	pub := &fakeEventPublisher{}
	store := state.NewLatestStateStore(state.NewOptions())
	hub := NewGameHub(HubConfig{EventPublisher: pub, StateStore: store})

	ev := newTestEventEnvelope("vendor.event")
	if err := hub.PublishEvent(context.Background(), ev); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := pub.snapshot()
	if len(events) != 1 {
		t.Fatalf("expected 1 event published, got %d", len(events))
	}
	if events[0].Method != "vendor.event" {
		t.Errorf("method: got %q, want vendor.event", events[0].Method)
	}
}

func TestGameHub_PublishEvent_Failure(t *testing.T) {
	pub := &fakeEventPublisher{err: context.DeadlineExceeded}
	store := state.NewLatestStateStore(state.NewOptions())
	hub := NewGameHub(HubConfig{EventPublisher: pub, StateStore: store})

	ev := newTestEventEnvelope("foo")
	err := hub.PublishEvent(context.Background(), ev)
	if err == nil {
		t.Fatal("expected error when publisher fails")
	}
}

func TestGameHub_PublishEvent_NilPublisher(t *testing.T) {
	store := state.NewLatestStateStore(state.NewOptions())
	hub := NewGameHub(HubConfig{EventPublisher: nil, StateStore: store})

	ev := newTestEventEnvelope("foo")
	err := hub.PublishEvent(context.Background(), ev)
	if err == nil {
		t.Fatal("expected error when event publisher is nil")
	}
}

func TestGameHub_PublishState(t *testing.T) {
	pub := &fakeEventPublisher{}
	store := state.NewLatestStateStore(state.NewOptions())
	hub := NewGameHub(HubConfig{EventPublisher: pub, StateStore: store})

	update := state.StateUpdate{
		ID:        "state-1",
		PluginID:  "plugin-s",
		RuntimeID: "runtime-s",
		ServiceID: "svc-s",
		Key:        "bot.status",
		Payload:   json.RawMessage(`{"status":"online"}`),
	}
	snap, err := hub.PublishState(context.Background(), update)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Key != "bot.status" {
		t.Errorf("state key: got %q, want bot.status", snap.Key)
	}
	if string(snap.Payload) != `{"status":"online"}` {
		t.Errorf("payload: got %q", snap.Payload)
	}
}

func TestGameHub_PublishState_NilStore(t *testing.T) {
	pub := &fakeEventPublisher{}
	hub := NewGameHub(HubConfig{EventPublisher: pub, StateStore: nil})

	_, err := hub.PublishState(context.Background(), state.StateUpdate{})
	if err == nil {
		t.Fatal("expected error when state store is nil")
	}
}

func TestGameHub_GetLatestState(t *testing.T) {
	store := state.NewLatestStateStore(state.NewOptions())
	hub := NewGameHub(HubConfig{StateStore: store})

	store.Put(context.Background(), state.StateUpdate{
		PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k", Payload: json.RawMessage(`{"x":1}`),
	})

	snap, err := hub.GetLatestState(context.Background(), state.StateKey{
		PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(snap.Payload) != `{"x":1}` {
		t.Errorf("unexpected payload: %q", snap.Payload)
	}
}

func TestGameHub_ListLatestState(t *testing.T) {
	store := state.NewLatestStateStore(state.NewOptions())
	hub := NewGameHub(HubConfig{StateStore: store})

	store.Put(context.Background(), state.StateUpdate{PluginID: "p", RuntimeID: "r1", ServiceID: "s", Key: "a", Payload: nil})
	store.Put(context.Background(), state.StateUpdate{PluginID: "p", RuntimeID: "r1", ServiceID: "s", Key: "b", Payload: nil})
	store.Put(context.Background(), state.StateUpdate{PluginID: "p", RuntimeID: "r2", ServiceID: "s", Key: "c", Payload: nil})

	rt1 := domain.RuntimeInstanceID("r1")
	list, err := hub.ListLatestState(context.Background(), state.StateFilter{RuntimeID: &rt1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 states for r1, got %d", len(list))
	}
}

func TestGameHub_RemoveState(t *testing.T) {
	store := state.NewLatestStateStore(state.NewOptions())
	hub := NewGameHub(HubConfig{StateStore: store})

	store.Put(context.Background(), state.StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k", Payload: nil})

	err := hub.RemoveState(context.Background(), state.StateKey{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = hub.GetLatestState(context.Background(), state.StateKey{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k"})
	if !domain.IsHostError(err, domain.ErrNotFound) {
		t.Errorf("expected not_found after remove, got %v", err)
	}
}

func TestGameHub_RemoveStateByRuntime(t *testing.T) {
	store := state.NewLatestStateStore(state.NewOptions())
	hub := NewGameHub(HubConfig{StateStore: store})

	store.Put(context.Background(), state.StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k1", Payload: nil})
	store.Put(context.Background(), state.StateUpdate{PluginID: "p", RuntimeID: "other", ServiceID: "s", Key: "k2", Payload: nil})

	if err := hub.RemoveStateByRuntime(context.Background(), "r"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.Count(context.Background()) != 1 {
		t.Errorf("expected 1 state after remove, got %d", store.Count(context.Background()))
	}
}

func TestGameHub_RemoveStateByService(t *testing.T) {
	store := state.NewLatestStateStore(state.NewOptions())
	hub := NewGameHub(HubConfig{StateStore: store})

	store.Put(context.Background(), state.StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "svc-a", Key: "k1", Payload: nil})
	store.Put(context.Background(), state.StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "svc-b", Key: "k2", Payload: nil})

	if err := hub.RemoveStateByService(context.Background(), "r", "svc-a"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.Count(context.Background()) != 1 {
		t.Errorf("expected 1 state after remove, got %d", store.Count(context.Background()))
	}
}

func TestGameHub_StateCount(t *testing.T) {
	store := state.NewLatestStateStore(state.NewOptions())
	hub := NewGameHub(HubConfig{StateStore: store})

	if hub.StateCount(context.Background()) != 0 {
		t.Error("expected 0 count initially")
	}
	store.Put(context.Background(), state.StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k", Payload: nil})
	if hub.StateCount(context.Background()) != 1 {
		t.Errorf("expected 1 count, got %d", hub.StateCount(context.Background()))
	}
}

func TestGameHub_StateCount_NilStore(t *testing.T) {
	hub := NewGameHub(HubConfig{StateStore: nil})
	if hub.StateCount(context.Background()) != 0 {
		t.Error("expected 0 count for nil store")
	}
}

func TestCompositeEventPublisher_PrimaryOnly(t *testing.T) {
	primary := &fakeEventPublisher{}
	c := NewCompositeEventPublisher(primary)

	ev := newTestEventEnvelope("a")
	if err := c.PublishEvent(context.Background(), ev); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(primary.snapshot()) != 1 {
		t.Error("primary should receive event")
	}
}

func TestCompositeEventPublisher_WithSecondaries(t *testing.T) {
	primary := &fakeEventPublisher{}
	sec1 := &fakeEventPublisher{}
	sec2 := &fakeEventPublisher{}
	c := NewCompositeEventPublisher(primary, sec1, sec2)

	ev := newTestEventEnvelope("a")
	if err := c.PublishEvent(context.Background(), ev); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(primary.snapshot()) != 1 {
		t.Error("primary should receive event")
	}
	if len(sec1.snapshot()) != 1 {
		t.Error("sec1 should receive event")
	}
	if len(sec2.snapshot()) != 1 {
		t.Error("sec2 should receive event")
	}
}

func TestCompositeEventPublisher_PrimaryFailure(t *testing.T) {
	primary := &fakeEventPublisher{err: context.DeadlineExceeded}
	sec := &fakeEventPublisher{}
	c := NewCompositeEventPublisher(primary, sec)

	ev := newTestEventEnvelope("a")
	err := c.PublishEvent(context.Background(), ev)
	if err == nil {
		t.Fatal("expected error when primary fails")
	}
	if len(sec.snapshot()) != 0 {
		t.Error("secondaries should not receive when primary fails")
	}
}

func TestCompositeEventPublisher_NilSecondary(t *testing.T) {
	primary := &fakeEventPublisher{}
	c := NewCompositeEventPublisher(primary, nil)

	ev := newTestEventEnvelope("a")
	if err := c.PublishEvent(context.Background(), ev); err != nil {
		t.Fatalf("nil secondary should not fail, got: %v", err)
	}
}

func TestCompositeEventPublisher_NilPrimary(t *testing.T) {
	sec := &fakeEventPublisher{}
	c := NewCompositeEventPublisher(nil, sec)

	ev := newTestEventEnvelope("a")
	if err := c.PublishEvent(context.Background(), ev); err != nil {
		t.Fatalf("should succeed when primary is nil, got: %v", err)
	}
}

func TestValidateEventEnvelope(t *testing.T) {
	cases := []struct {
		name    string
		env     EventEnvelope
		wantErr bool
	}{
		{"valid", EventEnvelope{PluginID: "p", RuntimeID: "r", ServiceID: "s", Method: "m"}, false},
		{"missing plugin", EventEnvelope{RuntimeID: "r", ServiceID: "s", Method: "m"}, true},
		{"missing runtime", EventEnvelope{PluginID: "p", ServiceID: "s", Method: "m"}, true},
		{"missing service", EventEnvelope{PluginID: "p", RuntimeID: "r", Method: "m"}, true},
		{"missing type and method", EventEnvelope{PluginID: "p", RuntimeID: "r", ServiceID: "s"}, true},
		{"type only OK", EventEnvelope{PluginID: "p", RuntimeID: "r", ServiceID: "s", TypeID: "t"}, false},
	}
	for _, c := range cases {
		err := validateEventEnvelope(c.env)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: wantErr=%v got err=%v", c.name, c.wantErr, err)
		}
	}
}

func TestPickProducerType(t *testing.T) {
	if pickProducerType("custom", "p") != "custom" {
		t.Error("configured type should win")
	}
	if pickProducerType("", "p") != "gamehost_plugin" {
		t.Error("default should be gamehost_plugin")
	}
	if pickProducerType("", "") != "host" {
		t.Error("empty plugin should use host")
	}
}

func TestPickTraceID(t *testing.T) {
	if pickTraceID("trace-1", "evt-1") != "trace-1" {
		t.Error("traceID should win")
	}
	if pickTraceID("", "evt-1") != "evt-1" {
		t.Error("should fall back to eventID")
	}
}

func TestPickPartitionKey(t *testing.T) {
	if pickPartitionKey("custom", "p", "r") != "custom" {
		t.Error("configured partition key should win")
	}
	if pickPartitionKey("", "p", "r") != "p/r" {
		t.Error("runtime partition should be plugin/runtime")
	}
	if pickPartitionKey("", "p", "") != "p" {
		t.Error("should fall back to plugin ID")
	}
}
