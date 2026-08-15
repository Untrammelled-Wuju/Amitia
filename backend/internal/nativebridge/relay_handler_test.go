package nativebridge

import (
	"context"
	"testing"
)

type fakeRelayBridge struct {
	attached    bool
	transport   RelayTransport
	generation  uint64
	health      Health
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

func (f *fakeRelayBridge) AttachRelaySession(transport RelayTransport) {
	f.attached = true
	f.transport = transport
	f.generation++
}

func (f *fakeRelayBridge) DetachSession() {
	f.attached = false
	f.transport = nil
	f.generation++
}

func (f *fakeRelayBridge) Generation() uint64 {
	return f.generation
}

func (f *fakeRelayBridge) SessionAttached() bool {
	return f.attached
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

	bridge.AttachRelaySession(nil)
	if !bridge.SessionAttached() {
		t.Fatal("expected attached after AttachRelaySession")
	}

	initialGen := bridge.Generation()
	bridge.DetachSession()
	if bridge.SessionAttached() {
		t.Fatal("expected not attached after DetachSession")
	}
	if bridge.Generation() <= initialGen {
		t.Fatal("expected generation to increment after detach")
	}
}

func TestRelayHandlerSessionTracking(t *testing.T) {
	handler := NewRelayHandler()
	_, ok := handler.GetSession("android")
	if ok {
		t.Fatal("expected no session initially")
	}
}
