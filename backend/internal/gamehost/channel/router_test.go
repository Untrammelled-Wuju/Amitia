package channel

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/state"
	"github.com/u-ai/backend/internal/gamehost/stream"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type mockEventPublisher struct {
	events []stream.EventEnvelope
}

func (m *mockEventPublisher) PublishEvent(ctx context.Context, event stream.EventEnvelope, opts ...stream.PublishEventOption) error {
	m.events = append(m.events, event)
	return nil
}

type mockStateStore struct {
	states map[state.StateKey]state.StateSnapshot
}

func newMockStateStore() *mockStateStore {
	return &mockStateStore{states: make(map[state.StateKey]state.StateSnapshot)}
}

func (m *mockStateStore) Put(ctx context.Context, update state.StateUpdate) (state.StateSnapshot, error) {
	key := state.NewStateKey(update.PluginID, update.RuntimeID, update.ServiceID, update.Key)
	var version uint64
	if existing, ok := m.states[key]; ok {
		version = existing.Version + 1
	} else {
		version = 1
	}
	snap := state.StateSnapshot{
		PluginID:        update.PluginID,
		RuntimeID:       update.RuntimeID,
		ServiceID:       update.ServiceID,
		Key:             update.Key,
		Payload:         update.Payload,
		Metadata:        update.Metadata,
		SourceMessageID: update.ID,
		Version:         version,
		UpdatedAt:       update.ReceivedAt,
	}
	m.states[key] = snap
	return snap, nil
}

func (m *mockStateStore) Get(ctx context.Context, key state.StateKey) (state.StateSnapshot, error) {
	snap, ok := m.states[key]
	if !ok {
		return state.StateSnapshot{}, domain.NewHostError(domain.ErrNotFound, "not found")
	}
	return snap, nil
}

func (m *mockStateStore) List(ctx context.Context, filter state.StateFilter) ([]state.StateSnapshot, error) {
	result := make([]state.StateSnapshot, 0)
	for _, snap := range m.states {
		if filter.Match(snap) {
			result = append(result, snap)
		}
	}
	return result, nil
}

func (m *mockStateStore) Remove(ctx context.Context, key state.StateKey) error {
	delete(m.states, key)
	return nil
}

func (m *mockStateStore) RemoveByService(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) error {
	for key := range m.states {
		if key.RuntimeID == runtimeID && key.ServiceID == serviceID {
			delete(m.states, key)
		}
	}
	return nil
}

func (m *mockStateStore) RemoveByRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	for key := range m.states {
		if key.RuntimeID == runtimeID {
			delete(m.states, key)
		}
	}
	return nil
}

func (m *mockStateStore) RemoveByPlugin(ctx context.Context, pluginID domain.PluginID) error {
	for key := range m.states {
		if key.PluginID == pluginID {
			delete(m.states, key)
		}
	}
	return nil
}

func (m *mockStateStore) Count(ctx context.Context) int {
	return len(m.states)
}

func (m *mockStateStore) CountByRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) int {
	count := 0
	for key := range m.states {
		if key.RuntimeID == runtimeID {
			count++
		}
	}
	return count
}

type mockGenericSink struct {
	messages []ChannelMessage
}

func (m *mockGenericSink) Publish(ctx context.Context, channel RuntimeChannel, message ChannelMessage) error {
	m.messages = append(m.messages, message)
	return nil
}

type mockTargetResolver struct {
	peers map[ipc.PeerKey]ipc.Peer
}

func (m *mockTargetResolver) ResolveConnection(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (ipc.Peer, bool, error) {
	peer, ok := m.peers[ipc.PeerKey{RuntimeID: runtimeID, ServiceID: serviceID}]
	return peer, ok, nil
}

func setupRouter() (*Router, *memoryRegistry, *mockEventPublisher, *mockStateStore) {
	return setupRouterWith(nil)
}

func setupRouterWith(generic *mockGenericSink) (*Router, *memoryRegistry, *mockEventPublisher, *mockStateStore) {
	reg := NewMemoryRegistry(Options{})
	eventPub := &mockEventPublisher{}
	stateStore := newMockStateStore()

	var sink GenericChannelSink
	if generic != nil {
		sink = generic
	}

	router := NewRouter(RouterConfig{
		Registry: reg,
		Events:   eventPub,
		States:   stateStore,
		Generic:  sink,
	})

	return router, reg, eventPub, stateStore
}

func registerSampleChannel(reg Registry) {
	ch := RuntimeChannel{
		ID:        NewRuntimeChannelID("runtime-1", "service-a", "events"),
		PluginID:  "plugin-x",
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
		ChannelID: "events",
		Kind:      domain.ChannelKindEvent,
		Direction: protocol.ChannelDirectionPluginToHost,
	}
	reg.Register(context.Background(), ch)
}

func TestRouter_Route_EventChannel(t *testing.T) {
	router, reg, events, _ := setupRouter()
	registerSampleChannel(reg)

	peer := ipc.Peer{PluginID: "plugin-x", RuntimeID: "runtime-1", ServiceID: "service-a"}
	msg := IncomingChannelMessage{
		Peer:      peer,
		ChannelID: "events",
		Payload:   json.RawMessage(`{"type":"test"}`),
	}

	if err := router.Route(context.Background(), msg); err != nil {
		t.Fatalf("route failed: %v", err)
	}

	if len(events.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events.events))
	}
	if events.events[0].PluginID != "plugin-x" {
		t.Fatalf("expected plugin-x, got %s", events.events[0].PluginID)
	}
}

func TestRouter_Route_StateChannel_Readable(t *testing.T) {
	router, reg, _, stateStore := setupRouter()
	ch := RuntimeChannel{
		ID:        NewRuntimeChannelID("runtime-1", "service-a", "world-state"),
		PluginID:  "plugin-x",
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
		ChannelID: "world-state",
		Kind:      domain.ChannelKindState,
		Direction: protocol.ChannelDirectionPluginToHost,
	}
	reg.Register(context.Background(), ch)

	peer := ipc.Peer{PluginID: "plugin-x", RuntimeID: "runtime-1", ServiceID: "service-a"}
	msg := IncomingChannelMessage{
		Peer:      peer,
		ChannelID: "world-state",
		Payload:   json.RawMessage(`{"state":"running"}`),
		Metadata:  map[string]json.RawMessage{"stateKey": []byte(`"server-status"`)},
	}

	if err := router.Route(context.Background(), msg); err != nil {
		t.Fatalf("route failed: %v", err)
	}

	key := state.NewStateKey("plugin-x", "runtime-1", "service-a", "world-state/server-status")
	snap, err := stateStore.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("state should be readable: %v", err)
	}
	if string(snap.Payload) != `{"state":"running"}` {
		t.Fatalf("payload mismatch: %s", snap.Payload)
	}
}

func TestRouter_Route_StateChannel_Isolation(t *testing.T) {
	router, reg, _, stateStore := setupRouter()

	chA := RuntimeChannel{
		ID:        NewRuntimeChannelID("runtime-1", "service-a", "status"),
		PluginID:  "plugin-x",
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
		ChannelID: "status",
		Kind:      domain.ChannelKindState,
		Direction: protocol.ChannelDirectionPluginToHost,
	}
	chB := chA
	chB.ChannelID = "health"
	chB.ID = NewRuntimeChannelID("runtime-1", "service-a", "health")

	reg.Register(context.Background(), chA)
	reg.Register(context.Background(), chB)

	peer := ipc.Peer{PluginID: "plugin-x", RuntimeID: "runtime-1", ServiceID: "service-a"}

	router.Route(context.Background(), IncomingChannelMessage{
		Peer:      peer,
		ChannelID: "status",
		Payload:   []byte(`"A"`),
	})
	router.Route(context.Background(), IncomingChannelMessage{
		Peer:      peer,
		ChannelID: "health",
		Payload:   []byte(`"B"`),
	})

	keyA := state.StateKey{PluginID: "plugin-x", RuntimeID: "runtime-1", ServiceID: "service-a", Key: "status"}
	if _, err := stateStore.Get(context.Background(), keyA); err != nil {
		t.Fatalf("status should exist: %v", err)
	}
	keyB := state.StateKey{PluginID: "plugin-x", RuntimeID: "runtime-1", ServiceID: "service-a", Key: "health"}
	if _, err := stateStore.Get(context.Background(), keyB); err != nil {
		t.Fatalf("health should exist: %v", err)
	}
}

func TestRouter_Route_LogChannel_CallsGenericOnce(t *testing.T) {
	generic := &mockGenericSink{}
	router, reg, _, _ := setupRouterWith(generic)

	ch := RuntimeChannel{
		ID:        NewRuntimeChannelID("runtime-1", "service-a", "logs"),
		PluginID:  "plugin-x",
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
		ChannelID: "logs",
		Kind:      domain.ChannelKindLog,
		Direction: protocol.ChannelDirectionPluginToHost,
	}
	reg.Register(context.Background(), ch)

	peer := ipc.Peer{PluginID: "plugin-x", RuntimeID: "runtime-1", ServiceID: "service-a"}
	if err := router.Route(context.Background(), IncomingChannelMessage{Peer: peer, ChannelID: "logs", Payload: []byte("hello")}); err != nil {
		t.Fatalf("route failed: %v", err)
	}

	if len(generic.messages) != 1 {
		t.Fatalf("expected 1 generic publish, got %d", len(generic.messages))
	}
}

func TestRouter_Route_MetricChannel_CallsGenericOnce(t *testing.T) {
	generic := &mockGenericSink{}
	router, reg, _, _ := setupRouterWith(generic)

	ch := RuntimeChannel{
		ID:        NewRuntimeChannelID("runtime-1", "service-a", "metrics"),
		PluginID:  "plugin-x",
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
		ChannelID: "metrics",
		Kind:      domain.ChannelKindMetric,
		Direction: protocol.ChannelDirectionPluginToHost,
	}
	reg.Register(context.Background(), ch)

	peer := ipc.Peer{PluginID: "plugin-x", RuntimeID: "runtime-1", ServiceID: "service-a"}
	if err := router.Route(context.Background(), IncomingChannelMessage{Peer: peer, ChannelID: "metrics", Payload: []byte("fps")}); err != nil {
		t.Fatalf("route failed: %v", err)
	}

	if len(generic.messages) != 1 {
		t.Fatalf("expected 1 generic publish, got %d", len(generic.messages))
	}
}

func TestRouter_Route_CustomRoute_PayloadOpaque(t *testing.T) {
	generic := &mockGenericSink{}
	router, reg, _, _ := setupRouterWith(generic)

	ch := RuntimeChannel{
		ID:        NewRuntimeChannelID("runtime-1", "service-a", "custom-stream"),
		PluginID:  "plugin-x",
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
		ChannelID: "custom-stream",
		Kind:      domain.ChannelKindCustom,
		Direction: protocol.ChannelDirectionPluginToHost,
	}
	reg.Register(context.Background(), ch)

	payload := json.RawMessage(`{"opaque":"data","value":42}`)
	peer := ipc.Peer{PluginID: "plugin-x", RuntimeID: "runtime-1", ServiceID: "service-a"}
	if err := router.Route(context.Background(), IncomingChannelMessage{Peer: peer, ChannelID: "custom-stream", Payload: payload}); err != nil {
		t.Fatalf("route failed: %v", err)
	}

	if len(generic.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(generic.messages))
	}
	if string(generic.messages[0].Payload) != string(payload) {
		t.Fatalf("payload should be opaque: %s", generic.messages[0].Payload)
	}
}

func TestRouter_Route_BinaryChannel_Unsupported(t *testing.T) {
	router, reg, _, _ := setupRouter()
	ch := RuntimeChannel{
		ID:        NewRuntimeChannelID("runtime-1", "service-a", "frames"),
		PluginID:  "plugin-x",
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
		ChannelID: "frames",
		Kind:      domain.ChannelKindBinary,
		Direction: protocol.ChannelDirectionPluginToHost,
	}
	reg.Register(context.Background(), ch)

	peer := ipc.Peer{PluginID: "plugin-x", RuntimeID: "runtime-1", ServiceID: "service-a"}
	err := router.Route(context.Background(), IncomingChannelMessage{Peer: peer, ChannelID: "frames", Payload: []byte("binary")})
	if err == nil {
		t.Fatal("expected error for binary kind, got nil")
	}
}

func TestRouter_Route_WrongDirection(t *testing.T) {
	router, reg, _, _ := setupRouter()
	ch := RuntimeChannel{
		ID:        NewRuntimeChannelID("runtime-1", "service-a", "events"),
		PluginID:  "plugin-x",
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
		ChannelID: "events",
		Kind:      domain.ChannelKindEvent,
		Direction: protocol.ChannelDirection("host_to_plugin"),
	}
	reg.Register(context.Background(), ch)

	peer := ipc.Peer{PluginID: "plugin-x", RuntimeID: "runtime-1", ServiceID: "service-a"}
	err := router.Route(context.Background(), IncomingChannelMessage{Peer: peer, ChannelID: "events", Payload: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected direction validation error")
	}
}

func TestRouter_Route_UnknownChannel(t *testing.T) {
	router, _, _, _ := setupRouter()

	peer := ipc.Peer{PluginID: "plugin-x", RuntimeID: "runtime-1", ServiceID: "service-a"}
	err := router.Route(context.Background(), IncomingChannelMessage{Peer: peer, ChannelID: "unknown", Payload: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestRouter_Route_PluginMismatch(t *testing.T) {
	router, reg, _, _ := setupRouter()
	registerSampleChannel(reg)

	peer := ipc.Peer{PluginID: "wrong-plugin", RuntimeID: "runtime-1", ServiceID: "service-a"}
	err := router.Route(context.Background(), IncomingChannelMessage{Peer: peer, ChannelID: "events", Payload: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected plugin mismatch error")
	}
}

func TestRouter_Route_PayloadNotCorrupted(t *testing.T) {
	router, reg, events, _ := setupRouter()
	registerSampleChannel(reg)

	original := json.RawMessage(`{"tick":9007199254740993,"foo":{"bar":[1,2,3]}}`)
	peer := ipc.Peer{PluginID: "plugin-x", RuntimeID: "runtime-1", ServiceID: "service-a"}
	if err := router.Route(context.Background(), IncomingChannelMessage{Peer: peer, ChannelID: "events", Payload: original}); err != nil {
		t.Fatalf("route failed: %v", err)
	}

	if string(events.events[0].Payload) != string(original) {
		t.Fatalf("payload corrupted: %s", events.events[0].Payload)
	}
}

func TestRouter_PeerValidation_Required(t *testing.T) {
	router, _, _, _ := setupRouter()

	peer := ipc.Peer{PluginID: "", RuntimeID: "", ServiceID: ""}
	err := router.Route(context.Background(), IncomingChannelMessage{Peer: peer, ChannelID: "events", Payload: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected peer validation error")
	}
}

func TestRouter_CancelledContext_Rejected(t *testing.T) {
	router, reg, _, _ := setupRouter()
	registerSampleChannel(reg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	peer := ipc.Peer{PluginID: "plugin-x", RuntimeID: "runtime-1", ServiceID: "service-a"}
	err := router.Route(ctx, IncomingChannelMessage{Peer: peer, ChannelID: "events", Payload: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestRouter_NowFunc_DefaultsToUTC(t *testing.T) {
	start := time.Now().UTC()
	r := NewRouter(RouterConfig{})
	captured := r.nowFunc()
	diff := captured.Sub(start)
	if diff < 0 || diff > time.Second {
		t.Fatalf("default nowFunc should return current UTC, got diff %v", diff)
	}
}
