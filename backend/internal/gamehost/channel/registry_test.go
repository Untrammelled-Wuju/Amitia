package channel

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func sampleChannel() RuntimeChannel {
	return RuntimeChannel{
		ID:        NewRuntimeChannelID("runtime-1", "service-a", "events"),
		PluginID:  "plugin-x",
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
		ChannelID: "events",
		Kind:      domain.ChannelKindEvent,
		Direction: protocol.ChannelDirectionPluginToHost,
	}
}

func TestRegistry_Register_SingleChannel(t *testing.T) {
	reg := NewRegistry(Options{})
	ch := sampleChannel()

	if err := reg.Register(context.Background(), ch); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	if reg.Count() != 1 {
		t.Fatalf("expected count 1, got %d", reg.Count())
	}
}

func TestRegistry_Register_DuplicateChannelRejected(t *testing.T) {
	reg := NewRegistry(Options{})
	ch := sampleChannel()

	if err := reg.Register(context.Background(), ch); err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	err := reg.Register(context.Background(), ch)
	if err == nil {
		t.Fatal("expected error for duplicate register, got nil")
	}
}

func TestRegistry_Register_EmptyChannels_Listed(t *testing.T) {
	reg := NewRegistry(Options{})
	channels, err := reg.List()
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(channels) != 0 {
		t.Fatalf("expected 0 channels, got %d", len(channels))
	}
}

func TestRegistry_Register_MultiChannel_AllResolve(t *testing.T) {
	reg := NewRegistry(Options{})
	ch1 := sampleChannel()
	ch2 := sampleChannel()
	ch2.ChannelID = "state"
	ch2.ID = NewRuntimeChannelID("runtime-1", "service-a", "state")
	ch2.Kind = domain.ChannelKindState

	if err := reg.Register(context.Background(), ch1); err != nil {
		t.Fatalf("register ch1 failed: %v", err)
	}
	if err := reg.Register(context.Background(), ch2); err != nil {
		t.Fatalf("register ch2 failed: %v", err)
	}

	if reg.Count() != 2 {
		t.Fatalf("expected count 2, got %d", reg.Count())
	}

	resolved, err := reg.Resolve(context.Background(), "runtime-1", "service-a", "events")
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if resolved.ChannelID != "events" {
		t.Fatalf("expected events channel, got %s", resolved.ChannelID)
	}
}

func TestRegistry_MultiService_SameChannelID(t *testing.T) {
	reg := NewRegistry(Options{})

	chA := sampleChannel()
	chB := sampleChannel()
	chB.ServiceID = "agent"
	chB.ID = NewRuntimeChannelID("runtime-1", "agent", "events")

	if err := reg.Register(context.Background(), chA); err != nil {
		t.Fatalf("register A failed: %v", err)
	}
	if err := reg.Register(context.Background(), chB); err != nil {
		t.Fatalf("register B failed: %v", err)
	}

	if reg.Count() != 2 {
		t.Fatalf("expected 2, got %d", reg.Count())
	}
}

func TestRegistry_RuntimeIsolation(t *testing.T) {
	reg := NewRegistry(Options{})

	chA := sampleChannel()
	chB := sampleChannel()
	chB.RuntimeID = "runtime-2"
	chB.ID = NewRuntimeChannelID("runtime-2", "service-a", "events")

	if err := reg.Register(context.Background(), chA); err != nil {
		t.Fatalf("register A failed: %v", err)
	}
	if err := reg.Register(context.Background(), chB); err != nil {
		t.Fatalf("register B failed: %v", err)
	}

	listA, _ := reg.ListByRuntime("runtime-1")
	if len(listA) != 1 {
		t.Fatalf("expected 1 for runtime-1, got %d", len(listA))
	}
	listB, _ := reg.ListByRuntime("runtime-2")
	if len(listB) != 1 {
		t.Fatalf("expected 1 for runtime-2, got %d", len(listB))
	}
}

func TestRegistry_PluginIsolation(t *testing.T) {
	reg := NewRegistry(Options{})

	chA := sampleChannel()
	chB := sampleChannel()
	chB.RuntimeID = "runtime-b"
	chB.PluginID = "plugin-y"
	chB.ID = NewRuntimeChannelID("runtime-b", "service-a", "events")

	if err := reg.Register(context.Background(), chA); err != nil {
		t.Fatalf("register A failed: %v", err)
	}
	if err := reg.Register(context.Background(), chB); err != nil {
		t.Fatalf("register B failed: %v", err)
	}

	_, err := reg.Resolve(context.Background(), "runtime-1", "service-a", "events")
	if err != nil {
		t.Fatalf("resolve should succeed: %v", err)
	}
}

func TestRegistry_UnknownOwnerService_Rejected(t *testing.T) {
	reg := NewRegistry(Options{
		ServiceResolver: &mockServiceResolver{services: map[domain.ServiceID]bool{
			"service-a": true,
		}},
	})

	ch := sampleChannel()
	ch.ServiceID = "unknown-service"
	ch.ID = NewRuntimeChannelID("runtime-1", "unknown-service", "events")

	err := reg.Register(context.Background(), ch)
	if err == nil {
		t.Fatal("expected error for unknown service, got nil")
	}
}

func TestRegistry_PluginIDMismatch_ResolveFails(t *testing.T) {
	reg := NewRegistry(Options{})
	ch := sampleChannel()
	if err := reg.Register(context.Background(), ch); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	_, err := reg.Get(context.Background(), ch.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
}

func TestRegistry_RuntimeIDMismatch_Fails(t *testing.T) {
	reg := NewRegistry(Options{})
	ch := sampleChannel()
	if err := reg.Register(context.Background(), ch); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	resolved, err := reg.Resolve(context.Background(), ch.RuntimeID, ch.ServiceID, ch.ChannelID)
	if err != nil || resolved.ID != ch.ID {
		t.Fatalf("resolve should succeed")
	}
}

func TestRegistry_DuplicateChannel_Rejected(t *testing.T) {
	reg := NewRegistry(Options{})
	ch := sampleChannel()

	if err := reg.Register(context.Background(), ch); err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	if err := reg.Register(context.Background(), ch); err == nil {
		t.Fatal("expected duplicate rejection")
	}
}

func TestRegistry_UnknownKind_Rejected(t *testing.T) {
	reg := NewRegistry(Options{})
	ch := sampleChannel()
	ch.Kind = domain.ChannelKind("video_stream")

	if err := reg.Register(context.Background(), ch); err == nil {
		t.Fatal("expected unknown kind rejection")
	}
}

func TestRegistry_UnknownDirection_Rejected(t *testing.T) {
	reg := NewRegistry(Options{})
	dir := protocol.ChannelDirection("input")
	ch := sampleChannel()
	ch.Direction = dir

	if err := reg.Register(context.Background(), ch); err == nil {
		t.Fatal("expected unknown direction rejection")
	}
}

func TestRegistry_UnknownFrequency_Rejected(t *testing.T) {
	reg := NewRegistry(Options{})
	freq := protocol.FrequencyHint("ultra_high")
	ch := sampleChannel()
	ch.Frequency = &freq

	if err := reg.Register(context.Background(), ch); err == nil {
		t.Fatal("expected unknown frequency rejection")
	}
}

func TestRegistry_pluginToHost_Direction_PublishAllowed(t *testing.T) {
	reg := NewRegistry(Options{})
	ch := sampleChannel()
	ch.Direction = protocol.ChannelDirectionPluginToHost

	if err := reg.Register(context.Background(), ch); err != nil {
		t.Fatalf("register failed: %v", err)
	}
}

func TestRegistry_hostToPlugin_Direction_PublishRejected(t *testing.T) {
	ch := sampleChannel()
	ch.Direction = protocol.ChannelDirectionHostToPlugin

	if err := ValidateDirection(ch, protocol.ChannelDirectionPluginToHost); err == nil {
		t.Fatal("expected direction rejection for host_to_plugin channel with plugin_to_host flow")
	}
}

func TestRegistry_bidirectional_Direction_BothDirectionsAllowed(t *testing.T) {
	ch := sampleChannel()
	ch.Direction = protocol.ChannelDirectionBidirectional

	if err := ValidateDirection(ch, protocol.ChannelDirectionPluginToHost); err != nil {
		t.Fatalf("bidirectional should allow plugin_to_host: %v", err)
	}
	if err := ValidateDirection(ch, protocol.ChannelDirectionHostToPlugin); err != nil {
		t.Fatalf("bidirectional should allow host_to_plugin: %v", err)
	}
}

func TestRegistry_WrongServicePublish_Rejected(t *testing.T) {
	reg := NewRegistry(Options{})
	ch := sampleChannel()
	if err := reg.Register(context.Background(), ch); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	_, err := reg.Resolve(context.Background(), ch.RuntimeID, "service-b", ch.ChannelID)
	if err == nil {
		t.Fatal("expected rejection for wrong service")
	}
}

func TestRegistry_WrongRuntimePublish_Rejected(t *testing.T) {
	reg := NewRegistry(Options{})
	ch := sampleChannel()
	if err := reg.Register(context.Background(), ch); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	_, err := reg.Resolve(context.Background(), "other-runtime", ch.ServiceID, ch.ChannelID)
	if err == nil {
		t.Fatal("expected rejection for wrong runtime")
	}
}

func TestRegistry_EventRoute_RegistryHasChannel(t *testing.T) {
	reg := NewRegistry(Options{})
	ch := sampleChannel()
	if err := reg.Register(context.Background(), ch); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	list, _ := reg.ListByRuntime(ch.RuntimeID)
	if len(list) != 1 || list[0].Kind != domain.ChannelKindEvent {
		t.Fatal("expected event channel registered")
	}
}

func TestRegistry_PayloadOpaque_Unmodified(t *testing.T) {
	reg := NewRegistry(Options{})
	ch := sampleChannel()
	payload := json.RawMessage(`{"tick":9007199254740993,"foo":{"bar":[1,2,3]}}`)
	ch.Metadata = map[string]json.RawMessage{"data": payload}

	if err := reg.Register(context.Background(), ch); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	stored, err := reg.Get(context.Background(), ch.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if string(stored.Metadata["data"]) != string(payload) {
		t.Fatalf("payload mismatch: got %s", stored.Metadata["data"])
	}
}

func TestRegistry_MetadataDeepCopy_RegisterImmutable(t *testing.T) {
	reg := NewRegistry(Options{})
	ch := sampleChannel()
	ch.Metadata = map[string]json.RawMessage{
		"key": json.RawMessage(`"value"`),
	}

	if err := reg.Register(context.Background(), ch); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	ch.Metadata["key"] = json.RawMessage(`"modified"`)

	stored, _ := reg.Get(context.Background(), ch.ID)
	if string(stored.Metadata["key"]) != `"value"` {
		t.Fatalf("stored should not be modified, got %s", stored.Metadata["key"])
	}
}

func TestRegistry_Get_DeepCopy(t *testing.T) {
	reg := NewRegistry(Options{})
	ch := sampleChannel()
	ch.Metadata = map[string]json.RawMessage{
		"key": json.RawMessage(`"value"`),
	}

	if err := reg.Register(context.Background(), ch); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	got, err := reg.Get(context.Background(), ch.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	got.Metadata["key"] = json.RawMessage(`"modified"`)

	stored, _ := reg.Get(context.Background(), ch.ID)
	if string(stored.Metadata["key"]) != `"value"` {
		t.Fatalf("stored should not be modified via get, got %s", stored.Metadata["key"])
	}
}

func TestRegistry_List_StableOrder(t *testing.T) {
	reg := NewRegistry(Options{})
	ids := []struct{ rt, svc, ch string }{
		{"r2", "s1", "c1"},
		{"r1", "s1", "c1"},
		{"r1", "s2", "c1"},
		{"r1", "s1", "c2"},
	}

	for _, id := range ids {
		ch := RuntimeChannel{
			ID:        NewRuntimeChannelID(domain.RuntimeInstanceID(id.rt), domain.ServiceID(id.svc), domain.ChannelID(id.ch)),
			PluginID:  "p",
			RuntimeID: domain.RuntimeInstanceID(id.rt),
			ServiceID: domain.ServiceID(id.svc),
			ChannelID: domain.ChannelID(id.ch),
			Kind:      domain.ChannelKindEvent,
		}
		if err := reg.Register(context.Background(), ch); err != nil {
			t.Fatalf("register failed: %v", err)
		}
	}

	list, _ := reg.List()
	for i := 1; i < len(list); i++ {
		if list[i-1].RuntimeID > list[i].RuntimeID ||
			(list[i-1].RuntimeID == list[i].RuntimeID && list[i-1].ServiceID > list[i].ServiceID) ||
			(list[i-1].RuntimeID == list[i].RuntimeID && list[i-1].ServiceID == list[i].ServiceID && list[i-1].ChannelID > list[i].ChannelID) {
			t.Fatalf("list not sorted: %v before %v", list[i-1], list[i])
		}
	}
}

func TestRegistry_RemoveByService(t *testing.T) {
	reg := NewRegistry(Options{})
	chA := sampleChannel()
	chB := sampleChannel()
	chB.ServiceID = "agent"
	chB.ID = NewRuntimeChannelID("runtime-1", "agent", "events")

	reg.Register(context.Background(), chA)
	reg.Register(context.Background(), chB)

	count, err := reg.RemoveByService(context.Background(), "runtime-1", "service-a")
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 removed, got %d", count)
	}

	if reg.Count() != 1 {
		t.Fatalf("expected 1 remaining, got %d", reg.Count())
	}
}

func TestRegistry_RemoveByRuntime(t *testing.T) {
	reg := NewRegistry(Options{})
	chA := sampleChannel()
	chB := sampleChannel()
	chB.RuntimeID = "runtime-2"
	chB.PluginID = "plugin-y"
	chB.ID = NewRuntimeChannelID("runtime-2", "service-a", "events")

	reg.Register(context.Background(), chA)
	reg.Register(context.Background(), chB)

	count, err := reg.RemoveByRuntime(context.Background(), "runtime-1")
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 removed, got %d", count)
	}
	if reg.Count() != 1 {
		t.Fatalf("expected 1 remaining, got %d", reg.Count())
	}
}

func TestRegistry_Cleanup_Idempotent(t *testing.T) {
	reg := NewRegistry(Options{})
	reg.RemoveByRuntime(context.Background(), "runtime-1")
	reg.RemoveByRuntime(context.Background(), "runtime-1")
}

func TestRegistry_Detach_NotAutoDelete(t *testing.T) {
	reg := NewRegistry(Options{})
	ch := sampleChannel()
	if err := reg.Register(context.Background(), ch); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	if reg.Count() != 1 {
		t.Fatalf("expected channel to still exist, got count %d", reg.Count())
	}
}

func TestRegistry_ConcurrentResolve_Reconcile(t *testing.T) {
	reg := NewRegistry(Options{})
	const n = 50

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ch := RuntimeChannel{
				ID:        NewRuntimeChannelID("rt", "svc", domain.ChannelID(rune('a'+i%26))),
				PluginID:  "p",
				RuntimeID: "rt",
				ServiceID: "svc",
				ChannelID: domain.ChannelID(rune('a' + i%26)),
				Kind:      domain.ChannelKindEvent,
			}
			reg.Register(context.Background(), ch)
		}(i)
	}

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reg.List()
			reg.Count()
		}()
	}

	wg.Wait()
}

func TestRegistry_LockScope_SinkNotBlocked(t *testing.T) {
	reg := NewRegistry(Options{})
	ch := sampleChannel()
	reg.Register(context.Background(), ch)

	_, _ = reg.Get(context.Background(), ch.ID)
	_, _ = reg.List()
	_ = reg.Count()
}

func TestRegistry_LimitChannelsPerRuntime(t *testing.T) {
	reg := NewRegistry(Options{MaxChannelsPerRuntime: 2})
	for i := 0; i < 3; i++ {
		ch := RuntimeChannel{
			ID:        NewRuntimeChannelID("rt", "svc", domain.ChannelID(rune('a'+i))),
			PluginID:  "p",
			RuntimeID: "rt",
			ServiceID: "svc",
			ChannelID: domain.ChannelID(rune('a' + i)),
			Kind:      domain.ChannelKindEvent,
		}
		err := reg.Register(context.Background(), ch)
		if i < 2 && err != nil {
			t.Fatalf("register %d should succeed: %v", i, err)
		}
		if i == 2 && err == nil {
			t.Fatal("third register should fail due to limit")
		}
	}
}

func TestRegistry_ResolverEmpty_AllowsAny(t *testing.T) {
	reg := NewRegistry(Options{})
	ch := sampleChannel()
	ch.ServiceID = "any-service"
	ch.ID = NewRuntimeChannelID("runtime-1", "any-service", "events")
	if err := reg.Register(context.Background(), ch); err != nil {
		t.Fatalf("should allow registration without resolver: %v", err)
	}
}

func TestRegistryUnregister_Idempotent(t *testing.T) {
	reg := NewMemoryRegistry(Options{})
	id := NewRuntimeChannelID("rt", "svc", "ch")
	reg.Unregister(context.Background(), id)
	reg.Unregister(context.Background(), id)
}

type mockServiceResolver struct {
	services map[domain.ServiceID]bool
}

func (m *mockServiceResolver) ServiceExists(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (bool, error) {
	return m.services[serviceID], nil
}
