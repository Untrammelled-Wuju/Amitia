package state

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func kh(plugin, runtime, service, k string) StateKey {
	return StateKey{
		PluginID:  domain.PluginID(plugin),
		RuntimeID: domain.RuntimeInstanceID(runtime),
		ServiceID: domain.ServiceID(service),
		Key:       k,
	}
}

func TestLatestStateStore_Put_FirstPut(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	update := StateUpdate{
		ID:        "msg-1",
		PluginID:  "plugin-a",
		RuntimeID: "runtime-1",
		ServiceID: "svc-main",
		Key:       "player.position",
		Payload:   json.RawMessage(`{"x":10,"y":20}`),
	}

	snap, err := store.Put(context.Background(), update)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.PluginID != "plugin-a" {
		t.Errorf("PluginID: got %q, want plugin-a", snap.PluginID)
	}
	if snap.RuntimeID != "runtime-1" {
		t.Errorf("RuntimeID: got %q, want runtime-1", snap.RuntimeID)
	}
	if snap.ServiceID != "svc-main" {
		t.Errorf("ServiceID: got %q, want svc-main", snap.ServiceID)
	}
	if snap.Key != "player.position" {
		t.Errorf("Key: got %q, want player.position", snap.Key)
	}
	if string(snap.Payload) != `{"x":10,"y":20}` {
		t.Errorf("Payload: got %q", snap.Payload)
	}
	if snap.SourceMessageID != "msg-1" {
		t.Errorf("SourceMessageID: got %q", snap.SourceMessageID)
	}
	if snap.Version != 1 {
		t.Errorf("Version: got %d, want 1", snap.Version)
	}
	if snap.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestLatestStateStore_Put_Replace(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	a := StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "pos", Payload: json.RawMessage(`{"x":1}`)}
	b := StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "pos", Payload: json.RawMessage(`{"x":2}`)}

	store.Put(context.Background(), a)
	snap2, err := store.Put(context.Background(), b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(snap2.Payload) != `{"x":2}` {
		t.Errorf("expected replaced payload, got %q", snap2.Payload)
	}
}

func TestLatestStateStore_Put_DifferentKeys(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "pos", Payload: json.RawMessage(`{}`)})
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "health", Payload: json.RawMessage(`{}`)})

	if store.Count(context.Background()) != 2 {
		t.Errorf("expected 2 states, got %d", store.Count(context.Background()))
	}
}

func TestLatestStateStore_Put_DifferentServices(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "svc-a", Key: "status", Payload: json.RawMessage(`{"v":"a"}`)})
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "svc-b", Key: "status", Payload: json.RawMessage(`{"v":"b"}`)})

	if store.Count(context.Background()) != 2 {
		t.Errorf("expected 2 states, got %d", store.Count(context.Background()))
	}

	got, err := store.Get(context.Background(), kh("p", "r", "svc-a", "status"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got.Payload) != `{"v":"a"}` {
		t.Errorf("svc-a status wrong: %q", got.Payload)
	}

	got2, err := store.Get(context.Background(), kh("p", "r", "svc-b", "status"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got2.Payload) != `{"v":"b"}` {
		t.Errorf("svc-b status wrong: %q", got2.Payload)
	}
}

func TestLatestStateStore_Put_DifferentRuntimes(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	store.Put(context.Background(), StateUpdate{PluginID: "plugin-beta", RuntimeID: "runtime-a", ServiceID: "svc", Key: "status", Payload: json.RawMessage(`{"r":"a"}`)})
	store.Put(context.Background(), StateUpdate{PluginID: "plugin-beta", RuntimeID: "runtime-b", ServiceID: "svc", Key: "status", Payload: json.RawMessage(`{"r":"b"}`)})

	gotA, _ := store.Get(context.Background(), kh("plugin-beta", "runtime-a", "svc", "status"))
	gotB, _ := store.Get(context.Background(), kh("plugin-beta", "runtime-b", "svc", "status"))

	if string(gotA.Payload) != `{"r":"a"}` {
		t.Errorf("runtime-a status wrong: %q", gotA.Payload)
	}
	if string(gotB.Payload) != `{"r":"b"}` {
		t.Errorf("runtime-b status wrong: %q", gotB.Payload)
	}
}

func TestLatestStateStore_Get_NotFound(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	_, err := store.Get(context.Background(), kh("unknown", "r", "s", "k"))
	if err == nil {
		t.Fatal("expected not_found error")
	}
	if !domain.IsHostError(err, domain.ErrNotFound) {
		t.Errorf("expected not_found, got %v", err)
	}
}

func TestLatestStateStore_Get_DeepCopy(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "data", Payload: json.RawMessage(`{"original":true}`)})

	snap1, _ := store.Get(context.Background(), kh("p", "r", "s", "data"))
	snap1.Payload[0] = 'X'

	snap2, _ := store.Get(context.Background(), kh("p", "r", "s", "data"))
	if string(snap2.Payload) != `{"original":true}` {
		t.Errorf("store state was mutated: %q", snap2.Payload)
	}
}

func TestLatestStateStore_Put_DeepCopy(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	payload := json.RawMessage(`{"mutable":true}`)
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "data", Payload: payload})

	payload[0] = 'X'

	snap, _ := store.Get(context.Background(), kh("p", "r", "s", "data"))
	if string(snap.Payload) != `{"mutable":true}` {
		t.Errorf("store state was mutated via Put input: %q", snap.Payload)
	}
}

func TestLatestStateStore_BigIntPreserved(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "tick", Payload: json.RawMessage(`{"tick":9007199254740993}`)})

	snap, _ := store.Get(context.Background(), kh("p", "r", "s", "tick"))
	if string(snap.Payload) != `{"tick":9007199254740993}` {
		t.Errorf("big int precision lost: %q", snap.Payload)
	}
}

func TestLatestStateStore_MetadataPreserved(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	md := map[string]json.RawMessage{
		"trace": json.RawMessage(`"abc"`),
	}
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "data", Payload: nil, Metadata: md})

	snap, _ := store.Get(context.Background(), kh("p", "r", "s", "data"))
	if string(snap.Metadata["trace"]) != `"abc"` {
		t.Errorf("trace metadata missing: %v", snap.Metadata)
	}

	md["trace"][0] = 'X'
	snap2, _ := store.Get(context.Background(), kh("p", "r", "s", "data"))
	if string(snap2.Metadata["trace"]) != `"abc"` {
		t.Errorf("metadata was mutated: %q", snap2.Metadata["trace"])
	}
}

func TestLatestStateStore_List_FilterRuntime(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r1", ServiceID: "sa", Key: "position", Payload: json.RawMessage(`{}`)})
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r1", ServiceID: "sa", Key: "health", Payload: json.RawMessage(`{}`)})
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r2", ServiceID: "sb", Key: "position", Payload: json.RawMessage(`{}`)})

	rt1 := domain.RuntimeInstanceID("r1")
	list, err := store.List(context.Background(), StateFilter{RuntimeID: &rt1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 for r1, got %d", len(list))
	}
}

func TestLatestStateStore_List_KeyPrefix(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "player.pos", Payload: json.RawMessage(`{}`)})
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "player.health", Payload: json.RawMessage(`{}`)})
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "world.sum", Payload: json.RawMessage(`{}`)})

	list, err := store.List(context.Background(), StateFilter{KeyPrefix: "player."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 player. keys, got %d", len(list))
	}
	for _, snap := range list {
		if !strings.HasPrefix(snap.Key, "player.") {
			t.Errorf("key %q doesn't have prefix player.", snap.Key)
		}
	}
}

func TestLatestStateStore_List_Empty(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	rtUnknown := domain.RuntimeInstanceID("unknown")
	list, err := store.List(context.Background(), StateFilter{RuntimeID: &rtUnknown})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

func TestLatestStateStore_List_StableOrder(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r2", ServiceID: "s", Key: "b", Payload: json.RawMessage(`{}`)})
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r1", ServiceID: "s", Key: "c", Payload: json.RawMessage(`{}`)})
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r1", ServiceID: "s", Key: "a", Payload: json.RawMessage(`{}`)})

	list, _ := store.List(context.Background(), StateFilter{})
	if len(list) != 3 {
		t.Fatalf("expected 3 results, got %d", len(list))
	}
	if list[0].RuntimeID != "r1" || list[0].Key != "a" {
		t.Errorf("first result wrong: %+v", list[0])
	}
	if list[1].RuntimeID != "r1" || list[1].Key != "c" {
		t.Errorf("second result wrong: %+v", list[1])
	}
	if list[2].RuntimeID != "r2" || list[2].Key != "b" {
		t.Errorf("third result wrong: %+v", list[2])
	}
}

func TestLatestStateStore_Remove(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k", Payload: json.RawMessage(`{}`)})

	store.Remove(context.Background(), kh("p", "r", "s", "k"))
	if store.Count(context.Background()) != 0 {
		t.Errorf("expected 0, got %d", store.Count(context.Background()))
	}
	store.Remove(context.Background(), kh("p", "r", "s", "k"))
}

func TestLatestStateStore_RemoveByService(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "svc-a", Key: "k1", Payload: json.RawMessage(`{}`)})
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "svc-a", Key: "k2", Payload: json.RawMessage(`{}`)})
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "svc-b", Key: "k3", Payload: json.RawMessage(`{}`)})

	store.RemoveByService(context.Background(), "r", "svc-a")
	if store.Count(context.Background()) != 1 {
		t.Errorf("expected 1, got %d", store.Count(context.Background()))
	}
}

func TestLatestStateStore_RemoveByRuntime(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "ra", ServiceID: "s", Key: "k1", Payload: json.RawMessage(`{}`)})
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "rb", ServiceID: "s", Key: "k2", Payload: json.RawMessage(`{}`)})

	store.RemoveByRuntime(context.Background(), "ra")
	if store.Count(context.Background()) != 1 {
		t.Errorf("expected 1, got %d", store.Count(context.Background()))
	}
	if store.CountByRuntime(context.Background(), "ra") != 0 {
		t.Error("ra should be 0")
	}
}

func TestLatestStateStore_RemoveByRuntime_Idempotent(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k", Payload: json.RawMessage(`{}`)})

	store.RemoveByRuntime(context.Background(), "r")
	store.RemoveByRuntime(context.Background(), "r")
}

func TestLatestStateStore_RemoveByPlugin(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	store.Put(context.Background(), StateUpdate{PluginID: "p1", RuntimeID: "ra", ServiceID: "s", Key: "k1", Payload: json.RawMessage(`{}`)})
	store.Put(context.Background(), StateUpdate{PluginID: "p2", RuntimeID: "rb", ServiceID: "s", Key: "k2", Payload: json.RawMessage(`{}`)})

	store.RemoveByPlugin(context.Background(), "p1")
	if store.Count(context.Background()) != 1 {
		t.Errorf("expected 1 (p2), got %d", store.Count(context.Background()))
	}
}

func TestLatestStateStore_KeyLimit(t *testing.T) {
	opts := NewOptions()
	opts.MaxStateKeysPerRuntime = 2
	store := NewLatestStateStore(opts)

	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k1", Payload: nil})
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k2", Payload: nil})

	_, err := store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k3", Payload: nil})
	if err == nil {
		t.Fatal("expected resource_exhausted")
	}
	if !domain.IsHostError(err, domain.ErrResourceExhausted) {
		t.Errorf("expected resource_exhausted, got %v", err)
	}
}

func TestLatestStateStore_KeyLimit_UpdateExistingOK(t *testing.T) {
	opts := NewOptions()
	opts.MaxStateKeysPerRuntime = 2
	store := NewLatestStateStore(opts)

	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k1", Payload: nil})
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k2", Payload: nil})

	_, err := store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k1", Payload: json.RawMessage(`{"updated":true}`)})
	if err != nil {
		t.Errorf("updating existing key should succeed, got %v", err)
	}
}

func TestLatestStateStore_PayloadSizeLimit(t *testing.T) {
	opts := NewOptions()
	opts.MaxStatePayloadBytes = 10
	store := NewLatestStateStore(opts)

	_, err := store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "big", Payload: json.RawMessage(`{"toolargepayload":true}`)})
	if err == nil {
		t.Fatal("expected size limit error")
	}
}

func TestLatestStateStore_ContextCancel(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Put(ctx, StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k", Payload: nil})
	if err == nil {
		t.Fatal("expected cancelled context error")
	}
}

func TestLatestStateStore_ConcurrentPuts(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			payload := json.RawMessage(`{"n":` + strings.Repeat("0", i%3) + `}`)
			store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "counter", Payload: payload})
		}(i)
	}
	wg.Wait()

	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "final", Payload: json.RawMessage(`{}`)})

	if store.Count(context.Background()) != 2 {
		t.Errorf("expected 2 keys, got %d", store.Count(context.Background()))
	}
}

func TestLatestStateStore_ConcurrentMultiRuntime(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	const NumRuntimes = 10
	const KeysPerRuntime = 3

	var wg sync.WaitGroup
	for r := 0; r < NumRuntimes; r++ {
		rtID := domain.RuntimeInstanceID("rt-" + strings.Repeat("0", r%2))
		for k := 0; k < KeysPerRuntime; k++ {
			wg.Add(1)
			go func(rt domain.RuntimeInstanceID, ki int) {
				defer wg.Done()
				store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: rt, ServiceID: "s", Key: "k" + strings.Repeat("x", ki%2), Payload: json.RawMessage(`{}`)})
				store.Get(context.Background(), StateKey{PluginID: "p", RuntimeID: rt, ServiceID: "s", Key: "k" + strings.Repeat("x", ki%2)})
				store.List(context.Background(), StateFilter{RuntimeID: &rt})
			}(rtID, k)
		}
	}
	wg.Wait()
}

func TestLatestStateStore_Clock(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	opts := NewOptions()
	opts.Clock = func() time.Time { return fixed }
	store := NewLatestStateStore(opts)

	snap, _ := store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k", Payload: nil})
	if !snap.UpdatedAt.Equal(fixed) {
		t.Errorf("expected UpdatedAt to be fixed clock, got %v", snap.UpdatedAt)
	}
}

func TestLatestStateStore_Get_Validation(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	_, err := store.Get(context.Background(), StateKey{PluginID: "", RuntimeID: "r", ServiceID: "s", Key: "k"})
	if err == nil {
		t.Fatal("expected validation error for empty plugin id")
	}
}

func TestLatestStateStore_Put_InvalidKey(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	_, err := store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "", Payload: nil})
	if err == nil {
		t.Fatal("expected validation error for empty key")
	}
}

func TestLatestStateStore_CountByRuntime(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "ra", ServiceID: "s", Key: "k1", Payload: nil})
	store.Put(context.Background(), StateUpdate{PluginID: "p", RuntimeID: "rb", ServiceID: "s", Key: "k2", Payload: nil})

	if store.CountByRuntime(context.Background(), "ra") != 1 {
		t.Errorf("expected 1 for ra, got %d", store.CountByRuntime(context.Background(), "ra"))
	}
	if store.CountByRuntime(context.Background(), "unknown") != 0 {
		t.Errorf("expected 0 for unknown, got %d", store.CountByRuntime(context.Background(), "unknown"))
	}
}

func TestLatestStateStore_TwoPluginsSameRuntime(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	store.Put(context.Background(), StateUpdate{PluginID: "p1", RuntimeID: "r", ServiceID: "s", Key: "k", Payload: json.RawMessage(`{"p":"1"}`)})
	store.Put(context.Background(), StateUpdate{PluginID: "p2", RuntimeID: "r", ServiceID: "s", Key: "k", Payload: json.RawMessage(`{"p":"2"}`)})

	got1, _ := store.Get(context.Background(), kh("p1", "r", "s", "k"))
	got2, _ := store.Get(context.Background(), kh("p2", "r", "s", "k"))

	if string(got1.Payload) != `{"p":"1"}` {
		t.Errorf("p1 state corrupted: %q", got1.Payload)
	}
	if string(got2.Payload) != `{"p":"2"}` {
		t.Errorf("p2 state corrupted: %q", got2.Payload)
	}
}

func TestLatestStateStore_RemoveByRuntime_MultiPlugin(t *testing.T) {
	store := NewLatestStateStore(NewOptions())
	store.Put(context.Background(), StateUpdate{PluginID: "p1", RuntimeID: "r", ServiceID: "s", Key: "k1", Payload: nil})
	store.Put(context.Background(), StateUpdate{PluginID: "p2", RuntimeID: "r", ServiceID: "s", Key: "k2", Payload: nil})
	store.Put(context.Background(), StateUpdate{PluginID: "p3", RuntimeID: "other", ServiceID: "s", Key: "k3", Payload: nil})

	store.RemoveByRuntime(context.Background(), "r")
	if store.Count(context.Background()) != 1 {
		t.Errorf("expected 1 (p3 at other), got %d", store.Count(context.Background()))
	}
}

func TestStateKey_Validate(t *testing.T) {
	if err := (StateKey{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "k"}).Validate(); err != nil {
		t.Errorf("valid key rejected: %v", err)
	}
	if err := (StateKey{}).Validate(); err == nil {
		t.Error("empty key should be rejected")
	}
	if err := (StateKey{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "bad\x01key"}).Validate(); err == nil {
		t.Error("control chars should be rejected")
	}
	key := StateKey{PluginID: "p", RuntimeID: "r", ServiceID: "s", Key: "valid-key_123"}
	if err := key.Validate(); err != nil {
		t.Errorf("valid key rejected: %v", err)
	}
}

func TestStateKey_Compare(t *testing.T) {
	a := StateKey{PluginID: "p", RuntimeID: "r1", ServiceID: "s", Key: "k"}
	b := StateKey{PluginID: "p", RuntimeID: "r2", ServiceID: "s", Key: "k"}
	if a.Compare(b) >= 0 {
		t.Errorf("expected a < b")
	}
	if b.Compare(a) <= 0 {
		t.Errorf("expected b > a")
	}
}
