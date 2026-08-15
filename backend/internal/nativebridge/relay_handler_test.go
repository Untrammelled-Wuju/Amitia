package nativebridge

import (
	"context"
	"encoding/json"
	"testing"
)

type fakeRelayBridge struct {
	attached    bool
	transport   RelayTransport
	generation  uint64
	health      Health
	envelopes   [][]byte
	executeFunc func(ctx context.Context, req Request) (Response, error)
}

func (f *fakeRelayBridge) Execute(ctx context.Context, req Request) (Response, error) {
	if f.executeFunc != nil {
		return f.executeFunc(ctx, req)
	}
	return Response{
		ProtocolVersion: req.ProtocolVersion,
		RequestID:       req.RequestID,
		Status:          "success",
		Result:          map[string]any{"ok": true},
	}, nil
}

func (f *fakeRelayBridge) Health(_ context.Context) Health {
	return f.health
}

func (f *fakeRelayBridge) AttachRelaySession(transport RelayTransport) uint64 {
	f.attached = true
	f.transport = transport
	f.generation++
	return f.generation
}

func (f *fakeRelayBridge) DetachRelaySession(expectedGeneration uint64) {
	if f.generation != expectedGeneration {
		return
	}
	f.attached = false
	f.transport = nil
}

func (f *fakeRelayBridge) Generation() uint64 {
	return f.generation
}

func (f *fakeRelayBridge) SessionAttached() bool {
	return f.attached
}

func (f *fakeRelayBridge) HandleRelayEnvelope(payload []byte) error {
	f.envelopes = append(f.envelopes, payload)
	var env RelayEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return err
	}
	if env.Type == "native_bridge.response" && env.RequestID != "" {
		resp := Response{
			ProtocolVersion: 1,
			RequestID:       env.RequestID,
			Status:          "success",
			Result:          map[string]any{"echo": true},
		}
		respData, _ := json.Marshal(resp)
		f.envelopes = append(f.envelopes, respData)
	}
	return nil
}

func TestRelayHandlerRegisterBridge(t *testing.T) {
	handler := NewRelayHandler()
	bridge := &fakeRelayBridge{health: HealthReady}
	handler.RegisterBridge("android", bridge)

	got, ok := handler.GetBridge("android")
	if !ok {
		t.Fatal("expected bridge to be registered")
	}
	if got != bridge {
		t.Fatal("expected same bridge instance")
	}
}

func TestRelayHandlerGetBridgeMissing(t *testing.T) {
	handler := NewRelayHandler()
	_, ok := handler.GetBridge("ios")
	if ok {
		t.Fatal("expected bridge not found")
	}
}

func TestRelayHandlerSingleSessionPrinciple(t *testing.T) {
	handler := NewRelayHandler()
	bridge1 := &fakeRelayBridge{health: HealthReady}
	bridge2 := &fakeRelayBridge{health: HealthReady}

	handler.RegisterBridge("test1", bridge1)
	handler.RegisterBridge("test2", bridge2)

	got1, _ := handler.GetBridge("test1")
	got2, _ := handler.GetBridge("test2")

	if got1 != bridge1 {
		t.Fatal("expected bridge1")
	}
	if got2 != bridge2 {
		t.Fatal("expected bridge2")
	}
}

func TestRelayBridgeLifecycle(t *testing.T) {
	bridge := &fakeRelayBridge{health: HealthUnknown}

	if bridge.SessionAttached() {
		t.Fatal("expected not attached initially")
	}

	gen := bridge.AttachRelaySession(nil)
	if !bridge.SessionAttached() {
		t.Fatal("expected attached after AttachRelaySession")
	}

	bridge.DetachRelaySession(gen)
	if bridge.SessionAttached() {
		t.Fatal("expected not attached after DetachRelaySession")
	}
}

func TestRelayHandlerSessionTracking(t *testing.T) {
	handler := NewRelayHandler()
	_, ok := handler.GetSession("android")
	if ok {
		t.Fatal("expected no session initially")
	}
}

func TestRelayBridgeDetachOnlyMatchingGeneration(t *testing.T) {
	bridge := &fakeRelayBridge{health: HealthUnknown}

	gen1 := bridge.AttachRelaySession(nil)
	gen2 := bridge.AttachRelaySession(nil)

	bridge.DetachRelaySession(gen1)
	if !bridge.SessionAttached() {
		t.Fatal("expected still attached after stale generation detach")
	}

	bridge.DetachRelaySession(gen2)
	if bridge.SessionAttached() {
		t.Fatal("expected not attached after current generation detach")
	}
}

func TestRelayBridgeHandleEnvelopeRoutesToBridge(t *testing.T) {
	bridge := &fakeRelayBridge{health: HealthReady}
	bridge.AttachRelaySession(nil)

	env := RelayEnvelope{
		Type:       "native_bridge.event",
		Generation: bridge.Generation(),
		Payload:    json.RawMessage(`{"domain":"test"}`),
	}
	data, _ := json.Marshal(env)
	if err := bridge.HandleRelayEnvelope(data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(bridge.envelopes) == 0 {
		t.Fatal("expected bridge to receive envelope")
	}
}

func TestRelayBridgeRejectsOldGenerationEnvelope(t *testing.T) {
	bridge := &fakeRelayBridge{health: HealthReady}
	bridge.AttachRelaySession(nil)
	oldGen := bridge.Generation()

	bridge.AttachRelaySession(nil)

	env := RelayEnvelope{
		Type:       "native_bridge.event",
		Generation: oldGen,
		Payload:    json.RawMessage(`{"domain":"test"}`),
	}
	data, _ := json.Marshal(env)
	if err := bridge.HandleRelayEnvelope(data); err != nil {
		t.Fatalf("unexpected error for old generation: %v", err)
	}
}
